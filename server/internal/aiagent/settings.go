// Package aiagent — «агент мира» ИИ-игроков (R-40, R-42, этап ИИ-1): решает, сколько ИИ-капитанов держать онлайн и кого именно
// (из профилей AI Studio), и добавляет их в мир. Правила R-42: ИИ учитываются отдельно от живых игроков (лимит ИИ 200),
// без живых игроков ИИ нет онлайн, при живых игроках около 20–50, скорость набора и снятия ограничена.
package aiagent

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Behavior-ключи поведения (совпадают с world.AI*).
var behaviors = []string{"miner", "trader", "scout", "patrol", "support"}

// Settings — настройки ИИ-игроков (правятся из админки).
type Settings struct {
	Enabled       bool           `json:"enabled"`         // выключатель: выключено — ИИ нет онлайн
	Paused        bool           `json:"paused"`          // пауза: число ИИ не меняется (ни набора, ни снятия)
	MinOnline     int            `json:"min_online"`      // сколько ИИ при появлении первого живого игрока
	MaxOnline     int            `json:"max_online"`      // сколько ИИ при PlayersForMax и более живых игроках
	AbsMax        int            `json:"abs_max"`         // жёсткий потолок ИИ (лимит 200 по R-42)
	PlayersForMax int            `json:"players_for_max"` // при скольких живых игроках достигается MaxOnline
	SpawnPerMin   int            `json:"spawn_per_min"`   // не больше стольких появлений в минуту
	DespawnPerMin int            `json:"despawn_per_min"` // не больше стольких уходов в минуту
	EmptyGraceSec int            `json:"empty_grace_sec"` // сколько секунд ИИ остаются после ухода последнего игрока
	ViewRadius    float64        `json:"view_radius"`     // радиус видимости ИИ для игрока
	Shares        map[string]int `json:"shares"`          // доли поведений (в процентах, сумма не обязана быть 100)
	// Реплики в чате (ИИ-2) и индивидуальное включение капитанов (ИИ-3).
	ChatReplyEnabled       bool     `json:"chat_reply_enabled"`        // ИИ отвечают в общем и личном чате
	ChatReplyChance        int      `json:"chat_reply_chance"`         // вероятность ответа в общем канале, 0–100
	ChatDelayMinMs         int      `json:"chat_delay_min_ms"`         // задержка ответа: от (естественность)
	ChatDelayMaxMs         int      `json:"chat_delay_max_ms"`         // задержка ответа: до
	ChatCaptainCooldownSec int      `json:"chat_captain_cooldown_sec"` // личный кулдаун капитана
	ChatMaxPerMin          int      `json:"chat_max_per_min"`          // общий потолок ИИ-реплик в минуту
	ChatAmbientEverySec    int      `json:"chat_ambient_every_sec"`    // ИИ пишут в общий чат сами раз в столько секунд (0 — не пишут сами)
	ChatFollowChance       int      `json:"chat_follow_chance"`        // вероятность, что другой ИИ ответит на реплику ИИ, 0–100
	DisabledCaptains       []string `json:"disabled_captains"`         // капитаны, которых не ставить в мир и не давать в чат
}

// DisabledSet возвращает множество отключённых капитанов.
func (s Settings) DisabledSet() map[string]bool {
	out := map[string]bool{}
	for _, id := range s.DisabledCaptains {
		out[id] = true
	}
	return out
}

// DefaultSettings — решения владельца R-42: 20–50 ИИ при живых игроках, потолок 200.
func DefaultSettings() Settings {
	return Settings{Enabled: true, MinOnline: 20, MaxOnline: 50, AbsMax: 200, PlayersForMax: 10, SpawnPerMin: 12, DespawnPerMin: 30,
		EmptyGraceSec: 30, ViewRadius: 450, Shares: map[string]int{"miner": 30, "trader": 20, "scout": 20, "patrol": 15, "support": 15},
		ChatReplyEnabled: true, ChatReplyChance: 25, ChatDelayMinMs: 800, ChatDelayMaxMs: 2500, ChatCaptainCooldownSec: 30, ChatMaxPerMin: 30, ChatAmbientEverySec: 45, ChatFollowChance: 40}
}

