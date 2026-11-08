package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func newSvc() (*Service, *MemStore, *time.Time) {
	st := NewMemStore()
	st.AddPlayer("p1", "Ковбой")
	st.AddPlayer("p2", "Вега")
	s := NewService(st)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s, st, &now
}

var me = Sender{ID: "p1", Name: "Ковбой"}

func TestSendGeneralAndHistory(t *testing.T) {
	s, _, now := newSvc()
	ctx := context.Background()
	m, err := s.Send(ctx, me, General, "", "  Привет,\n всем!  ")
	if err != nil || m.Text != "Привет, всем!" || m.ID == 0 || m.FromName != "Ковбой" {
		t.Fatalf("%+v %v", m, err)
	}
	*now = now.Add(3 * time.Second)
	_, _ = s.Send(ctx, Sender{ID: "p2", Name: "Вега"}, General, "", "Здравствуйте")
	g, _, err := s.History(ctx, "p1")
	if err != nil || len(g) != 2 || g[0].Text != "Привет, всем!" || g[1].FromName != "Вега" {
		t.Fatalf("история: %+v %v", g, err)
	}
}

func TestValidation(t *testing.T) {
	s, _, now := newSvc()
	ctx := context.Background()
	cases := []struct {
		name string
		text string
		want error
	}{
		{"пусто", "   \n\t ", ErrEmpty},
		{"длина", strings.Repeat("а", 201), ErrTooLong},
		{"ссылка http", "заходи на http://evil.example", ErrLinks},
		{"ссылка www", "смотри www.evil.example", ErrLinks},
		{"ссылка домен", "мой сайт vasya.ru", ErrLinks},
		{"мат", "ты сука", ErrBadWords},
		{"мат с подменой", "пиздец", ErrBadWords},
		{"мат с разделителями", "х.у.й", ErrBadWords},
		{"мат английский", "what the fuck", ErrBadWords},
	}
	for _, c := range cases {
		*now = now.Add(5 * time.Second)
		if _, err := s.Send(ctx, me, General, "", c.text); err != c.want {
			t.Errorf("%s: %v, ожидалось %v", c.name, err, c.want)
		}
	}
	// обычные слова с похожими корнями не блокируются
	for _, ok := range []string{"небанальная идея", "скука смертная", "оскорблять не буду", "хлеб и сукно", "Привет! Как дела?", "Продам 10 ед. руды за 40"} {
		*now = now.Add(5 * time.Second)
		if _, err := s.Send(ctx, me, General, "", ok); err != nil {
			t.Errorf("ложное срабатывание на %q: %v", ok, err)
		}
	}
}

