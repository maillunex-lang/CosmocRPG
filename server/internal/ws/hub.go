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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/account"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shop"
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
	World      *world.World     // игровой мир; nil: игровые сообщения отключены (Этап 00)
	Store      world.Store      // хранилище игроков; nil: позиции не сохраняются
	Quests     *quest.Service   // контракты и награды; nil: функция выключена
	Accounts   *account.Service // аккаунты (логин и пароль); nil: функция выключена
	Shop       *shop.Service    // VIP и магазин косметики; nil: функция выключена
	// RequireCharacter: новый игрок сначала создаёт персонажа (ник, пол, направление пилота), и только потом появляется в мире.
	RequireCharacter bool
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
	conn    *websocket.Conn
	cancel  context.CancelFunc
	states  chan Message
	pending *world.Record // игрок вошёл, но ещё не создал персонажа
	events  chan Message  // редкие события (выполнение контракта): не выбрасываются как устаревшие снимки
	pid     string
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
		case "register", "login":
			h.accountMsg(ctx, c, ss, r.RemoteAddr, m)
		case "join", "create_character", "move", "target", "autopilot", "attack", "ability", "set_skin", "set_spec", "mine", "mine_stop", "sell", "ships", "set_ship", "shop", "shop_buy", "shop_equip":
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
		if h.opts.Store != nil && h.opts.RequireCharacter && !rec.Created {
			// персонаж не создан: в мир не пускаем, предлагаем создать (ник, пол, направление пилота)
			r := rec
			ss.pending = &r
			reply(Message{Type: "welcome", ID: m.ID, Payload: mustJSON(map[string]any{
				"player_id": rec.ID, "name": rec.Name, "needs_character": true, "pilot_options": h.pilotOptions(),
			})})
			return true
		}
		return h.finishJoin(ctx, ss, log, m, rec)
	}
	if m.Type == "create_character" {
		return h.createCharacter(ctx, c, ss, log, m)
	}
	if ss.pid == "" {
		reply(errMsg(m.ID, "not_joined", "сначала выполните join"))
		return true
	}
	switch m.Type {
	case "attack", "ability", "set_skin", "set_spec", "mine", "mine_stop", "sell", "ships", "set_ship":
		h.gameShip(ctx, c, ss, m)
	case "shop", "shop_buy", "shop_equip":
		h.shopMsg(ctx, c, ss, m)
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
	case "mine":
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.ID == "" || len(p.ID) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректное месторождение"))
			return
		}
		if err := w.StartMining(ss.pid, p.ID); err != nil {
			fail(err)
		}
	case "mine_stop":
		w.StopMining(ss.pid)
	case "ships":
		reply(Message{Type: "ships", ID: m.ID, Payload: mustJSON(map[string]any{"ships": w.Fleet()})})
	case "set_ship":
		var p struct {
			Ship string `json:"ship"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Ship == "" || len(p.Ship) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректный корабль"))
			return
		}
		rec, err := w.SetShip(ss.pid, p.Ship)
		if err != nil {
			fail(err)
			return
		}
		h.save(rec) // смена корабля сохраняется сразу
	case "sell":
		if _, err := w.SellCargo(ss.pid); err != nil {
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
	case "no_module":
		return "на корабле нет добывающего модуля"
	case "hold_full":
		return "грузовой отсек полон"
	case "deposit_empty":
		return "месторождение истощено"
	case "no_deposit":
		return "такого месторождения нет"
	case "not_at_station":
		return "продавать груз можно только у станции"
	case "nothing_to_sell":
		return "в грузовом отсеке пусто"
	case "no_ship":
		return "такого корабля нет"
	case "hold_too_big":
		return "груз не поместится в этот корабль: сначала продайте его"
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
	if rs := w.DrainRewards(); len(rs) > 0 && h.opts.Quests != nil {
		go h.grantRewards(rs) // запись в базу не должна тормозить цикл симуляции
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

// grantRewards выдаёт награды локаций идемпотентно (ключ события) и сообщает игроку о зачислении.
func (h *Handler) grantRewards(rs []world.Reward) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, r := range rs {
		granted, total, err := h.opts.Quests.Grant(ctx, r.Player, r.Key, r.Credits)
		if err != nil {
			h.log.Error("награда локации", "player", r.Player, "key", r.Key, "err", err)
			continue
		}
		if !granted {
			continue
		}
		h.log.Info("награда выдана", "player", r.Player, "kind", r.Kind, "ref", r.Ref, "credits", r.Credits, "total", total)
		h.mu.Lock()
		ss := h.active[r.Player]
		h.mu.Unlock()
		if ss == nil {
			continue
		}
		select {
		case ss.events <- Message{Type: "reward", Payload: mustJSON(map[string]any{"kind": r.Kind, "ref": r.Ref, "credits": r.Credits, "total": total})}:
		default:
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

// accountMsg обрабатывает register и login (до входа в мир). Пароли не логируются.
func (h *Handler) accountMsg(ctx context.Context, c *websocket.Conn, ss *session, remote string, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if h.opts.Accounts == nil {
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	if ss.pid != "" {
		reply(errMsg(m.ID, "already_joined", "вход уже выполнен"))
		return
	}
	var p struct {
		Login    string `json:"login"`
		Password string `json:"password"`
		Token    string `json:"token"`
		Name     string `json:"name"`
	}
	if json.Unmarshal(m.Payload, &p) != nil {
		reply(errMsg(m.ID, "bad_message", "некорректный запрос"))
		return
	}
	ip := remote
	if i := strings.LastIndex(remote, ":"); i > 0 {
		ip = remote[:i]
	}
	actx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fail := func(err error) {
		code := err.Error()
		text := map[string]string{
			"bad_login": "логин: 3-32 символа, латиница, цифры, «_», «.», «-»", "weak_password": "пароль: от 8 до 128 символов, не повторяющийся символ и не логин",
			"bad_credentials": "неверный логин или пароль", "too_many_attempts": "слишком много попыток, попробуйте позже", "bad_token": "некорректный токен",
			"login_taken": "такой логин уже занят", "already_has_account": "у этого игрока уже есть аккаунт",
		}[code]
		if text == "" {
			h.log.Error("ws: аккаунт", "err", err)
			code, text = "server_error", "ошибка сервера"
		}
		reply(errMsg(m.ID, code, text))
	}
	switch m.Type {
	case "register":
		_, login, err := h.opts.Accounts.Register(actx, ip, p.Token, p.Name, p.Login, p.Password)
		if err != nil {
			fail(err)
			return
		}
		h.log.Info("ws: аккаунт создан", "login", login)
		reply(Message{Type: "account", ID: m.ID, Payload: mustJSON(map[string]any{"login": login, "created": true})})
	case "login":
		login, pid, err := h.opts.Accounts.Login(actx, ip, p.Token, p.Login, p.Password)
		if err != nil {
			fail(err)
			return
		}
		h.log.Info("ws: вход по аккаунту", "login", login, "player", pid)
		reply(Message{Type: "account", ID: m.ID, Payload: mustJSON(map[string]any{"login": login, "created": false})})
	}
}

// finishJoin регистрирует сессию, добавляет корабль в мир и отправляет welcome.
func (h *Handler) finishJoin(ctx context.Context, ss *session, log *slog.Logger, m Message, rec world.Record) bool {
	w := h.opts.World
	reply := func(msg Message) { _ = writeJSON(ctx, ss.conn, msg) }
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
	h.applyCosmetics(ctx, rec.ID)
	sys := w.System()
	welcome := map[string]any{
		"player_id": rec.ID, "name": rec.Name, "system": sys,
		"character": map[string]any{"gender": rec.Gender, "pilot_spec": rec.PilotSpec, "ship_type": rec.ShipType},
	}
	if lay, wcfg := w.Layout(); lay.System != "" {
		// раскладка системы и визуальные параметры: клиент строит декор по seed, серверные объекты приходят в state
		welcome["layout"] = lay
		welcome["visual"] = wcfg.Visual
	}
	reply(Message{Type: "welcome", ID: m.ID, Payload: mustJSON(welcome)})
	go h.pump(ctx, ss)
	if st, ok := w.Snapshot(ss.pid); ok {
		h.pushState(ss, st)
	}
	log.Info("ws: игрок вошёл", "player", rec.ID, "name", rec.Name)
	return true
}

// pilotOptions описывает стартовые направления пилота и корабль каждого (для экрана создания персонажа).
func (h *Handler) pilotOptions() []map[string]any {
	cat := h.opts.World.Catalog()
	var out []map[string]any
	for _, ps := range world.PilotSpecs {
		st, ok := cat.Starters[ps]
		if !ok {
			continue
		}
		sp, _ := cat.Spec(st.Ship)
		out = append(out, map[string]any{"pilot": ps, "ship": st.Ship, "ship_name": sp.Name, "class": sp.Class, "role": sp.Role,
			"temporary": sp.Temporary, "skin": st.Skin, "spec": st.Spec, "hull": sp.Hull, "shield": sp.Shield, "cargo": sp.Cargo, "speed": sp.Speed})
	}
	return out
}

// createCharacter завершает создание персонажа и впускает игрока в мир.
func (h *Handler) createCharacter(ctx context.Context, c *websocket.Conn, ss *session, log *slog.Logger, m Message) bool {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if ss.pending == nil {
		reply(errMsg(m.ID, "not_joined", "создание персонажа недоступно"))
		return true
	}
	var p struct {
		Nick      string `json:"nick"`
		Gender    string `json:"gender"`
		PilotSpec string `json:"pilot_spec"`
	}
	if json.Unmarshal(m.Payload, &p) != nil {
		reply(errMsg(m.ID, "bad_message", "некорректный запрос"))
		return true
	}
	nick, err := world.CleanNick(p.Nick)
	if err == nil && !world.ValidGender(p.Gender) {
		err = world.ErrBadGender
	}
	st, ok := h.opts.World.Starter(p.PilotSpec)
	if err == nil && !ok {
		err = world.ErrBadPilotSpec
	}
	var rec world.Record
	if err == nil {
		sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		rec, err = h.opts.Store.CreateCharacter(sctx, ss.pending.ID, nick, p.Gender, p.PilotSpec, st)
		cancel()
	}
	if err != nil {
		code := err.Error()
		text := map[string]string{
			"bad_nick": "ник: 3-16 символов, буквы (латиница и кириллица), цифры, «_» и «-» внутри", "nick_reserved": "этот ник недоступен",
			"nick_taken": "такой ник уже занят", "bad_gender": "выберите пол", "bad_pilot_spec": "выберите стартовое направление",
			"character_exists": "персонаж уже создан",
		}[code]
		if text == "" {
			log.Error("ws: создание персонажа", "err", err)
			code, text = "server_error", "ошибка сервера"
		}
		reply(errMsg(m.ID, code, text))
		return true
	}
	ss.pending = nil
	log.Info("ws: персонаж создан", "player", rec.ID, "nick", rec.Name, "pilot", rec.PilotSpec, "ship", rec.ShipType)
	return h.finishJoin(ctx, ss, log, m, rec)
}

// applyCosmetics загружает покупки игрока и применяет внешний вид к его кораблю в мире.
func (h *Handler) applyCosmetics(ctx context.Context, pid string) {
	if h.opts.Shop == nil {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	st, err := h.opts.Shop.State(cctx, pid)
	if err != nil {
		h.log.Error("магазин: состояние игрока", "player", pid, "err", err)
		return
	}
	h.setCosmetics(pid, st)
}

func (h *Handler) setCosmetics(pid string, st shop.State) shop.Cosmetics {
	c := h.opts.Shop.Cosmetics(st)
	h.opts.World.SetCosmetics(pid, world.Cosmetics{Vip: c.Vip, Badge: c.Badge, NameColor: c.NameColor, Engine: c.Engine})
	return c
}

func shopText(code string) string {
	switch code {
	case "no_item":
		return "такого товара нет"
	case "already_owned":
		return "товар уже куплен"
	case "vip_required":
		return "нужен более высокий VIP-уровень"
	case "not_owned":
		return "сначала купите этот товар"
	case "bad_slot":
		return "товар не подходит к этому слоту"
	case "bad_request":
		return "некорректный запрос"
	}
	return "операция отклонена"
}

// shopMsg обрабатывает shop (каталог и состояние), shop_buy (покупка) и shop_equip (экипировка). Сервер единственный источник результата.
func (h *Handler) shopMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if h.opts.Shop == nil {
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	var st shop.State
	var err error
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch m.Type {
	case "shop":
		st, err = h.opts.Shop.State(sctx, ss.pid)
	case "shop_buy":
		var p struct {
			Item      string `json:"item"`
			RequestID string `json:"request_id"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Item == "" || len(p.Item) > 48 {
			reply(errMsg(m.ID, "bad_message", "некорректная покупка"))
			return
		}
		st, err = h.opts.Shop.Buy(sctx, ss.pid, p.Item, p.RequestID)
	case "shop_equip":
		var p struct {
			Slot string `json:"slot"`
			Item string `json:"item"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || len(p.Slot) > 16 || len(p.Item) > 48 {
			reply(errMsg(m.ID, "bad_message", "некорректная команда"))
			return
		}
		st, err = h.opts.Shop.Equip(sctx, ss.pid, p.Slot, p.Item)
	}
	if err != nil {
		code := err.Error()
		switch err {
		case shop.ErrNoItem, shop.ErrOwned, shop.ErrVipNeeded, shop.ErrNotOwned, shop.ErrBadSlot, shop.ErrBadReq:
		default:
			h.log.Error("магазин", "player", ss.pid, "type", m.Type, "err", err)
			code = "internal"
		}
		reply(errMsg(m.ID, code, shopText(code)))
		return
	}
	cos := h.setCosmetics(ss.pid, st)
	reply(Message{Type: "shop", ID: m.ID, Payload: mustJSON(map[string]any{"catalog": h.opts.Shop.Config(), "state": st, "cosmetics": cos})})
}
