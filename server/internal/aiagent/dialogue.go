package aiagent

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
)

// ИИ-2 — AI Dialogue Provider: ответы ИИ-капитанов в общем и личном чате на материале обучения AI Studio.
// Правила: без внешних LLM (движок на шаблонах и данных активной версии студии); общая защита от повторов
// между ИИ; частота ограничена настройками; отвечают только капитаны, которые сейчас онлайн в мире.

// minGapAI — общий минимальный интервал между ИИ-репликами (решение R-42 №4: не чаще 1 в 2 секунды).
const minGapAI = 2 * time.Second

const engineRefresh = 30 * time.Second

// EngineSource отдаёт действующую конфигурацию AI Studio (aistudio.Service её реализует; в тестах — фейк).
type EngineSource interface {
	Active(ctx context.Context) (aistudio.Config, int64, error)
}

// Dialogue — провайдер реплик ИИ в чате.
type Dialogue struct {
	src  EngineSource
	chat *chat.Service
	mgr  *Manager
	// Push доставляет отправленную реплику подключённым игрокам (хаб: PushChat). chat.Send только сохраняет сообщение;
	// без рассылки реплика попадает в историю, но живые клиенты её не видят (R-60).
	Push func(chat.Message)
	log  *slog.Logger
	now  func() time.Time

	mu          sync.Mutex
	eng         *aistudio.Engine
	engineAt    time.Time
	engineVer   int64
	convos      map[string]*aistudio.Conversation
	lastCaptain map[string]time.Time
	sent        []time.Time // окно отправок (потолок в минуту)
	recent      []uint64    // хеши последних реплик всех ИИ (защита от повторов между капитанами)
	lastAny     time.Time   // последняя ИИ-реплика (общий интервал)
	rng         uint32
	rmu         sync.Mutex                    // защита генератора: им пользуются обработчик сообщений, самостоятельные реплики и отложенные ответы
	followDelay func(r float64) time.Duration // задержка ответа ИИ на ИИ (в тестах — короткая)
}

// NewDialogue создаёт провайдер. chatSvc и mgr обязательны.
func NewDialogue(src EngineSource, chatSvc *chat.Service, mgr *Manager, log *slog.Logger) *Dialogue {
	return &Dialogue{src: src, chat: chatSvc, mgr: mgr, log: log, now: time.Now,
		convos: map[string]*aistudio.Conversation{}, lastCaptain: map[string]time.Time{}, rng: 88172645463325252 & 0x7fffffff}
}

func (d *Dialogue) rand() float64 {
	d.rmu.Lock()
	defer d.rmu.Unlock()
	d.rng ^= d.rng << 13
	d.rng ^= d.rng >> 17
	d.rng ^= d.rng << 5
	return float64(d.rng%100000) / 100000.0
}

// Handle — обработчик наблюдателя OnChat: решает, отвечать ли, и ставит отложенный ответ.
// Сама отправка идёт через time.AfterFunc, чтобы не блокировать доставку сообщения игрокам.
func (d *Dialogue) Handle(ctx context.Context, m chat.Message) {
	if m.FromAI || m.Channel == chat.System || d.chat == nil {
		return
	}
	// хаб вызывает наблюдателя синхронно в цикле отправителя: вся работа идёт в отдельной горутине,
	// чтобы любая задержка или сбой провайдера не останавливали игрока, который пишет в чат
	go d.handle(ctx, m)
}