func TestRateAndDuplicate(t *testing.T) {
	s, _, now := newSvc()
	ctx := context.Background()
	if _, err := s.Send(ctx, me, General, "", "раз"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(ctx, me, General, "", "два"); err != ErrRate {
		t.Fatalf("слишком часто: %v", err)
	}
	*now = now.Add(2100 * time.Millisecond)
	if _, err := s.Send(ctx, me, General, "", "два"); err != nil {
		t.Fatalf("через 2 секунды можно: %v", err)
	}
	*now = now.Add(3 * time.Second)
	if _, err := s.Send(ctx, me, General, "", "ДВА"); err != ErrDuplicate {
		t.Fatalf("повтор того же текста: %v", err)
	}
	// у другого игрока свой счётчик
	if _, err := s.Send(ctx, Sender{ID: "p2", Name: "Вега"}, General, "", "два"); err != nil {
		t.Fatalf("другой игрок: %v", err)
	}
}

func TestDirectMessages(t *testing.T) {
	s, _, now := newSvc()
	ctx := context.Background()
	m, err := s.Send(ctx, me, DM, "вега", "Привет лично")
	if err != nil || m.ToID != "p2" || m.ToName != "Вега" {
		t.Fatalf("%+v %v", m, err)
	}
	*now = now.Add(3 * time.Second)
	if _, err := s.Send(ctx, me, DM, "Ковбой", "себе"); err != ErrSelfDM {
		t.Fatalf("себе: %v", err)
	}
	if _, err := s.Send(ctx, me, DM, "Нет-такого", "эй"); err != ErrNoRecipient {
		t.Fatalf("нет адресата: %v", err)
	}
	_, d1, _ := s.History(ctx, "p2")
	_, d3, _ := s.History(ctx, "p-other")
	if len(d1) != 1 || len(d3) != 0 {
		t.Fatalf("личные видны только участникам: %d и %d", len(d1), len(d3))
	}
	// адресат из справочника ИИ
	s.Directory = func(_ context.Context, q string) (Person, bool) {
		if strings.EqualFold(q, "Орион-7") {
			return Person{ID: "ai-7", Name: "Орион-7", AI: true}, true
		}
		return Person{}, false
	}
	*now = now.Add(3 * time.Second)
	m, err = s.Send(ctx, me, DM, "орион-7", "Ты здесь?")
	if err != nil || m.ToID != "ai-7" {
		t.Fatalf("ИИ как адресат: %+v %v", m, err)
	}
}

func TestMuteAndAIBypass(t *testing.T) {
	s, _, now := newSvc()
	ctx := context.Background()
	if _, err := s.MuteBy(ctx, "админ", "Ковбой", 10, "спам"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(ctx, me, General, "", "привет"); err != ErrMuted {
		t.Fatalf("заглушён: %v", err)
	}
	if ms, _ := s.Mutes(ctx); len(ms) != 1 || ms[0].PlayerName != "Ковбой" || ms[0].Reason != "спам" {
		t.Fatalf("список: %+v", ms)
	}
	*now = now.Add(11 * time.Minute)
	if _, err := s.Send(ctx, me, General, "", "привет"); err != nil {
		t.Fatalf("срок мьюта вышел: %v", err)
	}
	if ms, _ := s.Mutes(ctx); len(ms) != 0 {
		t.Fatalf("истёкший мьют в списке: %+v", ms)
	}
	// бессрочный и снятие
	_, _ = s.MuteBy(ctx, "админ", "p1", 0, "")
	*now = now.Add(24 * time.Hour)
	if _, err := s.Send(ctx, me, General, "", "ещё"); err != ErrMuted {
		t.Fatalf("бессрочный: %v", err)
	}
	_ = s.Unmute(ctx, "p1")
	if _, err := s.Send(ctx, me, General, "", "ещё раз"); err != nil {
		t.Fatalf("после снятия: %v", err)
	}
	if _, err := s.MuteBy(ctx, "админ", "нет такого", 5, ""); err != ErrNoRecipient {
		t.Fatalf("мьют несуществующего: %v", err)
	}
	// ИИ-отправитель не блокируется мьютом игроков
	if _, err := s.Send(ctx, Sender{ID: "ai-1", Name: "Орион", AI: true}, General, "", "я тут"); err != nil {
		t.Fatalf("ИИ: %v", err)
	}
}

func TestSettingsAndPurge(t *testing.T) {
	s, _, now := newSvc()
	ctx := context.Background()
	st := s.Settings()
	st.Enabled = false
	if err := s.UpdateSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(ctx, me, General, "", "привет"); err != ErrDisabled {
		t.Fatalf("выключен: %v", err)
	}
	st.Enabled, st.MinIntervalMs, st.ExtraBlocked = true, 0, []string{"синхрофазотрон"}
	_ = s.UpdateSettings(ctx, st)
	if _, err := s.Send(ctx, me, General, "", "купи синхрофазотрон"); err != ErrBadWords {
		t.Fatalf("дополнительное слово: %v", err)
	}
	st.LinksAllowed, st.ProfanityFilter = true, false
	_ = s.UpdateSettings(ctx, st)
	if _, err := s.Send(ctx, me, General, "", "см. http://a.ru и сука"); err != nil {
		t.Fatalf("фильтры выключены: %v", err)
	}
	bad := st
	bad.MaxLen = 5
	if err := s.UpdateSettings(ctx, bad); err == nil {
		t.Fatal("некорректные настройки приняты")
	}
	// хранение: сообщения старше срока удаляются
	*now = now.AddDate(0, 0, 8)
	_, _ = s.Send(ctx, Sender{ID: "p2", Name: "Вега"}, General, "", "свежее")
	n, err := s.Purge(ctx)
	g, _, _ := s.History(ctx, "p1")
	if err != nil || n != 1 || len(g) != 1 || g[0].Text != "свежее" {
		t.Fatalf("очистка: n=%d %v %+v", n, err, g)
	}
}

func TestPGStoreRoundTrip(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	ps := world.NewPGStore(pool)
	a, _ := ps.Resolve(ctx, "chattoken000000001", "")
	b, _ := ps.Resolve(ctx, "chattoken000000002", "")
	if _, err := ps.CreateCharacter(ctx, a.ID, "Ковбой", "m", "miner", world.Starter{Ship: "TMP-MINE", Spec: "miner", Skin: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ps.CreateCharacter(ctx, b.ID, "Вега", "f", "scout", world.Starter{Ship: "FTR-001", Spec: "scout", Skin: "b"}); err != nil {
		t.Fatal(err)
	}
	s := NewService(NewPGStore(pool))
	if err := s.Load(ctx); err != nil {
		t.Fatal(err)
	}
	m, err := s.Send(ctx, Sender{ID: a.ID, Name: "Ковбой"}, General, "", "Привет из базы")
	if err != nil || m.ID == 0 || m.At == 0 {
		t.Fatalf("%+v %v", m, err)
	}
	d, err := s.Send(ctx, Sender{ID: b.ID, Name: "Вега"}, DM, "ковбой", "Лично")
	if err != nil || d.ToID != a.ID {
		t.Fatalf("личное: %+v %v", d, err)
	}
	g, dms, err := s.History(ctx, a.ID)
	if err != nil || len(g) != 1 || len(dms) != 1 || dms[0].Text != "Лично" {
		t.Fatalf("история: %+v %+v %v", g, dms, err)
	}
	if _, err := s.MuteBy(ctx, "админ", "Вега", 5, "тест"); err != nil {
		t.Fatal(err)
	}
	if ms, err := s.Mutes(ctx); err != nil || len(ms) != 1 || ms[0].PlayerName != "Вега" {
		t.Fatalf("мьюты: %+v %v", ms, err)
	}
	if res, _ := s.Search(ctx, "", "лично", 10); len(res) != 1 {
		t.Fatalf("поиск: %+v", res)
	}
	st := s.Settings()
	st.RetentionDays = 3
	if err := s.UpdateSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	s2 := NewService(NewPGStore(pool))
	_ = s2.Load(ctx)
	if s2.Settings().RetentionDays != 3 {
		t.Fatal("настройки не сохранились")
	}
	if err := s.Unmute(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
}