// Validate проверяет настройки и возвращает понятную ошибку.
func (s Settings) Validate() error {
	switch {
	case s.AbsMax < 0 || s.AbsMax > 200:
		return errors.New("потолок ИИ должен быть от 0 до 200")
	case s.MinOnline < 0 || s.MaxOnline < s.MinOnline:
		return errors.New("минимум ИИ не может быть больше максимума")
	case s.MaxOnline > s.AbsMax:
		return fmt.Errorf("максимум ИИ (%d) выше потолка (%d)", s.MaxOnline, s.AbsMax)
	case s.PlayersForMax < 1 || s.PlayersForMax > 50:
		return errors.New("«игроков для максимума» — от 1 до 50")
	case s.SpawnPerMin < 1 || s.SpawnPerMin > 600 || s.DespawnPerMin < 1 || s.DespawnPerMin > 600:
		return errors.New("скорость набора и снятия — от 1 до 600 в минуту")
	case s.EmptyGraceSec < 0 || s.EmptyGraceSec > 3600:
		return errors.New("задержка снятия — от 0 до 3600 секунд")
	case s.ViewRadius < 100 || s.ViewRadius > 2000:
		return errors.New("радиус видимости — от 100 до 2000")
	}
	switch {
	case s.ChatReplyChance < 0 || s.ChatReplyChance > 100:
		return errors.New("вероятность ответа — от 0 до 100")
	case s.ChatDelayMinMs < 0 || s.ChatDelayMaxMs < s.ChatDelayMinMs || s.ChatDelayMaxMs > 30000:
		return errors.New("задержка ответа: 0…30000 мс, минимум не больше максимума")
	case s.ChatCaptainCooldownSec < 0 || s.ChatCaptainCooldownSec > 3600:
		return errors.New("кулдаун капитана — от 0 до 3600 секунд")
	case s.ChatMaxPerMin < 0 || s.ChatMaxPerMin > 120:
		return errors.New("потолок реплик — от 0 до 120 в минуту")
	case s.ChatAmbientEverySec != 0 && (s.ChatAmbientEverySec < 10 || s.ChatAmbientEverySec > 3600):
		return errors.New("самостоятельные реплики: 0 (выключено) или от 10 до 3600 секунд")
	case s.ChatFollowChance < 0 || s.ChatFollowChance > 100:
		return errors.New("вероятность ответа ИИ на ИИ — от 0 до 100")
	}
	for _, id := range s.DisabledCaptains {
		if id == "" || len(id) > 48 {
			return errors.New("некорректный идентификатор отключённого капитана")
		}
	}
	total := 0
	for k, v := range s.Shares {
		ok := false
		for _, b := range behaviors {
			ok = ok || b == k
		}
		if !ok || v < 0 || v > 100 {
			return fmt.Errorf("доля «%s» некорректна", k)
		}
		total += v
	}
	if total <= 0 {
		return errors.New("хотя бы одна доля поведения должна быть больше нуля")
	}
	return nil
}

// Target — сколько ИИ держать онлайн при live живых игроках (до проверки лимитов набора). Нет игроков или ИИ выключены — 0.
func (s Settings) Target(live int) int {
	if !s.Enabled || live <= 0 {
		return 0
	}
	span := s.MaxOnline - s.MinOnline
	k := live
	if k > s.PlayersForMax {
		k = s.PlayersForMax
	}
	t := s.MinOnline + span*k/s.PlayersForMax
	if t > s.AbsMax {
		t = s.AbsMax
	}
	return t
}

// ShareList возвращает поведения с положительными долями в стабильном порядке.
func (s Settings) shareList() []string {
	var out []string
	for k, v := range s.Shares {
		if v > 0 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// SettingsStore — хранилище настроек.
type SettingsStore interface {
	Load(ctx context.Context) (Settings, bool, error)
	Save(ctx context.Context, s Settings, admin string) error
}
