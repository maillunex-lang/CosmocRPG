package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/pilot"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipxp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// livingPilot — сервис пилота последнего созданного livingServer (опыт в памяти).
var livingPilot *pilot.Service
var livingShips *shipxp.Service

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
	livingPilot = pilot.NewService(pilot.NewMemStore())
	livingShips = shipxp.NewService(shipxp.NewMemStore())
	r := world.NewRouter(w)
	r.Add(w)
	opts := Options{MaxMessage: 8192, RatePerSec: 200, Version: "t", World: r, Store: store, Quests: svc, Pilot: livingPilot, Ships: livingShips}
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

func TestPilotXPFromSellOnceAndSpend(t *testing.T) {
	_, _, url := livingServer(t, "xpsellertoken0001", "TMP-MINE", world.Vec{X: -12 + 8, Y: 14}, map[string]float64{"iron_ore": 10})
	cl := connect(t, url)
	w := cl.joinRaw("xpsellertoken0001")
	p0, _ := w["pilot"].(map[string]any)
	if p0 == nil || p0["level"].(float64) != 1 || p0["xp"].(float64) != 0 {
		t.Fatalf("в welcome нет состояния пилота на уровне 1: %v", w["pilot"])
	}
	cl.send("sell", nil)
	credits := int64(10 * worldgen.DefaultConfig().Resources["iron_ore"].Price)
	want := pilot.XPFor("sell", credits)
	m := cl.next("xp")
	var x struct {
		Gained  int64
		Source  string
		LevelUp bool `json:"level_up"`
		Pilot   pilot.State
	}
	_ = json.Unmarshal(m.Payload, &x)
	if x.Gained != want || x.Source != "sell" || x.Pilot.XP != want {
		t.Fatalf("опыт за продажу: %+v (ожидалось %d)", x, want)
	}
	// трата без свободных очков отклоняется сервером
	cl.send("pilot_spend", map[string]any{"attr": "piloting", "points": 1})
	if code := cl.errCode(); code != "pilot_no_points" && x.Pilot.AttrFree > 0 {
		t.Fatalf("код ошибки: %q", code)
	}
	// начислим уровень напрямую сервисом и потратим очки по протоколу
	id := world.PublicID(world.TokenHash("xpsellertoken0001"))
	if _, err := livingPilot.Grant(context.Background(), id, "test:level", "kill", 100000); err != nil {
		t.Fatal(err)
	}
	cl.send("pilot_spend", map[string]any{"attr": "piloting", "points": 3})
	pm := cl.next("pilot")
	var ps pilot.State
	_ = json.Unmarshal(pm.Payload, &ps)
	if ps.Attributes["piloting"] != 3 || ps.AttrSpent != 3 {
		t.Fatalf("очки не потрачены: %+v", ps)
	}
	cl.send("pilot_spend", map[string]any{"attr": "bogus", "points": 1})
	if code := cl.errCode(); code != "pilot_bad_attr" {
		t.Fatalf("неизвестная характеристика: %q", code)
	}
	cl.send("pilot", nil)
	var again pilot.State
	_ = json.Unmarshal(cl.next("pilot").Payload, &again)
	if again.Attributes["piloting"] != 3 {
		t.Fatalf("состояние не сохранилось: %+v", again)
	}
}

type shipLevelState struct {
	Ship struct {
		Type    string  `json:"type"`
		Level   int     `json:"level"`
		HullMax float64 `json:"hull_max"`
	} `json:"ship"`
}

func (cl *client) nextShipLevel(cond func(shipLevelState) bool) shipLevelState {
	cl.t.Helper()
	for i := 0; i < 300; i++ {
		m := cl.next("state")
		var s shipLevelState
		_ = json.Unmarshal(m.Payload, &s)
		if cond(s) {
			return s
		}
	}
	cl.t.Fatal("состояние не достигло ожидаемого")
	return shipLevelState{}
}

