package world

import (
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

func TestTakeCargoOnlyAtStation(t *testing.T) {
	w, lay := livingWorld(t)
	p := joinAt(w, "hauler", "FTR-001", Vec{lay.Hub[0], lay.Hub[1]})
	p.hold = map[string]float64{"iron_ore": 5}
	if _, ok := w.TakeCargo("hauler", "iron_ore", 5); !ok {
		t.Fatal("у станции руда должна списываться")
	}
	if p.hold["iron_ore"] != 0 {
		t.Fatalf("груз остался: %v", p.hold)
	}
	p.hold["iron_ore"] = 2
	p.Pos = Vec{lay.Hub[0] + 400, lay.Hub[1]}
	if _, ok := w.TakeCargo("hauler", "iron_ore", 2); ok {
		t.Fatal("вне станции груз списался")
	}
	if p.hold["iron_ore"] != 2 {
		t.Fatal("отказ изменил трюм")
	}
	p.Pos = Vec{lay.Hub[0], lay.Hub[1]}
	if _, ok := w.TakeCargo("hauler", "iron_ore", 5); ok {
		t.Fatal("нельзя списать больше, чем есть")
	}
	rec, ok := w.GiveCargo("hauler", "iron_ore", 3)
	if !ok || rec.Hold["iron_ore"] != 5 {
		t.Fatalf("возврат груза: %v %v", ok, rec.Hold)
	}
}

func TestArrivalAndVisit(t *testing.T) {
	cfg := worldgen.DefaultConfig()
	var spec worldgen.SystemParams
	for _, s := range cfg.Systems {
		if s.ID == "meridian" {
			spec = s
		}
	}
	if spec.ID == "" {
		t.Fatal("нет Меридиана")
	}
	lay := worldgen.Generate(spec, cfg)
	w := New()
	if err := w.ApplyLayout(lay, cfg); err != nil {
		t.Fatal(err)
	}
	if w.station.Pos != ([2]float64{lay.Hub[0], lay.Hub[1]}) {
		t.Fatalf("главная станция не на узле Меридиана: %+v", w.station.Pos)
	}
	var anomaly, hidden *npc
	for _, n := range w.npcs {
		if n.Kind == "anomaly" && anomaly == nil {
			anomaly = n
		}
		if n.Hidden && hidden == nil {
			hidden = n
		}
	}
	if anomaly == nil {
		t.Fatal("в Меридиане нет аномалии")
	}
	joinAt(w, "scout", "FTR-001", anomaly.Pos)
	w.Step(0.05, time.Now())
	found := false
	for _, a := range w.DrainArrivals() {
		if a.Player == "scout" && a.Kind == "anomaly" && a.ID == anomaly.ID && a.System == "meridian" {
			found = true
		}
		if hidden != nil && a.ID == hidden.ID {
			t.Fatal("скрытый сигнал попал в журнал без раскрытия")
		}
	}
	if !found {
		t.Fatal("прибытие к аномалии не записано")
	}
	if !w.AtKind("scout", "anomaly") {
		t.Fatal("игрок на аномалии не засчитан")
	}
	if w.AtKind("scout", "station") {
		t.Fatal("аномалия не должна считаться станцией")
	}
	w.Step(0.05, time.Now())
	if again := w.DrainArrivals(); len(again) != 0 {
		t.Fatal("повторный шаг снова прислал прибытие")
	}
	joinAt(w, "docker", "FTR-001", Vec{lay.Hub[0], lay.Hub[1]})
	w.Step(0.05, time.Now())
	station := false
	for _, a := range w.DrainArrivals() {
		if a.Player == "docker" && a.Kind == "station" && a.System == "meridian" {
			station = true
		}
	}
	if !station || !w.AtKind("docker", "station") {
		t.Fatal("прибытие на станцию Меридиана не засчитано")
	}
}
