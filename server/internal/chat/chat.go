// Package chat — чат игры: общий канал и личные сообщения. Проверки (длина, частота, дубликаты, ссылки, недопустимые слова),
// мьюты, история с ограниченным сроком хранения. Отправителем может быть игрок или ИИ-капитан (метка ai).
package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Ошибки отправки (коды совпадают с кодами протокола).
var (
	ErrDisabled    = errors.New("chat_disabled")
	ErrEmpty       = errors.New("chat_empty")
	ErrTooLong     = errors.New("chat_too_long")
	ErrRate        = errors.New("chat_rate")
	ErrDuplicate   = errors.New("chat_duplicate")
	ErrLinks       = errors.New("chat_links")
	ErrBadWords    = errors.New("chat_bad_words")
	ErrMuted       = errors.New("chat_muted")
	ErrNoRecipient = errors.New("chat_no_player")
	ErrSelfDM      = errors.New("chat_self")
	ErrBadChannel  = errors.New("chat_bad_channel")
)

// Channels.
const (
	General = "general"
	DM      = "dm"
	System  = "system"
)

// Message — сообщение чата.
type Message struct {
	ID       int64  `json:"id"`
	Channel  string `json:"channel"`
	FromID   string `json:"from_id"`
	FromName string `json:"from_name"`
	FromAI   bool   `json:"from_ai,omitempty"`
	ToID     string `json:"to_id,omitempty"`
	ToName   string `json:"to_name,omitempty"`
	Text     string `json:"text"`
	At       int64  `json:"at"` // unix, мс
}

// Settings — настройки чата (меняются в админке на лету).
type Settings struct {
	Enabled         bool     `json:"enabled"`
	RetentionDays   int      `json:"retention_days"`
	MinIntervalMs   int      `json:"min_interval_ms"`
	MaxLen          int      `json:"max_len"`
	HistoryLimit    int      `json:"history_limit"`
	ProfanityFilter bool     `json:"profanity_filter"`
	LinksAllowed    bool     `json:"links_allowed"`
	ExtraBlocked    []string `json:"extra_blocked"`
}

// DefaultSettings — значения по умолчанию (решение владельца): история 7 дней, фильтр включён, не чаще 1 сообщения в 2 секунды.
func DefaultSettings() Settings {
	return Settings{Enabled: true, RetentionDays: 7, MinIntervalMs: 2000, MaxLen: 200, HistoryLimit: 50, ProfanityFilter: true, ExtraBlocked: []string{}}
}

// Validate проверяет диапазоны настроек.
func (s Settings) Validate() error {
	switch {
	case s.RetentionDays < 1 || s.RetentionDays > 90:
		return errors.New("retention_days: от 1 до 90")
	case s.MinIntervalMs < 0 || s.MinIntervalMs > 60000:
		return errors.New("min_interval_ms: от 0 до 60000")
	case s.MaxLen < 20 || s.MaxLen > 500:
		return errors.New("max_len: от 20 до 500")
	case s.HistoryLimit < 10 || s.HistoryLimit > 200:
		return errors.New("history_limit: от 10 до 200")
	case len(s.ExtraBlocked) > 200:
		return errors.New("extra_blocked: не более 200 слов")
	}
	for _, w := range s.ExtraBlocked {
		if utf8.RuneCountInString(w) > 32 {
			return errors.New("extra_blocked: слово длиннее 32 символов")
		}
	}
	return nil
}