func (d *Dialogue) handle(ctx context.Context, m chat.Message) {
	s := d.mgr.Settings()
	if !s.ChatReplyEnabled {
		return
	}
	var captainID, replyTo, channel string
	switch m.Channel {
	case chat.DM:
		if !d.mgr.IsOnline(m.ToID) {
			d.log.Info("ИИ-чат: адресат не онлайн, ответа нет", "to", m.ToID)
			return
		}
		captainID, replyTo, channel = m.ToID, m.FromID, chat.DM
	case chat.General:
		if s.ChatReplyChance <= 0 || d.rand()*100.0 >= float64(s.ChatReplyChance) {
			return
		}
		id, ok := d.mgr.RandomOnline(m.FromID, d.rand())
		if !ok {
			return
		}
		captainID, channel = id, chat.General
	default:
		return
	}
	if !d.reserve(s, captainID) {
		d.log.Info("ИИ-чат: лимит или кулдаун, ответа нет", "captain", captainID, "канал", channel)
		return
	}
	delay := time.Duration(s.ChatDelayMinMs) * time.Millisecond
	if span := s.ChatDelayMaxMs - s.ChatDelayMinMs; span > 0 {
		delay += time.Duration(d.rand() * float64(span) * float64(time.Millisecond))
	}
	captainName := d.mgr.NameOf(captainID)
	input := m.Text
	// копии для горутины: контекст от запроса может отмениться раньше задержки
	go func() {
		t := time.NewTimer(delay)
		defer t.Stop()
		select {
		case <-ctx.Done():
			// соединение закрылось: всё равно отвечаем фоновым контекстом с таймаутом
		case <-t.C:
		}
		rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.reply(rctx, captainID, captainName, channel, replyTo, input)
	}()
}

// reserve проверяет лимиты и бронирует слот на момент планирования ответа,
// чтобы параллельные сообщения не заняли один и тот же интервал.
func (d *Dialogue) reserve(s Settings, captainID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	if now.Sub(d.lastAny) < minGapAI {
		return false
	}
	if until, ok := d.lastCaptain[captainID]; ok && now.Sub(until) < time.Duration(s.ChatCaptainCooldownSec)*time.Second {
		return false
	}
	if s.ChatMaxPerMin > 0 {
		cut := now.Add(-time.Minute)
		kept := d.sent[:0]
		for _, t := range d.sent {
			if t.After(cut) {
				kept = append(kept, t)
			}
		}
		d.sent = kept
		if len(d.sent) >= s.ChatMaxPerMin {
			return false
		}
		d.sent = append(d.sent, now)
	}
	d.lastAny = now
	return true
}

// engine действующего конфигурации студии (кэш 30 с или смена версии).
func (d *Dialogue) engine(ctx context.Context) (*aistudio.Engine, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.eng != nil && d.now().Sub(d.engineAt) < engineRefresh {
		return d.eng, nil
	}
	cfg, ver, err := d.src.Active(ctx)
	if err != nil {
		return nil, err
	}
	if d.eng != nil && ver == d.engineVer {
		d.engineAt = d.now()
		return d.eng, nil
	}
	d.eng = aistudio.NewEngine(&cfg)
	d.engineVer = ver
	d.engineAt = d.now()
	d.convos = map[string]*aistudio.Conversation{} // смена версии: контексты не переносим
	return d.eng, nil
}

// reply генерирует и отправляет реплику капитана.
func (d *Dialogue) reply(ctx context.Context, captainID, captainName, channel, replyTo, input string) string {
	engine, err := d.engine(ctx)
	if err != nil {
		d.log.Error("ИИ-чат: конфигурация студии", "err", err)
		return ""
	}
	key := captainID + "|" + replyTo
	d.mu.Lock()
	conv, ok := d.convos[key]
	if !ok {
		conv = aistudio.NewConversation(captainID)
		d.convos[key] = conv
	}
	turn := engine.Reply(conv, input)
	// общая защита от повторов: реплика не должна совпадать с недавними других капитанов
	for attempts := 0; d.repeats(turn.Reply) && attempts < 3; attempts++ {
		fresh := aistudio.NewConversation(captainID)
		turn = engine.Reply(fresh, input)
	}
	if d.repeats(turn.Reply) || turn.Reply == "" || turn.Reply == "…" || turn.Source == "no_captain" {
		d.mu.Unlock() // ответа нет: обязательно отпустить блокировку (иначе зависнут все следующие сообщения чата)
		d.log.Info("ИИ-чат: движок не дал реплики", "captain", captainID, "источник", turn.Source, "реплика", turn.Reply)
		return ""
	}
	d.note(captainID, turn.Reply)
	d.mu.Unlock()
	to := ""
	if channel == chat.DM {
		to = replyTo
	}
	sent, err := d.chat.Send(ctx, chat.Sender{ID: captainID, Name: captainName, AI: true}, channel, to, turn.Reply)
	switch {
	case err == nil:
		if d.Push != nil {
			d.Push(sent)
		}
		d.log.Info("ИИ-чат: реплика отправлена", "captain", captainID, "канал", channel, "адресат", to, "источник", turn.Source)
		return turn.Reply
	case errors.Is(err, chat.ErrRate) || errors.Is(err, chat.ErrDuplicate):
		d.log.Info("ИИ-чат: реплика отклонена чатом", "captain", captainID, "причина", err.Error())
	default:
		d.log.Error("ИИ-чат: отправка", "captain", captainID, "err", err)
	}
	return ""
}

