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

// FarmController joins one reserved captain as a real player and then runs a long-term goal.
type FarmController struct {
	Bootstrap *Bootstrapper
	Store     *Store
	World     WorldView
	Endpoint  string
	Fallback  []string
	LowHull   float64
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

// Join ensures the captain has a player and opens a Gateway. Does not start combat.
func (f *FarmController) Join(ctx context.Context, captainID string) (playerID string, g *Gateway, err error) {
	if f == nil || f.Bootstrap == nil || f.Store == nil || f.Endpoint == "" || captainID == "" {
		return "", nil, ErrInput
	}
	rec, err := f.Bootstrap.EnsurePlayer(ctx, CharacterRequest{
		CaptainID: captainID,
		Nick:      "AI-" + captainID,
		Gender:    "m",
		PilotSpec: "interceptor",
	})
	if err != nil {
		return "", nil, err
	}
	g, err = OpenGateway(ctx, f.Store, captainID, f.Endpoint)
	if err != nil {
		return "", nil, err
	}
	return rec.ID, g, nil
}

// Hold keeps the gateway open until the context is cancelled, then closes it.
func Hold(ctx context.Context, g *Gateway) {
	if g == nil {
		return
	}
	<-ctx.Done()
	g.Close()
}

// RunGoalOnce joins the first candidate and runs a long-term goal chain (no aimless flying).
func (f *FarmController) RunGoalOnce(ctx context.Context, candidates []string, goal GoalKind) {
	if len(candidates) == 0 {
		return
	}
	id := candidates[0]
	joinCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	playerID, g, err := f.Join(joinCtx, id)
	if err != nil {
		if f.Log != nil {
			f.Log.Error("AI join failed", "captain", id, "err", err)
		}
		return
	}
	if f.Log != nil {
		f.Log.Info("AI civilization: agent joined, starting goal", "captain", id, "player", playerID)
	}
	runner := &GoalRunner{World: f.World, Store: f.Store, Log: f.Log, LowHull: f.LowHull}
	if goal == "" { goal = GoalFarmCredits }
	go runner.RunGoal(ctx, g, id, playerID, goal)
	Hold(ctx, g)
}
