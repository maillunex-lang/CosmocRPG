package world

import (
	"math"
	"testing"
)

func hasRole(list []AbilityInfo, role string) bool {
	for _, a := range list {
		if a.Role == role {
			return true
		}
	}
	return false
}

func TestEachHullHasAttackUltimateAndModuleSkills(t *testing.T) {
	w := New()
	for id := range shipKits {
		if _, ok := w.Join(Record{ID: id, Name: id, Created: true, ShipType: id}); !ok {
			t.Fatal(id)
		}
		st, _ := w.Snapshot(id)
		strike, ult, module := 0, 0, 0
		for _, a := range st.Ship.Abilities {
			switch a.Role {
			case "strike":
				strike++
			case "ult":
				ult++
			case "module":
				module++
			}
		}
		if strike != 1 || ult != 1 || module != 2 {
			t.Fatalf("%s умения %+v", id, st.Ship.Abilities)
		}
	}
}

func TestShipStrikeHitsTheTarget(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	if e == nil {
		t.Fatal("нет врага")
	}
	p := joinAt(w, "gun", "FTR-001", e.Pos)
	if !w.SetTarget(p.ID, e.ID) {
		t.Fatal("цель")
	}
	before := e.Hull + e.Shield
	if err := w.UseAbility(p.ID, "ftr1_salvo"); err != nil {
		t.Fatal(err)
	}
	if e.Hull+e.Shield >= before {
		t.Fatal("залп не нанёс урон")
	}
	if err := w.UseAbility(p.ID, "ftr1_salvo"); err != ErrCooldown {
		t.Fatalf("перезарядка: %v", err)
	}
	w.SetModuleCategories(p.ID, []string{"support", "drone"})
	st, _ := w.Snapshot(p.ID)
	got := map[string]bool{}
	for _, a := range st.Ship.Abilities {
		if a.Role == "module" {
			got[a.ID] = true
		}
	}
	if !got["mod_mend"] || !got["mod_drone"] {
		t.Fatalf("модули не сменили умения: %+v", st.Ship.Abilities)
	}
}

func TestModuleChoiceBeatsTheStockGuns(t *testing.T) {
	w := New()
	if _, ok := w.Join(Record{ID: "fit", Name: "fit", Created: true, ShipType: "FTR-001"}); !ok {
		t.Fatal("вход")
	}
	w.SetModuleCategories("fit", []string{"weapon", "shield", "engine", "reactor", "electronics", "drone"})
	st, _ := w.Snapshot("fit")
	got := map[string]bool{}
	for _, a := range st.Ship.Abilities {
		if a.Role == "module" {
			got[a.ID] = true
		}
	}
	if !got["mod_drone"] || !got["mod_barrier"] || got["mod_over"] || len(got) != 2 {
		t.Fatalf("дрон должен занять слот вместо перегрузки: %+v", st.Ship.Abilities)
	}
	var ultLast bool
	if n := len(st.Ship.Abilities); n > 0 && st.Ship.Abilities[n-1].Role == "ult" {
		ultLast = true
	}
	if !ultLast {
		t.Fatal("ульта должна быть последней, на большой кнопке")
	}
}

func TestFortifyReducesIncomingDamage(t *testing.T) {
	w := New()
	p := join(t, w, "tank")
	w.SetModuleCategories(p.ID, []string{"armor"})
	if err := w.UseAbility(p.ID, "mod_fortify"); err != nil {
		t.Fatal(err)
	}
	before := p.Hull + p.Shield
	w.DamagePlayer(p.ID, 100)
	lost := before - (p.Hull + p.Shield)
	if math.Abs(lost-65) > 0.01 {
		t.Fatalf("бронеперегрузка: урон %.1f, ждали 65", lost)
	}
}
