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
	recent  []uint64          // hashes of last lines (global anti-repeat)
	context map[string][]line // channel -> last few lines
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

// Seen reports whether a line was said recently (global).
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

// Last returns the last line in a channel (for follow-ups). Empty if none.
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
func (s *ChatSpeaker) Say(ctx context.Context, captainID, channel, text string) bool {
	if s == nil || s.Chat == nil || text == "" || captainID == "" {
		return false
	}
	if s.Seen(text) {
		return false
	}
	m := chat.Message{
		Channel: channel,
		FromID:  captainID,
		Text:    text,
		FromAI:  true,
		At:      time.Now(),
	}
	if err := s.Chat.Send(ctx, m); err != nil {
		if s.Log != nil {
			s.Log.Error("AI chat send failed", "captain", captainID, "err", err)
		}
		return false
	}
	s.remember(channel, captainID, text)
	return true
}

// LineForGoal returns a short contextual line based on the agent's current goal.
// Variants are chosen by time so the same agent does not spam the same sentence.
func LineForGoal(goal GoalKind, name string, now time.Time) string {
	slot := int(now.Unix()/30) % 4
	switch goal {
	case GoalFarmCredits:
		switch slot {
		case 0:
			return name + ": вижу вражеский корабль, иду на перехват."
		case 1:
			return name + ": если кто рядом — могу прикрыть."
		case 2:
			return name + ": корпус в порядке, продолжаю патруль."
		default:
			return name + ": цель снята, ищу следующую."
		}
	case GoalMineAndSell:
		switch slot {
		case 0:
			return name + ": нашёл месторождение, начинаю добычу."
		case 1:
			return name + ": трюм почти полный, скоро на станцию."
		case 2:
			return name + ": если кто знает хорошую жилу — пишите."
		default:
			return name + ": продал груз, ищу следующее."
		}
	case GoalExplore:
		switch slot {
		case 0:
			return name + ": иду проверить сигнал."
		case 1:
			return name + ": ничего опасного пока не вижу."
		default:
			return name + ": отметил точку, двигаюсь дальше."
		}
	default:
		return ""
	}
}

// FollowUp builds a short reply to the last line in the channel, if any.
// Returns empty if there is nothing to follow or the line would repeat.
func (s *ChatSpeaker) FollowUp(channel, myName string) string {
	from, text := s.Last(channel)
	if from == "" || text == "" || from == myName {
		return ""
	}
	// simple contextual follow-ups; real variety comes from not repeating
	switch {
	case containsAny(text, "враг", "атак", "бой"):
		return myName + ": понял, держусь подальше."
	case containsAny(text, "руда", "добы", "месторож"):
		return myName + ": удачи с грузом."
	case containsAny(text, "станц", "док", "причал"):
		return myName + ": тоже скоро буду."
	default:
		return myName + ": принято."
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
