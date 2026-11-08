package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newServer(t *testing.T, opts Options) (*httptest.Server, *Handler) {
	t.Helper()
	h := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), opts)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, h
}

func dial(t *testing.T, srv *httptest.Server) (*websocket.Conn, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c, ctx
}

func read(t *testing.T, ctx context.Context, c *websocket.Conn) Message {
	t.Helper()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("чтение: %v", err)
	}
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func errCode(t *testing.T, m Message) string {
	t.Helper()
	var p map[string]string
	if err := json.Unmarshal(m.Payload, &p); err != nil {
		t.Fatal(err)
	}
	return p["code"]
}

func TestHelloHasUniqueSession(t *testing.T) {
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, Version: "v1"})
	ids := map[string]bool{}
	for i := 0; i < 2; i++ {
		c, ctx := dial(t, srv)
		m := read(t, ctx, c)
		if m.Type != "hello" {
			t.Fatalf("ожидался hello, получен %q", m.Type)
		}
		var p map[string]string
		_ = json.Unmarshal(m.Payload, &p)
		if len(p["session"]) != 32 || p["server_version"] != "v1" {
			t.Fatalf("payload: %v", p)
		}
		ids[p["session"]] = true
	}
	if len(ids) != 2 {
		t.Fatal("идентификаторы сессий совпали")
	}
}

func TestPingPong(t *testing.T) {
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100})
	c, ctx := dial(t, srv)
	read(t, ctx, c) // hello
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"ping","id":"42"}`)); err != nil {
		t.Fatal(err)
	}
	m := read(t, ctx, c)
	if m.Type != "pong" || m.ID != "42" {
		t.Fatalf("получено %+v", m)
	}
}

func TestInvalidAndUnknownMessages(t *testing.T) {
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100})
	c, ctx := dial(t, srv)
	read(t, ctx, c)
	for _, tc := range []struct{ in, code string }{
		{`не json`, "bad_message"},
		{`{"id":"1"}`, "bad_message"},
		{`{"type":"teleport","id":"7"}`, "unknown_type"},
	} {
		if err := c.Write(ctx, websocket.MessageText, []byte(tc.in)); err != nil {
			t.Fatal(err)
		}
		m := read(t, ctx, c)
		if m.Type != "error" || errCode(t, m) != tc.code {
			t.Fatalf("вход %q: получено %+v", tc.in, m)
		}
	}
}

func TestOversizedMessageClosesSession(t *testing.T) {
	srv, _ := newServer(t, Options{MaxMessage: 64, RatePerSec: 100})
	c, ctx := dial(t, srv)
	read(t, ctx, c)
	_ = c.Write(ctx, websocket.MessageText, []byte(`{"type":"ping","payload":"`+strings.Repeat("x", 500)+`"}`))
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("ожидалось закрытие соединения")
	} else if websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("статус закрытия: %v", websocket.CloseStatus(err))
	}
}

func TestRateLimitClosesSession(t *testing.T) {
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 3})
	c, ctx := dial(t, srv)
	read(t, ctx, c)
	for i := 0; i < 10; i++ {
		_ = c.Write(ctx, websocket.MessageText, []byte(`{"type":"ping"}`))
	}
	limited := false
	for i := 0; i < 20; i++ {
		_, data, err := c.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("статус закрытия: %v", websocket.CloseStatus(err))
			}
			if !limited {
				t.Fatal("соединение закрыто без сообщения rate_limited")
			}
			return
		}
		if strings.Contains(string(data), "rate_limited") {
			limited = true
		}
	}
	t.Fatal("лимит частоты не сработал")
}

func TestSessionCounter(t *testing.T) {
	srv, h := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100})
	c, ctx := dial(t, srv)
	read(t, ctx, c)
	if h.Sessions() != 1 {
		t.Fatalf("сессий: %d", h.Sessions())
	}
	c.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(2 * time.Second)
	for h.Sessions() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if h.Sessions() != 0 {
		t.Fatalf("сессия не освобождена: %d", h.Sessions())
	}
}

func TestLimiterWindow(t *testing.T) {
	l := newLimiter(2)
	now := time.Now()
	if !l.allow(now) || !l.allow(now) || l.allow(now) {
		t.Fatal("лимит 2 в секунду не соблюдён")
	}
	if !l.allow(now.Add(1100 * time.Millisecond)) {
		t.Fatal("после окна лимит должен сброситься")
	}
}
