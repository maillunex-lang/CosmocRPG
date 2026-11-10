package aiciv

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func freshID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func playerPos(st world.State, name string) ([2]float64, bool) {
	for _, o := range st.Objects {
		if o.Kind == "player" && o.Name == name && !o.Dead {
			return o.Pos, true
		}
	}
	return [2]float64{}, false
}

// execPact flies toward the captain who was promised help and fights there.
// A troll who asked for help flies away. A noob stops short. A dull one wanders, then arrives.
func (r *GoalRunner) execPact(ctx context.Context, g *Gateway, ord Order, snap world.State) {
	me := snap.Ship.Pos
	pos, ok := playerPos(snap, "AI-"+ord.Leader)
	if !ok {
		_ = IssueMove(ctx, g, freshID("hold"), 0, 0)
		return
	}
	dx, dy := pos[0]-me[0], pos[1]-me[1]
	d := math.Hypot(dx, dy)
	if d < 1 {
		dx, dy, d = 1, 0, 1
	}
	switch ord.Style {
	case ArchTroll:
		_ = StopAttack(ctx, g, freshID("stop"))
		_ = IssueMove(ctx, g, freshID("away"), -dx/d, -dy/d)
	case ArchDull:
		if d > 80 {
			_ = IssueMove(ctx, g, freshID("side"), -dy/d, dx/d)
			return
		}
		_ = IssueMove(ctx, g, freshID("to"), dx/d, dy/d)
	case ArchNoob:
		if d > 110 {
			_ = IssueMove(ctx, g, freshID("to"), dx/d, dy/d)
			return
		}
		_ = IssueMove(ctx, g, freshID("stop"), 0, 0)
		if id, _ := nearestHostile(snap, me); id != "" {
			_ = IssueCombat(ctx, g, freshID("atk"), CombatIntent{TargetID: id, Attack: true})
		}
	default:
		if d > 45 {
			_ = IssueMove(ctx, g, freshID("to"), dx/d, dy/d)
			return
		}
		_ = IssueMove(ctx, g, freshID("stop"), 0, 0)
		if id, _ := nearestHostile(snap, me); id != "" {
			_ = IssueCombat(ctx, g, freshID("atk"), CombatIntent{TargetID: id, Attack: true})
		}
	}
}
