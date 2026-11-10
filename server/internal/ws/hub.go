// Package ws — WebSocket-сессии игроков: проверка команд, ограничение частоты.
package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/account"
	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
	"github.com/maillunex-lang/CosmocRPG/server/internal/corp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/location"
	"github.com/maillunex-lang/CosmocRPG/server/internal/pilot"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipxp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shop"
	"github.com/maillunex-lang/CosmocRPG/server/internal/social"
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
	MaxMessage    int64 // максимальный размер входящего сообщения, байт
	RatePerSec    int   // допустимое число сообщений в секунду на сессию
	Version       string
	World         *world.Router                // миры систем и маршрутизация игроков (Э-1); nil: игровые сообщения отключены
	Equipment     equipment.StarterStore       // persistent starter; nil preserves legacy fixed equipment
	Fitting       *equipment.Service           // module inventory; nil hides the hangar bag
	Store         world.Store                  // хранилище игроков; nil: позиции не сохраняются
	Quests        *quest.Service               // контракты и награды; nil: функция выключена
	Locations     location.Store               // журнал прибытий; nil: не пишется в базу
	Accounts      *account.Service             // аккаунты (логин и пароль); nil: функция выключена
	Shop          *shop.Service                // VIP и магазин косметики; nil: функция выключена
	Chat          *chat.Service                // чат (общий и личные сообщения); nil: функция выключена
	Social        *social.Service              // список друзей; nil: функция выключена
	Corp          *corp.Service                // корпорации минимум (R-64); nil: функция выключена
	Pilot         *pilot.Service               // опыт, уровень и характеристики пилота; nil: функция выключена
	Ships         *shipxp.Service              // опыт и уровни экземпляров кораблей; nil: функция выключена
	ShipPassports *shipcatalog.PassportCatalog // optional read-only RPG metadata; first integration is FTR-001 only
	OnChat        func(chat.Message)           // наблюдатель отправленных сообщений (ответы ИИ-капитанов); может быть nil
	// RequireCharacter: новый игрок сначала создаёт персонажа (ник, пол, направление пилота), и только потом появляется в мире.
	AIActorGuard     func(context.Context, string, [32]byte) error // проверка lease перед каждой AI-командой
	RequireCharacter bool
}

// Handler обслуживает WebSocket-соединения.
type Handler struct {
	log      *slog.Logger
	opts     Options
	sessions atomic.Int64

	mu           sync.Mutex
	active       map[string]*session // публичный id игрока -> его текущая сессия
	operationsMu sync.Mutex
	operations   map[string]*playerOperation
}

