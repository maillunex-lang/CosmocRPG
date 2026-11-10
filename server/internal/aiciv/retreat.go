package aiciv

import (
	"context"
	"encoding/json"
	"math"
)

// IssueMove sends a move command (direction vector, will be limited to length 1 by the world).
func IssueMove(ctx context.Context, g *Gateway, id string, x, y float64) error {
	if g == nil {
		return ErrInput
	}
	payload, err := json.Marshal(map[string]float64{"x": x, "y": y})
	if err != nil {
		return err
	}
	_, err = g.Execute(ctx, id+"-move", "move", payload)
	return err
}

// RetreatAway stops attacking and moves in the opposite direction from the threat.
// threatPos is the enemy position; me is the agent position. Both are [x,y].
func RetreatAway(ctx context.Context, g *Gateway, id string, me, threat [2]float64) error {
	if err := StopAttack(ctx, g, id); err != nil {
		return err
	}
	dx, dy := me[0]-threat[0], me[1]-threat[1]
	l := math.Hypot(dx, dy)
	if l < 0.1 {
		// already on top; pick an arbitrary safe direction
		dx, dy = 1, 0
		l = 1
	}
	return IssueMove(ctx, g, id, dx/l, dy/l)
}
