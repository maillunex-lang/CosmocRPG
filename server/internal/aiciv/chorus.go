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

// chatScene is one conversation. Length is chosen once, from 3 to 15 lines.
type chatScene struct {
	topic string
	step  int
	need  int
	pact  bool
	from  string
	kind  PactKind
}

// Chorus runs AI↔AI talks.
// Most talks are off-topic, the way people actually write in local.
// Sometimes a line is a real request: agreement still becomes a flight order.
type Chorus struct {
	Speaker *ChatSpeaker
	Board   *Board
	Live    *Live
	Log     *slog.Logger

	mu         sync.Mutex
	voices     []Voice
	lastID     string
	rng        uint32
	scene      *chatScene
	follow     string
	followLine string
	pauseUntil time.Time
	nextLine   time.Time
	baseGap    time.Duration
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

func (c *Chorus) roll() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rng == 0 {
		c.rng = uint32(time.Now().UnixNano()) | 1
	}
	c.rng ^= c.rng << 13
	c.rng ^= c.rng >> 17
	c.rng ^= c.rng << 5
	if c.rng == 0 {
		c.rng = 1
	}
	return c.rng
}

func (c *Chorus) Run(ctx context.Context, interval time.Duration) {
	if c == nil || c.Speaker == nil {
		return
	}
	if interval < 4*time.Second {
		interval = 12 * time.Second
	}
	c.baseGap = interval
	t := time.NewTicker(4 * time.Second)
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

func (c *Chorus) gap(fallback time.Duration) time.Duration {
	if c.Live == nil {
		return fallback
	}
	sec := c.Live.Get().IntervalSec
	if sec < 4 {
		sec = 12
	}
	return time.Duration(sec) * time.Second
}

func (c *Chorus) turn(ctx context.Context, now time.Time) {
	if now.Before(c.pauseUntil) || now.Before(c.nextLine) {
		return
	}
	if c.Live != nil && !c.Live.Get().Chat {
		return
	}
	c.nextLine = now.Add(c.gap(c.baseGap))
	voices := c.snapshot()
	if len(voices) < 2 {
		return
	}
	if c.scene == nil {
		c.scene = c.pickScene(voices)
		if c.Log != nil {
			c.Log.Info("AI talk started", "topic", c.scene.topic, "lines", c.scene.need, "pact", c.scene.pact)
		}
	}
	s := c.scene
	if s.step >= s.need {
		c.scene = nil
		c.follow, c.followLine = "", ""
		c.pauseUntil = now.Add(time.Duration(18+c.roll()%25) * time.Second)
		return
	}
	if s.pact && s.step < 3 {
		c.pactTurn(ctx, voices, s, now)
		return
	}
	speaker := pickOther(voices, c.lastID, int(c.roll()))
	if speaker.ID == "" {
		s.step++
		return
	}
	topic := s.topic
	if topic == "" {
		topic = "quiet"
	}
	lines := banterLines(topic, s.step, speaker.Arch, int(c.roll()))
	if c.sayFirst(ctx, speaker, lines) {
		c.lastID = speaker.ID
	}
	s.step++
}

func (c *Chorus) pickScene(voices []Voice) *chatScene {
	minN, maxN := 3, 15
	help := c.Board != nil
	if c.Live != nil {
		k := c.Live.Get()
		if k.TalkMin >= 1 {
			minN = k.TalkMin
		}
		if k.TalkMax >= minN {
			maxN = k.TalkMax
		}
		help = help && k.Help
	}
	span := maxN - minN + 1
	need := minN
	if span > 1 {
		need = minN + int(c.roll()%uint32(span))
	}
	if help && c.roll()%5 == 0 {
		v := voices[int(c.roll())%len(voices)]
		return &chatScene{pact: true, need: need, from: v.ID, kind: askKind(v.Goal), topic: "quiet"}
	}
	topic := topicIDs[int(c.roll())%len(topicIDs)]
	return &chatScene{topic: topic, need: need}
}

func (c *Chorus) pactTurn(ctx context.Context, voices []Voice, s *chatScene, now time.Time) {
	switch s.step {
	case 0:
		speaker := findVoice(voices, s.from)
		if speaker.ID == "" {
			speaker = voices[int(c.roll())%len(voices)]
			s.from = speaker.ID
			s.kind = askKind(speaker.Goal)
		}
		lines := requestLines(speaker.Arch, s.kind)
		if c.sayFirst(ctx, speaker, lines) {
			c.Board.Ask(speaker, s.kind, now)
			c.lastID = speaker.ID
		}
		s.step++
	case 1:
		speaker := pickOther(voices, s.from, int(c.roll()))
		if speaker.ID == "" {
			s.step++
			return
		}
		d, ok := c.Board.Decide(speaker, now)
		if !ok || d.Line == "" || !c.Speaker.Say(ctx, speaker.ID, speaker.Name, "general", d.Line) {
			// Nobody took the request. Continue as ordinary talk.
			s.pact = false
			s.step++
			return
		}
		c.Board.Commit(d, speaker.ID)
		c.lastID = speaker.ID
		c.follow = d.AskerID
		c.followLine = closeLine(d)
		s.step++
		if c.Log != nil {
			c.Log.Info("AI pact", "from", speaker.Name, "arch", string(speaker.Arch), "asker", d.AskerID, "goes", d.Order != nil, "lie", d.Troll)
		}
	default:
		speaker := findVoice(voices, c.follow)
		line := c.followLine
		c.follow, c.followLine = "", ""
		if speaker.ID != "" && line != "" {
			_ = c.Speaker.Say(ctx, speaker.ID, speaker.Name, "general", line)
			c.lastID = speaker.ID
		}
		s.pact = false
		s.step++
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
	if n < 1 {
		n = 1
	}
	if n > 500 {
		n = 500
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