// session — игровая сессия: состояние рассылается через очередь, чтобы медленный клиент не тормозил мир.
type session struct {
	purchaseSession string
	ai              bool
	tokenHash       [32]byte
	conn            *websocket.Conn
	cancel          context.CancelFunc
	states          chan Message
	pending         *world.Record // игрок вошёл, но ещё не создал персонажа
	events          chan Message  // редкие события (выполнение контракта): не выбрасываются как устаревшие снимки
	pid             string
	name            string       // игровое имя (для чата)
	seenSystem      string       // последняя система, о которой клиент уже знает
	chat            chan Message // сообщения чата: отдельная очередь, чтобы не мешать снимкам состояния
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

	ss := &session{purchaseSession: newSessionID(), conn: c, cancel: cancel, states: make(chan Message, 2), events: make(chan Message, 16), chat: make(chan Message, 64)}
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
		case "join", "create_character", "move", "target", "waypoint", "autopilot", "attack", "ability", "set_skin", "set_spec", "mine", "mine_stop", "sell", "ships", "set_ship", "set_option", "shop", "shop_buy", "shop_equip", "chat_send", "chat_history", "pilot", "pilot_spend", "jump", "market", "repair", "buy", "purchase_status", "ship_upgrade", "corp_create", "corp_join", "corp_leave", "corp_info", "alliance_found", "alliance_join", "alliance_vote", "equipment_set", "equipment_get", "fitting_get", "fitting_set", "talk", "party_invite", "party_join", "party_leave", "party_start", "party_arm", "party_ready", "party_follow", "party_fill", "friend_add", "friend_remove", "friend_list":
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
		ss.tokenHash = world.TokenHash(p.Token)
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
		if strings.HasPrefix(p.Token, world.AIServiceTokenPrefix) && !rec.AI {
			reply(errMsg(m.ID, "ai_gateway_unavailable", "вход ИИ не настроен"))
			return false
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
	unlock, err := h.lockPlayer(ctx, ss.pid)
	if err != nil {
		return false
	}
	defer unlock()
	h.mu.Lock()
	current := h.active[ss.pid] == ss
	h.mu.Unlock()
	if !current {
		reply(errMsg(m.ID, "replaced", "сессия заменена"))
		return false
	}
	if ss.ai {
		if h.opts.AIActorGuard == nil || h.opts.AIActorGuard(ctx, ss.pid, ss.tokenHash) != nil {
			reply(errMsg(m.ID, "ai_session_invalid", "сессия ИИ недействительна"))
			return false
		}
		switch m.Type {
		case "move", "target", "autopilot", "mine", "mine_stop", "attack", "pilot", "ships", "market", "buy", "purchase_status":
		default:
			reply(errMsg(m.ID, "capability_unavailable", "команда не подключена для ИИ"))
			return true
		}
	}
	pw := h.opts.World.For(ss.pid)
	if pw == nil {
		reply(errMsg(m.ID, "not_joined", "вы не в мире"))
		return true
	}
	switch m.Type {
	case "equipment_set", "equipment_get":
		h.equipmentMsg(ctx, ss, m)
	case "fitting_get", "fitting_set":
		h.fittingMsg(ctx, ss, m)
	case "buy", "purchase_status":
		h.purchaseMsg(ctx, ss, m)
	case "attack", "ability", "set_skin", "set_spec", "mine", "mine_stop", "sell", "ships", "set_ship", "set_option", "jump", "market", "repair", "ship_upgrade":
		h.gameShip(ctx, c, ss, m)
	case "shop", "shop_buy", "shop_equip":
		h.shopMsg(ctx, c, ss, m)
	case "chat_send", "chat_history":
		h.chatMsg(ctx, c, ss, m)
	case "corp_create", "corp_join", "corp_leave", "corp_info", "alliance_found", "alliance_join", "alliance_vote":
		h.corpMsg(ctx, ss, m)
	case "pilot", "pilot_spend":
		h.pilotMsg(ctx, c, ss, m)
	case "talk":
		h.talkMsg(ctx, c, ss, m)
	case "party_invite", "party_join", "party_leave", "party_start", "party_arm", "party_ready", "party_follow", "party_fill":
		h.partyMsg(ctx, c, ss, m)
	case "friend_add", "friend_remove", "friend_list":
		h.friendMsg(ctx, c, ss, m)
	case "move":
		var p struct{ X, Y float64 }
		if err := json.Unmarshal(m.Payload, &p); err != nil || math.IsNaN(p.X+p.Y) || math.IsInf(p.X+p.Y, 0) {
			reply(errMsg(m.ID, "bad_message", "некорректное движение"))
			return true
		}
		if !pw.SetMove(ss.pid, world.Vec{X: p.X, Y: p.Y}) {
			reply(errMsg(m.ID, "bad_move", "движение недоступно"))
		} else {
			h.aiAck(ctx, c, ss, m)
		}
	case "target":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			reply(errMsg(m.ID, "bad_message", "некорректная цель"))
			return true
		}
		if !pw.SetTarget(ss.pid, p.ID) {
			reply(errMsg(m.ID, "bad_target", "такой цели нет"))
		} else {
			h.aiAck(ctx, c, ss, m)
		}
	case "waypoint":
		var p struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		if err := json.Unmarshal(m.Payload, &p); err != nil || math.IsNaN(p.X+p.Y) || math.IsInf(p.X+p.Y, 0) {
			reply(errMsg(m.ID, "bad_message", "некорректная точка"))
			return true
		}
		if !pw.SetWaypoint(ss.pid, world.Vec{X: p.X, Y: p.Y}) {
			reply(errMsg(m.ID, "bad_move", "маршрут недоступен"))
		} else {
			h.aiAck(ctx, c, ss, m)
		}
	case "autopilot":
		var p struct {
			On bool `json:"on"`
		}
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			reply(errMsg(m.ID, "bad_message", "некорректная команда"))
			return true
		}
		if !pw.SetAutopilot(ss.pid, p.On) {
			reply(errMsg(m.ID, "no_target", "для автопилота выберите цель"))
		} else {
			h.aiAck(ctx, c, ss, m)
		}
	}
	return true
}

