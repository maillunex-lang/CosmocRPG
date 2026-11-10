package aiciv

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrUncertain = errors.New("ai_action_outcome_uncertain")
var ErrReplay = errors.New("ai_action_id_conflict")

// claimAction — уникальный action_id на капитана. Pending никогда не повторно
// исполняется: после аварии требуется отдельная сверка состояния, не blind retry.
func (s *Store) claimAction(ctx context.Context, agent, id string, request []byte) (json.RawMessage, bool, error) {
	if !validID(agent) || !validID(id) {
		return nil, false, ErrInput
	}
	hash := sha256.Sum256(request)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)
	var player string
	if err = tx.QueryRow(ctx, `SELECT player_id FROM ai_agents WHERE studio_captain_id=$1 AND player_id IS NOT NULL FOR UPDATE`, agent).Scan(&player); err != nil {
		return nil, false, ErrIdentity
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_action_receipts WHERE agent_id=$1 AND action_id=$2)`, agent, id).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM ai_action_receipts WHERE agent_id=$1`, agent).Scan(&count); err != nil {
			return nil, false, err
		}
		// Ограниченный canary ledger: старые ключи не удаляются ради обхода лимита.
		if count >= 4096 {
			return nil, false, ErrBudget
		}
		if _, err = tx.Exec(ctx, `INSERT INTO ai_action_receipts(agent_id,action_id,request_hash) VALUES($1,$2,$3)`, agent, id, hash[:]); err != nil {
			return nil, false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	var existing []byte
	var state string
	var result json.RawMessage
	err = tx.QueryRow(ctx, `SELECT request_hash,state,result FROM ai_action_receipts WHERE agent_id=$1 AND action_id=$2`, agent, id).Scan(&existing, &state, &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrIdentity
	}
	if err != nil {
		return nil, false, err
	}
	if string(existing) != string(hash[:]) {
		return nil, false, ErrReplay
	}
	if state != "completed" {
		return nil, false, ErrUncertain
	}
	return result, false, nil
}
func (s *Store) finishAction(ctx context.Context, agent, id string, result json.RawMessage) error {
	tag, err := s.pool.Exec(ctx, `UPDATE ai_action_receipts SET state='completed',result=$3 WHERE agent_id=$1 AND action_id=$2 AND state='pending'`, agent, id, result)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrUncertain
	}
	return nil
}
func (s *Store) uncertainAction(ctx context.Context, agent, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE ai_action_receipts SET state='uncertain' WHERE agent_id=$1 AND action_id=$2 AND state='pending'`, agent, id)
}
