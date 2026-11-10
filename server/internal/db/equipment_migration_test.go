package db

import (
	"bytes"
	"context"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestEquipmentMigrationPreservesLegacyPlayerAndXP(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	migrations, err := loadMigrations(migrationsFS)
	if err != nil {
		t.Fatal(err)
	}
	var equipmentSQL string
	for _, m := range migrations {
		if m.name == "0020_equipment_storage.sql" {
			equipmentSQL = m.sql
			break
		}
		if _, err := pool.Exec(ctx, m.sql); err != nil {
			t.Fatalf("legacy %s: %v", m.name, err)
		}
	}
	if equipmentSQL == "" {
		t.Fatal("equipment migration missing")
	}
	// Старую схему заполняем старым SQL, а не runtime, который уже требует
	// новые столбцы. Сравниваем все поля legacy row непосредственно до/после0020.
	hash := world.TokenHash("equipment-migration-legacy-01")
	pid := world.PublicID(hash)
	if _, err := pool.Exec(ctx, `INSERT INTO players(token_hash,public_id,name) VALUES($1,$2,'Legacy Pilot')`, hash[:], pid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE players SET pilot_xp=1234 WHERE public_id=$1`, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO player_ships(player_id,ship_type,ship_xp) VALUES($1,'FTR-001',567) ON CONFLICT(player_id,ship_type) DO UPDATE SET ship_xp=567`, pid); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(p) FROM players p WHERE public_id=$1`, pid).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, equipmentSQL); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(p) FROM players p WHERE public_id=$1`, pid).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("legacy row changed: before=%s after=%s", before, after)
	}
	var pilotXP, shipXP int64
	if err := pool.QueryRow(ctx, `SELECT pilot_xp FROM players WHERE public_id=$1`, pid).Scan(&pilotXP); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT ship_xp FROM player_ships WHERE player_id=$1 AND ship_type='FTR-001'`, pid).Scan(&shipXP); err != nil {
		t.Fatal(err)
	}
	if pilotXP != 1234 || shipXP != 567 {
		t.Fatalf("legacy XP changed: %d/%d", pilotXP, shipXP)
	}
	for _, table := range []string{"player_module_items", "player_ship_loadouts", "player_ship_loadout_slots"} {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("migration implicitly issued equipment: %s %d %v", table, n, err)
		}
	}
}
