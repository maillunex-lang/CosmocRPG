package aiciv

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestPlannerRealCapabilitiesAndCosts(t *testing.T) {
	steps := []Step{{ID: "travel", Kind: Travel, Adds: 1, Cost: 1}, {ID: "mine", Kind: Mine, Requires: 1, Adds: 2, Cost: 2}, {ID: "sell", Kind: Sell, Requires: 3, Adds: 4, Removes: 2, Cost: 1}, {ID: "unavailable-manufacture", Kind: Manufacture, Adds: 4, Cost: .1}}
	caps := Capabilities{Travel: true, Mine: true, Sell: true}
	p, err := Plan(context.Background(), 0, 4, steps, caps, Limits{100, 8})
	if err != nil || len(p) != 3 || p[0].ID != "travel" || p[2].ID != "sell" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err = Plan(context.Background(), 0, 4, steps, nil, Limits{100, 8}); !errors.Is(err, ErrNoPlan) {
		t.Fatal(err)
	}
	if _, err = Plan(context.Background(), 0, 4, steps, caps, Limits{1, 8}); !errors.Is(err, ErrBudget) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Plan(ctx, 0, 4, steps, caps, Limits{100, 8}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestPolicyCannotEnableForbiddenActions(t *testing.T) {
	for _, a := range []Action{Enterprise, CorpLeadership, PermanentDeath, ExternalLLM} {
		if !errors.Is((Capabilities{a: true}).Authorize(a), ErrPolicy) {
			t.Fatal(a)
		}
	}
	if !errors.Is((Capabilities{Action("invented"): true}).Authorize("invented"), ErrUnavailable) {
		t.Fatal("unknown action accepted")
	}
}
func TestPlannerRejectsInvalidDefinitions(t *testing.T) {
	for _, cost := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := Plan(context.Background(), 0, 1, []Step{{ID: "bad", Kind: Travel, Adds: 1, Cost: cost}}, Capabilities{Travel: true}, Limits{10, 3}); !errors.Is(err, ErrPlan) {
			t.Fatal(cost, err)
		}
	}
	if _, err := Utility(Factors{Risk: math.NaN()}); !errors.Is(err, ErrPlan) {
		t.Fatal(err)
	}
	good, _ := Utility(Factors{Need: 1})
	bad, _ := Utility(Factors{Need: 1, Risk: 1})
	if bad >= good {
		t.Fatal("risk increases utility")
	}
}

func TestDepthBudgetDoesNotDiscardShorterPath(t *testing.T) {
	// Дешёвый путь 0→1→3 слишком длинный для завершения. Прямой дорогой
	// переход к 3 нужен, чтобы достичь цели 7 за два шага.
	steps := []Step{{"a", Travel, 0, 1, 0, 1}, {"b", Travel, 1, 2, 0, 1}, {"direct", Travel, 0, 3, 0, 5}, {"finish", Travel, 3, 4, 0, 1}}
	p, err := Plan(context.Background(), 0, 7, steps, Capabilities{Travel: true}, Limits{100, 2})
	if err != nil || len(p) != 2 || p[0].ID != "direct" {
		t.Fatal(p, err)
	}
}