// repeats говорит, что такая реплика недавно звучала от любого ИИ. Вызывать под d.mu.
func (d *Dialogue) repeats(text string) bool {
	h := hashOf(text)
	for _, r := range d.recent {
		if r == h {
			return true
		}
	}
	return false
}

// note фиксирует реплику: кулдаун капитана и кольцо повторов (общий интервал и потолок бронирует reserve).
// Вызывать под d.mu.
func (d *Dialogue) note(captainID, text string) {
	d.lastCaptain[captainID] = d.now()
	d.recent = append(d.recent, hashOf(text))
	if len(d.recent) > 24 {
		d.recent = d.recent[len(d.recent)-24:]
	}
}

// hashOf — хеш реплики для защиты от повторов.
func hashOf(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// ambientTopics — нейтральные «поводы»: движок отвечает на них в манере капитана, получаются живые реплики вроде «как дела» или «что нового».
var ambientTopics = []string{"привет всем", "как дела", "что нового в секторе", "кто что добывает", "где сегодня лучше торговать",
	"есть тут кто живой", "как вам сектор", "кто летает рядом", "как тут с пиратами", "чем занимаетесь", "что слышно", "как полёт"}

// RunAmbient периодически пишет в общий чат от имени случайного ИИ-капитана и иногда добавляет ответ другого ИИ:
// чат живой, даже когда игроки молчат. Работает только когда есть живые игроки и включены самостоятельные реплики.
func (d *Dialogue) RunAmbient(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.ambientTick(ctx, &last)
		}
	}
}

func (d *Dialogue) ambientTick(ctx context.Context, last *time.Time) {
	s := d.mgr.Settings()
	if !s.ChatReplyEnabled || s.ChatAmbientEverySec <= 0 || d.mgr.PlayersOnline() == 0 {
		return
	}
	now := d.now()
	wait := time.Duration(float64(s.ChatAmbientEverySec)*(0.7+0.6*d.rand())) * time.Second // разброс, чтобы не звучало по часам
	topic := ambientTopics[int(d.rand()*float64(len(ambientTopics)))%len(ambientTopics)]
	r1, r2 := d.rand(), d.rand()
	if !last.IsZero() && now.Sub(*last) < wait {
		return
	}
	id, ok := d.mgr.RandomOnline("", r1)
	if !ok || !d.reserve(s, id) {
		return
	}
	*last = now
	name := d.mgr.NameOf(id)
	text := d.reply(ctx, id, name, chat.General, "ambient", topic)
	if text == "" || s.ChatFollowChance <= 0 || r2*100.0 >= float64(s.ChatFollowChance) {
		return
	}
	// другой ИИ откликается на сказанное через несколько секунд
	other, ok := d.mgr.RandomOnline(id, d.rand())
	if !ok {
		return
	}
	delay := time.Duration(4+int(r1*8)) * time.Second
	if d.followDelay != nil {
		delay = d.followDelay(r1)
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if !d.reserve(d.mgr.Settings(), other) {
			return
		}
		d.reply(ctx, other, d.mgr.NameOf(other), chat.General, "follow:"+id, text)
	}()
}
