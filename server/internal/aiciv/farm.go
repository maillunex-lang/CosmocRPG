package aiciv

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// WorldView is the minimal world surface the farm controller needs.
type WorldView interface {
	Snapshot(id string) (world.State, bool)
}

// FarmController joins one reserved captain as a real player and attacks the nearest hostile from the world snapshot.
// It also monitors hull and retreats when health is low.
type FarmController struct {
	Bootstrap *Bootstrapper
	Store     *Store
	World     WorldView
	Endpoint  string // loopback WS, e.g. "ws://127.0.0.1:8080/ws"
	Fallback  []string // used only if snapshot has no hostiles
	LowHull   float64  // fraction (0-1) below which the agent retreats; default 0.3
	Log       *slog.Logger
}

func nearestHostile(st world.State, me [2]float64) (id string, pos [2]float64) {
	best, bd := "", math.MaxFloat64
	var bp [2]float64
	for _, o := range st.Objects {
		if o.Kind != "hostile" || o.Dead || o.ID == "" {
			continue
		}
		dx, dy := o.Pos[0]-me[0], o.Pos[1]-me[1]
		d := math.Hypot(dx, dy)
		if d < bd {
			best, bd, bp = o.ID, d, o.Pos
		}
	}
	return best, bp
}

// JoinAndFarm ensures the captain has a player, opens a Gateway, reads the snapshot and attacks the nearest hostile.
func (f *FarmController) JoinAndFarm(ctx context.Context, captainID string) (*Gateway, error) {
	if f == nil || f.Bootstrap == nil || f.Store == nil || f.Endpoint == "" || captainID == "" {
		return nil, ErrInput
	}
	rec, err := f.Bootstrap.EnsurePlayer(ctx, CharacterRequest{
		CaptainID: captainID,
		Nick:      "AI-" + captainID,
		Gender:    "m",
		PilotSpec: "interceptor",
	})
	if err != nil {
		return nil, err
	}
	g, err := OpenGateway(ctx, f.Store, captainID, f.Endpoint)
	if err != nil {
		return nil, err
	}
	time.Sleep(300 * time.Millisecond)

	target := ""
	if f.World != nil {
		if st, ok := f.World.Snapshot(rec.ID); ok {
			target, _ = nearestHostile(st, st.Ship.Pos)
		}
	}
	if target == "" {
		cands := f.Fallback
		if len(cands) == 0 {
			cands = []string{"dr1", "dr2"}
		}
		var e error
		target, e = ChooseTarget(ctx, f.Store, captainID, cands, time.Now())
		if e != nil || target == "" {
			target = cands[0]
		}
	}
	if f.Log != nil {
		f.Log.Info("AI civilization: agent joined", "captain", captainID, "player", rec.ID, "target", target)
	}
	err = IssueCombat(ctx, g, "farm-1", CombatIntent{TargetID: target, Attack: true})
	if err != nil {
		_ = RecordFailure(ctx, f.Store, captainID, target, time.Now())
		g.Close()
		return nil, err
	}
	_ = RecordObserved(ctx, f.Store, captainID, target, time.Now())
	if f.Log != nil {
		f.Log.Info("AI civilization: farming started", "captain", captainID, "target", target)
	}
	// Start health monitor in background.
	go f.monitorHealth(ctx, g, captainID, rec.ID)
	return g, nil
}

// monitorHealth periodically checks hull and retreats when below LowHull.
func (f *FarmController) monitorHealth(ctx context.Context, g *Gateway, captainID, playerID string) {
	threshold := f.LowHull
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.3
	}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	retreating := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if f.World == nil {
				continue
			}
			st, ok := f.World.Snapshot(playerID)
			if !ok || st.Ship.HullMax <= 0 {
				continue
			}
			frac := st.Ship.Hull / st.Ship.HullMax
			if frac < threshold && !retreating {
				_, threatPos := nearestHostile(st, st.Ship.Pos)
				if err := RetreatAway(ctx, g, "retreat-1", st.Ship.Pos, threatPos); err != nil {
					if f.Log != nil {
						f.Log.Error("AI retreat failed", "captain", captainID, "err", err)
					}
					continue
				}
				retreating = true
				_ = RecordFailure(ctx, f.Store, captainID, "low-hull", time.Now())
				if f.Log != nil {
					f.Log.Info("AI civilization: retreating (low hull)", "captain", captainID, "hull_frac", frac)
				}
			}
			if frac > threshold+0.15 && retreating {
				// recovered enough; stop forced move so the agent can re-engage later
				_ = IssueMove(ctx, g, "resume-1", 0, 0)
				retreating = false
				if f.Log != nil {
					f.Log.Info("AI civilization: recovered, can re-engage", "captain", captainID, "hull_frac", frac)
				}
			}
		}
	}
}

// Hold keeps the gateway open until the context is cancelled, then closes it.
func Hold(ctx context.Context, g *Gateway) {
	if g == nil {
		return
	}
	<-ctx.Done()
	g.Close()
}

// RunOnce tries to join the first candidate and farm for the lifetime of ctx.
func (f *FarmController) RunOnce(ctx context.Context, candidates []string) {
	if len(candidates) == 0 {
		return
	}
	id := candidates[0]
	joinCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	g, err := f.JoinAndFarm(joinCtx, id)
	if err != nil {
		if f.Log != nil {
			f.Log.Error("AI farm join failed", "captain", id, "err", err)
		}
		return
	}
	Hold(ctx, g)
}
