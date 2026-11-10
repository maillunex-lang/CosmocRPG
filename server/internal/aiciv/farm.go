package aiciv

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/corp"
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
	Board     *Board
	Live      *Live
	Res       *Reservation
	Endpoint  string
	Fallback  []string
	LowHull   float64
	Log       *slog.Logger
	Clans     *corp.Service
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
		arch := ArchetypeFor(m.ID, goal)
		if f.Chorus != nil {
			f.Chorus.Add(Voice{ID: m.ID, Name: name, Goal: goal, Arch: arch})
		}
		if f.Log != nil {
			f.Log.Info("AI civilization: agent joined", "captain", m.ID, "name", name, "goal", string(goal), "player", playerID)
		}
		runner := &GoalRunner{World: f.World, Store: f.Store, Board: f.Board, Live: f.Live, Log: f.Log, LowHull: f.LowHull, Name: name, Clans: f.Clans}
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


// Supervise keeps the cast aligned with admin knobs. Changing the count reconnects captains.
func (f *FarmController) Supervise(ctx context.Context, src func(n int) []CastMember) {
	if f == nil || src == nil {
		return
	}
	var cancel context.CancelFunc
	sig := ""
	stop := func() {
		if cancel != nil {
			cancel()
			cancel = nil
		}
	}
	defer stop()
	wait := time.NewTimer(3 * time.Second)
	select {
	case <-ctx.Done():
		return
	case <-wait.C:
	}
	apply := func() {
		k := DefaultKnobs()
		if f.Live != nil {
			k = f.Live.Get()
		}
		want := 0
		if k.Enabled {
			want = k.Cast
		}
		next := fmt.Sprintf("%t:%d", k.Enabled, want)
		if next == sig && (cancel != nil || want == 0) {
			return
		}
		if want == 0 {
			stop()
			sig = next
			if f.Log != nil {
				f.Log.Info("AI civilization cast stopped")
			}
			return
		}
		members := src(want)
		if len(members) == 0 {
			return
		}
		stop()
		sig = next
		cctx, ccancel := context.WithCancel(ctx)
		cancel = ccancel
		if f.Log != nil {
			f.Log.Info("AI civilization cast", "captains", len(members))
		}
		go f.RunCast(cctx, members)
	}
	apply()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			apply()
		}
	}
}
