package world

import (
	"testing"
	"time"
)

func TestAuroraEventsRunWithoutPlayers(t *testing.T) {
	w, _ := livingWorld(t)
	if w.system.ID != "aurora" {
		t.Fatal(w.system.ID)
	}
	w.Step(1, time.Now())
	if w.live == nil || w.live.kind != "convoy" || w.anyNPC("dyn-convoy") == nil {
		t.Fatalf("конвой не вышел сам: %+v", w.live)
	}
	start := w.anyNPC("dyn-convoy").Pos
	w.Step(5, time.Now())
	if dist(w.anyNPC("dyn-convoy").Pos, start) < 50 {
		t.Fatal("грузовик не двигается без игроков")
	}
	if w.npcByID("aur-beacon", nil) == nil && w.anyNPC("aur-beacon") == nil {
		t.Fatal("учебный маяк пропал из-за события")
	}
	w.Step(eventSpan, time.Now())
	if w.live == nil || w.live.kind != "raid" || w.anyNPC("dyn-convoy") != nil {
		t.Fatalf("слот не сменился: %+v", w.live)
	}
	if w.anyNPC("dyn-raid-0") == nil || w.anyNPC("dyn-raid-1") == nil {
		t.Fatal("налёт не появился")
	}
	w.Step(eventSpan, time.Now())
	d := w.depositByID("dyn-bloom")
	if w.live.kind != "bloom" || d == nil || d.Amount >= 48 {
		t.Fatalf("жила не тает сама: %+v", w.live)
	}
	before := d.Amount
	w.Step(10, time.Now())
	if w.depositByID("dyn-bloom").Amount >= before {
		t.Fatal("без добычи жила не уменьшилась")
	}
}

func TestAuroraEventsAreSharedAndDeterministic(t *testing.T) {
	a, _ := livingWorld(t)
	b, _ := livingWorld(t)
	for i := 0; i < 4; i++ {
		a.Step(3, time.Now())
		b.Step(3, time.Now())
	}
	if a.live.id != b.live.id || a.anyNPC("dyn-convoy") == nil {
		t.Fatal("миры разошлись")
	}
	pa, pb := a.anyNPC("dyn-convoy").Pos, b.anyNPC("dyn-convoy").Pos
	if pa != pb {
		t.Fatalf("позиция конвоя %v и %v", pa, pb)
	}
	plain := New()
	plain.Step(30, time.Now())
	for _, n := range plain.npcs {
		if len(n.ID) >= 4 && n.ID[:4] == "dyn-" {
			t.Fatal("событие Авроры попало в пустой мир")
		}
	}
}

func TestEventRotationVisitsEveryKind(t *testing.T) {
	w, _ := livingWorld(t)
	seen := map[string]bool{}
	for i := 0; i < len(auroraEvents); i++ {
		w.Step(0.1, time.Now())
		if w.live == nil {
			t.Fatal("нет события")
		}
		seen[w.live.kind] = true
		p := joinAt(w, "watcher", "FTR-001", Vec{})
		st, ok := w.Snapshot(p.ID)
		if !ok || st.Event == nil || st.Event.Kind != w.live.kind || st.Event.Title == "" || st.Event.Left <= 0 {
			t.Fatalf("снимок: %+v", st.Event)
		}
		w.Remove(p.ID)
		w.Step(eventSpan, time.Now())
	}
	if len(seen) != len(auroraEvents) {
		t.Fatalf("виды: %v", seen)
	}
}
