package aiciv

import (
	"context"
	"encoding/json"
)

// CombatIntent describes a simple PvE farm action: target a hostile NPC and start attacking.
// The actual rules (range, energy, death, etc.) stay inside the world simulation.
type CombatIntent struct {
	TargetID string `json:"target_id"`
	Attack   bool   `json:"attack"`
}

// IssueCombat sends target + optional attack commands through the Gateway.
// Enemies already aggro any player in the world (including AI-flagged ones).
func IssueCombat(ctx context.Context, g *Gateway, id string, intent CombatIntent) error {
	if g == nil || intent.TargetID == "" {
		return ErrInput
	}
	targetPayload, _ := json.Marshal(map[string]string{"id": intent.TargetID})
	if _, err := g.Execute(ctx, id+"-target", "target", targetPayload); err != nil {
		return err
	}
	if intent.Attack {
		attackPayload, _ := json.Marshal(map[string]bool{"on": true})
		if _, err := g.Execute(ctx, id+"-attack", "attack", attackPayload); err != nil {
			return err
		}
	}
	return nil
}

// StopAttack turns off auto-attack.
func StopAttack(ctx context.Context, g *Gateway, id string) error {
	if g == nil {
		return ErrInput
	}
	payload, _ := json.Marshal(map[string]bool{"on": false})
	_, err := g.Execute(ctx, id+"-stop", "attack", payload)
	return err
}
