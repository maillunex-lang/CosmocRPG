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

func catalogDef(id string) Def {
	for _, d := range Catalog() {
		if d.ID == id {
			return d
		}
	}
	panic("нет задания " + id)
}

func seedLegacy(t *testing.T, ctx context.Context, st Store, pid string) {
	t.Helper()
	if _, err := st.Assign(ctx, pid, catalogDef("first_flight")); err != nil {
		t.Fatal(err)
	}
}

func mustTalk(t *testing.T, s *Service, pid, choice string) (Reply, []Completion) {
	t.Helper()
	reply, done, err := s.Talk(context.Background(), pid, "aurora", choice)
	if err != nil {
		t.Fatalf("ответ %q: %v", choice, err)
	}
	return reply, done
}

// playAurora проводит главу 1. sides=true закрывает три необязательные работы, чтобы журнал остался чистым.
func playAurora(t *testing.T, s *Service, pid string, sides bool) {
	t.Helper()
	ctx := context.Background()
	open, done := mustTalk(t, s, pid, "")
	if open.Speaker != "Ирина Ларс" || len(done) != 0 || open.QuestID != "ch01_q01" {
		t.Fatalf("допуск не открылся: %+v %+v", open, done)
	}
	if _, done = mustTalk(t, s, pid, "ready"); len(done) != 0 {
		t.Fatal("инструктор не должен закрывать допуск")
	}
	if _, done = mustTalk(t, s, pid, "go"); len(done) != 1 || done[0].QuestID != "ch01_q01" || done[0].Credits != 200 || done[0].Next != "Первый маршрут" {
		t.Fatalf("допуск: %+v", done)
	}
	if got := s.EventAt(ctx, pid, "aurora", "visit", "aur-az02", 1); len(got) != 0 {
		t.Fatal("чужая точка закрыла маяк", got)
	}
	got := s.EventAt(ctx, pid, "aurora", "visit", "aur-beacon", 1)
	if len(got) != 1 || got[0].QuestID != "ch01_q02" || got[0].Credits != 300 || got[0].Next != "Сигнал вне маршрута" {
		t.Fatalf("маяк: %+v", got)
	}
	if sides {
		if got = s.Event(ctx, pid, "mine", "scrap", 5); len(got) != 0 {
			t.Fatal("чужая руда закрыла побочное", got)
		}
		if got = s.Event(ctx, pid, "kill", "", 1); len(got) != 1 || got[0].QuestID != "aur_side_pirate" || got[0].Credits != 400 {
			t.Fatalf("пират: %+v", got)
		}
		if got = s.Event(ctx, pid, "mine", "iron_ore", 5); len(got) != 1 || got[0].QuestID != "aur_side_ore" || got[0].Next != "Сдача на Рубеже" {
			t.Fatalf("руда: %+v", got)
		}
		if got = s.Event(ctx, pid, "sell", "", 1); len(got) != 1 || got[0].QuestID != "aur_side_sell" || got[0].Credits != 300 {
			t.Fatalf("сдача: %+v", got)
		}
	}
	if got = s.EventAt(ctx, pid, "meridian", "visit", "aur-az02", 1); len(got) != 0 {
		t.Fatal("помехи закрылись в Меридиане", got)
	}
	if got = s.EventAt(ctx, pid, "aurora", "visit", "aur-az02", 1); len(got) != 1 || got[0].QuestID != "ch01_q03" || got[0].Credits != 400 {
		t.Fatalf("помехи: %+v", got)
	}
	if got = s.EventAt(ctx, pid, "aurora", "visit", "aur-az04", 1); len(got) != 1 || got[0].QuestID != "ch01_q04" || got[0].Credits != 500 || got[0].Next != "Несовпадающие записи" {
		t.Fatalf("буфер: %+v", got)
	}
	if _, done = mustTalk(t, s, pid, "here"); len(done) != 0 {
		t.Fatal("сверка ещё не закончена", done)
	}
	if _, done = mustTalk(t, s, pid, "next"); len(done) != 1 || done[0].QuestID != "ch01_q05" || done[0].Credits != 400 {
		t.Fatalf("тея: %+v", done)
	}
	if _, done = mustTalk(t, s, pid, "hold"); len(done) != 1 || done[0].QuestID != "ch01_q06" || done[0].Credits != 300 {
		t.Fatalf("интерес: %+v", done)
	}
	if _, done = mustTalk(t, s, pid, "ship"); len(done) != 0 {
		t.Fatal("корабль и пакет не одно и то же", done)
	}
	if _, done = mustTalk(t, s, pid, "data"); len(done) != 0 {
		t.Fatal("ирина ещё не закрыла главу", done)
	}
	if _, done = mustTalk(t, s, pid, "go"); len(done) != 1 || done[0].QuestID != "ch01_q07" || done[0].Credits != 800 || done[0].Next != "За границей" {
		t.Fatalf("рубеж: %+v", done)
	}
}

