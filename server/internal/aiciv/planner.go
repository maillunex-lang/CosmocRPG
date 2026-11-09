package aiciv

import (
	"context"
	"errors"
	"math"
	"sort"
)

// Facts — ограниченный снимок подтверждённых фактов, не всеведение планировщика.
type Facts uint64

type Step struct {
	ID                      string
	Kind                    Action
	Requires, Adds, Removes Facts
	Cost                    float64
}

type Limits struct{ MaxExpanded, MaxDepth int }

var ErrBudget = errors.New("planner_budget_exhausted")
var ErrNoPlan = errors.New("no_feasible_plan")
var ErrPlan = errors.New("invalid_action_definition")

// Plan выполняет bounded uniform-cost GOAP; равные стоимости разрешаются по ID.
// Эффекты — прогноз, а не фактическая награда. Исполнение требует ActionGateway.
func Plan(ctx context.Context, initial, goal Facts, definitions []Step, caps Capabilities, limits Limits) ([]Step, error) {
	if len(definitions) > 128 {
		return nil, ErrBudget
	}
	if limits.MaxExpanded < 1 || limits.MaxExpanded > 10000 || limits.MaxDepth < 1 || limits.MaxDepth > 64 {
		return nil, ErrBudget
	}
	steps := append([]Step(nil), definitions...)
	sort.Slice(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
	for i, s := range steps {
		if s.ID == "" || math.IsNaN(s.Cost) || math.IsInf(s.Cost, 0) || s.Cost <= 0 || s.Adds&s.Removes != 0 || (i > 0 && steps[i-1].ID == s.ID) {
			return nil, ErrPlan
		}
	}
	type node struct {
		facts Facts
		cost  float64
		path  []Step
		seq   int
	}
	queue := []node{{facts: initial}}
	type stateKey struct {
		facts Facts
		depth int
	}
	best := map[stateKey]float64{{initial, 0}: 0}
	expanded, seq := 0, 0
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sort.Slice(queue, func(i, j int) bool {
			if queue[i].cost == queue[j].cost {
				return queue[i].seq < queue[j].seq
			}
			return queue[i].cost < queue[j].cost
		})
		n := queue[0]
		queue = queue[1:]
		if n.cost > best[stateKey{n.facts, len(n.path)}] {
			continue
		}
		if n.facts&goal == goal {
			return n.path, nil
		}
		if expanded >= limits.MaxExpanded {
			return nil, ErrBudget
		}
		expanded++
		if len(n.path) >= limits.MaxDepth {
			continue
		}
		for _, s := range steps {
			if caps.Authorize(s.Kind) != nil || n.facts&s.Requires != s.Requires {
				continue
			}
			next := (n.facts &^ s.Removes) | s.Adds
			cost := n.cost + s.Cost
			if math.IsInf(cost, 0) {
				continue
			}
			key := stateKey{next, len(n.path) + 1}
			if old, ok := best[key]; ok && old <= cost {
				continue
			}
			if len(queue) >= limits.MaxExpanded {
				return nil, ErrBudget
			}
			best[key] = cost
			seq++
			path := append(append([]Step(nil), n.path...), s)
			queue = append(queue, node{next, cost, path, seq})
		}
	}
	return nil, ErrNoPlan
}

// Utility нормирует критерии [0,1]; некорректные входы отклоняются.
// Стоимости риска/времени не превращаются в положительные награды.
type Factors struct{ Need, Reward, Personality, Social, Progress, Risk, Cost, Time float64 }

func Utility(f Factors) (float64, error) {
	v := []float64{f.Need, f.Reward, f.Personality, f.Social, f.Progress, f.Risk, f.Cost, f.Time}
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > 1 {
			return 0, ErrPlan
		}
	}
	return .25*f.Need + .20*f.Reward + .15*f.Personality + .15*f.Social + .25*f.Progress - .4*f.Risk - .3*f.Cost - .3*f.Time, nil
}
