package world

import (
	"testing"
	"time"
)

func TestChapterRouteAutopilotReachesPins(t *testing.T) {
	w, lay := livingWorld(t)
	if w.system.ID != "aurora" {
		t.Fatal(w.system.ID)
	}
	// Пираты случайного налёта не должны сбивать проверку учебного маршрута.
	w.live = &liveEvent{id: "hold", until: 1e12, kind: "hold"}
	p := joinAt(w, "cadet", "FTR-001", Vec{lay.Hub[0], lay.Hub[1]})
	if p.spec.Speed < 5 {
		t.Fatalf("учебный корабль не летит: скорость %.1f", p.spec.Speed)
	}
	hub := Vec{lay.Hub[0], lay.Hub[1]}
	for _, id := range []string{"aur-beacon", "aur-az01", "aur-az02", "aur-az03", "aur-az04", "aur-az05"} {
		n := w.anyNPC(id)
		if n == nil {
			t.Fatal("нет точки", id)
		}
		if dist(n.Pos, hub) < 80 {
			t.Fatalf("%s стоит у дока (%.0f), маршрут некуда вести", id, dist(n.Pos, hub))
		}
		p.Pos = hub
		p.Autopilot = false
		if !w.SetTarget(p.ID, id) || !w.SetAutopilot(p.ID, true) {
			t.Fatal("курс не взялся", id)
		}
		moved := false
		arrived := false
		for i := 0; i < 5000; i++ {
			w.Step(0.1, time.Now())
			if dist(p.Pos, hub) > 40 {
				moved = true
			}
			if w.AtKind(p.ID, id) {
				arrived = true
				break
			}
		}
		if !moved || !arrived || !p.Autopilot {
			t.Fatalf("%s: автопилот не довёл, pos %v moved %v arrived %v on %v", id, p.Pos, moved, arrived, p.Autopilot)
		}
	}
	gate := Vec{1150, 200}
	p.Pos = hub
	if !w.SetWaypoint(p.ID, gate) {
		t.Fatal("ворота Меридиана не приняты")
	}
	for i := 0; i < 8000 && dist(p.Pos, gate) > 70; i++ {
		w.Step(0.1, time.Now())
	}
	if dist(p.Pos, gate) > 70 {
		t.Fatalf("автопилот не дошёл до ворот: %v", p.Pos)
	}
}
