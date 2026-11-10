package aiciv

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
)

// ChatSpeaker posts non-scripted lines for AI civilization agents.
// It keeps short-term conversation context, avoids recent repeats, and can follow up on previous messages.
type ChatSpeaker struct {
	Chat *chat.Service
	Log  *slog.Logger

	mu      sync.Mutex
	recent  []uint64
	context map[string][]line
}

type line struct {
	From, Text string
	At         time.Time
}

const maxRecent = 24
const maxContext = 6

func NewChatSpeaker(c *chat.Service, log *slog.Logger) *ChatSpeaker {
	return &ChatSpeaker{Chat: c, Log: log, context: map[string][]line{}}
}

func hashLine(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func (s *ChatSpeaker) Seen(text string) bool {
	h := hashLine(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.recent {
		if x == h {
			return true
		}
	}
	return false
}

func (s *ChatSpeaker) remember(channel, from, text string) {
	h := hashLine(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, h)
	if len(s.recent) > maxRecent {
		s.recent = s.recent[len(s.recent)-maxRecent:]
	}
	l := s.context[channel]
	l = append(l, line{From: from, Text: text, At: time.Now()})
	if len(l) > maxContext {
		l = l[len(l)-maxContext:]
	}
	s.context[channel] = l
}

func (s *ChatSpeaker) Last(channel string) (from, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.context[channel]
	if len(l) == 0 {
		return "", ""
	}
	last := l[len(l)-1]
	return last.From, last.Text
}

// Say posts a line if it was not said recently. Returns false if skipped.
func (s *ChatSpeaker) Say(ctx context.Context, captainID, name, channel, text string) bool {
	if s == nil || s.Chat == nil || text == "" || captainID == "" {
		return false
	}
	if s.Seen(text) {
		return false
	}
	_, err := s.Chat.Send(ctx, chat.Sender{ID: captainID, Name: name, AI: true}, channel, "", text)
	if err != nil {
		if s.Log != nil {
			s.Log.Error("AI chat send failed", "captain", captainID, "err", err)
		}
		return false
	}
	s.remember(channel, captainID, text)
	return true
}

// LineForGoal returns a short contextual line based on the agent's current goal.
// Variants rotate by time so the same agent does not spam the same sentence.
func LineForGoal(goal GoalKind, name string, now time.Time) string {
	slot := int(now.Unix()/45) % 4
	switch goal {
	case GoalFarmCredits:
		switch slot {
		case 0:
			return "вижу вражеский корабль, иду на перехват."
		case 1:
			return "если кто рядом — могу прикрыть."
		case 2:
			return "корпус в порядке, продолжаю патруль."
		default:
			return "цель снята, ищу следующую."
		}
	case GoalMineAndSell:
		switch slot {
		case 0:
			return "нашёл месторождение, начинаю добычу."
		case 1:
			return "трюм почти полный, скоро на станцию."
		case 2:
			return "если кто знает хорошую жилу — пишите."
		default:
			return "продал груз, ищу следующее."
		}
	case GoalExplore:
		switch slot {
		case 0:
			return "иду проверить сигнал."
		case 1:
			return "ничего опасного пока не вижу."
		default:
			return "отметил точку, двигаюсь дальше."
		}
	default:
		return ""
	}
}

// FollowUp builds a short reply to the last line in the channel, if any.
func (s *ChatSpeaker) FollowUp(channel, myID string) string {
	from, text := s.Last(channel)
	if from == "" || text == "" || from == myID {
		return ""
	}
	switch {
	case containsAny(text, "враг", "атак", "бой"):
		return "понял, держусь подальше."
	case containsAny(text, "руда", "добы", "месторож"):
		return "удачи с грузом."
	case containsAny(text, "станц", "док", "причал"):
		return "тоже скоро буду."
	default:
		return "принято."
	}
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if len(w) > 0 && len(s) >= len(w) {
			for i := 0; i+len(w) <= len(s); i++ {
				if s[i:i+len(w)] == w {
					return true
				}
			}
		}
	}
	return false
}
