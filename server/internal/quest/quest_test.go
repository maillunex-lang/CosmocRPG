package quest

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

var station = world.Vec{X: -12, Y: 14}

func newSvc(st Store) *Service {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(log, st, func(id string) (world.Vec, bool) { return station, id == world.StationID })
}

func TestContractFlowAndIdempotentReward(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	s := newSvc(st)
	if err := s.Join(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	v, _ := s.View("p1")
	if len(v.Quests) != 1 || v.Quests[0].ID != "first_flight" || v.Credits != 0 {
		t.Fatalf("после входа: %+v", v)
	}
	if got := s.Evaluate(ctx, "p1", world.Vec{X: 100, Y: 100}); len(got) != 0 {
		t.Fatal("далеко от станции контракт не выполняется")
	}
	got := s.Evaluate(ctx, "p1", station)
	if len(got) != 1 || got[0].Credits != 500 || got[0].Total != 500 {
		t.Fatalf("выполнение: %+v", got)
	}
	// повтор (то же сообщение, повторная проверка): награды нет
	if again := s.Evaluate(ctx, "p1", station); len(again) != 0 {
		t.Fatalf("повторная награда: %+v", again)
	}
	v, _ = s.View("p1")
	if len(v.Quests) != 0 || v.Credits != 500 {
		t.Fatalf("после выполнения: %+v", v)
	}
	// перезаход: контракт не выдаётся заново, кредиты на месте
	s.Leave("p1")
	if err := s.Join(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	v, _ = s.View("p1")
	if len(v.Quests) != 0 || v.Credits != 500 {
		t.Fatalf("после перезахода: %+v", v)
	}
	if again := s.Evaluate(ctx, "p1", station); len(again) != 0 {
		t.Fatal("после перезахода награда не повторяется")
	}
}

func TestUnknownPlayerNothing(t *testing.T) {
	s := newSvc(NewMemStore())
	if _, ok := s.View("нет"); ok {
		t.Fatal("игрок не загружен")
	}
	if got := s.Evaluate(context.Background(), "нет", station); len(got) != 0 {
		t.Fatal("без игрока ничего не происходит")
	}
}

func TestPlayersAreIndependent(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	_ = s.Join(ctx, "a")
	_ = s.Join(ctx, "b")
	s.Evaluate(ctx, "a", station)
	va, _ := s.View("a")
	vb, _ := s.View("b")
	if va.Credits != 500 || vb.Credits != 0 || len(vb.Quests) != 1 {
		t.Fatalf("a=%+v b=%+v", va, vb)
	}
}

// --- PostgreSQL ---

func pgSetup(t *testing.T) (context.Context, *PGStore, string) {
	t.Helper()
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	h := world.TokenHash("questtokentoken0001")
	pid := world.PublicID(h)
	if _, err := world.NewPGStore(pool).Resolve(ctx, "questtokentoken0001", "Квестер"); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	if err := st.Sync(ctx, Catalog()); err != nil {
		t.Fatal(err)
	}
	if err := st.Sync(ctx, Catalog()); err != nil { // повторная синхронизация безопасна
		t.Fatal(err)
	}
	return ctx, st, pid
}

func TestPGRewardOnceAndPersistence(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	d := Catalog()[0]
	created, err := st.Assign(ctx, pid, d)
	if err != nil || !created {
		t.Fatalf("выдача: %v %v", created, err)
	}
	if again, _ := st.Assign(ctx, pid, d); again {
		t.Fatal("повторная выдача не должна создавать запись")
	}
	granted, total, err := st.Complete(ctx, pid, d)
	if err != nil || !granted || total != 500 {
		t.Fatalf("выполнение: %v %v %v", granted, total, err)
	}
	granted, total, err = st.Complete(ctx, pid, d)
	if err != nil || granted || total != 500 {
		t.Fatalf("повтор: %v %v %v", granted, total, err)
	}
	// «перезапуск сервера»: новый сервис читает то же из базы
	s2 := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), st, func(string) (world.Vec, bool) { return station, true })
	if err := s2.Join(ctx, pid); err != nil {
		t.Fatal(err)
	}
	v, _ := s2.View(pid)
	if v.Credits != 500 || len(v.Quests) != 0 {
		t.Fatalf("после перезапуска: %+v", v)
	}
}

func TestPGConcurrentCompleteGrantsOnce(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	d := Catalog()[0]
	if _, err := st.Assign(ctx, pid, d); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	grants := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, _, err := st.Complete(ctx, pid, d)
			if err != nil {
				t.Error(err)
				return
			}
			if g {
				mu.Lock()
				grants++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if grants != 1 {
		t.Fatalf("награда выдана %d раз", grants)
	}
	data, _ := st.Load(ctx, pid)
	if data.Credits != 500 {
		t.Fatalf("кредиты %d", data.Credits)
	}
	var n int
	_ = st.pool.QueryRow(ctx, "SELECT count(*) FROM reward_grants WHERE player_id = $1", pid).Scan(&n)
	if n != 1 {
		t.Fatalf("записей о награде %d", n)
	}
}

func TestPGDefinitionVersionsKeptAndRewardKeyedByVersion(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	v1 := Catalog()[0]
	_, _ = st.Assign(ctx, pid, v1)
	// опубликована версия 2 с другой наградой: активное задание по версии 1 завершается по версии 1
	v2 := v1
	v2.Version = 2
	v2.Reward = Reward{Credits: 900}
	if err := st.Sync(ctx, []Def{v1, v2}); err != nil {
		t.Fatal(err)
	}
	granted, total, err := st.Complete(ctx, pid, v1)
	if err != nil || !granted || total != 500 {
		t.Fatalf("v1: %v %v %v", granted, total, err)
	}
	var n int
	_ = st.pool.QueryRow(ctx, "SELECT count(*) FROM quest_definitions WHERE id = 'first_flight'").Scan(&n)
	if n != 2 {
		t.Fatalf("определений %d, ожидалось 2 версии", n)
	}
}

func TestPGGrantCreditsIdempotentAndConcurrent(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	const n = 8
	res := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			g, _, err := st.GrantCredits(ctx, pid, "kill:en-sp-1:1", 40)
			if err != nil {
				t.Error(err)
			}
			res <- g
		}()
	}
	granted := 0
	for i := 0; i < n; i++ {
		if <-res {
			granted++
		}
	}
	if granted != 1 {
		t.Fatalf("награда выдана %d раз, ожидался один", granted)
	}
	data, _ := st.Load(ctx, pid)
	if data.Credits != 40 {
		t.Fatalf("кредиты %d", data.Credits)
	}
	if g, _, _ := st.GrantCredits(ctx, pid, "kill:en-sp-1:2", 40); !g {
		t.Fatal("другой ключ должен выдаваться")
	}
}

func TestMemGrantCreditsIdempotent(t *testing.T) {
	m := NewMemStore()
	ctx := context.Background()
	g1, t1, _ := m.GrantCredits(ctx, "p", "k", 10)
	g2, t2, _ := m.GrantCredits(ctx, "p", "k", 10)
	if !g1 || g2 || t1 != 10 || t2 != 10 {
		t.Fatalf("%v %v %d %d", g1, g2, t1, t2)
	}
}
