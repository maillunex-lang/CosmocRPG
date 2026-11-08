package admin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func newChatEnv(t *testing.T) (*env, string, *chat.Service, string) {
	store, pool := testStore(t)
	ctx := context.Background()
	if _, err := store.CreateUser(ctx, "Admin", testPassword); err != nil {
		t.Fatal(err)
	}
	ps := world.NewPGStore(pool)
	rec, _ := ps.Resolve(ctx, "adminchattoken0001", "")
	if _, err := ps.CreateCharacter(ctx, rec.ID, "Нарушитель", "m", "miner", world.Starter{Ship: "TMP-MINE", Spec: "miner", Skin: "a"}); err != nil {
		t.Fatal(err)
	}
	svc := chat.NewService(chat.NewPGStore(pool))
	h := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{Version: "test", StartedAt: time.Now(), Chat: svc})
	e := &env{t: t, store: store, pool: pool, h: h, srv: h.Routes()}
	return e, e.login(), svc, rec.ID
}

func TestChatRoutesRequireLogin(t *testing.T) {
	e, _, _, _ := newChatEnv(t)
	for _, c := range [][2]string{{"GET", "/api/chat/messages"}, {"GET", "/api/chat/settings"}, {"PUT", "/api/chat/settings"}, {"GET", "/api/chat/mutes"}, {"POST", "/api/chat/mutes"}, {"DELETE", "/api/chat/mutes/x"}} {
		if rec := e.raw(c[0], c[1], []byte("{}"), "application/json", ""); rec.Code != 401 {
			t.Errorf("%s %s без входа: %d", c[0], c[1], rec.Code)
		}
	}
}

func TestChatAdminFlow(t *testing.T) {
	e, ck, svc, pid := newChatEnv(t)
	ctx := context.Background()
	_, _ = svc.Send(ctx, chat.Sender{ID: pid, Name: "Нарушитель"}, chat.General, "", "Всем привет из теста")
	// журнал и поиск
	rec := e.raw("GET", "/api/chat/messages?q=%D0%BF%D1%80%D0%B8%D0%B2%D0%B5%D1%82", nil, "", ck)
	var msgs struct{ Items []chat.Message }
	_ = json.Unmarshal(rec.Body.Bytes(), &msgs)
	if rec.Code != 200 || len(msgs.Items) != 1 || msgs.Items[0].FromName != "Нарушитель" {
		t.Fatalf("журнал: %d %s", rec.Code, rec.Body.String())
	}
	if rec = e.raw("GET", "/api/chat/messages?channel=wings", nil, "", ck); rec.Code != 400 {
		t.Fatalf("неизвестный канал: %d", rec.Code)
	}
	// мьют по имени
	body, _ := json.Marshal(map[string]any{"player": "нарушитель", "minutes": 30, "reason": "спам"})
	if rec = e.raw("POST", "/api/chat/mutes", body, "application/json", ck); rec.Code != 200 {
		t.Fatalf("мьют: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := svc.Send(ctx, chat.Sender{ID: pid, Name: "Нарушитель"}, chat.General, "", "ещё одно"); err != chat.ErrMuted {
		t.Fatalf("после мьюта: %v", err)
	}
	var ml struct{ Items []chat.Mute }
	rec = e.raw("GET", "/api/chat/mutes", nil, "", ck)
	_ = json.Unmarshal(rec.Body.Bytes(), &ml)
	if len(ml.Items) != 1 || ml.Items[0].Reason != "спам" || ml.Items[0].Until == nil {
		t.Fatalf("список мьютов: %s", rec.Body.String())
	}
	bad, _ := json.Marshal(map[string]any{"player": "нет такого", "minutes": 5})
	if rec = e.raw("POST", "/api/chat/mutes", bad, "application/json", ck); rec.Code != 404 {
		t.Fatalf("несуществующий игрок: %d", rec.Code)
	}
	neg, _ := json.Marshal(map[string]any{"player": "нарушитель", "minutes": -1})
	if rec = e.raw("POST", "/api/chat/mutes", neg, "application/json", ck); rec.Code != 400 {
		t.Fatalf("отрицательный срок: %d", rec.Code)
	}
	if rec = e.raw("DELETE", "/api/chat/mutes/"+pid, nil, "", ck); rec.Code != 200 {
		t.Fatalf("снятие: %d", rec.Code)
	}
	if ms, _ := svc.Mutes(ctx); len(ms) != 0 {
		t.Fatalf("мьют не снят: %+v", ms)
	}
	// настройки: чтение, правка, проверка диапазонов
	rec = e.raw("GET", "/api/chat/settings", nil, "", ck)
	var st chat.Settings
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if rec.Code != 200 || st.RetentionDays != 7 || st.MinIntervalMs != 2000 || !st.ProfanityFilter {
		t.Fatalf("настройки по умолчанию: %s", rec.Body.String())
	}
	st.MinIntervalMs = 500
	st.ExtraBlocked = []string{"синхрофазотрон"}
	b2, _ := json.Marshal(st)
	if rec = e.raw("PUT", "/api/chat/settings", b2, "application/json", ck); rec.Code != 200 || svc.Settings().MinIntervalMs != 500 {
		t.Fatalf("правка настроек: %d", rec.Code)
	}
	st.MaxLen = 3
	b3, _ := json.Marshal(st)
	if rec = e.raw("PUT", "/api/chat/settings", b3, "application/json", ck); rec.Code != 422 {
		t.Fatalf("некорректные настройки: %d", rec.Code)
	}
	var n int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_log WHERE action IN ('chat_mute','chat_unmute','chat_settings')`).Scan(&n)
	if n != 3 {
		t.Fatalf("записей журнала %d", n)
	}
}