func (h *Handler) gameShip(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	hww := h.opts.World.For(ss.pid)
	if hww == nil {
		reply(errMsg(m.ID, "not_joined", "вы не в мире"))
		return
	}
	sctx, scancel := context.WithTimeout(ctx, 5*time.Second)
	defer scancel()
	w := hww
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
		} else {
			h.aiAck(ctx, c, ss, m)
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
		} else {
			h.aiAck(ctx, c, ss, m)
		}
	case "mine_stop":
		w.StopMining(ss.pid)
		h.aiAck(ctx, c, ss, m)
	case "market":
		// Э-2: цены покупки ресурсов, стоимость ремонта и улучшения у станции
		prices := map[string]float64{}
		cfg := h.opts.World.For(ss.pid)
		if cfg != nil {
			for name := range marketResources {
				if pr, ok := cfg.ResourcePrice(name); ok {
					prices[name] = pr
				}
			}
		}
		var repairCost, upgradeCost int64 = -1, -1
		if c, err := h.opts.World.For(ss.pid).RepairCost(ss.pid); err == nil {
			repairCost = c
		}
		if lvl, ok := h.opts.World.For(ss.pid).ShipLevelOf(ss.pid); ok {
			if lvl < 50 {
				upgradeCost = int64(lvl) * 1000
			}
		}
		reply(Message{Type: "market", ID: m.ID, Payload: mustJSON(map[string]any{"prices": prices, "repair_cost": repairCost, "upgrade_cost": upgradeCost})})
	case "repair":
		cost, err := h.opts.World.For(ss.pid).RepairCost(ss.pid)
		if err != nil {
			reply(errMsg(m.ID, stationCode(err), stationText(err)))
			return
		}
		if cost <= 0 {
			reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "repair", "credits": 0})})
			return
		}
		total, err := h.opts.Quests.Spend(sctx, ss.pid, cost)
		if err != nil {
			code := "no_credits"
			if err != quest.ErrNotEnough {
				h.log.Error("ремонт", "player", ss.pid, "err", err)
				code = "internal"
			}
			reply(errMsg(m.ID, code, "не хватает кредитов на ремонт"))
			return
		}
		if err := h.opts.World.For(ss.pid).Repair(ss.pid); err != nil {
			reply(errMsg(m.ID, stationCode(err), stationText(err)))
			return
		}
		reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "repair", "credits": total, "spent": cost})})
	case "ship_upgrade":
		w := h.opts.World.For(ss.pid)
		shipType, ok := w.ShipTypeOf(ss.pid)
		if !ok {
			reply(errMsg(m.ID, "not_joined", "вы не в мире"))
			return
		}
		st, err := h.opts.Ships.State(sctx, ss.pid, shipType)
		if err != nil {
			h.log.Error("улучшение корабля: прогресс", "player", ss.pid, "err", err)
			reply(errMsg(m.ID, "internal", "внутренняя ошибка"))
			return
		}
		if st.Level >= st.MaxLevel {
			reply(errMsg(m.ID, "max_level", "корабль уже максимального уровня"))
			return
		}
		cost := int64(st.Level) * 1000
		total, err := h.opts.Quests.Spend(sctx, ss.pid, cost)
		if err != nil {
			code := "no_credits"
			if err != quest.ErrNotEnough {
				h.log.Error("улучшение корабля", "player", ss.pid, "err", err)
				code = "internal"
			}
			reply(errMsg(m.ID, code, "не хватает кредитов на улучшение"))
			return
		}
		// покупка = гарантированный уровень: докидываем опыт до порога следующего уровня (идемпотентно по ключу)
		if st.ToNext > 0 {
			key := fmt.Sprintf("buy_level:%s:%d", shipType, st.Level)
			if g, err := h.opts.Ships.Grant(sctx, ss.pid, shipType, key, "market", st.ToNext); err == nil && g.Granted {
				h.log.Info("корабль улучшен за кредиты", "player", ss.pid, "ship", shipType, "level", g.State.Level)
			}
		}
		var sp any = h.applyShipLevel(sctx, ss.pid, shipType)
		if sp == nil {
			sp = map[string]any{"level": st.Level + 1}
		}
		reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "ship_upgrade", "credits": total, "spent": cost, "ship_progress": sp})})
	case "jump":
		var p struct {
			To string `json:"to"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.To == "" || len(p.To) > 32 {
			reply(errMsg(m.ID, "bad_message", "некорректный прыжок"))
			return
		}
		rec, spawn, err := h.opts.World.Jump(ss.pid, p.To)
		if err != nil {
			reply(errMsg(m.ID, jumpCode(err), jumpText(err)))
			return
		}
		h.save(rec)
		if h.opts.Quests != nil {
			h.completeQuests(ss.pid, h.opts.Quests.Event(ctx, ss.pid, "jump", p.To, 1))
		}
		hw := h.opts.World.For(ss.pid)
		payload := map[string]any{"system": hw.System(), "spawn": spawn}
		if lay, wcfg := hw.Layout(); lay.System != "" {
			payload["layout"] = lay
			payload["visual"] = wcfg.Visual
		}
		payload["systems"] = h.opts.World.Systems()
		payload["gates"] = h.opts.World.Gates(hw.System().ID)
		payload["links"] = h.opts.World.Links()
		reply(Message{Type: "system_changed", ID: m.ID, Payload: mustJSON(payload)})
		if st, ok := h.opts.World.Snapshot(ss.pid); ok {
			h.pushState(ss, st)
		}
	case "set_option":
		var p struct {
			Name  string `json:"name"`
			Value bool   `json:"value"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Name == "" || len(p.Name) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректная настройка"))
			return
		}
		if err := w.SetOption(ss.pid, p.Name, p.Value); err != nil {
			fail(err)
		}
	case "ships":
		out := map[string]any{"ships": w.Fleet(), "ship_max_level": rpg.DefaultShipCurve.MaxLevel}
		if passport, ok := h.currentShipPassport(w, ss.pid); ok {
			out["passport"] = passport
		}
		if h.opts.Ships != nil {
			sctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if lv, err := h.opts.Ships.All(sctx, ss.pid); err == nil {
				out["levels"] = lv // тип корабля -> прогресс; нет записи — уровень 1
			}
			cancel()
		}
		reply(Message{Type: "ships", ID: m.ID, Payload: mustJSON(out)})
	case "set_ship":
		var p struct {
			Ship string `json:"ship"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Ship == "" || len(p.Ship) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректный корабль"))
			return
		}
		if p.Ship == world.DefaultShip && h.opts.Equipment != nil {
			rec := world.Record{ID: ss.pid, ShipType: p.Ship}
			if err := h.loadStarter(sctx, &rec); err != nil || !w.RestoreStarter(ss.pid, *rec.StarterLoadout) {
				reply(errMsg(m.ID, "equipment_unavailable", "не удалось загрузить оснащение"))
				return
			}
		}
		rec, err := w.SetShip(ss.pid, p.Ship)
		if err != nil {
			fail(err)
			return
		}
		h.applyShipLevel(ctx, ss.pid, rec.ShipType) // у каждого корабля свой уровень: он восстанавливается при смене
		h.save(rec)                                 // смена корабля сохраняется сразу
	case "sell":
		if _, err := w.SellCargo(ss.pid); err != nil {
			fail(err)
		} else if h.opts.Quests != nil {
			h.completeQuests(ss.pid, h.opts.Quests.Event(ctx, ss.pid, "sell", "", 1))
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
	case "equipment_busy":
		return "дождитесь завершения смены оснащения"
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
	case "module_unavailable":
		return "для форсажа нужен установленный двигатель"
	case "ability_not_learned":
		return "навык не изучен"
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
	case "bad_option":
		return "такой настройки нет"
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
		case m := <-ss.chat:
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
	unlock, _ := h.lockPlayer(context.Background(), ss.pid)
	defer unlock()
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
	// Не ждать row-lock покупателя из общего цикла автосохранения/симуляции.
	if rec.SkipCargo {
		return
	}
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
	if arr := w.DrainArrivals(); len(arr) > 0 {
		h.deliverArrivals(arr)
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
			pw := h.opts.World.For(t.pid)
			if pw != nil && !pw.PersistenceBusy(t.pid) {
				h.evaluateQuests(t.s, t.pid, st)
			}
		}
		h.noteSystem(t.s, t.pid)
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
func (h *Handler) evaluateQuests(_ *session, pid string, st world.State) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var pw *world.World
	sys := ""
	if h.opts.World != nil {
		pw = h.opts.World.For(pid)
		if pw != nil {
			sys = pw.System().ID
		}
	}
	for _, c := range h.opts.Quests.Evaluate(ctx, pid, world.Vec{X: st.Ship.Pos[0], Y: st.Ship.Pos[1]}) {
		h.finishQuest(pid, c)
	}
	if pw == nil {
		return
	}
	if d, ok := h.opts.Quests.Pending(pid, "deliver", sys); ok && st.Ship.InSafe && d.Goal > 0 {
		amt := float64(d.Goal)
		if rec, taken := pw.TakeCargo(pid, d.Target, amt); taken {
			done := h.opts.Quests.EventAt(ctx, pid, sys, "deliver", d.Target, d.Goal)
			if len(done) == 0 {
				if back, ok := pw.GiveCargo(pid, d.Target, amt); ok {
					rec = back
				} else {
					h.log.Error("возврат груза задания не удался", "player", pid, "resource", d.Target)
				}
			}
			h.save(rec)
			for _, c := range done {
				h.finishQuest(pid, c)
			}
		}
	}
	for i := 0; i < 4; i++ {
		d, ok := h.opts.Quests.Pending(pid, "visit", sys)
		if !ok || !pw.AtKind(pid, d.Target) {
			return
		}
		done := h.opts.Quests.EventAt(ctx, pid, sys, "visit", d.Target, 1)
		if len(done) == 0 {
			return
		}
		for _, c := range done {
			h.finishQuest(pid, c)
		}
	}
}

func (h *Handler) finishQuest(pid string, c quest.Completion) {
	h.completeQuests(pid, []quest.Completion{c})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h.grantXP(ctx, pid, "xp:quest:"+c.QuestID, "quest", pilot.XPFor("quest", c.Credits))
}

// deliverArrivals сообщает игроку о первом прибытии и, если журнал включён, сохраняет его.
func (h *Handler) deliverArrivals(arr []world.Arrival) {
	for _, a := range arr {
		h.mu.Lock()
		ss := h.active[a.Player]
		h.mu.Unlock()
		if ss == nil {
			continue
		}
		select {
		case ss.events <- Message{Type: "location_arrived", Payload: mustJSON(map[string]any{
			"id": a.ID, "name": a.Name, "kind": a.Kind, "system": a.System,
		})}:
		default:
			h.log.Warn("ws: очередь прибытий переполнена", "player", a.Player)
		}
	}
	if h.opts.Locations == nil {
		return
	}
	cp := append([]world.Arrival(nil), arr...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, a := range cp {
			err := h.opts.Locations.Record(ctx, location.Visit{Player: a.Player, System: a.System, ID: a.ID, Name: a.Name, Kind: a.Kind})
			if err != nil {
				h.log.Error("журнал локаций", "player", a.Player, "id", a.ID, "err", err)
			}
		}
	}()
}

// applyShipLevel читает уровень корабля игрока из хранилища и применяет его в мире; возвращает прогресс (nil: функция выключена).
func (h *Handler) applyShipLevel(ctx context.Context, pid, ship string) *shipxp.State {
	if h.opts.Ships == nil || h.opts.World == nil {
		return nil
	}
	sctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	st, err := h.opts.Ships.State(sctx, pid, ship)
	if err != nil {
		h.log.Error("уровень корабля", "player", pid, "ship", ship, "err", err)
		return nil
	}
	h.opts.World.SetShipLevel(pid, st.Level)
	return &st
}

// shipXPSources — события, за которые опыт получает и корабль (контракты — только пилоту).
var shipXPSources = map[string]bool{"kill": true, "discover": true, "sell": true}

// grantXP начисляет опыт пилоту и текущему кораблю за событие (идемпотентно по ключу) и сообщает игроку, если он в сети.
func (h *Handler) grantXP(ctx context.Context, pid, key, source string, amount int64) {
	if amount <= 0 || (h.opts.Pilot == nil && h.opts.Ships == nil) {
		return
	}
	payload := map[string]any{"source": source}
	anyGrant := false
	if h.opts.Pilot != nil {
		g, err := h.opts.Pilot.Grant(ctx, pid, key, source, amount)
		if err != nil {
			h.log.Error("опыт пилота", "player", pid, "key", key, "err", err)
		} else if g.Granted {
			anyGrant = true
			payload["gained"], payload["level_up"], payload["pilot"] = g.Amount, g.LevelUp, g.State
		}
	}
	if h.opts.Ships != nil && h.opts.World != nil && shipXPSources[source] {
		if typ, ok := h.opts.World.ShipTypeOf(pid); ok {
			g, err := h.opts.Ships.Grant(ctx, pid, typ, "s"+key, source, amount)
			if err != nil {
				h.log.Error("опыт корабля", "player", pid, "ship", typ, "key", key, "err", err)
			} else if g.Granted {
				anyGrant = true
				if g.LevelUp {
					h.opts.World.SetShipLevel(pid, g.State.Level)
				}
				payload["ship"], payload["ship_level_up"] = g.State, g.LevelUp
			}
		}
	}
	if !anyGrant {
		return
	}
	h.mu.Lock()
	ss := h.active[pid]
	h.mu.Unlock()
	if ss == nil {
		return
	}
	select {
	case ss.events <- Message{Type: "xp", Payload: mustJSON(payload)}:
	default:
	}
}

func pilotText(err error) (string, string) {
	switch {
	case errors.Is(err, pilot.ErrBadAttr):
		return "pilot_bad_attr", "неизвестная характеристика"
	case errors.Is(err, pilot.ErrBadCount):
		return "pilot_bad_count", "некорректное число очков"
	case errors.Is(err, pilot.ErrNoPoints):
		return "pilot_no_points", "недостаточно свободных очков"
	case errors.Is(err, pilot.ErrCap):
		return "pilot_cap", "достигнут предел характеристики"
	}
	return "internal", "внутренняя ошибка"
}

// pilotMsg: pilot (состояние) и pilot_spend (потратить очки на характеристику). Результат всегда считает сервер.
func (h *Handler) pilotMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if h.opts.Pilot == nil {
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var st pilot.State
	var err error
	if m.Type == "pilot_spend" {
		var p struct {
			Attr   string `json:"attr"`
			Points int    `json:"points"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || len(p.Attr) > 24 {
			reply(errMsg(m.ID, "bad_message", "некорректная команда"))
			return
		}
		st, err = h.opts.Pilot.Spend(pctx, ss.pid, p.Attr, p.Points)
	} else {
		st, err = h.opts.Pilot.State(pctx, ss.pid)
	}
	if err != nil {
		code, text := pilotText(err)
		if code == "internal" {
			h.log.Error("пилот", "player", ss.pid, "type", m.Type, "err", err)
		}
		reply(errMsg(m.ID, code, text))
		return
	}
	reply(Message{Type: "pilot", ID: m.ID, Payload: mustJSON(st)})
}

