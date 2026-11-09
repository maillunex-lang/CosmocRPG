package world

import "testing"

func TestShipLevelScalesStatsAndIsIdempotent(t *testing.T) {
	w := New()
	w.Join(Record{ID: "p1", Name: "Пилот"})
	st0, _ := w.Snapshot("p1")
	if st0.Ship.Level != 1 {
		t.Fatalf("уровень нового корабля: %d", st0.Ship.Level)
	}
	hull0, shield0, cargo0 := st0.Ship.HullMax, st0.Ship.ShieldMax, st0.Ship.Cargo
	w.SetShipLevel("p1", 11)
	st, _ := w.Snapshot("p1")
	if st.Ship.Level != 11 || st.Ship.HullMax <= hull0 || st.Ship.ShieldMax <= shield0 || st.Ship.Cargo <= cargo0 {
		t.Fatalf("уровень 11 должен увеличить характеристики: %+v", st.Ship)
	}
	want := hull0 * 1.3 // 3 % за каждый из 10 уровней выше первого
	if d := st.Ship.HullMax - want; d > 1e-6 || d < -1e-6 {
		t.Fatalf("корпус %v, ожидалось %v", st.Ship.HullMax, want)
	}
	if st.Ship.Hull != st.Ship.HullMax {
		t.Fatalf("целый корабль остаётся целым: %v из %v", st.Ship.Hull, st.Ship.HullMax)
	}
	w.SetShipLevel("p1", 11)
	w.SetShipLevel("p1", 11)
	st2, _ := w.Snapshot("p1")
	if st2.Ship.HullMax != st.Ship.HullMax || st2.Ship.Cargo != st.Ship.Cargo {
		t.Fatal("повторное применение уровня накопило рост")
	}
	w.SetShipLevel("p1", 999)
	if s, _ := w.Snapshot("p1"); s.Ship.Level != 50 {
		t.Fatalf("потолок уровня корабля: %d", s.Ship.Level)
	}
	w.SetShipLevel("p1", 0)
	if s, _ := w.Snapshot("p1"); s.Ship.Level != 1 || s.Ship.HullMax != hull0 {
		t.Fatalf("уровень ниже 1 считается первым: %+v", s.Ship)
	}
	if typ, ok := w.ShipTypeOf("p1"); !ok || typ == "" {
		t.Fatal("тип корабля не найден")
	}
	if _, ok := w.ShipTypeOf("нет"); ok {
		t.Fatal("неизвестный игрок")
	}
}

func TestShipLevelKeepsDamageTaken(t *testing.T) {
	w := New()
	w.Join(Record{ID: "p1", Name: "Пилот"})
	w.mu.Lock()
	p := w.players["p1"]
	p.Hull -= 100
	before := p.Hull
	w.mu.Unlock()
	w.SetShipLevel("p1", 6)
	st, _ := w.Snapshot("p1")
	if st.Ship.HullMax-st.Ship.Hull < 99.9 {
		t.Fatalf("повышение уровня не должно лечить: корпус %v из %v (до %v)", st.Ship.Hull, st.Ship.HullMax, before)
	}
}
