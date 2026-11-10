package world

import (
	"errors"
	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
	"testing"
	"time"
)

func TestPurchaseReservationDoesNotBlockWorldOrDamage(t *testing.T) {
	w, _ := livingWorld(t)
	pos, _ := w.ObjectPos(StationID)
	p, _ := w.Join(Record{ID: "buyer", Pos: pos, ShipType: "TMP-MINE"})
	other, _ := w.Join(Record{ID: "other", Pos: Vec{100, 100}})
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := w.PurchaseResource(p.ID, "op", "gas", 1, func(r economy.Request) (economy.Result, error) {
			close(entered)
			<-release
			_, spent, c, err := economy.Prepare(r)
			return economy.Result{Known: true, Found: true, Cargo: &c, Spent: spent, Credits: 100 - spent}, err
		})
		done <- err
	}()
	<-entered
	activity := make(chan struct{})
	go func() {
		w.SetMove(other.ID, Vec{1, 0})
		w.DamagePlayer(p.ID, 10)
		w.Step(.05, time.Now())
		w.Snapshot(other.ID)
		close(activity)
	}()
	select {
	case <-activity:
	case <-time.After(time.Second):
		close(release)
		<-done
		t.Fatal("SQL callback blocks world")
	}
	st, _ := w.Snapshot(p.ID)
	if st.Ship.Shield >= st.Ship.ShieldMax {
		close(release)
		<-done
		t.Fatal("purchase grants invulnerability")
	}
	if w.SetMove(p.ID, Vec{1, 0}) {
		close(release)
		<-done
		t.Fatal("buyer moved while pending")
	}
	for _, r := range w.Records() {
		if r.ID == p.ID && !r.SkipCargo {
			close(release)
			<-done
			t.Fatal("speculative autosave allowed")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	after, _ := w.Snapshot(p.ID)
	if after.Ship.Hold["gas"] != 1 || after.Ship.Shield >= after.Ship.ShieldMax {
		t.Fatal(after)
	}
}
func TestPurchaseUnknownOutcomeRequiresBarrier(t *testing.T) {
	w, _ := livingWorld(t)
	pos, _ := w.ObjectPos(StationID)
	w.Join(Record{ID: "p", Pos: pos, ShipType: "TMP-MINE"})
	_, err := w.PurchaseResource("p", "op", "gas", 1, func(economy.Request) (economy.Result, error) { return economy.Result{}, economy.ErrUnavailable })
	if !errors.Is(err, economy.ErrUnavailable) || w.SetMove("p", Vec{1, 0}) {
		t.Fatal(err)
	}
	records := w.Records()
	if len(records) != 1 || !records[0].SkipCargo {
		t.Fatal(records)
	}
	if _, ok := w.Join(Record{ID: "p"}); ok {
		t.Fatal("unconfirmed join cleared uncertainty")
	}
	if _, ok := w.Join(Record{ID: "p", CargoConfirmed: true, CargoRevision: 1, Hold: map[string]float64{"gas": 1}}); !ok {
		t.Fatal("barrier did not reconcile")
	}
	if !w.SetMove("p", Vec{1, 0}) {
		t.Fatal("actor remains frozen")
	}
}
