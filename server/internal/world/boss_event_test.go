package world

import (
	"testing"
)

func TestLeviathanPhases(t *testing.T) {
	w, _ := livingWorld(t)
	w.startLiveEvent(mustEvent(t, "leviathan"), eventMin)
	n := w.anyNPC("dyn-leviathan")
	if n == nil || n.ai == nil || n.HullMax != 18000 {
		t.Fatalf("boss not spawned: %+v", n)
	}
	n.Hull = 10000
	w.tickLiveEvent(0.1)
	if n.ai.cfg.Damage != 24 {
		t.Fatalf("phase 2 damage: %v", n.ai.cfg.Damage)
	}
	n.Hull = 4000
	w.tickLiveEvent(0.1)
	if n.ai.cfg.Damage != 34 {
		t.Fatalf("phase 3 damage: %v", n.ai.cfg.Damage)
	}
}
