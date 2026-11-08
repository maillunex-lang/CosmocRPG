// Package ws — WebSocket-сессии игроков: проверка команд, ограничение частоты.
package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// Message — формат всех сообщений протокола (см. docs/API.md).
type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Options — настройки обработчика.
type Options struct {
	MaxMessage int64 // максимальный размер входящего сообщения, байт
	RatePerSec int   // допустимое число сообщений в секунду на сессию
	Version    string
}

// Handler обслуживает WebSocket-соединения.
type Handler struct {
	log      *slog.Logger
	opts     Options
	sessions atomic.Int64
}

// NewHandler создаёт обработчик.
func NewHandler(log *slog.Logger, opts Options) *Handler {
	return &Handler{log: log, opts: opts}
}

// Sessions возвращает число активных сессий.
func (h *Handler) Sessions() int64 { return h.sessions.Load() }

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ServeHTTP апгрейдит соединение и ведёт сессию до закрытия.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Клиент — Godot-приложение, а не браузер, поэтому проверка Origin не применима.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		h.log.Warn("ws: отказ в апгрейде", "err", err)
		return
	}
	c.SetReadLimit(h.opts.MaxMessage)
	sid := newSessionID() // сессия одноразовая: идентификатор не переиспользуется
	h.sessions.Add(1)
	defer h.sessions.Add(-1)
	log := h.log.With("session", sid, "remote", r.RemoteAddr)
	log.Info("ws: сессия открыта")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if err := writeJSON(ctx, c, Message{Type: "hello", Payload: mustJSON(map[string]any{
		"session": sid, "server_version": h.opts.Version,
	})}); err != nil {
		c.CloseNow()
		return
	}

	limiter := newLimiter(h.opts.RatePerSec)
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			log.Info("ws: сессия закрыта", "status", int(status))
			c.CloseNow()
			return
		}
		if !limiter.allow(time.Now()) {
			_ = writeJSON(ctx, c, errMsg("", "rate_limited", "слишком много сообщений"))
			c.Close(websocket.StatusPolicyViolation, "rate limit")
			log.Warn("ws: превышен лимит частоты")
			return
		}
		var m Message
		if err := json.Unmarshal(data, &m); err != nil || m.Type == "" {
			_ = writeJSON(ctx, c, errMsg("", "bad_message", "некорректное сообщение"))
			continue
		}
		switch m.Type {
		case "ping":
			_ = writeJSON(ctx, c, Message{Type: "pong", ID: m.ID})
		default:
			_ = writeJSON(ctx, c, errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		}
	}
}

func errMsg(id, code, text string) Message {
	return Message{Type: "error", ID: id, Payload: mustJSON(map[string]string{"code": code, "message": text})}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func writeJSON(ctx context.Context, c *websocket.Conn, m Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, b)
}

// limiter — простое окно в одну секунду.
type limiter struct {
	max    int
	window time.Time
	count  int
}

func newLimiter(max int) *limiter { return &limiter{max: max} }

func (l *limiter) allow(now time.Time) bool {
	if now.Sub(l.window) >= time.Second {
		l.window = now
		l.count = 0
	}
	l.count++
	return l.count <= l.max
}
