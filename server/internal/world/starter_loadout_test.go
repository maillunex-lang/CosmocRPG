package world

import (
	"reflect"
	"testing"
)

func savedStarter(id string, fitted bool, revision int64) StarterLoadout {
	return StarterLoadout{OwnerID: id, ShipType: DefaultShip, InstanceID: starterEngineID(id), PrototypeID: StrizhEnginePrototype, SlotID: StrizhEngineSlot, Equipped: fitted, Revision: revision}
}

func TestSavedStarterSurvivesWorldTransitions(t *testing.T) {
	w := New()
	s := savedStarter("saved", false, 1)
	rec := Record{ID: s.OwnerID, StarterLoadout: &s}
	p, ok := w.Join(rec)
	if !ok || p.boostAccess() != ErrModuleUnavailable {
		t.Fatal("loaded unequipped engine enabled")
	}
	// Reconnect must keep current simulation resources rather than reinitialize.
	p.Energy = 7
	w.Join(rec)
	if p.Energy != 7 || p.boostAccess() != ErrModuleUnavailable {
		t.Fatal("reconnect reset ship")
	}
	for _, hull := range []string{"TMP-MINE", DefaultShip} {
		if _, err := w.SetShip(p.ID, hull); err != nil {
			t.Fatal(err)
		}
	}
	if p.boostAccess() != ErrModuleUnavailable {
		t.Fatal("hull roundtrip re-equipped engine")
	}
	rec, _ = w.Remove(p.ID)
	*rec.StarterLoadout = savedStarter(p.ID, false, 3)
	if p.starterLoadout.Revision != 1 {
		t.Fatal("record shares mutable loadout")
	}
	other := New()
	q, _ := other.Join(rec)
	if q.boostAccess() != ErrModuleUnavailable || q.starterLoadout.Revision != 3 {
		t.Fatal("world transfer reset loadout")
	}
	// A delayed bootstrap cannot overwrite a later applied commit.
	if !other.RestoreStarter(q.ID, savedStarter(q.ID, true, 4)) {
		t.Fatal("apply newer")
	}
	other.Join(rec)
	if q.starterLoadout.Revision != 4 || q.boostAccess() != nil {
		t.Fatal("stale join overwrote latest")
	}
	bad := savedStarter(q.ID, false, 4)
	before, _ := other.Snapshot(q.ID)
	if other.RestoreStarter(q.ID, bad) {
		t.Fatal("same revision with conflicting state accepted")
	}
	after, _ := other.Snapshot(q.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rejected load changed world")
	}
}

func TestSavedStarterJumpRetainsUnequippedState(t *testing.T) {
	r, cfg := testUniverse(t)
	s := savedStarter("jump-engine", false, 7)
	gate := cfg.Systems[0].Jumps[0]
	r.Join(Record{ID: s.OwnerID, Pos: Vec{gate.Pos[0], gate.Pos[1]}, StarterLoadout: &s})
	if _, _, err := r.Jump(s.OwnerID, gate.To); err != nil {
		t.Fatal(err)
	}
	w := r.For(s.OwnerID)
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[s.OwnerID]
	if *p.starterLoadout != s || p.boostAccess() != ErrModuleUnavailable {
		t.Fatal("jump restored native equipment")
	}
}
