package equipment

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPGSlotConstraintsPreventTheftAndDoubleFit(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	alice, err := f.store.Ensure(ctx, f.owners[0])
	if err != nil {
		t.Fatal(err)
	}
	bob, err := f.store.Ensure(ctx, f.owners[1])
	if err != nil {
		t.Fatal(err)
	}
	// Keep Alice's item unfitted: otherwise the unique-instance index fires
	// before the ownership FK and the test would not exercise that FK.
	alice, err = f.store.SetEngine(ctx, alice.OwnerID, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ owner, ship, slot, instance, code string }{
		{bob.OwnerID, ShipType, "tech-2", alice.InstanceID, "23503"},
		{bob.OwnerID, ShipType, "tech-2", bob.InstanceID, "23505"},
		{alice.OwnerID, "FTR-002", "tech-1", alice.InstanceID, "23503"},
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO player_ship_loadouts(owner_id,ship_type) VALUES($1,'FTR-002')`, alice.OwnerID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		_, err := f.pool.Exec(ctx, `INSERT INTO player_ship_loadout_slots(owner_id,ship_type,slot_id,instance_id) VALUES($1,$2,$3,$4)`, tc.owner, tc.ship, tc.slot, tc.instance)
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != tc.code {
			t.Fatalf("constraint %s: %v", tc.code, err)
		}
	}
	requireUnchanged(t, f.store, alice)
	requireUnchanged(t, f.store, bob)
}

func TestPGFailedRevisionWriteRollsBackSlotDeletion(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	before, err := f.store.Ensure(ctx, f.owners[0])
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.pool.Exec(ctx, `CREATE FUNCTION reject_equipment_revision() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'equipment test failure'; END $$;
 CREATE TRIGGER fail_revision BEFORE UPDATE ON player_ship_loadouts FOR EACH ROW EXECUTE FUNCTION reject_equipment_revision()`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.store.SetEngine(ctx, before.OwnerID, 0, false)
	if err == nil || got != (StarterState{}) {
		t.Fatalf("injected failure accepted or returned partial state: %+v %v", got, err)
	}
	requireUnchanged(t, f.store, before)
	var slots int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM player_ship_loadout_slots WHERE owner_id=$1`, before.OwnerID).Scan(&slots); err != nil || slots != 1 {
		t.Fatalf("slot deletion escaped rollback: %d %v", slots, err)
	}
}

func TestPGOldAutosaveCannotResetEquipment(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	old, err := f.players.Resolve(ctx, "equipment-owner-a-0001", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Ensure(ctx, old.ID); err != nil {
		t.Fatal(err)
	}
	removed, err := f.store.SetEngine(ctx, old.ID, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	old.Pos.X = 10
	if err := f.players.Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	requireUnchanged(t, NewPGStore(f.pool), removed)
}

func TestPGCorruptExistingGrantFailsWithoutRegrant(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	state, err := f.store.Ensure(ctx, f.owners[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE player_module_items SET prototype_id='WRONG-PROTOTYPE' WHERE instance_id=$1`, state.InstanceID); err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() (StarterState, error){
		func() (StarterState, error) { return f.store.Load(ctx, state.OwnerID) },
		func() (StarterState, error) { return f.store.Ensure(ctx, state.OwnerID) },
		func() (StarterState, error) { return f.store.SetEngine(ctx, state.OwnerID, 0, false) },
	} {
		got, err := call()
		if !errors.Is(err, ErrInvalidState) || got != (StarterState{}) {
			t.Fatalf("corrupt state accepted: %+v %v", got, err)
		}
	}
	var items int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM player_module_items WHERE owner_id=$1`, state.OwnerID).Scan(&items); err != nil || items != 1 {
		t.Fatalf("corrupt grant duplicated: %d %v", items, err)
	}
}

func TestPGDeletingOwnerCascadesEquipmentOnly(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	a, err := f.store.Ensure(ctx, f.owners[0])
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.store.Ensure(ctx, f.owners[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM players WHERE public_id=$1`, a.OwnerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Load(ctx, a.OwnerID); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("deleted owner loadout survived: %v", err)
	}
	requireUnchanged(t, f.store, b)
	for _, table := range []string{"player_module_items", "player_ship_loadouts", "player_ship_loadout_slots"} {
		var n int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE owner_id=$1`, a.OwnerID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("orphan %s: %d %v", table, n, err)
		}
	}
}

func TestPGCannotDeleteFittedItemWithoutUnfitting(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	before, err := f.store.Ensure(ctx, f.owners[0])
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.pool.Exec(ctx, `DELETE FROM player_module_items WHERE instance_id=$1`, before.InstanceID)
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "23503" {
		t.Fatalf("fitted item deleted: %v", err)
	}
	requireUnchanged(t, f.store, before)
}

func TestPGStarterPreservesAdditionalSlots(t *testing.T) {
	f := makeFixture(t, "postgres")
	ctx := context.Background()
	before, err := f.store.Ensure(ctx, f.owners[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO player_module_items(instance_id,owner_id,ship_type,prototype_id,grant_key) VALUES('extra-module',$1,$2,'EXTRA-PROTOTYPE','extra-grant')`, before.OwnerID, ShipType); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO player_ship_loadout_slots(owner_id,ship_type,slot_id,instance_id) VALUES($1,$2,'tech-2','extra-module')`, before.OwnerID, ShipType); err != nil {
		t.Fatal(err)
	}
	// Адаптер стартового двигателя обслуживает только свой слот. Остальные
	// модули принадлежат GeneralPGStore и не должны исчезать при reconnect.
	if got, err := f.store.Ensure(ctx, before.OwnerID); err != nil || got != before {
		t.Fatalf("starter bootstrap changed with additional module: %+v %v", got, err)
	}
	removed, err := f.store.SetEngine(ctx, before.OwnerID, 0, false)
	if err != nil || removed.Equipped || removed.Revision != 1 {
		t.Fatalf("starter engine removal failed: %+v %v", removed, err)
	}
	var retained string
	if err := f.pool.QueryRow(ctx, `SELECT instance_id FROM player_ship_loadout_slots WHERE owner_id=$1 AND slot_id='tech-2'`, before.OwnerID).Scan(&retained); err != nil || retained != "extra-module" {
		t.Fatalf("additional module lost: %q %v", retained, err)
	}
	if _, err := f.store.SetEngine(ctx, before.OwnerID, 1, true); err != nil {
		t.Fatal(err)
	}
	var count int
	var revision int64
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM player_ship_loadout_slots WHERE owner_id=$1`, before.OwnerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2`, before.OwnerID, ShipType).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if count != 2 || revision != 2 {
		t.Fatalf("additional slots lost or revision incorrect: %d/%d", count, revision)
	}
}
