package ws

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func chatServer(t *testing.T, tokens ...string) (*chat.Service, *chat.MemStore, string) {
	t.Helper()
	cs := chat.NewMemStore()
	svc := chat.NewService(cs)
	st := svc.Settings()
	st.MinIntervalMs = 0 // частоту проверяют отдельные тесты
	_ = svc.UpdateSettings(context.Background(), st)
	for _, tk := range tokens {
		id := world.PublicID(world.TokenHash(tk))
		cs.AddPlayer(id, "Игрок-"+id[:4])
	}
	store := world.NewMemStore()
	opts := Options{MaxMessage: 8192, RatePerSec: 200, Version: "t", World: world.New(), Store: store, Chat: svc}
	srv, h := newServer(t, opts)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return svc, cs, srv.URL
}

type chatPush struct {
	Message chat.Message `json:"message"`
}

func (cl *client) nextChat() chat.Message {
	cl.t.Helper()
	m := cl.next("chat")
	var p chatPush
	_ = json.Unmarshal(m.Payload, &p)
	return p.Message
}

func (cl *client) errCode() string {
	cl.t.Helper()
	m := cl.next("error")
	var e struct{ Code string }
	_ = json.Unmarshal(m.Payload, &e)
	return e.Code
}

func TestGeneralChatReachesEveryone(t *testing.T) {
	_, _, url := chatServer(t, "chatwsaaaaaaaaaaaa1", "chatwsbbbbbbbbbbbb2")
	a, b := connect(t, url), connect(t, url)
	a.joinRaw("chatwsaaaaaaaaaaaa1")
	b.joinRaw("chatwsbbbbbbbbbbbb2")
	a.send("chat_send", map[string]any{"channel": "general", "text": "Всем привет!"})
	ma, mb := a.nextChat(), b.nextChat()
	if ma.Text != "Всем привет!" || mb.Text != "Всем привет!" || mb.FromID != world.PublicID(world.TokenHash("chatwsaaaaaaaaaaaa1")) || mb.Channel != "general" {
		t.Fatalf("общий чат: %+v / %+v", ma, mb)
	}
	// игрок не может подделать отправителя: имя и id берутся из сессии
	a.send("chat_send", map[string]any{"channel": "general", "text": "второе", "from_name": "Админ", "from_id": "x"})
	m2 := b.nextChat()
	if m2.FromName == "Админ" || m2.FromID == "x" {
		t.Fatalf("подмена отправителя: %+v", m2)
	}
}

func TestDirectMessageOnlyForParticipants(t *testing.T) {
	_, cs, url := chatServer(t, "chatdmaaaaaaaaaaaa1", "chatdmbbbbbbbbbbbb2", "chatdmcccccccccccc3")
	a, b, c := connect(t, url), connect(t, url), connect(t, url)
	a.joinRaw("chatdmaaaaaaaaaaaa1")
	b.joinRaw("chatdmbbbbbbbbbbbb2")
	c.joinRaw("chatdmcccccccccccc3")
	bid := world.PublicID(world.TokenHash("chatdmbbbbbbbbbbbb2"))
	a.send("chat_send", map[string]any{"channel": "dm", "to": bid, "text": "Только тебе"})
	if m := a.nextChat(); m.Channel != "dm" || m.ToID != bid {
		t.Fatalf("отправитель: %+v", m)
	}
	if m := b.nextChat(); m.Text != "Только тебе" {
		t.Fatalf("получатель: %+v", m)
	}
	// третий игрок ничего не получает: следующее его сообщение — общее, отправленное после
	a.send("chat_send", map[string]any{"channel": "general", "text": "Общее после"})
	if m := c.nextChat(); m.Text != "Общее после" {
		t.Fatalf("третий игрок получил чужое личное сообщение: %+v", m)
	}
	// история: личные только участникам
	c.send("chat_history", map[string]any{})
	hm := c.next("chat_history")
	var h struct{ DMs []chat.Message }
	_ = json.Unmarshal(hm.Payload, &h)
	if len(h.DMs) != 0 {
		t.Fatalf("в истории третьего чужие личные: %+v", h.DMs)
	}
	b.send("chat_history", map[string]any{})
	_ = json.Unmarshal(b.next("chat_history").Payload, &h)
	if len(h.DMs) != 1 || h.DMs[0].Text != "Только тебе" {
		t.Fatalf("история получателя: %+v", h.DMs)
	}
	_ = cs
}

func TestChatErrorsAndMute(t *testing.T) {
	svc, _, url := chatServer(t, "chaterraaaaaaaaaaa1")
	a := connect(t, url)
	a.joinRaw("chaterraaaaaaaaaaa1")
	for _, c := range []struct{ text, code string }{
		{"   ", "chat_empty"}, {strings.Repeat("а", 300), "chat_too_long"}, {"сайт http://a.ru", "chat_links"}, {"ты сука", "chat_bad_words"},
	} {
		a.send("chat_send", map[string]any{"channel": "general", "text": c.text})
		if got := a.errCode(); got != c.code {
			t.Fatalf("%q: код %q, ожидался %q", c.text, got, c.code)
		}
	}
	a.send("chat_send", map[string]any{"channel": "dm", "to": "нет-такого", "text": "эй"})
	if got := a.errCode(); got != "chat_no_player" {
		t.Fatalf("адресата нет: %q", got)
	}
	a.send("chat_send", map[string]any{"channel": "wings", "text": "эй"})
	if got := a.errCode(); got != "chat_bad_channel" {
		t.Fatalf("канал: %q", got)
	}
	// заглушённый игрок
	id := world.PublicID(world.TokenHash("chaterraaaaaaaaaaa1"))
	if _, err := svc.MuteBy(context.Background(), "админ", id, 5, "тест"); err != nil {
		t.Fatal(err)
	}
	a.send("chat_send", map[string]any{"channel": "general", "text": "привет"})
	if got := a.errCode(); got != "chat_muted" {
		t.Fatalf("мьют: %q", got)
	}
	_ = svc.Unmute(context.Background(), id)
	a.send("chat_send", map[string]any{"channel": "general", "text": "привет снова"})
	if m := a.nextChat(); m.Text != "привет снова" {
		t.Fatalf("после снятия: %+v", m)
	}
}

func TestChatRequiresJoinAndDisabledWhenNotConfigured(t *testing.T) {
	_, _, url := chatServer(t, "chatjoinaaaaaaaaa1")
	a := connect(t, url)
	a.send("chat_send", map[string]any{"channel": "general", "text": "до входа"})
	if got := a.errCode(); got != "not_joined" {
		t.Fatalf("до входа: %q", got)
	}
	_, _, url2 := gameServer(t)
	b := connect(t, url2)
	b.join("chatnoneaaaaaaaaa01")
	b.send("chat_send", map[string]any{"channel": "general", "text": "привет"})
	if got := b.errCode(); got != "unknown_type" {
		t.Fatalf("чат не подключён: %q", got)
	}
}

func TestChatRateLimitThroughProtocol(t *testing.T) {
	svc, _, url := chatServer(t, "chatrateaaaaaaaaa1")
	st := svc.Settings()
	st.MinIntervalMs = 2000
	_ = svc.UpdateSettings(context.Background(), st)
	a := connect(t, url)
	a.joinRaw("chatrateaaaaaaaaa1")
	a.send("chat_send", map[string]any{"channel": "general", "text": "раз"})
	a.nextChat()
	a.send("chat_send", map[string]any{"channel": "general", "text": "два"})
	if got := a.errCode(); got != "chat_rate" {
		t.Fatalf("слишком часто: %q", got)
	}
}
