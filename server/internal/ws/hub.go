// Package ws — WebSocket-сессии игроков: проверка команд, ограничение частоты.
package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// ProtocolVersion — версия игрового протокола (сообщение join).
const ProtocolVersion = 1

const (
	tickHz       = 20
	stateEvery   = 2 // состояние рассылается каждый второй тик: 10 Гц
	saveInterval = 5 * time.Second
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
	World      *world.World   // игровой мир; nil: игровые сообщения отключены (Этап 00)
	Store      world.Store    // хранилище игроков; nil: позиции не сохраняются
	Quests     *quest.Service // контракты и награды; nil: функция выключена
}

// Handler обслуживает WebSocket-соединения.
type Handler struct {
	log      *slog.Logger
	opts     Options
	sessions atomic.Int64

	mu     sync.Mutex
	active map[string]*session // публичный id игрока -> его текущая сессия
}

// session — игровая сессия: состояние рассылается через очередь, чтобы медленный клиент не тормозил мир.
type session struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
	states chan Message
	events chan Message // редкие события (выполнение контракта): не выбрасываются как устаревшие снимки
	pid    string
}

// NewHandler создаёт обработчик.
func NewHandler(log *slog.Logger, opts Options) *Handler {
	return &Handler{log: log, opts: opts, active: map[string]*session{}}
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

	ss := &session{conn: c, cancel: cancel, states: make(chan Message, 2), events: make(chan Message, 16)}
	defer h.leave(ss, log)

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
		case "join", "move", "target", "autopilot", "attack", "ability", "set_skin", "set_spec":
			if h.opts.World == nil {
				_ = writeJSON(ctx, c, errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
				continue
			}
			if !h.game(ctx, c, ss, log, m) {
				c.CloseNow()
				return
			}
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

// game обрабатывает игровые сообщения. false: соединение нужно закрыть.
func (h *Handler) game(ctx context.Context, c *websocket.Conn, ss *session, log *slog.Logger, m Message) bool {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	w := h.opts.World
	if m.Type == "join" {
		if ss.pid != "" {
			reply(errMsg(m.ID, "already_joined", "вход уже выполнен"))
			return true
		}
		var p struct {
			Token    string `json:"token"`
			Protocol int    `json:"protocol"`
			Name     string `json:"name"`
		}
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			reply(errMsg(m.ID, "bad_message", "некорректный join"))
			return true
		}
		if p.Protocol != ProtocolVersion {
			reply(errMsg(m.ID, "bad_protocol", "версия протокола не поддерживается: обновите игру"))
			return false
		}
		if !world.ValidToken(p.Token) {
			reply(errMsg(m.ID, "bad_token", "некорректный токен"))
			return true
		}
		var rec world.Record
		if h.opts.Store != nil {
			sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			r, err := h.opts.Store.Resolve(sctx, p.Token, p.Name)
			cancel()
			if err != nil {
				log.Error("ws: вход игрока", "err", err)
				reply(errMsg(m.ID, "server_error", "ошибка сервера"))
				return false
			}
			rec = r
		} else {
			hs := world.TokenHash(p.Token)
			rec = world.Record{ID: world.PublicID(hs), Name: world.DefaultName(hs), Dir: world.Vec{X: 1}}
			if n := world.CleanName(p.Name); n != "" {
				rec.Name = n
			}
		}
		// тот же игрок с другого устройства: старая сессия закрывается
		h.mu.Lock()
		old := h.active[rec.ID]
		h.active[rec.ID] = ss
		ss.pid = rec.ID
		h.mu.Unlock()
		if old != nil && old != ss {
			_ = writeJSON(ctx, old.conn, errMsg("", "replaced", "вход с другого устройства"))
			old.cancel()
			old.conn.CloseNow()
		}
		if _, ok := w.Join(rec); !ok {
			h.mu.Lock()
			if h.active[rec.ID] == ss {
				delete(h.active, rec.ID)
			}
			ss.pid = ""
			h.mu.Unlock()
			reply(errMsg(m.ID, "server_full", "сервер переполнен"))
			return false
		}
		if h.opts.Quests != nil {
			qctx, qcancel := context.WithTimeout(ctx, 5*time.Second)
			if err := h.opts.Quests.Join(qctx, rec.ID); err != nil {
				log.Error("ws: контракты игрока", "player", rec.ID, "err", err)
			}
			qcancel()
		}
		sys := w.System()
		reply(Message{Type: "welcome", ID: m.ID, Payload: mustJSON(map[string]any{
			"player_id": rec.ID, "name": rec.Name, "system": sys,
		})})
		go h.pump(ctx, ss)
		if st, ok := w.Snapshot(ss.pid); ok {
			h.pushState(ss, st)
		}
		log.Info("ws: игрок вошёл", "player", rec.ID, "name", rec.Name)
		return true
	}
	if ss.pid == "" {
		reply(errMsg(m.ID, "not_joined", "сначала выполните join"))
		return true
	}
	switch m.Type {
	case "attack", "ability", "set_skin", "set_spec":
		h.gameShip(ctx, c, ss, m)
	case "move":
		var p struct{ X, Y float64 }
		if err := json.Unmarshal(m.Payload, &p); err != nil || math.IsNaN(p.X+p.Y) || math.IsInf(p.X+p.Y, 0) {
			reply(errMsg(m.ID, "bad_message", "некорректное движение"))
			return true
		}
		w.SetMove(ss.pid, world.Vec{X: p.X, Y: p.Y})
	case "target":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			reply(errMsg(m.ID, "bad_message", "некорректная цель"))
			return true
		}
		if !w.SetTarget(ss.pid, p.ID) {
			reply(errMsg(m.ID, "bad_target", "такой цели нет"))
		}
	case "autopilot":
		var p struct {
			On bool `json:"on"`
		}
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			reply(errMsg(m.ID, "bad_message", "некорректная команда"))
			return true
		}
		if !w.SetAutopilot(ss.pid, p.On) {
			reply(errMsg(m.ID, "no_target", "для автопилота выберите цель"))
		}
	}
	return true
}