// Mute — запрет писать. Until == nil: бессрочно (бан в чате).
type Mute struct {
	PlayerID   string     `json:"player_id"`
	PlayerName string     `json:"player_name"`
	Until      *time.Time `json:"until"`
	Reason     string     `json:"reason"`
	Admin      string     `json:"admin"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Person — адресат: игрок или ИИ-капитан.
type Person struct {
	ID, Name string
	AI       bool
}

// Store — хранилище чата.
type Store interface {
	Save(ctx context.Context, m Message) (Message, error)
	General(ctx context.Context, limit int) ([]Message, error)
	DMs(ctx context.Context, playerID string, limit int) ([]Message, error)
	Search(ctx context.Context, channel, q string, limit int) ([]Message, error)
	Purge(ctx context.Context, before time.Time) (int64, error)
	FindPlayer(ctx context.Context, q string) (Person, bool, error)
	MuteOf(ctx context.Context, playerID string) (Mute, bool, error)
	SetMute(ctx context.Context, m Mute) error
	Unmute(ctx context.Context, playerID string) error
	Mutes(ctx context.Context) ([]Mute, error)
	LoadSettings(ctx context.Context) (Settings, bool, error)
	SaveSettings(ctx context.Context, s Settings) error
}

// Sender — отправитель.
type Sender struct {
	ID, Name string
	AI       bool
}

// Service — правила чата.
type Service struct {
	store Store
	now   func() time.Time

	// Directory — дополнительный справочник адресатов (ИИ-капитаны в мире); nil: только игроки.
	Directory func(ctx context.Context, q string) (Person, bool)

	mu       sync.Mutex
	settings Settings
	last     map[string]time.Time
	lastText map[string]struct {
		text string
		at   time.Time
	}
}

// NewService создаёт службу с настройками по умолчанию.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now, settings: DefaultSettings(), last: map[string]time.Time{},
		lastText: map[string]struct {
			text string
			at   time.Time
		}{}}
}

// Load читает сохранённые настройки.
func (s *Service) Load(ctx context.Context) error {
	st, ok, err := s.store.LoadSettings(ctx)
	if err != nil {
		return err
	}
	if ok && st.Validate() == nil {
		s.mu.Lock()
		s.settings = st
		s.mu.Unlock()
	}
	return nil
}

// Settings возвращает текущие настройки.
func (s *Service) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.settings
	st.ExtraBlocked = append([]string{}, st.ExtraBlocked...)
	return st
}

// UpdateSettings проверяет, сохраняет и применяет настройки.
func (s *Service) UpdateSettings(ctx context.Context, st Settings) error {
	if st.ExtraBlocked == nil {
		st.ExtraBlocked = []string{}
	}
	if err := st.Validate(); err != nil {
		return err
	}
	if err := s.store.SaveSettings(ctx, st); err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	return nil
}

// Resolve находит адресата по имени или идентификатору (игроки, затем справочник ИИ).
func (s *Service) Resolve(ctx context.Context, q string) (Person, bool, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return Person{}, false, nil
	}
	p, ok, err := s.store.FindPlayer(ctx, q)
	if err != nil || ok {
		return p, ok, err
	}
	if s.Directory != nil {
		if p, ok := s.Directory(ctx, q); ok {
			return p, true, nil
		}
	}
	return Person{}, false, nil
}

// Send проверяет и сохраняет сообщение. channel: general или dm (для dm нужен адресат to).
func (s *Service) Send(ctx context.Context, from Sender, channel, to, text string) (Message, error) {
	st := s.Settings()
	if !st.Enabled {
		return Message{}, ErrDisabled
	}
	if channel != General && channel != DM {
		return Message{}, ErrBadChannel
	}
	text = CleanText(text)
	if text == "" {
		return Message{}, ErrEmpty
	}
	if utf8.RuneCountInString(text) > st.MaxLen {
		return Message{}, ErrTooLong
	}
	if !from.AI {
		if m, ok, err := s.store.MuteOf(ctx, from.ID); err != nil {
			return Message{}, err
		} else if ok && (m.Until == nil || m.Until.After(s.now())) {
			return Message{}, ErrMuted
		}
	}
	if !st.LinksAllowed && HasLink(text) {
		return Message{}, ErrLinks
	}
	if st.ProfanityFilter && HasBadWord(text, st.ExtraBlocked) {
		return Message{}, ErrBadWords
	}
	msg := Message{Channel: channel, FromID: from.ID, FromName: from.Name, FromAI: from.AI, Text: text}
	if channel == DM {
		p, ok, err := s.Resolve(ctx, to)
		if err != nil {
			return Message{}, err
		}
		if !ok {
			return Message{}, ErrNoRecipient
		}
		if p.ID == from.ID {
			return Message{}, ErrSelfDM
		}
		msg.ToID, msg.ToName = p.ID, p.Name
	}
	now := s.now()
	s.mu.Lock()
	if prev, ok := s.last[from.ID]; ok && now.Sub(prev) < time.Duration(st.MinIntervalMs)*time.Millisecond {
		s.mu.Unlock()
		return Message{}, ErrRate
	}
	if lt, ok := s.lastText[from.ID]; ok && lt.text == strings.ToLower(text) && now.Sub(lt.at) < 30*time.Second {
		s.mu.Unlock()
		return Message{}, ErrDuplicate
	}
	s.last[from.ID] = now
	s.lastText[from.ID] = struct {
		text string
		at   time.Time
	}{strings.ToLower(text), now}
	s.mu.Unlock()
	msg.At = now.UnixMilli()
	return s.store.Save(ctx, msg)
}

// SystemMessage публикует системное сообщение в общий канал (без проверок отправителя).
func (s *Service) SystemMessage(ctx context.Context, text string) (Message, error) {
	return s.store.Save(ctx, Message{Channel: System, FromID: "system", FromName: "Система", Text: CleanText(text), At: s.now().UnixMilli()})
}

// History возвращает последние сообщения общего канала и личные сообщения игрока (по возрастанию времени).
func (s *Service) History(ctx context.Context, playerID string) (general, dms []Message, err error) {
	lim := s.Settings().HistoryLimit
	if general, err = s.store.General(ctx, lim); err != nil {
		return nil, nil, err
	}
	dms, err = s.store.DMs(ctx, playerID, lim)
	return general, dms, err
}

// Purge удаляет сообщения старше срока хранения. Возвращает число удалённых.
func (s *Service) Purge(ctx context.Context) (int64, error) {
	return s.store.Purge(ctx, s.now().AddDate(0, 0, -s.Settings().RetentionDays))
}

// MuteBy заглушает игрока (minutes <= 0: бессрочно).
func (s *Service) MuteBy(ctx context.Context, admin, who string, minutes int, reason string) (Mute, error) {
	p, ok, err := s.store.FindPlayer(ctx, who)
	if err != nil {
		return Mute{}, err
	}
	if !ok {
		return Mute{}, ErrNoRecipient
	}
	m := Mute{PlayerID: p.ID, PlayerName: p.Name, Reason: strings.TrimSpace(reason), Admin: admin, CreatedAt: s.now()}
	if len([]rune(m.Reason)) > 200 {
		m.Reason = string([]rune(m.Reason)[:200])
	}
	if minutes > 0 {
		u := s.now().Add(time.Duration(minutes) * time.Minute)
		m.Until = &u
	}
	return m, s.store.SetMute(ctx, m)
}

// Unmute снимает запрет.
func (s *Service) Unmute(ctx context.Context, playerID string) error {
	return s.store.Unmute(ctx, playerID)
}

// Mutes возвращает действующие запреты.
func (s *Service) Mutes(ctx context.Context) ([]Mute, error) {
	all, err := s.store.Mutes(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	var out []Mute
	for _, m := range all {
		if m.Until == nil || m.Until.After(now) {
			out = append(out, m)
		}
	}
	return out, nil
}

// Search ищет сообщения для админки.
func (s *Service) Search(ctx context.Context, channel, q string, limit int) ([]Message, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	return s.store.Search(ctx, channel, strings.TrimSpace(q), limit)
}
