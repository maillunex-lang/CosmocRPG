package observability

import (
	"sync"
	"time"
)

type Event struct {
	Time   time.Time      `json:"time"`
	Kind   string         `json:"kind"`
	Player string         `json:"player,omitempty"`
	Ref    string         `json:"ref,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type Log struct {
	mu     sync.Mutex
	max    int
	events []Event
}

func New(max int) *Log {
	if max < 1 {
		max = 256
	}
	return &Log{max: max, events: make([]Event, 0, max)}
}

func (l *Log) Add(kind, player, ref string, meta map[string]any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	clean := map[string]any{}
	for k, v := range meta {
		if k != "token" && k != "password" && k != "secret" {
			clean[k] = v
		}
	}
	l.events = append(l.events, Event{Time: time.Now().UTC(), Kind: kind, Player: player, Ref: ref, Meta: clean})
	if len(l.events) > l.max {
		l.events = l.events[len(l.events)-l.max:]
	}
}

func (l *Log) Snapshot(limit int) []Event {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > l.max {
		limit = l.max
	}
	start := len(l.events) - limit
	if start < 0 {
		start = 0
	}
	out := make([]Event, len(l.events[start:]))
	copy(out, l.events[start:])
	return out
}
