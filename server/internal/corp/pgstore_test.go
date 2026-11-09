package corp

import (
	"context"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

func TestPGStoreCorporationFlow(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	now := time.Now()

	mustPlayer := func(id, name string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO players (public_id, token_hash, name, character_created) VALUES ($1, $1, $2, true) ON CONFLICT (public_id) DO NOTHING`, id, name); err != nil {
			t.Fatal(err)
		}
	}
	mustPlayer("cp1", "Основатель")
	mustPlayer("cp2", "Соратник")

	c := Corporation{ID: "co-tst", Name: "Тестовый Путь", Tag: "TP", Owner: "cp1", Created: now}
	if err := st.Create(ctx, c, "Основатель"); err != nil {
		t.Fatal(err)
	}
	if taken, _ := st.Exists(ctx, "тестовый путь", "TP"); !taken {
		t.Fatal("имя/тег должны быть заняты")
	}
	if _, ok, err := st.CorporationByPlayer(ctx, "cp1"); err != nil || !ok {
		t.Fatalf("корпорация основателя: %v", err)
	}
	joined, err := st.Join(ctx, "", "cp2", "tp") // по тегу в нижнем регистре
	if err != nil || !joined {
		t.Fatalf("вступление: %v", err)
	}
	mem, err := st.Members(ctx, "co-tst")
	if err != nil || len(mem) != 2 || mem[0].Role != "owner" || mem[0].Name != "Основатель" {
		t.Fatalf("состав: %+v %v", mem, err)
	}
	left, err := st.Leave(ctx, "cp2")
	if err != nil || !left {
		t.Fatalf("выход: %v", err)
	}
	if _, ok, _ := st.CorporationByPlayer(ctx, "cp2"); ok {
		t.Fatal("после выхода корпорации быть не должно")
	}
}