// grantRewards выдаёт награды локаций идемпотентно (ключ события) и сообщает игроку о зачислении.
// completeQuests доставляет клиенту уведомления о выполненных контрактах (и следующем звене цепочки).
func (h *Handler) completeQuests(pid string, cs []quest.Completion) {
	for _, c := range cs {
		h.mu.Lock()
		ss := h.active[pid]
		h.mu.Unlock()
		if ss == nil {
			continue
		}
		payload := map[string]any{"id": c.QuestID, "title": c.Title, "reward": map[string]any{"credits": c.Credits}, "credits": c.Total}
		if c.Next != "" {
			payload["next"] = c.Next
		}
		select {
		case ss.events <- Message{Type: "quest_completed", Payload: mustJSON(payload)}:
		default:
		}
	}
}

func (h *Handler) grantRewards(rs []world.Reward) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, r := range rs {
		if r.Kind == "mine" && r.Credits == 0 {
			if r.Amount > 0 && r.Amount <= math.MaxInt32 && !math.IsNaN(r.Amount) && !math.IsInf(r.Amount, 0) {
				h.completeQuests(r.Player, h.opts.Quests.EventOnce(ctx, r.Player, r.Key, "mine", r.Ref, int(r.Amount)))
			}
			continue
		}
		granted, total, err := h.opts.Quests.Grant(ctx, r.Player, r.Key, r.Credits)
		if err != nil {
			h.log.Error("награда локации", "player", r.Player, "key", r.Key, "err", err)
			continue
		}
		if !granted {
			continue
		}
		if r.Kind == "gear" && r.Ref != "" && h.opts.Shop != nil {
			if _, gerr := h.opts.Shop.Grant(ctx, r.Player, r.Ref, "raid:"+r.Key); gerr != nil && gerr != shop.ErrOwned && gerr != shop.ErrNoItem {
				h.log.Error("награда арсенала", "player", r.Player, "ref", r.Ref, "err", gerr)
			}
		}
		h.log.Info("награда выдана", "player", r.Player, "kind", r.Kind, "ref", r.Ref, "credits", r.Credits, "total", total)
		h.grantXP(ctx, r.Player, "xp:"+r.Key, r.Kind, pilot.XPFor(r.Kind, r.Credits))
		if r.Kind == "kill" {
			h.completeQuests(r.Player, h.opts.Quests.Event(ctx, r.Player, "kill", "", 1))
		} else if r.Kind == "mine" {
			h.completeQuests(r.Player, h.opts.Quests.Event(ctx, r.Player, "mine", r.Ref, int(r.Amount)))
		}
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
	unlock, lockErr := h.lockPlayer(ctx, rec.ID)
	if lockErr != nil {
		return false
	}
	defer unlock()
	if h.opts.Quests != nil {
		cargoCtx, cargoCancel := context.WithTimeout(ctx, time.Second)
		cargo, cargoErr := h.opts.Quests.PurchaseCargo(cargoCtx, rec.ID)
		cargoCancel()
		if cargoErr != nil {
			reply(errMsg(m.ID, "purchase_unavailable", "не удалось подтвердить груз"))
			return false
		}
		rec.CargoConfirmed = true
		if cargo != nil && cargo.Revision >= rec.CargoRevision {
			rec.Hold = cargo.Hold
			rec.CargoRevision = cargo.Revision
		}
	}
	if err := h.loadStarter(ctx, &rec); err != nil {
		log.Error("ws: load equipment", "player", rec.ID, "err", err)
		reply(errMsg(m.ID, "equipment_unavailable", "не удалось загрузить оснащение; повторите вход"))
		return false
	}
	if rec.AI {
		if h.opts.AIActorGuard == nil || h.opts.AIActorGuard(ctx, rec.ID, ss.tokenHash) != nil {
			reply(errMsg(m.ID, "ai_session_invalid", "вход ИИ недоступен"))
			return false
		}
	}
	ss.ai = rec.AI
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
	h.syncFitting(ctx, rec.ID, rec.ShipType, rec.Spec)
	if h.opts.Quests != nil {
		qctx, qcancel := context.WithTimeout(ctx, 5*time.Second)
		if err := h.opts.Quests.Join(qctx, rec.ID); err != nil {
			log.Error("ws: контракты игрока", "player", rec.ID, "err", err)
		}
		qcancel()
	}
	ss.name = rec.Name
	h.applyCosmetics(ctx, rec.ID)
	hw := w.For(rec.ID)
	sys := hw.System()
	h.mu.Lock()
	ss.seenSystem = sys.ID
	h.mu.Unlock()
	welcome := map[string]any{
		"player_id": rec.ID, "name": rec.Name, "system": sys,
		"character": map[string]any{"gender": rec.Gender, "pilot_spec": rec.PilotSpec, "ship_type": rec.ShipType},
	}
	if lay, wcfg := hw.Layout(); lay.System != "" {
		// раскладка системы и визуальные параметры: клиент строит декор по seed, серверные объекты приходят в state
		welcome["layout"] = lay
		welcome["visual"] = wcfg.Visual
	}
	welcome["systems"] = h.opts.World.Systems()   // звёздная карта (Э-1)
	welcome["gates"] = h.opts.World.Gates(sys.ID) // точки прыжков текущей системы
	welcome["links"] = h.opts.World.Links()       // рёбра переходов для карты галактики
	if h.opts.Pilot != nil {
		pctx, pcancel := context.WithTimeout(ctx, 3*time.Second)
		if ps, err := h.opts.Pilot.State(pctx, rec.ID); err == nil {
			welcome["pilot"] = ps
		}
		pcancel()
	}
	if sp := h.applyShipLevel(ctx, rec.ID, rec.ShipType); sp != nil {
		welcome["ship_progress"] = sp
	}
	reply(Message{Type: "welcome", ID: m.ID, Payload: mustJSON(welcome)})
	go h.pump(ctx, ss)
	h.pushJournal(ctx, ss)
	if st, ok := w.Snapshot(ss.pid); ok {
		h.pushState(ss, st)
	}
	log.Info("ws: игрок вошёл", "player", rec.ID, "name", rec.Name)
	return true
}

