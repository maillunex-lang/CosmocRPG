package aiciv

import (
	"context"
	"log/slog"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// GoalKind is a long-term objective. The agent does not fly without one.
type GoalKind string

const (
	GoalFarmCredits GoalKind = "farm_credits" // find hostile → approach → attack → (retreat if low) → repeat or dock
	GoalIdleDock    GoalKind = "idle_dock"    // stay near station, do not wander
)

// StepKind is one concrete action in a chain.
type StepKind string

const (
	StepFindTarget StepKind = "find_target"
	StepApproach   StepKind = "approach"
	StepAttack     StepKind = "attack"
	StepRetreat    StepKind = "retreat"
	StepWait       StepKind = "wait"
)

// GoalRunner executes a long-term goal as an ordered chain of steps.
// It never issues a move without a current goal and step.
type GoalRunner struct {
	World   WorldView
	Store   *Store
	Log     *slog.Logger
	LowHull float64
}

type agentState struct {
	goal     GoalKind
	step     StepKind
	target   string
	targetPos [2]float64
	playerID string
}

// RunGoal drives one agent for the lifetime of ctx.
// Currently supports GoalFarmCredits: find → approach → attack → (retreat if low) → find again.
func (r *GoalRunner) RunGoal(ctx context.Context, g *Gateway, captainID, playerID string, goal GoalKind) {
	if r == nil || g == nil || playerID == "" {
		return
	}
	if goal == "" {
		goal = GoalFarmCredits
	}
	st := agentState{goal: goal, step: StepFindTarget, playerID: playerID}
	threshold := r.LowHull
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.3
	}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if r.World == nil {
				continue
			}
			snap, ok := r.World.Snapshot(playerID)
			if !ok {
				continue
			}
			// health override: if low, force retreat step
			if snap.Ship.HullMax > 0 && snap.Ship.Hull/snap.Ship.HullMax < threshold && st.step != StepRetreat && st.step != StepWait {
				st.step = StepRetreat
			}
			switch st.step {
			case StepFindTarget:
				id, pos := nearestHostile(snap, snap.Ship.Pos)
				if id == "" {
					// no target: do not wander; stay put
					_ = IssueMove(ctx, g, "hold", 0, 0)
					continue
				}
				st.target, st.targetPos = id, pos
				st.step = StepApproach
				if r.Log != nil {
					r.Log.Info("AI goal: target found", "captain", captainID, "target", id)
				}
			case StepApproach:
				// move toward target; if close enough, switch to attack
				dx, dy := st.targetPos[0]-snap.Ship.Pos[0], st.targetPos[1]-snap.Ship.Pos[1]
				d := mathHypot(dx, dy)
				if d < 40 {
					st.step = StepAttack
					continue
				}
				_ = IssueMove(ctx, g, "approach", dx/d, dy/d)
			case StepAttack:
				_ = IssueCombat(ctx, g, "atk", CombatIntent{TargetID: st.target, Attack: true})
				// stay in attack until health forces retreat or target dies (next snapshot will re-find)
				id, _ := nearestHostile(snap, snap.Ship.Pos)
				if id == "" || id != st.target {
					st.step = StepFindTarget
					_ = StopAttack(ctx, g, "done")
				}
			case StepRetreat:
				_, threat := nearestHostile(snap, snap.Ship.Pos)
				_ = RetreatAway(ctx, g, "ret", snap.Ship.Pos, threat)
				_ = RecordFailure(ctx, r.Store, captainID, "low-hull", time.Now())
				st.step = StepWait
				if r.Log != nil {
					r.Log.Info("AI goal: retreating", "captain", captainID)
				}
			case StepWait:
				// wait until hull recovers, then resume goal
				if snap.Ship.HullMax > 0 && snap.Ship.Hull/snap.Ship.HullMax > threshold+0.15 {
					_ = IssueMove(ctx, g, "resume", 0, 0)
					st.step = StepFindTarget
					if r.Log != nil {
						r.Log.Info("AI goal: recovered, resuming", "captain", captainID)
					}
				}
			}
		}
	}
}

func mathHypot(dx, dy float64) float64 {
	return (dx*dx + dy*dy)
}
