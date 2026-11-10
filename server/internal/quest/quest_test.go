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
	// цепочка: вместо выполненного контракта активировано следующее звено (Э-3)
	if len(v.Quests) != 1 || v.Quests[0].ID != "first_blood" || v.Credits != 500 {
		t.Fatalf("после выполнения: %+v", v)
	}
	// перезаход: выполненный не выдаётся заново, цепочка продолжается со следующего звена
	s.Leave("p1")
	if err := s.Join(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	v, _ = s.View("p1")
	if len(v.Quests) != 1 || v.Quests[0].ID != "first_blood" || v.Credits != 500 {
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
	// The campaign now continues after first_flight. A restart must preserve
	// the reward and activate first_blood rather than expect an empty journal.
	if v.Credits != 500 || len(v.Quests) != 1 || v.Quests[0].ID != "first_blood" || v.Quests[0].Progress != 0 {
		t.Fatalf("после перезапуска: %+v", v)
	}
	if again := s2.Evaluate(ctx, pid, station); len(again) != 0 {
		t.Fatal("после перезапуска повторная награда")
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

func TestEventKindsAndChain(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	if err := s.Join(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	// цепочка не выдаётся сразу: только first_flight
	if v, _ := s.View("p2"); len(v.Quests) != 1 || v.Quests[0].ID != "first_flight" {
		t.Fatalf("старт цепочки: %+v", v)
	}
	// чужие события не считаются
	if got := s.Event(ctx, "p2", "kill", "", 1); len(got) != 0 {
		t.Fatalf("kill без контракта: %+v", got)
	}
	// выполняем звенья событиями
	if got := s.Event(ctx, "p2", "kill", "", 1); len(got) != 0 {
		t.Fatalf("kill рано: %+v", got)
	}
	// first_flight выполняем через Evaluate (позиция станции)
	cs := s.Evaluate(ctx, "p2", station)
	if len(cs) != 1 || cs[0].Next != "Сигнал тревоги" {
		t.Fatalf("следующее звено: %+v", cs)
	}
	// kill ×1 → first_blood; mine 5 → first_ore; sell → first_deal; jump → first_jump
	if got := s.Event(ctx, "p2", "kill", "", 1); len(got) != 1 || got[0].Next != "Руда из пустоты" {
		t.Fatalf("kill: %+v", got)
	}
	if got := s.Event(ctx, "p2", "mine", "iron_ore", 3); len(got) != 0 {
		t.Fatalf("mine 3/5: %+v", got)
	}
	if got := s.Event(ctx, "p2", "mine", "scrap", 5); len(got) != 0 {
		t.Fatalf("другой ресурс не считается: %+v", got)
	}
	if got := s.Event(ctx, "p2", "mine", "iron_ore", 2); len(got) != 1 || got[0].Next != "Первый контракт" {
		t.Fatalf("mine 5/5: %+v", got)
	}
	if got := s.Event(ctx, "p2", "sell", "", 1); len(got) != 1 || got[0].Next != "За границей" {
		t.Fatalf("sell: %+v", got)
	}
	if got := s.Event(ctx, "p2", "jump", "meridian", 1); len(got) != 1 || got[0].Next != "Груз для Меридиана" {
		t.Fatalf("jump: %+v", got)
	}
	if v, _ := s.View("p2"); len(v.Quests) != 1 || v.Quests[0].ID != "meridian_delivery" {
		t.Fatalf("после прыжка открыта доставка: %+v", v.Quests)
	}
	s.Leave("p2")
	if err := s.Join(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.View("p2"); len(v.Quests) != 1 || v.Quests[0].ID != "meridian_delivery" || v.Credits != 3600 {
		t.Fatalf("после перезахода: %+v", v)
	}
}

func TestSpendCreditsAtomic(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	s := newSvc(st)
	_ = s.Join(ctx, "p3")
	// начислим 500
	if _, _, err := st.GrantCredits(ctx, "p3", "k1", 500); err != nil {
		t.Fatal(err)
	}
	total, err := s.Spend(ctx, "p3", 200)
	if err != nil || total != 300 {
		t.Fatalf("списание: %d %v", total, err)
	}
	if _, err := s.Spend(ctx, "p3", 1000); err != ErrNotEnough {
		t.Fatalf("перерасход должен отклоняться: %v", err)
	}
	if v, _ := s.View("p3"); v.Credits != 300 {
		t.Fatalf("баланс после отказа: %d", v.Credits)
	}
	if total, err := s.Spend(ctx, "p3", 300); err != nil || total != 0 {
		t.Fatalf("полное списание: %d %v", total, err)
	}
}

// Частично добытая руда должна переживать перезапуск сервиса, а не только награда.
func TestPartialMiningSurvivesRestart(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			ctx := context.Background()
			var st Store = NewMemStore()
			pid := "pilot-a"
			if backend == "postgres" {
				_, pg, id := pgSetup(t)
				st, pid = pg, id
			}
			s := newSvc(st)
			if err := s.Join(ctx, pid); err != nil {
				t.Fatal(err)
			}
			s.Evaluate(ctx, pid, station)
			s.Event(ctx, pid, "kill", "", 1)
			s.Event(ctx, pid, "mine", "iron_ore", 2)
			s = newSvc(st)
			if err := s.Join(ctx, pid); err != nil {
				t.Fatal(err)
			}
			v, _ := s.View(pid)
			if len(v.Quests) != 1 || v.Quests[0].ID != "first_ore" || v.Quests[0].Progress != 2 {
				t.Fatalf("lost partial progress: %+v", v)
			}
			got := s.Event(ctx, pid, "mine", "iron_ore", 3)
			if len(got) != 1 || got[0].QuestID != "first_ore" {
				t.Fatalf("completion after restart: %+v", got)
			}
			if again := s.Event(ctx, pid, "mine", "iron_ore", 3); len(again) != 0 {
				t.Fatal("duplicate reward")
			}
		})
	}
}

// Два пилота проходят цепочку независимо, включая рестарт посреди добычи.
// Это серверный сценарий; он не заменяет прохождение двумя Android-клиентами.
func TestTwoPilotCampaignRestart(t *testing.T) {
	ctx, st, a := pgSetup(t)
	token := "beta-second-pilot-0001"
	rec, err := world.NewPGStore(st.pool).Resolve(ctx, token, "БетаВторой")
	if err != nil {
		t.Fatal(err)
	}
	b := rec.ID
	s := newSvc(st)
	for _, pid := range []string{a, b} {
		if err := s.Join(ctx, pid); err != nil {
			t.Fatal(err)
		}
		if got := s.Evaluate(ctx, pid, station); len(got) != 1 {
			t.Fatal("first flight", got)
		}
		if got := s.Event(ctx, pid, "kill", "", 1); len(got) != 1 {
			t.Fatal("kill", got)
		}
	}
	s.Event(ctx, a, "mine", "iron_ore", 2)
	s.Event(ctx, b, "mine", "iron_ore", 4)
	s = newSvc(st)
	for _, tc := range []struct {
		pid       string
		remaining int
	}{{a, 3}, {b, 1}} {
		if err := s.Join(ctx, tc.pid); err != nil {
			t.Fatal(err)
		}
		if got := s.Event(ctx, tc.pid, "mine", "iron_ore", tc.remaining); len(got) != 1 {
			t.Fatal("mine", got)
		}
		if got := s.Event(ctx, tc.pid, "sell", "", 1); len(got) != 1 {
			t.Fatal("sell", got)
		}
		if got := s.Event(ctx, tc.pid, "jump", "aurora", 1); len(got) != 0 {
			t.Fatal("wrong system completed quest")
		}
		if got := s.Event(ctx, tc.pid, "jump", "meridian", 1); len(got) != 1 {
			t.Fatal("jump", got)
		}
	}
	s = newSvc(st)
	for _, pid := range []string{a, b} {
		if err := s.Join(ctx, pid); err != nil {
			t.Fatal(err)
		}
		v, _ := s.View(pid)
		if len(v.Quests) != 1 || v.Quests[0].ID != "meridian_delivery" || v.Credits != 3600 {
			t.Fatalf("campaign restore: %+v", v)
		}
		s.Evaluate(ctx, pid, station)
		s.Event(ctx, pid, "kill", "", 1)
		v, _ = s.View(pid)
		if v.Credits != 3600 {
			t.Fatal("duplicate campaign rewards", v)
		}
	}
}

func TestConcurrentProgressBounded(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	d := Catalog()[2]
	if _, err := st.Assign(ctx, pid, d); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.Advance(ctx, pid, d, 1); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, err := st.Load(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Quests) != 1 || data.Quests[0].Progress != 5 {
		t.Fatalf("progress bound: %+v", data)
	}
}

type failingProgressStore struct{ Store }

func (s failingProgressStore) Advance(ctx context.Context, pid string, d Def, n int) (int, error) {
	return 0, context.DeadlineExceeded
}

func TestProgressWriteFailureDoesNotAdvanceClient(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	s := newSvc(failingProgressStore{st})
	if err := s.Join(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	s.Evaluate(ctx, "a", station)
	if got := s.Event(ctx, "a", "kill", "", 1); len(got) != 0 {
		t.Fatal("reward on failed write")
	}
	v, _ := s.View("a")
	if len(v.Quests) != 1 || v.Quests[0].Progress != 0 || v.Credits != 500 {
		t.Fatalf("unpersisted progress exposed: %+v", v)
	}
}

func TestEventReceiptRestartAndPendingCompletion(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			ctx := context.Background()
			var st Store = NewMemStore()
			pid := "event-receipt-player"
			if backend == "postgres" {
				_, pg, id := pgSetup(t)
				st, pid = pg, id
			}
			s := newSvc(st)
			if err := s.Join(ctx, pid); err != nil {
				t.Fatal(err)
			}
			s.Evaluate(ctx, pid, station)
			s.Event(ctx, pid, "kill", "", 1)
			s.EventOnce(ctx, pid, "mine-partial", "mine", "iron_ore", 2)
			s = newSvc(st)
			if err := s.Join(ctx, pid); err != nil {
				t.Fatal(err)
			}
			s.EventOnce(ctx, pid, "mine-partial", "mine", "iron_ore", 2)
			v, _ := s.View(pid)
			if v.Quests[0].Progress != 2 {
				t.Fatal("event replay advanced progress", v)
			}
			// Сохраняем событие в обход completeOne, моделируя точку аварии до награды.
			d := Catalog()[2]
			hash := [32]byte{1}
			if _, err := st.AdvanceOnce(ctx, pid, "mine-final", hash, []Def{d}, 3); err != nil {
				t.Fatal(err)
			}
			if _, err := st.AdvanceOnce(ctx, pid, "mine-final", [32]byte{2}, []Def{d}, 3); err != ErrEventConflict {
				t.Fatal("payload mismatch accepted", err)
			}
			s = newSvc(st)
			if err := s.Join(ctx, pid); err != nil {
				t.Fatal(err)
			}
			v, _ = s.View(pid)
			if len(v.Quests) != 1 || v.Quests[0].ID != "first_deal" || v.Credits != 2000 {
				t.Fatal("pending completion not restored", v)
			}
			s = newSvc(st)
			if err := s.Join(ctx, pid); err != nil {
				t.Fatal(err)
			}
			v, _ = s.View(pid)
			if v.Credits != 2000 {
				t.Fatal("duplicate recovery reward", v)
			}
		})
	}
}