func (h *Handler) pushJournal(ctx context.Context, ss *session) {
	if h.opts.Locations == nil || ss.pid == "" {
		return
	}
	jctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	visits, err := h.opts.Locations.List(jctx, ss.pid)
	if err != nil {
		h.log.Error("журнал локаций", "player", ss.pid, "err", err)
		return
	}
	if len(visits) == 0 {
		return
	}
	select {
	case ss.events <- Message{Type: "location_journal", Payload: mustJSON(map[string]any{"visits": visits})}:
	default:
	}
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
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	rec, err := world.CreatePilot(sctx, h.opts.Store, h.opts.World, ss.pending.ID, p.Nick, p.Gender, p.PilotSpec)
	cancel()
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
	h.grantSubscriptionDaily(ctx, pid, st)
}

func (h *Handler) setCosmetics(pid string, st shop.State) shop.Cosmetics {
	c := h.opts.Shop.Cosmetics(st)
	h.opts.World.SetCosmetics(pid, world.Cosmetics{Vip: c.Vip, Badge: c.Badge, NameColor: c.NameColor, Engine: c.Engine, Avatar: c.Avatar, Frame: c.Frame, Title: c.Title})
	return c
}

// grantSubscriptionDaily начисляет ежедневную награду подписки при первом входе дня.
// Идемпотентно: ключ награды содержит дату, повторный вход не даёт дубликатов.
func (h *Handler) grantSubscriptionDaily(ctx context.Context, pid string, st shop.State) {
	now := time.Now()
	if h.opts.Quests == nil || !st.SubscriptionActive(now) {
		return
	}
	daily := 0
	for _, it := range h.opts.Shop.Config().Items {
		if it.Kind == "subscription" {
			daily = max(daily, it.Daily)
		}
	}
	if daily <= 0 {
		return
	}
	key := "sub_daily:" + now.Format("2006-01-02")
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	granted, _, err := h.opts.Quests.Grant(cctx, pid, key, int64(daily))
	if err != nil {
		h.log.Error("подписка: ежедневная награда", "player", pid, "err", err)
		return
	}
	if granted {
		h.log.Info("подписка: ежедневная награда выдана", "player", pid, "credits", daily)
	}
}

