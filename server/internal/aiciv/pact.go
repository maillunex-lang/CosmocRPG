package aiciv

import (
	"sync"
	"time"
)

// PactKind is something said in chat that can become a real flight.
type PactKind string

const (
	PactPirates PactKind = "pirates"
	PactQuest   PactKind = "quest"
	PactWing    PactKind = "wing"
)

// Order is a promise already accepted. GoalRunner executes it instead of the private goal.
type Order struct {
	Kind   PactKind
	Leader string // captain id to fly toward
	Until  time.Time
	Style  Archetype
}

// Decision is a reply that is not applied until the line is actually sent.
type Decision struct {
	Line     string
	AskerID  string
	AskerArch Archetype
	Order    *Order
	Bait     *Order // troll who asked runs away from the helper
	Troll    bool
	Refused  bool
}

type storedAsk struct {
	Kind PactKind
	From Voice
	At   time.Time
}

// Board binds chat to movement. There is no player party window in the game;
// a wing is ships that really fly together and shoot the same pirates.
type Board struct {
	mu     sync.Mutex
	asks   map[string]storedAsk
	orders map[string]Order
}

func NewBoard() *Board {
	return &Board{asks: map[string]storedAsk{}, orders: map[string]Order{}}
}

func (b *Board) Ask(from Voice, kind PactKind, now time.Time) {
	if b == nil || from.ID == "" {
		return
	}
	b.mu.Lock()
	b.asks[from.ID] = storedAsk{Kind: kind, From: from, At: now}
	b.mu.Unlock()
}

// Decide picks someone else's open request and a personality-based answer.
// It does not change the board.
func (b *Board) Decide(who Voice, now time.Time) (Decision, bool) {
	if b == nil || who.ID == "" {
		return Decision{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var found storedAsk
	var id string
	var ok bool
	for k, a := range b.asks {
		if k == who.ID || now.Sub(a.At) > 2*time.Minute {
			continue
		}
		if !ok || a.At.Before(found.At) {
			found, id, ok = a, k, true
		}
	}
	if !ok {
		return Decision{}, false
	}
	d := Decision{AskerID: id, AskerArch: found.From.Arch}
	switch willHelp(who.Arch, who.Goal, found.Kind) {
	case "lie":
		d.Line = lieLine(who.Arch)
		d.Troll = true
	case "refuse":
		d.Line = refuseLine(who.Arch, found.Kind)
		d.Refused = true
	default:
		d.Line = acceptLine(who.Arch, found.Kind)
		style := who.Arch
		if style == "" {
			style = ArchPro
		}
		d.Order = &Order{Kind: found.Kind, Leader: id, Until: now.Add(90 * time.Second), Style: style}
		if found.From.Arch == ArchTroll {
			bait := Order{Kind: found.Kind, Leader: who.ID, Until: now.Add(70 * time.Second), Style: ArchTroll}
			d.Bait = &bait
		}
	}
	return d, true
}

// Commit applies a decision after the reply was sent.
func (b *Board) Commit(d Decision, responder string) {
	if b == nil || d.AskerID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.asks, d.AskerID)
	if d.Order != nil && responder != "" {
		b.orders[responder] = *d.Order
	}
	if d.Bait != nil && d.AskerID != "" {
		b.orders[d.AskerID] = *d.Bait
	}
}

func (b *Board) OrderFor(id string, now time.Time) (Order, bool) {
	if b == nil || id == "" {
		return Order{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	ord, ok := b.orders[id]
	if !ok {
		return Order{}, false
	}
	if !ord.Until.After(now) {
		delete(b.orders, id)
		return Order{}, false
	}
	return ord, true
}
