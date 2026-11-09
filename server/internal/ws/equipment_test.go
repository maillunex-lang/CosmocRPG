package ws

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type failedEquipment struct{ equipment.StarterStore }

func (failedEquipment) Ensure(context.Context, string) (equipment.StarterState, error) {
	return equipment.StarterState{}, errors.New("test storage failure")
}

func TestEquipmentJoinFailsClosed(t *testing.T) {
	r := testRouter()
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: r, Store: world.NewMemStore(), Equipment: failedEquipment{}})
	cl := connect(t, srv.URL)
	cl.send("join", map[string]any{"token": strings.Repeat("f", 24), "protocol": ProtocolVersion})
	if code := errCode(t, cl.next("error")); code != "equipment_unavailable" {
		t.Fatal(code)
	}
	if r.Count() != 0 {
		t.Fatal("failed load entered world")
	}
}

func TestEquipmentPGJoinReconnectAndRestart(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	players := world.NewPGStore(pool)
	token := strings.Repeat("e", 24)
	rec, err := players.Resolve(ctx, token, "Saved engine")
	if err != nil {
		t.Fatal(err)
	}
	gear := equipment.NewPGStore(pool)
	s, err := gear.Ensure(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	s, err = gear.SetEngine(ctx, rec.ID, s.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	// Reconnect to an already-live ship and a new server/router instance.
	for _, restart := range []bool{false, true} {
		t.Run(map[bool]string{false: "live reconnect", true: "fresh server"}[restart], func(t *testing.T) {
			r := testRouter()
			srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: r, Store: players, Equipment: equipment.NewPGStore(pool)})
			for i := 0; i < 2; i++ {
				cl := connect(t, srv.URL)
				cl.join(token)
				st, _ := r.Snapshot(rec.ID)
				if len(st.Ship.Modules) != 1 || st.Ship.Modules[0].Equipped {
					t.Fatal("persisted engine reset on join")
				}
				for _, ab := range []string{"boost", "M01"} {
					cl.send("ability", map[string]any{"id": ab})
					if code := errCode(t, cl.next("error")); code != "module_unavailable" {
						t.Fatal(code)
					}
				}
				cl.c.CloseNow()
			}
		})
	}
	if got, err := gear.Load(ctx, rec.ID); err != nil || got != s {
		t.Fatalf("join changed SQL state: %+v %v", got, err)
	}
}
