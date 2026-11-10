package aiciv

import (
	"context"
	"log/slog"
	"time"
)

// Director is the minimal civilization director (AIC-030).
// It claims captains via Reservation so the old Manager does not spawn them as FSM,
// and provides a tick hook for future goal selection / combat / planning.
// Autonomous behavior stays behind feature flags; this type does not open Gateways by itself.
type Director struct {
	res       *Reservation
	maxActive int
	log       *slog.Logger
}

func NewDirector(res *Reservation, maxActive int, log *slog.Logger) *Director {
	if maxActive <= 0 {
		maxActive = 5
	}
	if log == nil {
		log = slog.Default()
	}
	return &Director{res: res, maxActive: maxActive, log: log}
}

// Tick claims up to maxActive candidates that are not already reserved.
// Returns the number of newly reserved captains.
// Actual actions (combat, mine, etc.) are issued separately once a Gateway is open for the agent.
func (d *Director) Tick(ctx context.Context, candidates []string) int {
	if d.res == nil || len(candidates) == 0 {
		return 0
	}
	claimed := 0
	for _, id := range candidates {
		if claimed >= d.maxActive {
			break
		}
		if reserved, _ := d.res.IsReserved(ctx, id); reserved {
			continue
		}
		d.res.Reserve(id)
		claimed++
		d.log.Info("AI civilization: reserved captain", "id", id)
	}
	return claimed
}

// ReleaseAll drops all current reservations (e.g. on shutdown or empty world).
func (d *Director) ReleaseAll(ids []string) {
	if d.res == nil {
		return
	}
	for _, id := range ids {
		d.res.Release(id)
	}
}

// Run runs a simple loop. Caller should cancel the context to stop.
// candidatesFn is called each tick to obtain the current pool of studio captain IDs.
func (d *Director) Run(ctx context.Context, interval time.Duration, candidatesFn func() []string) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cands := []string{}
			if candidatesFn != nil {
				cands = candidatesFn()
			}
			n := d.Tick(ctx, cands)
			if n > 0 {
				d.log.Info("AI civilization director tick", "newly_reserved", n, "active", d.res.Active())
			}
		}
	}
}
