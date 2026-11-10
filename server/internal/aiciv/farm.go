package aiciv

import (
	"context"
	"log/slog"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// FarmController joins one reserved captain as a real player and issues a simple PvE attack.
// It is intentionally minimal: one agent, one known training target, no planner yet.
type FarmController struct {
	Bootstrap *Bootstrapper
	Store     *Store
	Endpoint  string // loopback WS, e.g. "ws://127.0.0.1:8080/ws"
	TargetID  string // hostile NPC id to farm (default "dr1")
	Log       *slog.Logger
}

// JoinAndFarm ensures the captain has a player, opens a Gateway, and starts attacking TargetID.
// Returns the open Gateway (caller must Close it). Safe to call only for a reserved captain.
func (f *FarmController) JoinAndFarm(ctx context.Context, captainID string) (*Gateway, error) {
	if f == nil || f.Bootstrap == nil || f.Store == nil || f.Endpoint == "" || captainID == "" {
		return nil, ErrInput
	}
	target := f.TargetID
	if target == "" {
		target = "dr1"
	}
	// Ensure persistent player exists (uses Studio profile check inside Bootstrapper).
	_, err := f.Bootstrap.EnsurePlayer(ctx, CharacterRequest{
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
	// Start farming the known training drone. Real world rules apply.
	err = IssueCombat(ctx, g, "farm-1", CombatIntent{TargetID: target, Attack: true})
	if err != nil {
		g.Close()
		return nil, err
	}
	if f.Log != nil {
		f.Log.Info("AI civilization: farming started", "captain", captainID, "target", target)
	}
	return g, nil
}

// RunOnce is a convenience for a single tick: pick the first candidate, reserve is expected already done by Director.
func (f *FarmController) RunOnce(ctx context.Context, candidates []string) {
	if len(candidates) == 0 {
		return
	}
	// Only try the first one for the minimal version.
	id := candidates[0]
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	g, err := f.JoinAndFarm(ctx, id)
	if err != nil {
		if f.Log != nil {
			f.Log.Error("AI farm join failed", "captain", id, "err", err)
		}
		return
	}
	// Keep the gateway open for a while so the agent stays in the world and farms.
	// Caller (Director) should manage lifetime; here we just leave it open until context ends.
	go func() {
		<-ctx.Done()
		g.Close()
	}()
	_ = world.DefaultShip // keep import if needed later
	_ = aiagent.StudioSource{}
}
