package aiciv

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

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
	recent  []string
	context map[string][]line
}

type line struct {
	From, Text string
	At         time.Time
}

const maxRecent = 180
const maxContext = 12

func normLine(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "ё", "е"))
	var b strings.Builder
	space := false
	for _, r := range s {
		switch r {
		case '!', '?', '.', ',', ':', ';', '«', '»', '"', '\'', '—', '–', '-', '(', ')', '…':
			r = ' '
		}
		if unicode.IsSpace(r) {
			if space || b.Len() == 0 {
				continue
			}
			b.WriteByte(' ')
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func contentWords(s string) []string {
	var out []string
	for _, w := range strings.Fields(s) {
		if len([]rune(w)) >= 4 {
			out = append(out, w)
		}
	}
	return out
}

// tooClose — одно и то же предложение или почти те же слова.
func tooClose(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	wa, wb := strings.Fields(a), strings.Fields(b)
	n := 0
	for n < len(wa) && n < len(wb) && wa[n] == wb[n] {
		n++
	}
	if n >= 4 {
		return true
	}
	ca, cb := contentWords(a), contentWords(b)
	if len(ca) == 0 || len(cb) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, w := range ca {
		set[w] = struct{}{}
	}
	inter := 0
	seen := map[string]struct{}{}
	for _, w := range cb {
		if _, ok := set[w]; !ok {
			continue
		}
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		inter++
	}
	union := len(set)
	for _, w := range cb {
		if _, ok := set[w]; !ok {
			union++
		}
	}
	if union == 0 {
		return false
	}
	return inter*10 >= union*6
}

func NewChatSpeaker(c *chat.Service, log *slog.Logger) *ChatSpeaker {
	return &ChatSpeaker{Chat: c, Log: log, context: map[string][]line{}}
}

func (s *ChatSpeaker) Seen(text string) bool {
	n := normLine(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.recent {
		if tooClose(n, x) {
			return true
		}
	}
	return false
}

func (s *ChatSpeaker) remember(channel, from, text string) {
	n := normLine(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, n)
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
