package ws

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestEquipmentPostgresRowLockDoesNotBlockOtherPlayers(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	players := world.NewPGStore(pool)
	gear := equipment.NewPGStore(pool)
	r := testRouter()
	srv, h := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: r, Store: players, Equipment: gear})
	a, b := connect(t, srv.URL), connect(t, srv.URL)
	token := strings.Repeat("rowlock-a", 3)
	id := a.join(token)["player_id"].(string)
	b.join(strings.Repeat("rowlock-b", 3))
	// Hold a real PostgreSQL row lock; SetEngine must wait in the driver,
	// not a fake callback, while other players/ticks/join keep working.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2 FOR UPDATE`, id, equipment.ShipType).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	a.send("equipment_set", map[string]any{"revision": revision, "equipped": false})
	deadline := time.Now().Add(time.Second)
	pending := false
	for time.Now().Before(deadline) {
		st, _ := r.Snapshot(id)
		if st.Ship.Equipment.Pending {
			pending = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !pending {
		t.Fatal("SQL mutation never reserved its actor")
	}
	activity := make(chan struct{})
	go func() {
		defer close(activity)
		b.send("move", map[string]any{"x": 1, "y": 0})
		b.send("ships", map[string]any{})
		b.next("ships")
		h.Tick(2, time.Now())
		c := connect(t, srv.URL)
		c.join(strings.Repeat("rowlock-c", 3))
	}()
	select {
	case <-activity:
	case <-time.After(time.Second):
		_ = tx.Rollback(ctx)
		<-activity
		t.Fatal("PostgreSQL row wait blocked another player's command/tick/join")
	}
	st, _ := r.Snapshot(id)
	if !st.Ship.Equipment.Pending || st.Time <= 0 {
		t.Fatal("SQL lock did not overlap simulation")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	a.next("equipment")
	s, err := gear.Load(ctx, id)
	if err != nil || s.Equipped || s.Revision != revision+1 {
		t.Fatalf("row-locked write lost: %+v %v", s, err)
	}
}
