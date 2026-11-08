package shop

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func pgShop(t *testing.T) (*Service, string, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	rec, err := world.NewPGStore(pool).Resolve(ctx, "shoptokentoken00001", "Покупатель")
	if err != nil {
		t.Fatal(err)
	}
	return NewService(NewPGStore(pool)), rec.ID, pool
}

func TestPGShopFlow(t *testing.T) {
	svc, pid, _ := pgShop(t)
	ctx := context.Background()
	if _, err := svc.Buy(ctx, pid, "engine_gold", "req-pg-0000001"); err != ErrVipNeeded {
		t.Fatalf("без VIP: %v", err)
	}
	st, err := svc.Buy(ctx, pid, "vip_1", "req-pg-0000002")
	if err != nil || st.VipLevel != 1 {
		t.Fatalf("%v %+v", err, st)
	}
	if _, err := svc.Buy(ctx, pid, "badge_star", "req-pg-0000003"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Buy(ctx, pid, "badge_star", "req-pg-0000004"); err != ErrOwned {
		t.Fatalf("повторная покупка: %v", err)
	}
	st, err = svc.Equip(ctx, pid, "badge", "badge_star")
	if err != nil || st.Equipped["badge"] != "badge_star" {
		t.Fatalf("%v %+v", err, st)
	}
	st, _ = svc.State(ctx, pid)
	if len(st.Owned) != 1 || st.VipLevel != 1 {
		t.Fatalf("итоговое состояние: %+v", st)
	}
}

func TestPGShopConcurrentSameRequest(t *testing.T) {
	svc, pid, pool := pgShop(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Buy(ctx, pid, "vip_1", "req-pg-same-0001")
		}()
	}
	wg.Wait()
	st, _ := svc.State(ctx, pid)
	// один запрос продлевает VIP один раз: не больше 30 дней вперёд
	if st.VipLevel != 1 || st.VipUntil == nil {
		t.Fatalf("%+v", st)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop_purchases WHERE player_id = $1 AND request_id = 'req-pg-same-0001'`, pid).Scan(&n)
	if n != 1 {
		t.Fatalf("записей покупки %d", n)
	}
}