// marketResources — ресурсы, доступные для покупки у станции (Э-2).
var marketResources = map[string]bool{"iron_ore": true, "crystals": true, "gas": true, "scrap": true}

func stationCode(err error) string {
	switch err {
	case world.ErrNoStable:
		return "not_at_station"
	case world.ErrHoldFull:
		return "hold_full"
	case world.ErrNoResource:
		return "no_item"
	case world.ErrDead:
		return "dead"
	default:
		return "bad_request"
	}
}

func stationText(err error) string {
	switch err {
	case world.ErrNoStable:
		return "услуги станции доступны только у станции"
	case world.ErrHoldFull:
		return "трюм полон"
	case world.ErrNoResource:
		return "этот товар не продаётся"
	case world.ErrDead:
		return "корабль уничтожен: дождитесь восстановления"
	default:
		return "операция отклонена"
	}
}

// corpMsg обрабатывает корпоративные сообщения (R-64, К-1).
func (h *Handler) corpMsg(ctx context.Context, ss *session, m Message) {
	reply := func(msg Message) {
		select {
		case ss.events <- msg:
		default:
		}
	}
	if h.opts.Corp == nil {
		reply(errMsg(m.ID, "unknown_type", "корпорации выключены"))
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch m.Type {
	case "corp_create":
		var p struct {
			Name string `json:"name"`
			Tag  string `json:"tag"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || len(p.Name) > 32 || len(p.Tag) > 4 {
			reply(errMsg(m.ID, "bad_message", "некорректные имя или тег"))
			return
		}
		c, err := h.opts.Corp.Create(cctx, ss.pid, p.Name, p.Tag)
		if err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		reply(Message{Type: "corp", ID: m.ID, Payload: mustJSON(map[string]any{"corp": c, "members": []corp.Member{{ID: ss.pid, Name: ss.name, Role: "owner"}}})})
	case "corp_join":
		var p struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Name == "" || len(p.Name) > 32 {
			reply(errMsg(m.ID, "bad_message", "некорректное имя корпорации"))
			return
		}
		c, err := h.opts.Corp.Join(cctx, ss.pid, p.Name, ss.name)
		if err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		reply(Message{Type: "corp", ID: m.ID, Payload: mustJSON(map[string]any{"corp": c})})
	case "corp_leave":
		if err := h.opts.Corp.Leave(cctx, ss.pid); err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		reply(Message{Type: "corp_left", ID: m.ID, Payload: mustJSON(map[string]any{"ok": true})})
	case "corp_info":
		c, mem, err := h.opts.Corp.Info(cctx, ss.pid)
		if err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		body := map[string]any{"corp": c, "members": mem}
		if task, ok, al, seats, hasAl, snapErr := h.opts.Corp.Snapshot(cctx, c.ID); snapErr == nil {
			if ok {
				body["task"] = task
			}
			if hasAl {
				body["alliance"] = al
				body["alliance_clans"] = seats
			}
		}
		reply(Message{Type: "corp", ID: m.ID, Payload: mustJSON(body)})
	case "alliance_found":
		var p struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Name == "" {
			reply(errMsg(m.ID, "bad_message", "некорректное имя союза"))
			return
		}
		al, err := h.opts.Corp.FoundAlliance(cctx, ss.pid, p.Name)
		if err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		reply(Message{Type: "corp", ID: m.ID, Payload: mustJSON(map[string]any{"alliance": al})})
	case "alliance_join":
		var p struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Name == "" {
			reply(errMsg(m.ID, "bad_message", "некорректное имя союза"))
			return
		}
		al, err := h.opts.Corp.JoinAlliance(cctx, ss.pid, p.Name)
		if err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		reply(Message{Type: "corp", ID: m.ID, Payload: mustJSON(map[string]any{"alliance": al})})
	case "alliance_vote":
		var p struct {
			Governor string `json:"governor"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.Governor == "" {
			reply(errMsg(m.ID, "bad_message", "не выбран управляющий"))
			return
		}
		al, err := h.opts.Corp.Vote(cctx, ss.pid, p.Governor)
		if err != nil {
			reply(errMsg(m.ID, corpCode(err), corpText(err)))
			return
		}
		reply(Message{Type: "corp", ID: m.ID, Payload: mustJSON(map[string]any{"alliance": al})})
	}
}

func corpCode(err error) string {
	switch err {
	case corp.ErrBadName:
		return "bad_name"
	case corp.ErrBadTag:
		return "bad_tag"
	case corp.ErrTaken:
		return "corp_taken"
	case corp.ErrHasCorp:
		return "already_in_corp"
	case corp.ErrNoCorp, corp.ErrNotMember:
		return "no_corp"
	case corp.ErrNotOwner:
		return "not_owner"
	case corp.ErrAllianceFull:
		return "alliance_full"
	case corp.ErrHasAlliance:
		return "has_alliance"
	case corp.ErrNoAlliance:
		return "no_alliance"
	case corp.ErrBadVote:
		return "bad_vote"
	default:
		if errors.Is(err, quest.ErrNotEnough) {
			return "no_credits"
		}
		return "internal"
	}
}

func corpText(err error) string {
	switch err {
	case corp.ErrBadName:
		return "имя корпорации: 3–32 символа, русские/латинские буквы, цифры, пробел, «-», «_»"
	case corp.ErrBadTag:
		return "тег: 2–4 символа A–Z или цифры"
	case corp.ErrTaken:
		return "имя или тег уже заняты"
	case corp.ErrHasCorp:
		return "вы уже в корпорации: сначала выйдите"
	case corp.ErrNoCorp, corp.ErrNotMember:
		return "вы не состоите в корпорации"
	case corp.ErrNotOwner:
		return "это может сделать только основатель клана"
	case corp.ErrAllianceFull:
		return "в союзе уже три клана"
	case corp.ErrHasAlliance:
		return "клан уже в союзе"
	case corp.ErrNoAlliance:
		return "союз не найден"
	case corp.ErrBadVote:
		return "управляющим можно выбрать только основателя клана из союза"
	default:
		if errors.Is(err, quest.ErrNotEnough) {
			return "взнос 10 000 кр.: не хватает кредитов"
		}
		return "операция отклонена"
	}
}

func jumpCode(err error) string {
	switch err {
	case world.ErrEquipmentBusy:
		return "equipment_busy"
	case world.ErrNoLink:
		return "no_link"
	case world.ErrNoGate:
		return "no_gate"
	default:
		return "no_system"
	}
}

func jumpText(err error) string {
	switch err {
	case world.ErrEquipmentBusy:
		return "дождитесь завершения смены оснащения"
	case world.ErrNoLink:
		return "из этой системы нет перехода туда"
	case world.ErrNoGate:
		return "подлетите к точке гиперперехода"
	default:
		return "система недоступна"
	}
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

func chatText(code string) string {
	switch code {
	case "chat_disabled":
		return "чат выключен администратором"
	case "chat_empty":
		return "пустое сообщение"
	case "chat_too_long":
		return "сообщение слишком длинное"
	case "chat_rate":
		return "слишком часто: подождите пару секунд"
	case "chat_duplicate":
		return "это сообщение вы уже отправляли"
	case "chat_links":
		return "ссылки в чате запрещены"
	case "chat_bad_words":
		return "сообщение содержит недопустимые слова"
	case "chat_muted":
		return "вам запрещено писать в чат"
	case "chat_no_player":
		return "такого игрока нет"
	case "chat_self":
		return "нельзя написать самому себе"
	case "chat_bad_channel":
		return "неизвестный канал"
	}
	return "сообщение не отправлено"
}

// SetOnChat задаёт наблюдателя отправленных сообщений (ответы ИИ-капитанов, ИИ-2).
func (h *Handler) SetOnChat(f func(chat.Message)) { h.opts.OnChat = f }

// PushChat доставляет сообщение чата подключённым игрокам: общий и системный канал всем, личное адресату и отправителю.
func (h *Handler) PushChat(m chat.Message) {
	out := Message{Type: "chat", Payload: mustJSON(map[string]any{"message": m})}
	h.mu.Lock()
	var targets []*session
	for pid, s := range h.active {
		if m.Channel != chat.DM || pid == m.ToID || pid == m.FromID {
			targets = append(targets, s)
		}
	}
	h.mu.Unlock()
	for _, s := range targets {
		select {
		case s.chat <- out:
		default: // очередь переполнена: медленный клиент пропускает сообщение, остальные не страдают
		}
	}
}

// PushServerUpdated говорит игрокам в мире, что сервер уже применил настройки или перезапускается.
func (h *Handler) PushServerUpdated(reason string) {
	if h == nil {
		return
	}
	if reason == "" {
		reason = "настройки"
	}
	out := Message{Type: "server_updated", Payload: mustJSON(map[string]any{"reason": reason})}
	h.mu.Lock()
	targets := make([]*session, 0, len(h.active))
	for _, s := range h.active {
		if s != nil && !s.ai {
			targets = append(targets, s)
		}
	}
	h.mu.Unlock()
	for _, s := range targets {
		select {
		case s.events <- out:
		default:
		}
	}
}

// chatMsg обрабатывает chat_history (история и параметры) и chat_send (отправка).
func (h *Handler) chatMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if h.opts.Chat == nil {
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch m.Type {
	case "chat_history":
		g, d, err := h.opts.Chat.History(cctx, ss.pid)
		if err != nil {
			h.log.Error("чат: история", "player", ss.pid, "err", err)
			reply(errMsg(m.ID, "internal", "история недоступна"))
			return
		}
		st := h.opts.Chat.Settings()
		reply(Message{Type: "chat_history", ID: m.ID, Payload: mustJSON(map[string]any{"general": g, "dms": d,
			"settings": map[string]any{"enabled": st.Enabled, "max_len": st.MaxLen, "min_interval_ms": st.MinIntervalMs}})})
	case "chat_send":
		var p struct {
			Channel string `json:"channel"`
			To      string `json:"to"`
			Text    string `json:"text"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || len(p.To) > 64 || len(p.Text) > 4000 {
			reply(errMsg(m.ID, "bad_message", "некорректное сообщение"))
			return
		}
		msg, err := h.opts.Chat.Send(cctx, chat.Sender{ID: ss.pid, Name: ss.name}, p.Channel, p.To, p.Text)
		if err != nil {
			code := err.Error()
			switch err {
			case chat.ErrDisabled, chat.ErrEmpty, chat.ErrTooLong, chat.ErrRate, chat.ErrDuplicate, chat.ErrLinks, chat.ErrBadWords, chat.ErrMuted,
				chat.ErrNoRecipient, chat.ErrSelfDM, chat.ErrBadChannel:
			default:
				h.log.Error("чат: отправка", "player", ss.pid, "err", err)
				code = "internal"
			}
			reply(errMsg(m.ID, code, chatText(code)))
			return
		}
		h.PushChat(msg)
		if h.opts.OnChat != nil {
			h.opts.OnChat(msg)
		}
	}
}

// currentShipPassport overlays the current authoritative (including level-scaled)
// base on draft metadata. It never calls SetCatalog or publishes draft ships.
func (h *Handler) currentShipPassport(w *world.World, pid string) (shipcatalog.ShipPassport, bool) {
	if h.opts.ShipPassports == nil {
		return shipcatalog.ShipPassport{}, false
	}
	active, ok := w.SpecInfo(pid)
	if !ok || active.ID != "FTR-001" {
		return shipcatalog.ShipPassport{}, false
	}
	for _, passport := range h.opts.ShipPassports.Ships {
		if passport.ID == active.ID {
			passport.Base = active
			return passport, true
		}
	}
	return shipcatalog.ShipPassport{}, false
}

// Подтверждение AI-intent идёт после общего валидатора. Не является claim награды.
func (h *Handler) aiAck(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	if ss.ai {
		_ = writeJSON(ctx, c, Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": m.Type})})
	}
}
