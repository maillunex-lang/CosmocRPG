package world

import (
	"context"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

func TestPGStorePersistsPosition(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	a, err := s.Resolve(ctx, "pgtokentokentoken1", "Тест")
	if err != nil || a.Name != "Тест" {
		t.Fatalf("создание: %v %+v", err, a)
	}
	a.Pos = Vec{12.5, -7.25}
	a.Dir = Vec{0, 1}
	if err := s.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	b, err := s.Resolve(ctx, "pgtokentokentoken1", "Другое")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != a.ID || b.Name != "Тест" || b.Pos != (Vec{12.5, -7.25}) || b.Dir != (Vec{0, 1}) {
		t.Fatalf("после перезахода: %+v", b)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM players WHERE token_hash = $1", func() []byte { h := TokenHash("pgtokentokentoken1"); return h[:] }()).Scan(&n); err != nil || n != 1 {
		t.Fatalf("в базе должна быть одна запись, найдено %d (%v)", n, err)
	}
	var raw int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM players WHERE encode(token_hash,'escape') LIKE '%pgtoken%'").Scan(&raw)
	if raw != 0 {
		t.Fatal("токен не должен храниться в открытом виде")
	}
}
