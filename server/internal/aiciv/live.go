package aiciv

import "sync"

// Knobs is the part of AI behavior an admin can change without a restart.
type Knobs struct {
	Enabled     bool
	Cast        int
	Chat        bool
	TalkMin     int
	TalkMax     int
	IntervalSec int
	RetreatPct  int
	Help        bool
}

// DefaultKnobs matches a fresh admin settings row.
func DefaultKnobs() Knobs {
	return Knobs{Enabled: true, Cast: 3, Chat: true, TalkMin: 3, TalkMax: 15, IntervalSec: 12, RetreatPct: 30, Help: true}
}

// Live is the settings the running captains actually read.
type Live struct {
	mu sync.RWMutex
	k  Knobs
}

func NewLive(k Knobs) *Live {
	l := &Live{}
	l.Set(k)
	return l
}

func (l *Live) Get() Knobs {
	if l == nil {
		return DefaultKnobs()
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.k
}

func (l *Live) Set(k Knobs) {
	if l == nil {
		return
	}
	if k.Cast < 1 {
		k.Cast = 1
	}
	if k.Cast > 8 {
		k.Cast = 8
	}
	if k.TalkMin < 1 {
		k.TalkMin = 1
	}
	if k.TalkMax < k.TalkMin {
		k.TalkMax = k.TalkMin
	}
	if k.TalkMax > 30 {
		k.TalkMax = 30
	}
	if k.IntervalSec < 4 {
		k.IntervalSec = 4
	}
	if k.IntervalSec > 120 {
		k.IntervalSec = 120
	}
	if k.RetreatPct < 5 {
		k.RetreatPct = 5
	}
	if k.RetreatPct > 90 {
		k.RetreatPct = 90
	}
	l.mu.Lock()
	l.k = k
	l.mu.Unlock()
}
