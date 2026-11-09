package world

import (
	"testing"
	"time"
)

func TestEquipmentSlowStoreDoesNotBlockSimulation(t *testing.T) {
	w, p := dockedEngine(t)
	other, _ := w.Join(Record{ID: "other-moving", Pos: Vec{100, 100}})
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { close(entered); <-release; return savedStarter(p.ID, false, 1), nil })
	}()
	<-entered
	activity := make(chan struct{})
	go func() {
		w.SetMove(other.ID, Vec{1, 0})
		w.DamagePlayer(p.ID, 10)
		w.Step(0.05, time.Now())
		w.Snapshot(p.ID)
		close(activity)
	}()
	select {
	case <-activity:
	case <-time.After(time.Second):
		close(release)
		<-done
		<-activity
		t.Fatal("slow equipment store blocks simulation/snapshot/other player/damage")
	}
	st, _ := w.Snapshot(p.ID)
	if st.Time <= 0 || st.Ship.Shield >= st.Ship.ShieldMax {
		close(release)
		<-done
		t.Fatal("simulation paused or actor became invulnerable")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestEquipmentReservationGuardsOnlyItsActor(t *testing.T) {
	w, p := dockedEngine(t)
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { close(entered); <-release; return savedStarter(p.ID, false, 1), nil })
	}()
	<-entered
	defer func() {
		close(release)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	st, _ := w.Snapshot(p.ID)
	if !st.Ship.Equipment.Pending || st.Ship.Equipment.CanChange || st.Ship.Equipment.Reason != ErrEquipmentBusy.Error() {
		t.Fatal("pending status missing")
	}
	if w.SetMove(p.ID, Vec{1, 0}) || w.SetAutopilot(p.ID, true) {
		t.Fatal("reserved actor can leave dock")
	}
	for _, ab := range []string{"M01", "boost", "intercept", "scan"} {
		if err := w.UseAbility(p.ID, ab); err != ErrEquipmentBusy {
			t.Fatalf("cast %s while pending: %v", ab, err)
		}
	}
	if err := w.SetAttack(p.ID, true); err != ErrEquipmentBusy {
		t.Fatal(err)
	}
	if err := w.StartMining(p.ID, "any"); err != ErrEquipmentBusy {
		t.Fatal(err)
	}
	if _, err := w.SetShip(p.ID, "TMP-MINE"); err != ErrEquipmentBusy {
		t.Fatal(err)
	}
	if _, err := w.SetSpec(p.ID, "scout"); err != ErrEquipmentBusy {
		t.Fatal(err)
	}
	if _, ok := w.Remove(p.ID); ok {
		t.Fatal("reserved actor removed")
	}
	if _, ok := w.Join(Record{ID: p.ID}); ok {
		t.Fatal("uncoordinated reconnect accepted")
	}
	if w.RestoreStarter(p.ID, savedStarter(p.ID, true, 99)) {
		t.Fatal("reload raced pending commit")
	}
	if err := w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { t.Error("duplicate callback"); return StarterLoadout{}, nil }); err != ErrEquipmentBusy {
		t.Fatal(err)
	}
	other, ok := w.Join(Record{ID: "independent"})
	if !ok || !w.SetMove(other.ID, Vec{1, 0}) {
		t.Fatal("other player blocked")
	}
}

func TestEquipmentReservationReleasesOnPanic(t *testing.T) {
	w, p := dockedEngine(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("callback did not panic")
			}
		}()
		_ = w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { panic("test callback") })
	}()
	st, _ := w.Snapshot(p.ID)
	if st.Ship.Equipment.Pending || st.Ship.Equipment.Revision != nil || st.Ship.Modules[0].Equipped {
		t.Fatal("panic leaked reservation or kept ambiguous boost enabled")
	}
	if !w.SetMove(p.ID, Vec{1, 0}) {
		t.Fatal("actor remained locked")
	}
}

func TestEquipmentReservationRetainsDisconnectedActorUntilResolved(t *testing.T) {
	w, p := dockedEngine(t)
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { close(entered); <-release; return savedStarter(p.ID, false, 1), nil })
	}()
	<-entered
	w.Disconnect(p.ID, time.Now().Add(-Grace-time.Second))
	w.Step(0.05, time.Now())
	_, present := w.Snapshot(p.ID)
	close(release)
	err := <-done
	if !present || err != nil {
		t.Fatalf("pruned pending actor: present=%v err=%v", present, err)
	}
	removed := w.Step(0.05, time.Now())
	if len(removed) != 1 || removed[0].StarterLoadout.Equipped {
		t.Fatal("normal cleanup did not retain committed state")
	}
}

func TestEquipmentReservationBlocksRouterTransfer(t *testing.T) {
	w, p := dockedEngine(t)
	r := NewRouter(w)
	r.Add(w)
	r.byPlayer[p.ID] = w
	to := New()
	to.system.ID = "other"
	r.Add(to)
	r.SetGates(w.System().ID, []Gate{{To: "other", Pos: p.Pos}})
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- w.ChangeStarter(p.ID, 0, false, func() (StarterLoadout, error) { close(entered); <-release; return savedStarter(p.ID, false, 1), nil })
	}()
	<-entered
	_, _, jumpErr := r.Jump(p.ID, "other")
	r.Leave(p.ID)
	bound := r.For(p.ID)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if jumpErr != ErrEquipmentBusy || bound != w {
		t.Fatalf("pending transfer lost actor: jump=%v bound=%p", jumpErr, bound)
	}
	if _, _, err := r.Jump(p.ID, "other"); err != nil {
		t.Fatal(err)
	}
	st, _ := r.Snapshot(p.ID)
	if st.Ship.Modules[0].Equipped || *st.Ship.Equipment.Revision != 1 {
		t.Fatal("post-commit jump reset equipment")
	}
}
