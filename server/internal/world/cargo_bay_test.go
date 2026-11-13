package world

import "testing"

func TestCargoBaysFollowTheShip(t *testing.T) {
	w, _ := livingWorld(t)
	at := Vec{-12, 14}
	f := joinAt(w, "f", "FTR-001", at)
	if err := w.AddCargo("f", "iron_ore", 1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddCargo("f", "iron_ore", 8); err != ErrHoldFull {
		t.Fatalf("истребитель принял руду сверх рудного отсека: %v", err)
	}
	if f.hold["iron_ore"] > StarterOreCapacity+0.01 {
		t.Fatalf("истребитель увёз %.1f руды", f.hold["iron_ore"])
	}
	mine := joinAt(w, "m", "TMP-MINE", at)
	if err := w.AddCargo("m", "iron_ore", 30); err != nil {
		t.Fatalf("добытчик не взял руду: %v", err)
	}
	if mine.hold["iron_ore"] < 30 {
		t.Fatalf("добытчик взял только %.1f", mine.hold["iron_ore"])
	}
	st, ok := w.Snapshot("m")
	if !ok || len(st.Ship.Bays) < 3 || st.Ship.Bays[0].ID != "ore" || st.Ship.Bays[0].Used < 30 {
		t.Fatalf("снимка отсеков нет: %+v", st.Ship.Bays)
	}
}

func TestWreckIsCollectedOnce(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	p := joinAt(w, "p", "FTR-001", e.Pos)
	other := joinAt(w, "q", "FTR-001", e.Pos)
	w.mu.Lock()
	e.Hull, e.Shield = 1, 0
	w.hitNPC(p, e, 100)
	w.mu.Unlock()
	step(w, 1)
	if p.hold["scrap"]+other.hold["scrap"] < 1 {
		t.Fatalf("обломки не подобраны: %v %v", p.hold, other.hold)
	}
	got := p.hold["scrap"] + other.hold["scrap"]
	step(w, 2)
	if p.hold["scrap"]+other.hold["scrap"] > got+0.01 {
		t.Fatal("обломки подобраны дважды")
	}
}
