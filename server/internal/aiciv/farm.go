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

// FarmController joins reserved captains as real players and runs their goals.
type FarmController struct {
	Bootstrap *Bootstrapper
	Store     *Store
	World     WorldView
	Speaker   *ChatSpeaker
	Chorus    *Chorus
	Res       *Reservation
	Endpoint  string
	Fallback  []string
	LowHull   float64
	Log       *slog.Logger
}

// CastMember is one captain to put in the world and in the chat.
type CastMember struct {
	ID, Name string
	Goal     GoalKind
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
	if f.Res != nil {
		f.Res.Reserve(captainID)
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

// RunCast joins several captains, runs each goal, and registers them as chat voices.
// Blocks until ctx is cancelled, then closes every gateway.
func (f *FarmController) RunCast(ctx context.Context, members []CastMember) {
	if f == nil || len(members) == 0 {
		return
	}
	var open []*Gateway
	for i, m := range members {
		if m.ID == "" {
			continue
		}
		if i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
		joinCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		playerID, g, err := f.Join(joinCtx, m.ID)
		cancel()
		if err != nil {
			if f.Log != nil {
				f.Log.Error("AI join failed", "captain", m.ID, "err", err)
			}
			continue
		}
		name := m.Name
		if name == "" {
			name = "AI-" + m.ID
		}
		goal := m.Goal
		if goal == "" {
			goal = GoalFarmCredits
		}
		if f.Chorus != nil {
			f.Chorus.Add(Voice{ID: m.ID, Name: name, Goal: goal})
		}
		if f.Log != nil {
			f.Log.Info("AI civilization: agent joined", "captain", m.ID, "name", name, "goal", string(goal), "player", playerID)
		}
		runner := &GoalRunner{World: f.World, Store: f.Store, Log: f.Log, LowHull: f.LowHull, Name: name}
		go runner.RunGoal(ctx, g, m.ID, playerID, goal)
		open = append(open, g)
	}
	if f.Res != nil && len(open) > 0 {
		go f.keepReserved(ctx, members)
	}
	<-ctx.Done()
	for _, g := range open {
		g.Close()
	}
}

func (f *FarmController) keepReserved(ctx context.Context, members []CastMember) {
	t := time.NewTicker(2 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, m := range members {
				f.Res.Reserve(m.ID)
			}
		}
	}
}
