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
// It keeps short-term conversation context and avoids recent repeats.
type ChatSpeaker struct {
	Chat *chat.Service
	Log  *slog.Logger
	// Push delivers a saved line to connected clients. chat.Send only stores history.
	Push func(chat.Message)

	mu      sync.Mutex
	recent  []uint64
	context map[string][]line
}

type line struct {
	From, Text string
	At         time.Time
}

const maxRecent = 48
const maxContext = 8

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
	msg, err := s.Chat.Send(ctx, chat.Sender{ID: captainID, Name: name, AI: true}, channel, "", text)
	if err != nil {
		if s.Log != nil {
			s.Log.Error("AI chat send failed", "captain", captainID, "err", err)
		}
		return false
	}
	s.remember(channel, captainID, text)
	if s.Push != nil {
		s.Push(msg)
	}
	return true
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