func (h *Handler) gameShip(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	w := h.opts.World
	fail := func(err error) {
		code := err.Error()
		switch err {
		case world.ErrNoPlayer:
			code = "not_joined"
		}
		reply(errMsg(m.ID, code, abilityText(code)))
	}
	switch m.Type {
	case "attack":
		var p struct {
			On bool `json:"on"`
		}
		if json.Unmarshal(m.Payload, &p) != nil {
			reply(errMsg(m.ID, "bad_message", "некорректная команда"))
			return
		}
		if err := w.SetAttack(ss.pid, p.On); err != nil {
			fail(err)
		}
	case "ability":
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.ID == "" || len(p.ID) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректная способность"))
			return
		}
		if err := w.UseAbility(ss.pid, p.ID); err != nil {
			fail(err)
		}
	case "set_skin", "set_spec":
		var p struct {
			Value string `json:"value"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Value == "" || len(p.Value) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректное значение"))
			return
		}
		var rec world.Record
		var err error
		if m.Type == "set_skin" {
			rec, err = w.SetSkin(ss.pid, p.Value)
		} else {
			rec, err = w.SetSpec(ss.pid, p.Value)
		}
		if err != nil {
			fail(err)
			return
		}
		h.save(rec) // выбор сохраняется сразу
	}
}

func abilityText(code string) string {
	switch code {
	case "cooldown":
		return "способность перезаряжается"
	case "no_energy":
		return "не хватает энергии"
	case "no_target":
		return "выберите цель"
	case "out_of_range":
		return "цель слишком далеко"
	case "wrong_spec":
		return "способность другой специализации"
	case "dead":
		return "корабль уничтожен"
	case "unknown_ability":
		return "неизвестная способность"
	case "bad_skin":
		return "такого облика нет"
	case "bad_spec":
		return "такой специализации нет"
	case "bad_target":
		return "эту цель атаковать нельзя"
	}
	return "команда отклонена"
}

// pump пересылает состояние из очереди в соединение до его закрытия.
func (h *Handler) pump(ctx context.Context, ss *session) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-ss.states:
			if err := writeJSON(ctx, ss.conn, m); err != nil {
				ss.cancel()
				return
			}
		case m := <-ss.events:
			if err := writeJSON(ctx, ss.conn, m); err != nil {
				ss.cancel()
				return
			}
		}
	}
}

// pushState кладёт снимок в очередь; если клиент отстал, старый снимок выбрасывается.
func (h *Handler) pushState(ss *session, st world.State) {
	out := struct {
		world.State
		Credits *int64            `json:"credits,omitempty"`
		Quests  []quest.QuestView `json:"quests,omitempty"`
	}{State: st}
	if h.opts.Quests != nil {
		if v, ok := h.opts.Quests.View(ss.pid); ok {
			out.Credits = &v.Credits
			out.Quests = v.Quests
		}
	}
	m := Message{Type: "state", Payload: mustJSON(out)}
	for {
		select {
		case ss.states <- m:
			return
		default:
			select {
			case <-ss.states:
			default:
			}
		}
	}
}

// leave вызывается при закрытии сессии: корабль остаётся в мире на время Grace, позиция сохраняется.
func (h *Handler) leave(ss *session, log *slog.Logger) {
	if ss.pid == "" {
		return
	}
	h.mu.Lock()
	current := h.active[ss.pid] == ss
	if current {
		delete(h.active, ss.pid)
	}
	h.mu.Unlock()
	if !current {
		return // игрока уже ведёт новая сессия
	}
	if h.opts.Quests != nil {
		h.opts.Quests.Leave(ss.pid)
	}
	if rec, ok := h.opts.World.Disconnect(ss.pid, time.Now()); ok {
		h.save(rec)
	}
	log.Info("ws: игрок отключился", "player", ss.pid)
}

func (h *Handler) save(rec world.Record) {
	if h.opts.Store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.opts.Store.Save(ctx, rec); err != nil {
		h.log.Error("сохранение игрока", "player", rec.ID, "err", err)
	}
}

// Tick продвигает мир и (на каждом stateEvery-м тике) рассылает состояние. Вызывается циклом Run.
func (h *Handler) Tick(n int, now time.Time) {
	w := h.opts.World
	for _, rec := range w.Step(1.0/tickHz, now) {
		h.save(rec)
	}
	evs := w.DrainEvents()
	if len(evs) > 0 {
		h.broadcastEvents(evs)
	}
	if n%stateEvery != 0 {
		return
	}
	type target struct {
		s   *session
		pid string
	}
	h.mu.Lock()
	targets := make([]target, 0, len(h.active))
	for pid, s := range h.active {
		targets = append(targets, target{s, pid})
	}
	h.mu.Unlock()
	for _, t := range targets {
		st, ok := w.Snapshot(t.pid)
		if !ok {
			continue
		}
		if h.opts.Quests != nil {
			h.evaluateQuests(t.s, t.pid, st)
		}
		h.pushState(t.s, st)
	}
}

// Run ведёт симуляцию мира и периодическое сохранение до отмены ctx; при остановке сохраняет всех.
func (h *Handler) Run(ctx context.Context) {
	if h.opts.World == nil {
		return
	}
	tick := time.NewTicker(time.Second / tickHz)
	defer tick.Stop()
	save := time.NewTicker(saveInterval)
	defer save.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			for _, rec := range h.opts.World.Records() {
				h.save(rec)
			}
			return
		case now := <-tick.C:
			n++
			h.Tick(n, now)
		case <-save.C:
			for _, rec := range h.opts.World.Records() {
				h.save(rec)
			}
		}
	}
}

// evaluateQuests проверяет контракты игрока и сообщает о выполненных. Награду выдаёт сервис (атомарно и однократно).
func (h *Handler) evaluateQuests(ss *session, pid string, st world.State) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, c := range h.opts.Quests.Evaluate(ctx, pid, world.Vec{X: st.Ship.Pos[0], Y: st.Ship.Pos[1]}) {
		m := Message{Type: "quest_completed", Payload: mustJSON(map[string]any{
			"id": c.QuestID, "title": c.Title, "reward": map[string]int64{"credits": c.Credits}, "credits": c.Total,
		})}
		select {
		case ss.events <- m:
		default:
			h.log.Warn("ws: очередь событий переполнена", "player", pid)
		}
	}
}

// broadcastEvents рассылает боевые события всем подключённым игрокам (визуальные эффекты; решения уже приняты сервером).
func (h *Handler) broadcastEvents(evs []world.Event) {
	h.mu.Lock()
	sessions := make([]*session, 0, len(h.active))
	for _, s := range h.active {
		sessions = append(sessions, s)
	}
	h.mu.Unlock()
	m := Message{Type: "combat_events", Payload: mustJSON(map[string]any{"events": evs})}
	for _, s := range sessions {
		select {
		case s.events <- m:
		default:
		}
	}
}
