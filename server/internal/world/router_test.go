package world

import (
	"math"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// testWorld — мир с раскладкой системы для тестов Router (Э-1).
func testWorld(t *testing.T, cfg worldgen.Config, idx int) *World {
	t.Helper()
	w := New()
	if err := w.ApplyLayout(worldgen.Generate(cfg.Systems[idx], cfg), cfg); err != nil {
		t.Fatal(err)
	}
	return w
}

func testUniverse(t *testing.T) (*Router, worldgen.Config) {
	t.Helper()
	cfg := worldgen.DefaultConfig()
	if len(cfg.Systems) < 3 {
		t.Fatal("в конфигурации меньше трёх систем")
	}
	aurora := New()
	if err := aurora.ApplyLayout(worldgen.Generate(cfg.Systems[0], cfg), cfg); err != nil {
		t.Fatal(err)
	}
	r := NewRouter(aurora)
	r.Add(aurora)
	for i := 1; i < len(cfg.Systems); i++ {
		r.Add(testWorld(t, cfg, i))
	}
	for _, sp := range cfg.Systems {
		gates := make([]Gate, 0, len(sp.Jumps))
		for _, j := range sp.Jumps {
			gates = append(gates, Gate{To: j.To, Pos: [2]float64{j.Pos[0], j.Pos[1]}})
		}
		r.SetGates(sp.ID, gates)
	}
	return r, cfg
}

func TestRouterJoinsBySystem(t *testing.T) {
	r, cfg := testUniverse(t)
	p1, ok := r.Join(Record{ID: "p1", Name: "A", Created: true, System: cfg.Systems[1].ID})
	if !ok || p1 == nil {
		t.Fatal("join meridian")
	}
	if got := r.For("p1").System().ID; got != cfg.Systems[1].ID {
		t.Fatalf("p1 в системе %s", got)
	}
	// без системы — в главный мир
	r.Join(Record{ID: "p2", Name: "B", Created: true})
	if r.For("p2") != r.Primary() {
		t.Fatal("p2 должен быть в главном мире")
	}
	// изоляция: Records и Count суммируют, но Snapshot видит только свою систему
	if n := r.Count(); n != 2 {
		t.Fatalf("Count=%d", n)
	}
	st, ok := r.Snapshot("p1")
	if !ok || len(st.Objects) == 0 {
		t.Fatalf("снимок p1: %+v", st)
	}
}

func TestRouterJump(t *testing.T) {
	r, cfg := testUniverse(t)
	fromID, toID := cfg.Systems[0].ID, cfg.Systems[1].ID
	gate := Vec{X: cfg.Systems[0].Jumps[0].Pos[0], Y: cfg.Systems[0].Jumps[0].Pos[1]}
	r.Join(Record{ID: "p1", Name: "A", Created: true, Pos: Vec{X: 0, Y: 0}, System: fromID, Hold: map[string]float64{"iron_ore": 5}})

	// далеко от точки — отказ
	if _, _, err := r.Jump("p1", toID); err != ErrNoGate {
		t.Fatalf("ожидался no_gate, получено %v", err)
	}
	// нет связи — отказ (подменяем позицию: пересаживаем игрока у точки)
	r.Leave("p1")
	r.Join(Record{ID: "p1", Name: "A", Created: true, Pos: gate, System: fromID, Hold: map[string]float64{"iron_ore": 5}})
	if _, _, err := r.Jump("p1", "nowhere"); err != ErrNoLink {
		t.Fatalf("ожидался no_link, получено %v", err)
	}
	// прыжок у точки — успех, груз и система сохраняются
	rec, spawn, err := r.Jump("p1", toID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.System != toID {
		t.Fatalf("система после прыжка: %s", rec.System)
	}
	if rec.Hold["iron_ore"] != 5 {
		t.Fatalf("груз потерян: %+v", rec.Hold)
	}
	if r.For("p1").System().ID != toID {
		t.Fatal("игрок не в целевой системе")
	}
	// появление у зеркальной точки
	mirror := Vec{}
	for _, j := range cfg.Systems[1].Jumps {
		if j.To == fromID {
			mirror = Vec{X: j.Pos[0], Y: j.Pos[1]}
		}
	}
	if spawn != mirror {
		t.Fatalf("появление %v, ожидалось у зеркала %v", spawn, mirror)
	}
	// обратный прыжок
	if _, _, err := r.Jump("p1", fromID); err != nil {
		t.Fatalf("обратный прыжок: %v", err)
	}
	if r.For("p1").System().ID != fromID {
		t.Fatal("игрок не вернулся")
	}
}

func TestRouterAggregate(t *testing.T) {
	r, cfg := testUniverse(t)
	r.Join(Record{ID: "p1", Name: "A", Created: true, System: cfg.Systems[0].ID})
	r.Join(Record{ID: "p2", Name: "B", Created: true, System: cfg.Systems[2].ID})
	if len(r.Records()) != 2 {
		t.Fatalf("Records: %d", len(r.Records()))
	}
	now := time.Now()
	if recs := r.Step(1.0/10, now); len(recs) != 0 {
		t.Fatalf("неожиданные записи шага: %d", len(recs))
	}
	if len(r.Systems()) != len(cfg.Systems) {
		t.Fatalf("Systems: %d", len(r.Systems()))
	}
	if len(r.Gates(cfg.Systems[0].ID)) == 0 {
		t.Fatal("нет точек прыжка у Авроры")
	}
}

func TestRouterPlaceScattersAI(t *testing.T) {
	r, cfg := testUniverse(t)
	from, to := cfg.Systems[0].ID, cfg.Systems[2].ID
	if _, ok := r.Join(Record{ID: "ai1", Name: "Кира", Created: true, AI: true, AICaptain: "cpt-0001", System: from, Pos: Vec{}}); !ok {
		t.Fatal("join")
	}
	if err := r.Place("ai1", to, Vec{X: 10, Y: 20}); err != nil {
		t.Fatal(err)
	}
	if got := r.SystemID("ai1"); got != to {
		t.Fatalf("система %s", got)
	}
	if !r.SetPos("ai1", Vec{X: 400, Y: -200}) {
		t.Fatal("setpos")
	}
	st, ok := r.Snapshot("ai1")
	if !ok {
		t.Fatal("нет снимка")
	}
	if math.Hypot(st.Ship.Pos[0]-400, st.Ship.Pos[1]+200) > 1 {
		t.Fatalf("позиция %+v", st.Ship.Pos)
	}
}
