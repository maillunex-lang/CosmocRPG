package shipxp

import (
	"context"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func suites() map[string]func(t *testing.T) (*Service, string) {
	return map[string]func(t *testing.T) (*Service, string){
		"память": func(*testing.T) (*Service, string) {
			return NewService(NewMemStore()), "p1"
		},
		"postgres": func(t *testing.T) (*Service, string) {
			pool := testdb.Pool(t)
			ctx := context.Background()
			if _, err := db.Migrate(ctx, pool); err != nil {
				t.Fatal(err)
			}
			tok := "shipxptokentoken01"
			if _, err := world.NewPGStore(pool).Resolve(ctx, tok, "Корабельщик"); err != nil {
				t.Fatal(err)
			}
			return NewService(NewPGStore(pool)), world.PublicID(world.TokenHash(tok))
		},
	}
}

func TestShipLevelsIndependentAndIdempotent(t *testing.T) {
	for name, mk := range suites() {
		t.Run(name, func(t *testing.T) {
			s, pid := mk(t)
			ctx := context.Background()
			st, _ := s.State(ctx, pid, "FTR-001")
			if st.Level != 1 || st.XP != 0 || st.MaxLevel != 50 {
				t.Fatalf("новый корабль: %+v", st)
			}
			g, err := s.Grant(ctx, pid, "FTR-001", "sxp:a", "kill", 200)
			if err != nil || !g.Granted || !g.LevelUp || g.State.Level != 2 {
				t.Fatalf("первое начисление: %+v %v", g, err)
			}
			if g, _ = s.Grant(ctx, pid, "FTR-001", "sxp:a", "kill", 200); g.Granted || g.State.XP != 200 {
				t.Fatalf("повтор ключа: %+v", g)
			}
			// другой корабль: свой уровень, первый не меняется
			if g, _ = s.Grant(ctx, pid, "TMP-MINE", "sxp:b", "sell", 10); !g.Granted || g.State.Level != 1 || g.State.XP != 10 {
				t.Fatalf("второй корабль: %+v", g)
			}
			all, _ := s.All(ctx, pid)
			if all["FTR-001"].Level != 2 || all["TMP-MINE"].Level != 1 {
				t.Fatalf("уровни при смене корабля не сохранились: %+v", all)
			}
			g, _ = s.Grant(ctx, pid, "FTR-001", "sxp:huge", "kill", 1<<40)
			if g.State.Level != 50 || g.State.ToNext != 0 {
				t.Fatalf("потолок уровня: %+v", g.State)
			}
			if g, _ = s.Grant(ctx, pid, "FTR-001", "zero", "kill", 0); g.Granted {
				t.Fatal("нулевой опыт не начисляется")
			}
		})
	}
}

func TestConcurrentGrantOnce(t *testing.T) {
	for name, mk := range suites() {
		t.Run(name, func(t *testing.T) {
			s, pid := mk(t)
			var wg sync.WaitGroup
			var mu sync.Mutex
			n := 0
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					g, err := s.Grant(context.Background(), pid, "FTR-001", "same", "kill", 30)
					if err != nil {
						t.Error(err)
						return
					}
					if g.Granted {
						mu.Lock()
						n++
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			st, _ := s.State(context.Background(), pid, "FTR-001")
			if n != 1 || st.XP != 30 {
				t.Fatalf("начислено %d раз, опыт %d", n, st.XP)
			}
		})
	}
}

func TestCurveMatches(t *testing.T) {
	s := NewService(NewMemStore())
	for l := 1; l <= rpg.DefaultShipCurve.MaxLevel; l++ {
		if st := s.build("X", rpg.DefaultShipCurve.TotalXP(l)); st.Level != l {
			t.Fatalf("уровень %d: %+v", l, st)
		}
	}
}

func TestBackfillFromShipType(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	pid := world.PublicID(world.TokenHash("backfilltokentok001"))
	if _, err := world.NewPGStore(pool).Resolve(ctx, "backfilltokentok001", "Старожил"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE players SET ship_type = 'FRG-001' WHERE public_id = $1`, pid); err != nil {
		t.Fatal(err)
	}
	// повторное выполнение заполнения безопасно и создаёт экземпляр текущего корабля
	if _, err := pool.Exec(ctx, `INSERT INTO player_ships (player_id, ship_type) SELECT public_id, ship_type FROM players ON CONFLICT (player_id, ship_type) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM player_ships WHERE player_id = $1 AND ship_type = 'FRG-001' AND ship_xp = 0`, pid).Scan(&n); err != nil || n != 1 {
		t.Fatalf("экземпляр текущего корабля: n=%d err=%v", n, err)
	}
}
