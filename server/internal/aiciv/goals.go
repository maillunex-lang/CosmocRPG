package aiciv

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// GoalKind is a long-term objective. The agent does not fly without one.
type GoalKind string

const (
	GoalFarmCredits GoalKind = "farm_credits" // combat: find → approach → attack → retreat
	GoalMineAndSell GoalKind = "mine_and_sell" // find deposit → approach → mine → sell
	GoalTradeRun    GoalKind = "trade_run"    // move between stations (placeholder chain)
	GoalExplore     GoalKind = "explore"      // visit points / deposits
	GoalIdleDock    GoalKind = "idle_dock"    // stay near station
)

// StepKind is one concrete action in a chain.
type StepKind string

const (
	StepFindTarget StepKind = "find_target"
	StepApproach   StepKind = "approach"
	StepAttack     StepKind = "attack"
	StepMine       StepKind = "mine"
	StepSell       StepKind = "sell"
	StepRetreat    StepKind = "retreat"
	StepWait       StepKind = "wait"
	StepHold       StepKind = "hold"
)

// GoalRunner executes a long-term goal as an ordered chain of steps.
type GoalRunner struct {
	World   WorldView
	Store   *Store
	Log     *slog.Logger
	LowHull float64
}

type agentState struct {
	goal      GoalKind
	step      StepKind
	target    string
	targetPos [2]float64
	playerID  string
}

// RunGoal drives one agent for the lifetime of ctx according to the given goal.
func (r *GoalRunner) RunGoal(ctx context.Context, g *Gateway, captainID, playerID string, goal GoalKind) {
	if r == nil || g == nil || playerID == "" {
		return
	}
	if goal == "" {
		goal = GoalIdleDock
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
			if snap.Ship.HullMax > 0 && snap.Ship.Hull/snap.Ship.HullMax < threshold && st.step != StepRetreat && st.step != StepWait {
				st.step = StepRetreat
			}
			switch st.goal {
			case GoalFarmCredits:
				r.stepCombat(ctx, g, captainID, &st, snap)
			case GoalMineAndSell:
				r.stepMine(ctx, g, captainID, &st, snap)
			case GoalExplore:
				r.stepExplore(ctx, g, captainID, &st, snap)
			default: // trade_run and idle_dock: hold near station
				_ = IssueMove(ctx, g, "hold", 0, 0)
			}
		}
	}
}

func (r *GoalRunner) stepCombat(ctx context.Context, g *Gateway, captainID string, st *agentState, snap world.State) {
	switch st.step {
	case StepFindTarget:
		id, pos := nearestHostile(snap, snap.Ship.Pos)
		if id == "" {
			_ = IssueMove(ctx, g, "hold", 0, 0)
			return
		}
		st.target, st.targetPos = id, pos
		st.step = StepApproach
	case StepApproach:
		r.moveToward(ctx, g, st, snap, 40, StepAttack)
	case StepAttack:
		_ = IssueCombat(ctx, g, "atk", CombatIntent{TargetID: st.target, Attack: true})
		id, _ := nearestHostile(snap, snap.Ship.Pos)
		if id == "" || id != st.target {
			st.step = StepFindTarget
			_ = StopAttack(ctx, g, "done")
		}
	case StepRetreat:
		r.doRetreat(ctx, g, captainID, st, snap)
	case StepWait:
		r.waitRecover(ctx, g, captainID, st, snap)
	}
}

func (r *GoalRunner) stepMine(ctx context.Context, g *Gateway, captainID string, st *agentState, snap world.State) {
	switch st.step {
	case StepFindTarget:
		id, pos := nearestDeposit(snap, snap.Ship.Pos)
		if id == "" {
			_ = IssueMove(ctx, g, "hold", 0, 0)
			return
		}
		st.target, st.targetPos = id, pos
		st.step = StepApproach
	case StepApproach:
		r.moveToward(ctx, g, st, snap, 15, StepMine)
	case StepMine:
		// start mining; if hold looks full or deposit gone, go sell
		_ = IssueMine(ctx, g, "mine", st.target)
		if snap.Ship.CargoUsed > snap.Ship.Cargo*0.8 {
			_ = IssueMineStop(ctx, g, "mine-stop")
			st.step = StepSell
		}
	case StepSell:
		// move toward station if we know one, then sell
		_ = IssueSell(ctx, g, "sell")
		st.step = StepFindTarget
	case StepRetreat:
		r.doRetreat(ctx, g, captainID, st, snap)
	case StepWait:
		r.waitRecover(ctx, g, captainID, st, snap)
	}
}

func (r *GoalRunner) stepExplore(ctx context.Context, g *Gateway, captainID string, st *agentState, snap world.State) {
	switch st.step {
	case StepFindTarget:
		id, pos := nearestPoint(snap, snap.Ship.Pos)
		if id == "" {
			_ = IssueMove(ctx, g, "hold", 0, 0)
			return
		}
		st.target, st.targetPos = id, pos
		st.step = StepApproach
	case StepApproach:
		r.moveToward(ctx, g, st, snap, 20, StepFindTarget)
	case StepRetreat:
		r.doRetreat(ctx, g, captainID, st, snap)
	case StepWait:
		r.waitRecover(ctx, g, captainID, st, snap)
	default:
		st.step = StepFindTarget
	}
}

func (r *GoalRunner) moveToward(ctx context.Context, g *Gateway, st *agentState, snap world.State, arrive float64, next StepKind) {
	dx, dy := st.targetPos[0]-snap.Ship.Pos[0], st.targetPos[1]-snap.Ship.Pos[1]
	d := math.Hypot(dx, dy)
	if d < arrive {
		st.step = next
		return
	}
	_ = IssueMove(ctx, g, "approach", dx/d, dy/d)
}

func (r *GoalRunner) doRetreat(ctx context.Context, g *Gateway, captainID string, st *agentState, snap world.State) {
	_, threat := nearestHostile(snap, snap.Ship.Pos)
	_ = RetreatAway(ctx, g, "ret", snap.Ship.Pos, threat)
	_ = RecordFailure(ctx, r.Store, captainID, "low-hull", time.Now())
	st.step = StepWait
}

func (r *GoalRunner) waitRecover(ctx context.Context, g *Gateway, captainID string, st *agentState, snap world.State) {
	threshold := r.LowHull
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.3
	}
	if snap.Ship.HullMax > 0 && snap.Ship.Hull/snap.Ship.HullMax > threshold+0.15 {
		_ = IssueMove(ctx, g, "resume", 0, 0)
		st.step = StepFindTarget
	}
}

func nearestDeposit(st world.State, me [2]float64) (string, [2]float64) {
	best, bd := "", math.MaxFloat64
	var bp [2]float64
	for _, o := range st.Objects {
		if o.Kind != "deposit" || o.ID == "" {
			continue
		}
		d := math.Hypot(o.Pos[0]-me[0], o.Pos[1]-me[1])
		if d < bd {
			best, bd, bp = o.ID, d, o.Pos
		}
	}
	return best, bp
}

func nearestPoint(st world.State, me [2]float64) (string, [2]float64) {
	best, bd := "", math.MaxFloat64
	var bp [2]float64
	for _, o := range st.Objects {
		if (o.Kind != "anomaly" && o.Kind != "signal" && o.Kind != "wreck") || o.ID == "" || o.Dead {
			continue
		}
		d := math.Hypot(o.Pos[0]-me[0], o.Pos[1]-me[1])
		if d < bd {
			best, bd, bp = o.ID, d, o.Pos
		}
	}
	return best, bp
}