func TestSkipClosesCurrentTalkOnly(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	if err := s.Join(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Talk(ctx, "p", "aurora", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Talk(ctx, "p", "aurora", "route"); err != nil {
		t.Fatal(err)
	}
	reply, done, err := s.Talk(ctx, "p", "aurora", "skip")
	if err != nil || !reply.Done || reply.Speaker != "Ирина Ларс" || reply.Text == "" || len(done) != 1 || done[0].QuestID != "ch01_q01" || done[0].Credits != 200 {
		t.Fatalf("пропуск: %+v %+v %v", reply, done, err)
	}
	if _, _, err := s.Talk(ctx, "p", "aurora", "skip"); err != ErrNoDialogue {
		t.Fatal("пропуск не должен открыть следующий разговор", err)
	}
	v, _ := s.View("p")
	if v.Credits != 200 || len(v.Quests) != 1 || v.Quests[0].ID != "ch01_q02" {
		t.Fatalf("после пропуска: %+v", v)
	}
	if _, _, err := s.Talk(ctx, "p", "aurora", "skip"); err != ErrNoDialogue {
		t.Fatal("повторный пропуск", err)
	}
	if err := s.Join(ctx, "q"); err != nil {
		t.Fatal(err)
	}
	reply, done, err = s.Talk(ctx, "q", "aurora", "skip")
	sc, _ := Dialogue("ch01_q01")
	if err != nil || !reply.Done || reply.Text != sc.Closing || len(done) != 1 || done[0].Credits != 200 {
		t.Fatalf("пропуск с первой реплики: %+v %+v %v", reply, done, err)
	}
	if _, _, err := s.Talk(ctx, "q", "aurora", "нет"); err != ErrNoDialogue {
		t.Fatal(err)
	}
	sc, ok := Dialogue("ch01_q06")
	if !ok || sc.Skip == "" || sc.Skip == sc.Closing {
		t.Fatal("у выбора без ответа должен быть свой итог пропуска")
	}
}

func TestContractFlowAndIdempotentReward(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	s := newSvc(st)
	if err := s.Join(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	v, _ := s.View("p1")
	if len(v.Quests) != 1 || v.Quests[0].ID != "ch01_q01" || v.Credits != 0 {
		t.Fatalf("после входа: %+v", v)
	}
	if _, _, err := s.Talk(ctx, "p1", "meridian", ""); err != ErrNoDialogue {
		t.Fatal("допуск Авроры открылся в Меридиане", err)
	}
	if _, done := mustTalk(t, s, "p1", ""); len(done) != 0 {
		t.Fatal(done)
	}
	if _, done := mustTalk(t, s, "p1", "ready"); len(done) != 0 {
		t.Fatal(done)
	}
	_, done := mustTalk(t, s, "p1", "go")
	if len(done) != 1 || done[0].Credits != 200 || done[0].Total != 200 || done[0].QuestID != "ch01_q01" {
		t.Fatalf("выполнение: %+v", done)
	}
	if _, _, err := s.Talk(ctx, "p1", "aurora", "go"); err != ErrNoDialogue {
		t.Fatal("повторная награда за допуск", err)
	}
	v, _ = s.View("p1")
	if len(v.Quests) != 1 || v.Quests[0].ID != "ch01_q02" || v.Credits != 200 {
		t.Fatalf("после выполнения: %+v", v)
	}
	s.Leave("p1")
	if err := s.Join(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	v, _ = s.View("p1")
	if len(v.Quests) != 1 || v.Quests[0].ID != "ch01_q02" || v.Credits != 200 {
		t.Fatalf("после перезахода: %+v", v)
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
	mustTalk(t, s, "a", "")
	mustTalk(t, s, "a", "ready")
	mustTalk(t, s, "a", "go")
	va, _ := s.View("a")
	vb, _ := s.View("b")
	if va.Credits != 200 || vb.Credits != 0 || len(vb.Quests) != 1 || vb.Quests[0].ID != "ch01_q01" {
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
	d := catalogDef("first_flight")
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
	d := catalogDef("first_flight")
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
	v1 := catalogDef("first_flight")
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
	if v, _ := s.View("p2"); len(v.Quests) != 1 || v.Quests[0].ID != "ch01_q01" {
		t.Fatalf("старт цепочки: %+v", v)
	}
	if got := s.Event(ctx, "p2", "kill", "", 1); len(got) != 0 {
		t.Fatalf("kill до побочного: %+v", got)
	}
	playAurora(t, s, "p2", true)
	if v, _ := s.View("p2"); len(v.Quests) != 1 || v.Quests[0].ID != "first_jump" || v.Credits != 4000 {
		t.Fatalf("после главы открыт прыжок: %+v", v)
	}
	if got := s.Event(ctx, "p2", "jump", "redstorm", 1); len(got) != 0 {
		t.Fatal("прыжок не в Меридиан закрыл главу", got)
	}
	if got := s.Event(ctx, "p2", "jump", "meridian", 1); len(got) != 1 || got[0].Next != "Груз для Меридиана" || got[0].Credits != 1000 {
		t.Fatalf("jump: %+v", got)
	}
	s.Leave("p2")
	if err := s.Join(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.View("p2"); len(v.Quests) != 1 || v.Quests[0].ID != "meridian_delivery" || v.Credits != 5000 {
		t.Fatalf("после перезахода: %+v", v)
	}
}

func TestLegacyTutorialStillReachesMeridian(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	s := newSvc(st)
	seedLegacy(t, ctx, st, "old")
	if err := s.Join(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	v, _ := s.View("old")
	if len(v.Quests) != 1 || v.Quests[0].ID != "first_flight" {
		t.Fatalf("старый пилот не должен начинать главу заново: %+v", v)
	}
	cs := s.Evaluate(ctx, "old", station)
	if len(cs) != 1 || cs[0].QuestID != "first_flight" || cs[0].Credits != 500 || cs[0].Next != "Сигнал тревоги" {
		t.Fatalf("первый полёт: %+v", cs)
	}
	if got := s.Event(ctx, "old", "kill", "", 1); len(got) != 1 || got[0].Next != "Руда из пустоты" {
		t.Fatalf("kill: %+v", got)
	}
	if got := s.Event(ctx, "old", "mine", "iron_ore", 5); len(got) != 1 || got[0].Next != "Первый контракт" {
		t.Fatalf("mine: %+v", got)
	}
	if got := s.Event(ctx, "old", "sell", "", 1); len(got) != 1 || got[0].Next != "За границей" {
		t.Fatalf("sell: %+v", got)
	}
	if got := s.Event(ctx, "old", "jump", "meridian", 1); len(got) != 1 || got[0].Next != "Груз для Меридиана" {
		t.Fatalf("jump: %+v", got)
	}
	if v, _ = s.View("old"); len(v.Quests) != 1 || v.Quests[0].ID != "meridian_delivery" || v.Credits != 3600 {
		t.Fatalf("старая цепочка: %+v", v)
	}
}

func TestAuroraSidesDoNotGateTheJump(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	if err := s.Join(ctx, "cadet"); err != nil {
		t.Fatal(err)
	}
	playAurora(t, s, "cadet", false)
	v, _ := s.View("cadet")
	got := map[string]bool{}
	for _, q := range v.Quests {
		got[q.ID] = true
	}
	if !got["first_jump"] || !got["aur_side_pirate"] || !got["aur_side_ore"] || got["aur_side_sell"] || v.Credits != 2900 {
		t.Fatalf("побочные не обязательны: %+v", v)
	}
	if _, _, err := s.Talk(ctx, "cadet", "aurora", "meta"); err != ErrNoDialogue {
		t.Fatal("закрытый выбор ещё доступен", err)
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
			seedLegacy(t, ctx, st, pid)
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
		seedLegacy(t, ctx, st, pid)
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
	d := catalogDef("first_ore")
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
	seedLegacy(t, ctx, st, "a")
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
			seedLegacy(t, ctx, st, pid)
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
			d := catalogDef("first_ore")
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
	playAurora(t, s, "pilot", true)
	if _, ok := s.Pending("pilot", "deliver", "meridian"); ok {
		t.Fatal("доставка не должна быть доступна до прыжка")
	}
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
	if len(got) != 1 || got[0].QuestID != "meridian_report" || got[0].Next != "Голос диспетчера" || got[0].Total != 7800 {
		t.Fatalf("доклад: %+v", got)
	}
	if v, _ := s.View("pilot"); len(v.Quests) != 1 || v.Quests[0].ID != "meridian_voice" || v.Quests[0].Kind != "talk" || v.Credits != 7800 {
		t.Fatalf("после доклада ждёт разговор: %+v", v)
	}
}

func TestDispatcherThenRedStorm(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	if err := s.Join(ctx, "pilot"); err != nil {
		t.Fatal(err)
	}
	playAurora(t, s, "pilot", true)
	s.Event(ctx, "pilot", "jump", "meridian", 1)
	s.EventAt(ctx, "pilot", "meridian", "deliver", "iron_ore", 5)
	s.EventAt(ctx, "pilot", "meridian", "visit", "anomaly", 1)
	s.EventAt(ctx, "pilot", "meridian", "visit", "station", 1)
	if _, _, err := s.Talk(ctx, "pilot", "aurora", ""); err != ErrNoDialogue {
		t.Fatal("разговор Меридиана открылся в Авроре", err)
	}
	open, done, err := s.Talk(ctx, "pilot", "meridian", "")
	if err != nil || open.Speaker != "Вора Кесс" || len(open.Choices) != 2 || len(done) != 0 || open.Done {
		t.Fatalf("открытие: %+v %+v %v", open, done, err)
	}
	if _, _, err := s.Talk(ctx, "pilot", "meridian", "нет"); err != ErrBadChoice {
		t.Fatal(err)
	}
	who, done, err := s.Talk(ctx, "pilot", "meridian", "ask")
	if err != nil || len(who.Choices) != 1 || len(done) != 0 || who.Text == open.Text {
		t.Fatalf("уточнение: %+v %v", who, err)
	}
	if _, ok := s.Pending("pilot", "talk", "meridian"); !ok {
		t.Fatal("вопрос не должен закрывать задание")
	}
	end, done, err := s.Talk(ctx, "pilot", "meridian", "go")
	if err != nil || !end.Done || len(done) != 1 || done[0].QuestID != "meridian_voice" || done[0].Next != "Ворота Шторма" || done[0].Credits != 400 {
		t.Fatalf("согласие: %+v %+v %v", end, done, err)
	}
	if got := s.Event(ctx, "pilot", "jump", "aurora", 1); len(got) != 0 {
		t.Fatal("обратный прыжок закрыл Шторм", got)
	}
	got := s.Event(ctx, "pilot", "jump", "redstorm", 1)
	if len(got) != 1 || got[0].QuestID != "storm_jump" || got[0].Next != "Обломки у поля" {
		t.Fatalf("прыжок: %+v", got)
	}
	if got := s.EventAt(ctx, "pilot", "meridian", "visit", "derelict", 1); len(got) != 0 {
		t.Fatal("обломки Меридиана закрыли Шторм", got)
	}
	got = s.EventAt(ctx, "pilot", "redstorm", "visit", "derelict", 1)
	if len(got) != 1 || got[0].QuestID != "storm_wreck" || got[0].Next != "Голос рубежа" || got[0].Credits != 1100 {
		t.Fatalf("обломки: %+v", got)
	}
	if v, _ := s.View("pilot"); len(v.Quests) != 1 || v.Quests[0].ID != "storm_dispatch" {
		t.Fatalf("после обломков ждёт диспетчера: %+v", v)
	}
}

func TestRedStormReport(t *testing.T) {
	ctx := context.Background()
	s := newSvc(NewMemStore())
	if err := s.Join(ctx, "pilot"); err != nil {
		t.Fatal(err)
	}
	playAurora(t, s, "pilot", true)
	s.Event(ctx, "pilot", "jump", "meridian", 1)
	s.EventAt(ctx, "pilot", "meridian", "deliver", "iron_ore", 5)
	s.EventAt(ctx, "pilot", "meridian", "visit", "anomaly", 1)
	s.EventAt(ctx, "pilot", "meridian", "visit", "station", 1)
	if _, _, err := s.Talk(ctx, "pilot", "meridian", "go"); err != nil {
		t.Fatal(err)
	}
	s.Event(ctx, "pilot", "jump", "redstorm", 1)
	s.EventAt(ctx, "pilot", "redstorm", "visit", "derelict", 1)
	if got := s.EventAt(ctx, "pilot", "redstorm", "visit", "anomaly", 1); len(got) != 0 {
		t.Fatal("контур открылся до разговора с Кирой", got)
	}
	if _, _, err := s.Talk(ctx, "pilot", "meridian", ""); err != ErrNoDialogue {
		t.Fatal("Кира отвечает из Меридиана", err)
	}
	open, done, err := s.Talk(ctx, "pilot", "redstorm", "")
	if err != nil || open.Speaker != "Кира Роан" || len(done) != 0 || len(open.Choices) != 2 {
		t.Fatalf("Кира: %+v %v", open, err)
	}
	if _, _, err := s.Talk(ctx, "pilot", "redstorm", "danger"); err != nil {
		t.Fatal(err)
	}
	end, done, err := s.Talk(ctx, "pilot", "redstorm", "go")
	if err != nil || !end.Done || len(done) != 1 || done[0].Next != "Старый контур" || done[0].Credits != 500 {
		t.Fatalf("Кира закрыла не то: %+v %+v %v", end, done, err)
	}
	if got := s.EventAt(ctx, "pilot", "meridian", "visit", "anomaly", 1); len(got) != 0 {
		t.Fatal("аномалия Меридиана закрыла контур Шторма", got)
	}
	got := s.EventAt(ctx, "pilot", "redstorm", "visit", "anomaly", 1)
	if len(got) != 1 || got[0].QuestID != "storm_contour" || got[0].Next != "Граница известного" || got[0].Credits != 1000 {
		t.Fatalf("контур: %+v", got)
	}
	open, done, err = s.Talk(ctx, "pilot", "redstorm", "")
	if err != nil || open.Speaker != "Вера Элл" || len(done) != 0 {
		t.Fatalf("Вера: %+v %v", open, err)
	}
	if _, _, err := s.Talk(ctx, "pilot", "redstorm", "ship"); err != nil {
		t.Fatal(err)
	}
	end, done, err = s.Talk(ctx, "pilot", "redstorm", "close")
	if err != nil || !end.Done || len(done) != 1 || done[0].QuestID != "storm_report" || done[0].Credits != 800 || done[0].Total != 12600 || done[0].Next != "" {
		t.Fatalf("отчёт: %+v %+v %v", end, done, err)
	}
	if v, _ := s.View("pilot"); len(v.Quests) != 0 || v.Credits != 12600 {
		t.Fatalf("глава не закрыта: %+v", v)
	}
	if _, _, err := s.Talk(ctx, "pilot", "redstorm", ""); err != ErrNoDialogue {
		t.Fatal("после отчёта разговор ещё открыт", err)
	}
}
