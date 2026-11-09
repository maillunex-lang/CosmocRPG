package ws

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type delayedEquipment struct {
	equipment.StarterStore
	owner            string
	entered, release chan struct{}
	once             sync.Once
}

func (d *delayedEquipment) SetEngine(ctx context.Context, id string, revision int64, on bool) (equipment.StarterState, error) {
	if id == d.owner {
		d.once.Do(func() { close(d.entered) })
		select {
		case <-d.release:
		case <-ctx.Done():
			return equipment.StarterState{}, ctx.Err()
		}
	}
	return d.StarterStore.SetEngine(ctx, id, revision, on)
}

func TestEquipmentSlowStoreDoesNotBlockOtherSessions(t *testing.T) {
	tokens := []string{strings.Repeat("slow-a", 4), strings.Repeat("fast-b", 4), strings.Repeat("new-c", 4)}
	ids := make([]string, len(tokens))
	for i, token := range tokens {
		ids[i] = world.PublicID(world.TokenHash(token))
	}
	gear := &delayedEquipment{StarterStore: equipment.NewMemStore(ids...), owner: ids[0], entered: make(chan struct{}), release: make(chan struct{})}
	r := testRouter()
	srv, h := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: r, Store: world.NewMemStore(), Equipment: gear})
	a, b := connect(t, srv.URL), connect(t, srv.URL)
	a.join(tokens[0])
	b.join(tokens[1])
	a.send("equipment_set", map[string]any{"revision": 0, "equipped": false})
	<-gear.entered
	activity := make(chan struct{})
	go func() {
		defer close(activity)
		b.send("move", map[string]any{"x": 1, "y": 0})
		b.send("ships", map[string]any{})
		b.next("ships")
		h.Tick(2, time.Now())
		c := connect(t, srv.URL)
		c.join(tokens[2])
	}()
	select {
	case <-activity:
	case <-time.After(time.Second):
		close(gear.release)
		<-activity
		t.Fatal("slow equipment IO holds global session mutex/Tick/join")
	}
	st, _ := r.Snapshot(ids[0])
	if !st.Ship.Equipment.Pending || st.Time <= 0 {
		close(gear.release)
		t.Fatal("IO did not overlap simulation")
	}
	close(gear.release)
	a.next("equipment")
	s, err := gear.Load(context.Background(), ids[0])
	if err != nil || s.Equipped || s.Revision != 1 {
		t.Fatalf("commit not applied: %+v %v", s, err)
	}
}

func TestEquipmentReconnectWaitsForCommittedLoadout(t *testing.T) {
	token := strings.Repeat("replace", 4)
	id := world.PublicID(world.TokenHash(token))
	gear := &delayedEquipment{StarterStore: equipment.NewMemStore(id), owner: id, entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(gear.release) }) }
	defer unblock()
	r := testRouter()
	srv, h := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: r, Store: world.NewMemStore(), Equipment: gear})
	a := connect(t, srv.URL)
	a.join(token)
	a.send("equipment_set", map[string]any{"revision": 0, "equipped": false})
	<-gear.entered
	b := connect(t, srv.URL)
	b.send("join", map[string]any{"token": token, "protocol": ProtocolVersion})
	waitPlayerWaiter(t, h, id)
	// The old accepted operation still owns this actor; new bootstrap cannot
	// adopt its pre-commit revision or replace the session halfway through IO.
	st, _ := r.Snapshot(id)
	if !st.Ship.Equipment.Pending {
		t.Fatal("reservation lost on competing join")
	}
	unblock()
	a.next("equipment")
	b.next("welcome")
	st, _ = r.Snapshot(id)
	if st.Ship.Equipment.Pending || st.Ship.Modules[0].Equipped || *st.Ship.Equipment.Revision != 1 {
		t.Fatal("reconnect reset or lost committed loadout")
	}
	b.send("equipment_set", map[string]any{"revision": 1, "equipped": true})
	b.next("equipment")
	s, err := gear.Load(context.Background(), id)
	if err != nil || s.Revision != 2 || !s.Equipped {
		t.Fatalf("replacement cannot continue CAS: %+v %v", s, err)
	}
}
