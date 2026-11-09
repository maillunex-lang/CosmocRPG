package world

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func dockedEngine(t *testing.T) (*World, *Player) {
	t.Helper()
	w := New()
	s := savedStarter("docked", true, 0)
	p, ok := w.Join(Record{ID: s.OwnerID, Pos: Vec{w.station.Pos[0], w.station.Pos[1]}, PersistentEquipment: true, StarterLoadout: &s})
	if !ok {
		t.Fatal("join")
	}
	return w, p
}

func TestEquipmentRejectsUnsafeChangesWithoutStorage(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*World, *Player)
		want  error
	}{
		{"other hull", func(w *World, p *Player) { w.initShip(p, "TMP-MINE", "", "") }, ErrEquipmentHull},
		{"dead", func(w *World, p *Player) { p.deadUntil = w.t + 3 }, ErrDead},
		{"attacking", func(w *World, p *Player) { p.Attacking = true }, ErrEquipmentCombat},
		{"recent incoming", func(w *World, p *Player) { w.DamagePlayer(p.ID, 1) }, ErrEquipmentCombat},
		{"recent outgoing", func(w *World, p *Player) { w.hitNPC(p, w.npcs[0], 1) }, ErrEquipmentCombat},
		{"safe zone far from station", func(w *World, p *Player) {
			p.Pos = Vec{500, 500}
			w.safe = append(w.safe, safeZone{Pos: p.Pos, Radius: 100})
		}, ErrNoStable},
		{"station outer safe ring", func(w *World, p *Player) { p.Pos.X += EquipmentDockRadius + 1 }, ErrNoStable},
		{"move intent", func(w *World, p *Player) { p.Move = Vec{1, 0} }, ErrEquipmentBusy},
		{"still moving", func(w *World, p *Player) { p.Speed = 1 }, ErrEquipmentBusy},
		{"autopilot", func(w *World, p *Player) { p.Autopilot = true }, ErrEquipmentBusy},
		{"mining", func(w *World, p *Player) { p.mining = "deposit" }, ErrEquipmentBusy},
		{"active boost", func(w *World, p *Player) { p.boostUntil = w.t + 3 }, ErrEquipmentBusy},
		{"unloaded", func(w *World, p *Player) { p.starterLoadout = nil }, ErrEquipmentUnavailable},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w, p := dockedEngine(t)
			tt.setup(w, p)
			before, _ := w.Snapshot(p.ID)
			called := false
			err := w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { called = true; return StarterLoadout{}, nil })
			after, _ := w.Snapshot(p.ID)
			if !errors.Is(err, tt.want) || called || !reflect.DeepEqual(before, after) {
				t.Fatalf("err=%v called=%v world changed=%v", err, called, !reflect.DeepEqual(before, after))
			}
		})
	}
}

func TestEquipmentCommitReconcileAndFailClosed(t *testing.T) {
	w, p := dockedEngine(t)
	initial := *p.starterLoadout
	want := initial
	want.Equipped = false
	want.Revision++
	if err := w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { return want, nil }); err != nil {
		t.Fatal(err)
	}
	if p.boostAccess() != ErrModuleUnavailable || p.boostRank != 1 {
		t.Fatal("unequipped engine grants ability or erases learned rank")
	}
	if err := w.ChangeStarter(p.ID, 0, true, func() (StarterLoadout, error) { t.Fatal("stale called store"); return initial, nil }); err != ErrEquipmentConflict {
		t.Fatal(err)
	}
	// Definite rollback reconciles the unchanged DB revision, without granting.
	if err := w.ChangeStarter(p.ID, 1, true, func() (StarterLoadout, error) { return want, ErrEquipmentUnavailable }); err != ErrEquipmentUnavailable {
		t.Fatal(err)
	}
	if *p.starterLoadout != want || p.boostAccess() != ErrModuleUnavailable {
		t.Fatal("failed write changed state")
	}
	// Commit succeeded but the response was lost: reload is authoritative.
	fitted := want
	fitted.Equipped = true
	fitted.Revision++
	if err := w.ChangeStarter(p.ID, 1, true, func() (StarterLoadout, error) { return fitted, ErrEquipmentUnavailable }); err != ErrEquipmentUnavailable {
		t.Fatal(err)
	}
	if *p.starterLoadout != fitted || p.boostAccess() != nil {
		t.Fatal("ambiguous commit was rolled back in memory")
	}
	if err := w.ChangeStarter(p.ID, 2, false, func() (StarterLoadout, error) { return StarterLoadout{}, ErrEquipmentUnavailable }); err != ErrEquipmentUnavailable {
		t.Fatal(err)
	}
	if p.starterLoadout != nil || p.boostAccess() != ErrModuleUnavailable {
		t.Fatal("unknown outcome left boost enabled")
	}
	if !w.RestoreStarter(p.ID, fitted) || p.boostAccess() != nil {
		t.Fatal("explicit reload failed to recover")
	}
}

func TestEquipmentCombatGraceSurvivesTransfer(t *testing.T) {
	w, p := dockedEngine(t)
	w.DamagePlayer(p.ID, 1)
	rec, _ := w.Remove(p.ID)
	next := New()
	q, _ := next.Join(rec)
	if err := next.equipmentCheck(q); err != ErrEquipmentCombat {
		t.Fatalf("transfer erased recent combat: %v", err)
	}
	next.Step(equipmentCombatGrace+0.1, time.Now())
	if err := next.equipmentCheck(q); err != nil {
		t.Fatalf("combat grace did not expire: %v", err)
	}
}

func TestEquipmentCommitSerializesMovement(t *testing.T) {
	w, p := dockedEngine(t)
	entered, release, moved := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { close(entered); <-release; return savedStarter(p.ID, false, 1), nil })
	}()
	<-entered
	go func() { w.SetMove(p.ID, Vec{1, 0}); close(moved) }()
	select {
	case <-moved:
		t.Fatal("move entered before commit")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-moved
	st, _ := w.Snapshot(p.ID)
	if st.Ship.Modules[0].Equipped || st.Ship.Equipment.CanChange {
		t.Fatal("commit/movement ordering lost")
	}
}

func TestEquipmentDoesNotRewriteLearnedRankOrCombatResources(t *testing.T) {
	for _, rank := range []int{0, 4} {
		w, p := dockedEngine(t)
		p.boostRank = rank
		p.Energy = 37
		p.cooldowns["boost"] = 100
		if err := w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) {
			return savedStarter(p.ID, false, 1), nil
		}); err != nil {
			t.Fatal(err)
		}
		if p.boostRank != rank || p.Energy != 37 || p.cooldowns["boost"] != 100 {
			t.Fatalf("equipment rewrote learned/resources: rank=%d want=%d energy=%v cd=%v", p.boostRank, rank, p.Energy, p.cooldowns["boost"])
		}
		if !w.RestoreStarter(p.ID, savedStarter(p.ID, true, 2)) {
			t.Fatal("reload")
		}
		w.Join(Record{ID: p.ID, PersistentEquipment: true, StarterLoadout: copyStarter(p.starterLoadout)})
		if p.boostRank != rank || p.Energy != 37 || p.cooldowns["boost"] != 100 {
			t.Fatal("reload/reconnect rewrote learned/resources")
		}
	}
}
