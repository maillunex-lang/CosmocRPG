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
type FarmController struct {
	Bootstrap *Bootstrapper
	Store     *Store
	World     WorldView
	Endpoint  string // loopback WS, e.g. "ws://127.0.0.1:8080/ws"
	Fallback  []string // used only if snapshot has no hostiles
	Log       *slog.Logger
}

func nearestHostile(st world.State, me [2]float64) string {
	best, bd := "", math.MaxFloat64
	for _, o := range st.Objects {
		if o.Kind != "hostile" || o.Dead || o.ID == "" {
			continue
		}
		dx, dy := o.Pos[0]-me[0], o.Pos[1]-me[1]
		d := math.Hypot(dx, dy)
		if d < bd {
			best, bd = o.ID, d
		}
	}
	return best
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
	// Give the world a moment to register the joined player.
	time.Sleep(300 * time.Millisecond)

	target := ""
	if f.World != nil {
		if st, ok := f.World.Snapshot(rec.ID); ok {
			target = nearestHostile(st, st.Ship.Pos)
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
	return g, nil
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