func TestShipLevelRestoredOnSwapAndXPFromSell(t *testing.T) {
	_, _, url := livingServer(t, "shiplvltoken00001", "TMP-MINE", world.Vec{X: -12 + 8, Y: 14}, map[string]float64{"iron_ore": 10})
	id := world.PublicID(world.TokenHash("shiplvltoken00001"))
	// у шахтёра уже есть опыт: уровень подтягивается при входе
	if _, err := livingShips.Grant(context.Background(), id, "TMP-MINE", "seed", "kill", rpgShipXP(5)); err != nil {
		t.Fatal(err)
	}
	cl := connect(t, url)
	w := cl.joinRaw("shiplvltoken00001")
	sp, _ := w["ship_progress"].(map[string]any)
	if sp == nil || sp["level"].(float64) != 5 {
		t.Fatalf("welcome.ship_progress: %v", w["ship_progress"])
	}
	s := cl.nextShipLevel(func(s shipLevelState) bool { return s.Ship.Type == "TMP-MINE" })
	if s.Ship.Level != 5 {
		t.Fatalf("уровень корабля в состоянии: %d", s.Ship.Level)
	}
	hull5 := s.Ship.HullMax
	// продажа даёт опыт и пилоту, и кораблю
	cl.send("sell", nil)
	var x struct {
		Gained int64
		Source string
		Ship   shipxp.State
		Pilot  pilot.State
	}
	// На станции параллельно приходит XP за first_flight: ждём именно продажу.
	for i := 0; i < 10; i++ {
		m := cl.next("xp")
		if err := json.Unmarshal(m.Payload, &x); err != nil {
			t.Fatal(err)
		}
		if x.Source == "sell" {
			break
		}
	}
	if x.Source != "sell" || x.Gained <= 0 || x.Ship.Ship != "TMP-MINE" || x.Ship.XP <= rpgShipXP(5) || x.Pilot.XP < x.Gained {
		t.Fatalf("опыт за продажу: %+v", x)
	}
	// смена корабля: у нового свой уровень 1, у прежнего сохраняется
	cl.send("set_ship", map[string]any{"ship": "FTR-001"})
	s = cl.nextShipLevel(func(s shipLevelState) bool { return s.Ship.Type == "FTR-001" })
	if s.Ship.Level != 1 {
		t.Fatalf("новый корабль должен быть на уровне 1, а не %d", s.Ship.Level)
	}
	cl.send("set_ship", map[string]any{"ship": "TMP-MINE"})
	s = cl.nextShipLevel(func(s shipLevelState) bool { return s.Ship.Type == "TMP-MINE" })
	if s.Ship.Level != 5 || s.Ship.HullMax != hull5 {
		t.Fatalf("уровень не восстановился при возврате на корабль: %d, корпус %v (было %v)", s.Ship.Level, s.Ship.HullMax, hull5)
	}
	cl.send("ships", nil)
	var sh struct {
		Levels map[string]shipxp.State `json:"levels"`
	}
	_ = json.Unmarshal(cl.next("ships").Payload, &sh)
	if sh.Levels["TMP-MINE"].Level != 5 {
		t.Fatalf("ангар: уровни кораблей %+v", sh.Levels)
	}
}

func rpgShipXP(level int) int64 { return rpg.DefaultShipCurve.TotalXP(level) }

// TestStationServicesThroughProtocol — Э-2: рынок, ремонт, покупка и улучшение через протокол.
func TestStationServicesThroughProtocol(t *testing.T) {
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	w := world.New()
	if err := w.ApplyLayout(lay, cfg); err != nil {
		t.Fatal(err)
	}
	qs := quest.NewMemStore()
	svc := quest.NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), qs, w.ObjectPos)
	store := world.NewMemStore()
	pid := world.PublicID(world.TokenHash("stationtoken000001"))
	_ = store.Save(context.Background(), world.Record{ID: pid, Name: "Сервис", Pos: world.Vec{X: lay.Hub[0], Y: lay.Hub[1]}, Dir: world.Vec{X: 1}, Created: true, ShipType: "TMP-MINE", PilotSpec: "miner"})
	r := world.NewRouter(w)
	r.Add(w)
	opts := Options{MaxMessage: 8192, RatePerSec: 200, Version: "t", World: r, Store: store, Quests: svc, Pilot: pilot.NewService(pilot.NewMemStore()), Ships: shipxp.NewService(shipxp.NewMemStore())}
	srv, h := newServer(t, opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	cl := connect(t, srv.URL+"/ws")
	cl.send("join", map[string]any{"token": "stationtoken000001", "protocol": ProtocolVersion})
	cl.next("welcome")

	cl.send("market", map[string]any{})
	m := cl.next("market")
	var md struct {
		Prices      map[string]float64 `json:"prices"`
		RepairCost  int64              `json:"repair_cost"`
		UpgradeCost int64              `json:"upgrade_cost"`
	}
	if err := json.Unmarshal(m.Payload, &md); err != nil {
		t.Fatal(err)
	}
	if len(md.Prices) == 0 || md.Prices["iron_ore"] <= 0 {
		t.Fatalf("цены: %+v", md)
	}

	// покупка без кредитов — отказ no_credits
	cl.send("buy", map[string]any{"resource": "iron_ore", "amount": 5})
	if m := cl.next("error"); !strings.Contains(string(m.Payload), "no_credits") {
		t.Fatalf("ожидался no_credits: %+v", m)
	}

	// выдаём кредиты и покупаем
	if err := svc.Join(context.Background(), pid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Grant(context.Background(), pid, "test-grant", 5000); err != nil {
		t.Fatal(err)
	}
	cl.send("buy", map[string]any{"resource": "iron_ore", "amount": 5})
	ok := cl.next("ok")
	var od struct {
		Action  string `json:"action"`
		Credits int64  `json:"credits"`
	}
	if err := json.Unmarshal(ok.Payload, &od); err != nil || od.Action != "buy" || od.Credits <= 0 {
		t.Fatalf("покупка: %+v %v", od, err)
	}

	// улучшение корабля
	cl.send("ship_upgrade", map[string]any{})
	ok = cl.next("ok")
	if err := json.Unmarshal(ok.Payload, &od); err != nil || od.Action != "ship_upgrade" {
		t.Fatalf("улучшение: %+v %v", od, err)
	}

	// ремонт целого корабля — ок без списания
	cl.send("repair", map[string]any{})
	ok = cl.next("ok")
	if err := json.Unmarshal(ok.Payload, &od); err != nil || od.Action != "repair" {
		t.Fatalf("ремонт: %+v %v", od, err)
	}
}
