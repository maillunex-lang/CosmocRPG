package ws

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type equipmentFault struct {
	equipment.StarterStore
	mode atomic.Int32 // 1 rollback; 2 lost commit reply; 3 unknown outcome+failed reload
}

func (f *equipmentFault) SetEngine(ctx context.Context, id string, rev int64, on bool) (equipment.StarterState, error) {
	mode := f.mode.Load()
	if mode == 0 {
		return f.StarterStore.SetEngine(ctx, id, rev, on)
	}
	if mode == 2 {
		if _, err := f.StarterStore.SetEngine(ctx, id, rev, on); err != nil {
			return equipment.StarterState{}, err
		}
	}
	return equipment.StarterState{}, errors.New("injected storage error")
}
func (f *equipmentFault) Load(ctx context.Context, id string) (equipment.StarterState, error) {
	if f.mode.Load() == 3 {
		return equipment.StarterState{}, errors.New("injected read error")
	}
	return f.StarterStore.Load(ctx, id)
}

func TestEquipmentPGCommandAuthorizationCASAndRecovery(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	players := world.NewPGStore(pool)
	token := strings.Repeat("cmd", 8)
	rec, err := players.Resolve(ctx, token, "Docked test")
	if err != nil {
		t.Fatal(err)
	}
	r := testRouter()
	station, _ := r.Primary().ObjectPos(world.StationID)
	rec.Pos = station
	if err := players.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	gear := equipment.NewPGStore(pool)
	fault := &equipmentFault{StarterStore: gear}
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: r, Store: players, Equipment: fault})
	cl := connect(t, srv.URL)
	cl.send("equipment_set", map[string]any{"revision": 0, "equipped": false})
	if code := errCode(t, cl.next("error")); code != "not_joined" {
		t.Fatal(code)
	}
	cl.join(token)
	// Malformed/foreign selectors are not accepted as equipment authority.
	for _, payload := range []any{
		map[string]any{"equipped": false}, map[string]any{"revision": 0},
		map[string]any{"revision": 0, "equipped": false, "owner_id": "other"},
		map[string]any{"revision": 0, "equipped": false, "instance_id": "fake"},
		map[string]any{"revision": -1, "equipped": false},
	} {
		cl.send("equipment_set", payload)
		if code := errCode(t, cl.next("error")); code != "bad_message" {
			t.Fatal(code)
		}
	}
	cl.send("equipment_set", map[string]any{"revision": 0, "equipped": false})
	cl.next("equipment")
	st, _ := r.Snapshot(rec.ID)
	if st.Ship.Modules[0].Equipped || *st.Ship.Equipment.Revision != 1 {
		t.Fatal("command did not apply commit")
	}
	cl.send("equipment_set", map[string]any{"revision": 0, "equipped": true})
	if code := errCode(t, cl.next("error")); code != "equipment_revision_conflict" {
		t.Fatal(code)
	}
	for _, ab := range []string{"M01", "boost"} {
		cl.send("ability", map[string]any{"id": ab})
		if code := errCode(t, cl.next("error")); code != "module_unavailable" {
			t.Fatal(code)
		}
	}
	fault.mode.Store(1)
	cl.send("equipment_set", map[string]any{"revision": 1, "equipped": true})
	if code := errCode(t, cl.next("error")); code != "equipment_unavailable" {
		t.Fatal(code)
	}
	st, _ = r.Snapshot(rec.ID)
	if st.Ship.Modules[0].Equipped || *st.Ship.Equipment.Revision != 1 {
		t.Fatal("rollback world divergence")
	}
	fault.mode.Store(2)
	cl.send("equipment_set", map[string]any{"revision": 1, "equipped": true})
	if code := errCode(t, cl.next("error")); code != "equipment_unavailable" {
		t.Fatal(code)
	}
	st, _ = r.Snapshot(rec.ID)
	if !st.Ship.Modules[0].Equipped || *st.Ship.Equipment.Revision != 2 {
		t.Fatal("lost reply did not reconcile committed state")
	}
	fault.mode.Store(3)
	cl.send("equipment_set", map[string]any{"revision": 2, "equipped": false})
	if code := errCode(t, cl.next("error")); code != "equipment_unavailable" {
		t.Fatal(code)
	}
	st, _ = r.Snapshot(rec.ID)
	if st.Ship.Modules[0].Equipped || st.Ship.Equipment.Revision != nil {
		t.Fatal("unknown outcome left boost enabled")
	}
	fault.mode.Store(0)
	cl.send("equipment_get", map[string]any{})
	cl.next("equipment")
	st, _ = r.Snapshot(rec.ID)
	if !st.Ship.Modules[0].Equipped || *st.Ship.Equipment.Revision != 2 {
		t.Fatal("explicit reload did not recover")
	}
	// Genuine movement intent denies the mutation before storage.
	r.For(rec.ID).SetMove(rec.ID, world.Vec{X: 1})
	cl.send("equipment_set", map[string]any{"revision": 2, "equipped": false})
	if code := errCode(t, cl.next("error")); code != "equipment_busy" {
		t.Fatal(code)
	}
	s, err := gear.Load(ctx, rec.ID)
	if err != nil || s.Revision != 2 || !s.Equipped {
		t.Fatal("rejected command changed SQL")
	}
}
