package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// livingServer: мир с живыми локациями и контрактами; игрок заранее создан в хранилище в заданной точке.
func livingServer(t *testing.T, token, ship string, at world.Vec, hold map[string]float64) (*quest.MemStore, worldgen.Layout, string) {
	t.Helper()
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	w := world.New()
	if err := w.ApplyLayout(lay, cfg); err != nil {
		t.Fatal(err)
	}
	qs := quest.NewMemStore()
	svc := quest.NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), qs, w.ObjectPos)
	store := world.NewMemStore()
	id := world.PublicID(world.TokenHash(token))
	_ = store.Save(context.Background(), world.Record{ID: id, Name: "Шахтёр", Pos: at, Dir: world.Vec{X: 1}, Created: true, ShipType: ship, PilotSpec: "miner", Hold: hold})
	opts := Options{MaxMessage: 8192, RatePerSec: 200, Version: "t", World: w, Store: store, Quests: svc}
	srv, h := newServer(t, opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return qs, lay, srv.URL
}

type holdState struct {
	Credits *int64 `json:"credits"`
	Ship    struct {
		Hold      map[string]float64 `json:"hold"`
		CargoUsed float64            `json:"cargo_used"`
		Mining    string             `json:"mining"`
	} `json:"ship"`
	Objects []struct {
		ID, Kind string
		Amount   *float64
	} `json:"objects"`
}

func (cl *client) nextHold(cond func(holdState) bool) holdState {
	cl.t.Helper()
	for i := 0; i < 300; i++ {
		m := cl.next("state")
		var s holdState
		_ = json.Unmarshal(m.Payload, &s)
		if cond(s) {
			return s
		}
	}
	cl.t.Fatal("состояние не достигло ожидаемого")
	return holdState{}
}

func TestWelcomeCarriesLayoutAndDeposits(t *testing.T) {
	_, lay, url := livingServer(t, "livingtokentoken01", "TMP-MINE", world.Vec{X: 0, Y: 0}, nil)
	cl := connect(t, url)
	w := cl.joinRaw("livingtokentoken01")
	if w["layout"] == nil || w["visual"] == nil {
		t.Fatalf("в welcome нет раскладки или визуальных параметров: %v", w)
	}
	s := cl.nextHold(func(holdState) bool { return true })
	deps := 0
	for _, o := range s.Objects {
		if o.Kind == "deposit" && o.Amount != nil {
			deps++
		}
	}
	if deps != len(lay.Deposits) {
		t.Fatalf("месторождений в состоянии %d, в раскладке %d", deps, len(lay.Deposits))
	}
}

func TestMiningThroughProtocol(t *testing.T) {
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	d := lay.Deposits[0]
	_, _, url := livingServer(t, "miningtokentoken01", "TMP-MINE", world.Vec{X: d.Pos[0] + 4, Y: d.Pos[1]}, nil)
	cl := connect(t, url)
	cl.joinRaw("miningtokentoken01")
	cl.send("mine", map[string]any{"id": d.ID})
	s := cl.nextHold(func(s holdState) bool { return s.Ship.CargoUsed >= 3 })
	if s.Ship.Hold[d.Resource] < 3 {
		t.Fatalf("груз: %v", s.Ship.Hold)
	}
	cl.send("mine_stop", nil)
	cl.nextHold(func(s holdState) bool { return s.Ship.Mining == "" })
}

func TestMineRejectedWithoutModule(t *testing.T) {
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	d := lay.Deposits[0]
	_, _, url := livingServer(t, "nomoduletoken0001", "FTR-001", world.Vec{X: d.Pos[0], Y: d.Pos[1]}, nil)
	cl := connect(t, url)
	cl.joinRaw("nomoduletoken0001")
	cl.send("mine", map[string]any{"id": d.ID})
	m := cl.next("error")
	var e struct{ Code string }
	_ = json.Unmarshal(m.Payload, &e)
	if e.Code != "no_module" {
		t.Fatalf("код ошибки %q", e.Code)
	}
}

func TestSellGrantsCreditsOnceAndNotifies(t *testing.T) {
	qs, _, url := livingServer(t, "sellertokentoken01", "TMP-MINE", world.Vec{X: -12 + 8, Y: 14}, map[string]float64{"iron_ore": 10})
	cl := connect(t, url)
	cl.joinRaw("sellertokentoken01")
	cl.send("sell", nil)
	m := cl.next("reward")
	var r struct {
		Kind    string
		Credits int64
		Total   int64
	}
	_ = json.Unmarshal(m.Payload, &r)
	want := int64(10 * worldgen.DefaultConfig().Resources["iron_ore"].Price)
	if r.Kind != "sell" || r.Credits != want || r.Total < want {
		t.Fatalf("награда за продажу: %+v (ожидалось %d)", r, want)
	}
	s := cl.nextHold(func(s holdState) bool { return s.Credits != nil && *s.Credits >= want && s.Ship.CargoUsed == 0 })
	if s.Ship.CargoUsed != 0 {
		t.Fatalf("груз после продажи %v", s.Ship.Hold)
	}
	cl.send("sell", nil)
	e := cl.next("error")
	var ec struct{ Code string }
	_ = json.Unmarshal(e.Payload, &ec)
	if ec.Code != "nothing_to_sell" {
		t.Fatalf("повторная продажа: %q", ec.Code)
	}
	_ = qs
}

func TestSellRejectedAwayFromStation(t *testing.T) {
	_, _, url := livingServer(t, "farsellertoken0001", "TMP-MINE", world.Vec{X: 200, Y: 200}, map[string]float64{"iron_ore": 5})
	cl := connect(t, url)
	cl.joinRaw("farsellertoken0001")
	cl.send("sell", nil)
	e := cl.next("error")
	var ec struct{ Code string }
	_ = json.Unmarshal(e.Payload, &ec)
	if ec.Code != "not_at_station" {
		t.Fatalf("код %q", ec.Code)
	}
}

func TestSetOptionThroughProtocol(t *testing.T) {
	_, _, url := livingServer(t, "optiontokentoken01", "FTR-001", world.Vec{X: 0, Y: 0}, nil)
	cl := connect(t, url)
	cl.joinRaw("optiontokentoken01")
	cl.send("set_option", map[string]any{"name": "auto_retaliate", "value": false})
	cl.send("set_option", map[string]any{"name": "wings", "value": true})
	e := cl.next("error")
	var ec struct{ Code string }
	_ = json.Unmarshal(e.Payload, &ec)
	if ec.Code != "bad_option" {
		t.Fatalf("код %q", ec.Code)
	}
	cl.send("set_option", map[string]any{"name": "", "value": true})
	e = cl.next("error")
	_ = json.Unmarshal(e.Payload, &ec)
	if ec.Code != "bad_message" {
		t.Fatalf("код %q", ec.Code)
	}
}
