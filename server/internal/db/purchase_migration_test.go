package db

import (
	"bytes"
	"context"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"testing"
)

func TestPurchaseMigrationPreservesLegacyData(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	migrations, err := loadMigrations(migrationsFS)
	if err != nil {
		t.Fatal(err)
	}
	var purchaseSQL string
	for _, m := range migrations {
		if m.name == "0025_atomic_purchases.sql" {
			purchaseSQL = m.sql
			break
		}
		if _, err = pool.Exec(ctx, m.sql); err != nil {
			t.Fatal(m.name, err)
		}
	}
	if purchaseSQL == "" {
		t.Fatal("purchase migration missing")
	}
	hash := world.TokenHash("purchase-legacy-schema-token")
	pid := world.PublicID(hash)
	if _, err = pool.Exec(ctx, `INSERT INTO players(token_hash,public_id,name,hold,credits,pilot_xp) VALUES($1,$2,'Legacy', '{"iron_ore":7.5}'::jsonb,99,22)`, hash[:], pid); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	if err = pool.QueryRow(ctx, `SELECT to_jsonb(p) FROM players p WHERE public_id=$1`, pid).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, purchaseSQL); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT to_jsonb(p)-'cargo_revision' FROM players p WHERE public_id=$1`, pid).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("legacy changed: %s -> %s", before, after)
	}
	var rev, count int64
	if err = pool.QueryRow(ctx, `SELECT cargo_revision FROM players WHERE public_id=$1`, pid).Scan(&rev); err != nil || rev != 0 {
		t.Fatal(rev, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM resource_purchase_receipts`).Scan(&count); err != nil || count != 0 {
		t.Fatal("migration issued purchase", count, err)
	}
}
