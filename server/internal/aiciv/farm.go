package aiciv

import (
	"context"
	"log/slog"
	"time"
)

// FarmController joins one reserved captain as a real player and issues a simple PvE attack.
// Target is chosen with memory (recent failures are skipped).
type FarmController struct {
	Bootstrap *Bootstrapper
	Store     *Store
	Endpoint  string // loopback WS, e.g. "ws://127.0.0.1:8080/ws"
	Targets   []string // candidate hostile NPC ids; default ["dr1","dr2"]
	Log       *slog.Logger
}

// JoinAndFarm ensures the captain has a player, opens a Gateway, and starts attacking a memory-aware target.
// Returns the open Gateway (caller must Close it). Call only for a captain already reserved by the Director.
func (f *FarmController) JoinAndFarm(ctx context.Context, captainID string) (*Gateway, error) {
	if f == nil || f.Bootstrap == nil || f.Store == nil || f.Endpoint == "" || captainID == "" {
		return nil, ErrInput
	}
	cands := f.Targets
	if len(cands) == 0 {
		cands = []string{"dr1", "dr2"}
	}
	now := time.Now()
	target, err := ChooseTarget(ctx, f.Store, captainID, cands, now)
	if err != nil {
		return nil, err
	}
	_, err = f.Bootstrap.EnsurePlayer(ctx, CharacterRequest{
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
	if f.Log != nil {
		f.Log.Info("AI civilization: agent joined", "captain", captainID, "target", target)
	}
	err = IssueCombat(ctx, g, "farm-1", CombatIntent{TargetID: target, Attack: true})
	if err != nil {
		_ = RecordFailure(ctx, f.Store, captainID, target, now)
		g.Close()
		return nil, err
	}
	_ = RecordObserved(ctx, f.Store, captainID, target, now)
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