func TestMeridianChapter(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	if err := s.Join(ctx, "pilot"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Pending("pilot", "deliver", "meridian"); ok {
		t.Fatal("доставка не должна быть доступна до прыжка")
	}
	s.Evaluate(ctx, "pilot", station)
	s.Event(ctx, "pilot", "kill", "", 1)
	s.Event(ctx, "pilot", "mine", "iron_ore", 5)
	s.Event(ctx, "pilot", "sell", "", 1)
	if got := s.Event(ctx, "pilot", "jump", "redstorm", 1); len(got) != 0 {
		t.Fatal("прыжок не в Меридиан закрыл главу", got)
	}
	got := s.Event(ctx, "pilot", "jump", "meridian", 1)
	if len(got) != 1 || got[0].Next != "Груз для Меридиана" {
		t.Fatalf("прыжок: %+v", got)
	}
	if got := s.EventAt(ctx, "pilot", "aurora", "deliver", "iron_ore", 5); len(got) != 0 {
		t.Fatal("сдача в чужой системе", got)
	}
	d, ok := s.Pending("pilot", "deliver", "meridian")
	if !ok || d.Target != "iron_ore" || d.Goal != 5 || d.Marker != "station" {
		t.Fatalf("ожидалась доставка руды: %+v %v", d, ok)
	}
	got = s.EventAt(ctx, "pilot", "meridian", "deliver", "iron_ore", 5)
	if len(got) != 1 || got[0].QuestID != "meridian_delivery" || got[0].Next != "След в пустоте" || got[0].Credits != 1200 {
		t.Fatalf("доставка: %+v", got)
	}
	if _, ok := s.Pending("pilot", "visit", "aurora"); ok {
		t.Fatal("аномалия Меридиана открылась в Авроре")
	}
	if got := s.EventAt(ctx, "pilot", "meridian", "visit", "station", 1); len(got) != 0 {
		t.Fatal("станция закрыла ещё не открытый доклад", got)
	}
	got = s.EventAt(ctx, "pilot", "meridian", "visit", "anomaly", 1)
	if len(got) != 1 || got[0].Next != "Доклад на станции" {
		t.Fatalf("аномалия: %+v", got)
	}
	got = s.EventAt(ctx, "pilot", "meridian", "visit", "station", 1)
	if len(got) != 1 || got[0].QuestID != "meridian_report" || got[0].Next != "" || got[0].Total != 6400 {
		t.Fatalf("доклад: %+v", got)
	}
	if v, _ := s.View("pilot"); len(v.Quests) != 0 || v.Credits != 6400 {
		t.Fatalf("глава закрыта: %+v", v)
	}
}
