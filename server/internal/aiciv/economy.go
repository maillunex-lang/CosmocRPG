package aiciv

import (
	"context"
	"encoding/json"
)

// IssueMine starts mining a deposit.
func IssueMine(ctx context.Context, g *Gateway, id, depositID string) error {
	if g == nil || depositID == "" {
		return ErrInput
	}
	payload, _ := json.Marshal(map[string]string{"id": depositID})
	_, err := g.Execute(ctx, id, "mine", payload)
	return err
}

// IssueMineStop stops mining.
func IssueMineStop(ctx context.Context, g *Gateway, id string) error {
	if g == nil {
		return ErrInput
	}
	_, err := g.Execute(ctx, id, "mine_stop", nil)
	return err
}

// IssueSell sells cargo at station. May fail if not at station; caller should approach first.
func IssueSell(ctx context.Context, g *Gateway, id string) error {
	if g == nil {
		return ErrInput
	}
	_, err := g.Execute(ctx, id, "sell", nil)
	return err
}
