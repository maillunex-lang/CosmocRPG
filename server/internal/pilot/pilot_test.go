package pilot

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestXPFor(t *testing.T) {
	if XPFor("kill", 100) != 40 || XPFor("discover", 100) != 45 || XPFor("sell", 1000) != 50 || XPFor("quest", 500) != 100 {
		t.Fatal("неверный опыт по событиям")
	}
	if XPFor("sell", 10) != 0 || XPFor("неизвестно", 1000) != 0 || XPFor("kill", -5) != 15 {
		t.Fatal("граничные значения")
	}
}

func suites() map[string]func(t *testing.T) (*Service, string) {
	return map[string]func(t *testing.T) (*Service, string){
		"память": func(*testing.T) (*Service, string) { return NewService(NewMemStore()), "p1" },
		"postgres": func(t *testing.T) (*Service, string) {
			pool := testdb.Pool(t)
			ctx := context.Background()
			if _, err := db.Migrate(ctx, pool); err != nil {
				t.Fatal(err)
			}
			tok := "pilottokentoken0001"
			if _, err := world.NewPGStore(pool).Resolve(ctx, tok, "Пилот"); err != nil {
				t.Fatal(err)
			}
			return NewService(NewPGStore(pool)), world.PublicID(world.TokenHash(tok))
		},
	}
}

func TestGrantIdempotentAndLevels(t *testing.T) {
	for name, mk := range suites() {
		t.Run(name, func(t *testing.T) {
			s, pid := mk(t)
			ctx := context.Background()
			st, _ := s.State(ctx, pid)
			if st.Level != 1 || st.XP != 0 || st.AttrFree != 0 {
				t.Fatalf("новый игрок: %+v", st)
			}
			g, err := s.Grant(ctx, pid, "kill:a", "kill", 100)
			if err != nil || !g.Granted || g.Amount != 100 || g.State.XP != 100 || !g.LevelUp || g.State.Level != 2 {
				t.Fatalf("первое начисление: %+v %v", g, err)
			}
			g, _ = s.Grant(ctx, pid, "kill:a", "kill", 100)
			if g.Granted || g.State.XP != 100 || g.LevelUp {
				t.Fatalf("повтор ключа должен быть без эффекта: %+v", g)
			}
			g, _ = s.Grant(ctx, pid, "kill:b", "kill", 10)
			if !g.Granted || g.LevelUp || g.State.XP != 110 {
				t.Fatalf("второе событие: %+v", g)
			}
			if g, _ := s.Grant(ctx, pid, "zero", "kill", 0); g.Granted {
				t.Fatal("нулевой опыт не начисляется")
			}
			st, _ = s.State(ctx, pid)
			if st.AttrFree != 2 || st.TalentPoints != 1 || st.MaxLevel != 60 {
				t.Fatalf("очки на уровне 2: %+v", st)
			}
			g, _ = s.Grant(ctx, pid, "huge", "kill", 1<<40)
			if g.State.Level != 60 || g.State.ToNext != 0 {
				t.Fatalf("потолок уровня: %+v", g.State)
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
			granted := 0
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					g, err := s.Grant(context.Background(), pid, "same", "kill", 50)
					if err != nil {
						t.Error(err)
						return
					}
					if g.Granted {
						mu.Lock()
						granted++
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			st, _ := s.State(context.Background(), pid)
			if granted != 1 || st.XP != 50 {
				t.Fatalf("начислено %d раз, опыт %d", granted, st.XP)
			}
		})
	}
}

func TestSpend(t *testing.T) {
	for name, mk := range suites() {
		t.Run(name, func(t *testing.T) {
			s, pid := mk(t)
			ctx := context.Background()
			if _, err := s.Spend(ctx, pid, "piloting", 1); !errors.Is(err, ErrNoPoints) {
				t.Fatalf("без очков: %v", err)
			}
			s.Grant(ctx, pid, "k", "kill", rpg.DefaultPilotCurve.TotalXP(6)) // уровень 6: 10 очков
			if _, err := s.Spend(ctx, pid, "bogus", 1); !errors.Is(err, ErrBadAttr) {
				t.Fatalf("неизвестная характеристика: %v", err)
			}
			for _, n := range []int{0, -1, MaxPerAttribute + 1} {
				if _, err := s.Spend(ctx, pid, "piloting", n); !errors.Is(err, ErrBadCount) {
					t.Fatalf("число %d: %v", n, err)
				}
			}
			st, err := s.Spend(ctx, pid, "piloting", 4)
			if err != nil || st.Attributes["piloting"] != 4 || st.AttrFree != 6 || st.AttrSpent != 4 {
				t.Fatalf("трата: %+v %v", st, err)
			}
			if _, err := s.Spend(ctx, pid, "tactics", 7); !errors.Is(err, ErrNoPoints) {
				t.Fatalf("больше свободных: %v", err)
			}
			st, _ = s.State(ctx, pid)
			if st.AttrFree != 6 || st.Attributes["tactics"] != 0 {
				t.Fatalf("отказ не должен менять состояние: %+v", st)
			}
			if _, err := s.Spend(ctx, pid, "tactics", 6); err != nil {
				t.Fatal(err)
			}
			if st, _ = s.State(ctx, pid); st.AttrFree != 0 {
				t.Fatalf("очки потрачены: %+v", st)
			}
		})
	}
}

func TestSpendConcurrentNeverExceedsPoints(t *testing.T) {
	for name, mk := range suites() {
		t.Run(name, func(t *testing.T) {
			s, pid := mk(t)
			ctx := context.Background()
			s.Grant(ctx, pid, "k", "kill", rpg.DefaultPilotCurve.TotalXP(4)) // 6 очков
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); _, _ = s.Spend(ctx, pid, "logistics", 1) }()
			}
			wg.Wait()
			st, _ := s.State(ctx, pid)
			if st.AttrSpent != 6 || st.AttrFree != 0 {
				t.Fatalf("потрачено %d при 6 доступных: %+v", st.AttrSpent, st)
			}
		})
	}
}
