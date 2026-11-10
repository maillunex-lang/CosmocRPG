package aiciv

import (
	"context"
	"sync"
	"time"
)

// Reservation tracks captains that the new civilization system has claimed.
// The old aiagent.Manager must not spawn these as FSM ships to avoid duplicates.
type Reservation struct {
	mu   sync.RWMutex
	ids  map[string]time.Time
	ttl  time.Duration
}

func NewReservation(ttl time.Duration) *Reservation {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Reservation{ids: map[string]time.Time{}, ttl: ttl}
}

// Reserve marks a captain as managed by the new system. Idempotent.
func (r *Reservation) Reserve(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids[id] = time.Now()
}

// Release removes the reservation. Safe to call if not reserved.
func (r *Reservation) Release(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.ids, id)
}

// IsReserved reports whether the captain is currently reserved by the new system.
// Expired reservations are cleaned lazily.
func (r *Reservation) IsReserved(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	at, ok := r.ids[id]
	if !ok {
		return false, nil
	}
	if time.Since(at) > r.ttl {
		delete(r.ids, id)
		return false, nil
	}
	return true, nil
}

// Active returns the number of currently reserved captains (for metrics).
func (r *Reservation) Active() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.ids)
}
