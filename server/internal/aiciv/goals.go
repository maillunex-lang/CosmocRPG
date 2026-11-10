package aiciv

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/corp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// GoalKind is a long-term objective. The agent does not fly without one.
type GoalKind string

const (
	GoalFarmCredits GoalKind = "farm_credits"  // combat: find → approach → attack → retreat
	GoalMineAndSell GoalKind = "mine_and_sell" // find deposit → approach → mine → sell
	GoalTradeRun    GoalKind = "trade_run"     // move between stations (placeholder chain)
	GoalExplore     GoalKind = "explore"       // visit points / deposits
	GoalIdleDock    GoalKind = "idle_dock"     // stay near station
	GoalJump        GoalKind = "jump"          // fly to a gate and change system
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

// Galaxy переносит капитана между системами. nil — капитан остаётся, где вошёл.
type Galaxy interface {
	Place(pid, system string, pos world.Vec) error
	SetPos(pid string, pos world.Vec) bool
	SystemID(pid string) string
	Gates(system string) []world.Gate
	Jump(pid, to string) (world.Record, world.Vec, error)
}

// QuestMind — текущий контракт игрока. nil — только профессия.
type QuestMind interface {
	Focus(pid string) (kind, system, target string, done int, ok bool)
	Note(ctx context.Context, pid, kind, ref string)
}

// GoalRunner executes a long-term goal as an ordered chain of steps.
// Chat is owned by Chorus so several agents can answer each other.
type GoalRunner struct {
	World   WorldView
	Store   *Store
	Board   *Board
	Live    *Live
	Log     *slog.Logger
	LowHull float64
	Name    string
	Clans   *corp.Service
	Galaxy  Galaxy
	Quests  QuestMind
}

func (r *GoalRunner) hullLimit() float64 {
	if r != nil && r.Live != nil {
		p := r.Live.Get().RetreatPct
		if p >= 5 && p <= 90 {
			return float64(p) / 100
		}
	}
	if r != nil && r.LowHull > 0 && r.LowHull < 1 {
		return r.LowHull
	}
	return 0.3
}

func (r *GoalRunner) helpOn() bool {
	if r == nil || r.Live == nil {
		return true
	}
	return r.Live.Get().Help
}

type agentState struct {
	goal      GoalKind
	step      StepKind
	target    string
	targetPos [2]float64
	playerID  string
	jumpTo    string
	settled   bool
	clanAt    time.Time
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
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if r.Clans != nil && time.Since(st.clanAt) >= 45*time.Second {
				st.clanAt = time.Now()
				r.Clans.SettleAI(ctx, playerID, r.Name)
			}
			if r.World == nil {
				continue
			}
			snap, ok := r.World.Snapshot(playerID)
			if !ok {
				continue
			}
			r.followLife(captainID, &st, snap)
			if r.helpOn() && r.Board != nil {
				if ord, ok := r.Board.OrderFor(captainID, time.Now()); ok {
					r.execPact(ctx, g, ord, snap)
					continue
				}
			}
			threshold := r.hullLimit()
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
			case GoalJump:
				r.stepJump(ctx, g, &st, snap)
			case GoalTradeRun:
				r.stepTrade(ctx, g, &st, snap)
			default:
				r.stepTrade(ctx, g, &st, snap)
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
		_ = IssueMine(ctx, g, "mine", st.target)
		if snap.Ship.CargoUsed > snap.Ship.Cargo*0.8 {
			_ = IssueMineStop(ctx, g, "mine-stop")
			st.step = StepSell
		}
	case StepSell:
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
	threshold := r.hullLimit()
	if snap.Ship.HullMax > 0 && snap.Ship.Hull/snap.Ship.HullMax > threshold+0.15 {
		_ = IssueMove(ctx, g, "resume", 0, 0)
		st.step = StepFindTarget
	}
}

func (r *GoalRunner) followLife(captainID string, st *agentState, snap world.State) {
	kind, qsys, target, done, have := "", "", "", 0, false
	if r.Quests != nil {
		kind, qsys, target, done, have = r.Quests.Focus(st.playerID)
	}
	current := ""
	if r.Galaxy != nil {
		current = r.Galaxy.SystemID(st.playerID)
	}
	plan := LifeOf(captainID, done, have, kind, qsys, target, current, st.goal)
	if plan.Goal != "" && plan.Goal != st.goal {
		st.goal = plan.Goal
		st.step = StepFindTarget
	}
	st.jumpTo = plan.JumpTo
	if st.settled || r.Galaxy == nil || r.World == nil {
		return
	}
	st.settled = true
	if plan.JumpTo == "" && plan.System != "" && plan.System != current {
		if err := r.Galaxy.Place(st.playerID, plan.System, world.Vec{}); err != nil {
			if r.Log != nil {
				r.Log.Error("AI place", "captain", captainID, "system", plan.System, "err", err)
			}
			return
		}
		if next, ok := r.World.Snapshot(st.playerID); ok {
			snap = next
		}
	}
	r.scatterIfStacked(st.playerID, captainID, snap)
}

func (r *GoalRunner) scatterIfStacked(playerID, captainID string, snap world.State) {
	station, ok := stationPos(snap)
	if !ok || r.Galaxy == nil {
		return
	}
	me := snap.Ship.Pos
	fromStation := math.Hypot(me[0]-station[0], me[1]-station[1])
	fromOrigin := math.Hypot(me[0], me[1])
	if fromStation > 80 && fromOrigin > 40 {
		return
	}
	r.Galaxy.SetPos(playerID, Scatter(captainID, station))
}

func stationPos(st world.State) ([2]float64, bool) {
	for _, o := range st.Objects {
		if o.Kind == "station" && o.ID != "" {
			return o.Pos, true
		}
	}
	return [2]float64{}, false
}

func (r *GoalRunner) stepTrade(ctx context.Context, g *Gateway, st *agentState, snap world.State) {
	station, ok := stationPos(snap)
	if !ok {
		_ = IssueMove(ctx, g, "hold", 0, 0)
		return
	}
	dx, dy := station[0]-snap.Ship.Pos[0], station[1]-snap.Ship.Pos[1]
	d := math.Hypot(dx, dy)
	if d < 50 {
		_ = IssueSell(ctx, g, "sell")
		away := Scatter(st.playerID+"out", station)
		_ = IssueMove(ctx, g, "leave", away.X-snap.Ship.Pos[0], away.Y-snap.Ship.Pos[1])
		return
	}
	_ = IssueMove(ctx, g, "station", dx/d, dy/d)
}

func (r *GoalRunner) stepJump(ctx context.Context, g *Gateway, st *agentState, snap world.State) {
	if r.Galaxy == nil || st.jumpTo == "" {
		r.stepTrade(ctx, g, st, snap)
		return
	}
	sys := r.Galaxy.SystemID(st.playerID)
	var gate *world.Gate
	for _, item := range r.Galaxy.Gates(sys) {
		if item.To == st.jumpTo {
			item := item
			gate = &item
			break
		}
	}
	if gate == nil {
		_ = IssueMove(ctx, g, "hold", 0, 0)
		return
	}
	dx, dy := gate.Pos[0]-snap.Ship.Pos[0], gate.Pos[1]-snap.Ship.Pos[1]
	d := math.Hypot(dx, dy)
	if d > 55 {
		_ = IssueMove(ctx, g, "gate", dx/d, dy/d)
		return
	}
	if _, _, err := r.Galaxy.Jump(st.playerID, gate.To); err != nil {
		if r.Log != nil {
			r.Log.Error("AI jump", "player", st.playerID, "to", gate.To, "err", err)
		}
		return
	}
	if r.Quests != nil {
		r.Quests.Note(ctx, st.playerID, "jump", gate.To)
	}
	st.step = StepFindTarget
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
