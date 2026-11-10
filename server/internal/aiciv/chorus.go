package aiciv

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
)

// Voice is one AI captain that can speak in the shared channel.
type Voice struct {
	ID, Name string
	Goal     GoalKind
	Arch     Archetype
}

// Chorus runs short AI↔AI exchanges.
// A request is not just a line: if someone agrees, Board gives them a flight order.
type Chorus struct {
	Speaker *ChatSpeaker
	Board   *Board
	Log     *slog.Logger

	mu         sync.Mutex
	voices     []Voice
	lastID     string
	beat       int
	follow     string
	followLine string
	pauseUntil time.Time
}

func NewChorus(s *ChatSpeaker, log *slog.Logger) *Chorus {
	return &Chorus{Speaker: s, Log: log}
}

func (c *Chorus) Add(v Voice) {
	if c == nil || v.ID == "" {
		return
	}
	if v.Name == "" {
		v.Name = v.ID
	}
	if v.Arch == "" {
		v.Arch = ArchetypeFor(v.ID, v.Goal)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, x := range c.voices {
		if x.ID == v.ID {
			return
		}
	}
	c.voices = append(c.voices, v)
}

func (c *Chorus) snapshot() []Voice {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Voice, len(c.voices))
	copy(out, c.voices)
	return out
}

func (c *Chorus) Run(ctx context.Context, interval time.Duration) {
	if c == nil || c.Speaker == nil {
		return
	}
	if interval < 8*time.Second {
		interval = 12 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			c.turn(ctx, now)
		}
	}
}

func (c *Chorus) turn(ctx context.Context, now time.Time) {
	if now.Before(c.pauseUntil) {
		return
	}
	voices := c.snapshot()
	if len(voices) < 2 {
		return
	}
	if c.beat == 2 && c.follow != "" {
		speaker := findVoice(voices, c.follow)
		line := c.followLine
		c.follow, c.followLine = "", ""
		c.beat = 0
		c.pauseUntil = now.Add(40 * time.Second)
		if speaker.ID != "" && line != "" {
			_ = c.Speaker.Say(ctx, speaker.ID, speaker.Name, "general", line)
			c.lastID = speaker.ID
		}
		return
	}
	speaker := pickOther(voices, c.lastID, int(now.Unix()/12))
	if speaker.ID == "" {
		return
	}
	if c.beat == 1 && c.Board != nil {
		if d, ok := c.Board.Decide(speaker, now); ok && d.Line != "" {
			if c.Speaker.Say(ctx, speaker.ID, speaker.Name, "general", d.Line) {
				c.Board.Commit(d, speaker.ID)
				c.lastID = speaker.ID
				c.beat = 2
				c.follow = d.AskerID
				c.followLine = closeLine(d)
				if c.Log != nil {
					c.Log.Info("AI pact", "from", speaker.Name, "arch", string(speaker.Arch), "asker", d.AskerID, "goes", d.Order != nil, "lie", d.Troll)
				}
				return
			}
		}
	}
	if c.beat == 0 && c.Board != nil && now.Unix()/12%3 == 0 {
		kind := askKind(speaker.Goal)
		lines := requestLines(speaker.Arch, kind)
		if c.sayFirst(ctx, speaker, lines) {
			c.Board.Ask(speaker, kind, now)
			c.lastID = speaker.ID
			c.beat = 1
			return
		}
	}
	lines := rotate(personaLines(speaker.Arch, speaker.Goal), int(now.Unix()/30)+len(speaker.ID))
	if !c.sayFirst(ctx, speaker, lines) {
		return
	}
	c.lastID = speaker.ID
	c.beat++
	if c.beat >= 3 {
		c.beat = 0
		c.pauseUntil = now.Add(45 * time.Second)
	}
}

func closeLine(d Decision) string {
	switch {
	case d.Order != nil && d.Bait != nil:
		return "ладно, жду у обломков."
	case d.Order != nil:
		return "вижу, подходи на мой курс."
	case d.Troll:
		return "опять обещал. сам разберусь."
	default:
		return "понял, тогда не жду."
	}
}

func (c *Chorus) sayFirst(ctx context.Context, v Voice, lines []string) bool {
	for _, text := range lines {
		if text == "" {
			continue
		}
		if c.Speaker.Say(ctx, v.ID, v.Name, "general", text) {
			return true
		}
	}
	return false
}

func findVoice(voices []Voice, id string) Voice {
	for _, v := range voices {
		if v.ID == id {
			return v
		}
	}
	return Voice{}
}

func pickOther(voices []Voice, except string, salt int) Voice {
	if len(voices) == 0 {
		return Voice{}
	}
	if salt < 0 {
		salt = -salt
	}
	start := salt % len(voices)
	for i := 0; i < len(voices); i++ {
		v := voices[(start+i)%len(voices)]
		if v.ID != except {
			return v
		}
	}
	return Voice{}
}

func rotate(lines []string, salt int) []string {
	if len(lines) == 0 {
		return nil
	}
	if salt < 0 {
		salt = -salt
	}
	salt = salt % len(lines)
	out := make([]string, 0, len(lines))
	out = append(out, lines[salt:]...)
	return append(out, lines[:salt]...)
}

// PickCast chooses up to n captains, preferring different professions so the chat has different voices.
func PickCast(in []aiagent.Profile, n int) []aiagent.Profile {
	if n < 2 {
		n = 2
	}
	if n > 4 {
		n = 4
	}
	seenBeh := map[string]bool{}
	var out []aiagent.Profile
	for _, p := range in {
		if p.ID == "" {
			continue
		}
		b := aiagent.BehaviorFor(p.Profession)
		if seenBeh[b] {
			continue
		}
		seenBeh[b] = true
		out = append(out, p)
		if len(out) == n {
			return out
		}
	}
	have := map[string]bool{}
	for _, p := range out {
		have[p.ID] = true
	}
	for _, p := range in {
		if p.ID == "" || have[p.ID] {
			continue
		}
		out = append(out, p)
		if len(out) == n {
			break
		}
	}
	return out
}
