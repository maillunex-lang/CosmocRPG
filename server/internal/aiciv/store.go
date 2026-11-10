package aiciv

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
	"unicode/utf8"
)

type Identity struct {
	CaptainID, PlayerID, Mode string
	Revision                  int64
}
type MemoryEvent struct {
	ID, Type, Subject string
	At, Expires       time.Time
}

var ErrInput = errors.New("invalid_ai_state")
var ErrConflict = errors.New("ai_revision_conflict")

type Store struct{ pool *pgxpool.Pool }

func NewStore(p *pgxpool.Pool) *Store { return &Store{p} }
func validID(s string) bool           { return len(s) > 0 && len(s) <= 128 && utf8.ValidString(s) }

// Ensure сохраняет Studio ID даже после смены активного пакета. Не выдаёт имущество.
func (s *Store) Ensure(ctx context.Context, id string) (Identity, error) {
	if !validID(id) {
		return Identity{}, ErrInput
	}
	var a Identity
	err := s.pool.QueryRow(ctx, `INSERT INTO ai_agents(studio_captain_id) VALUES($1)
 ON CONFLICT(studio_captain_id) DO UPDATE SET studio_captain_id=EXCLUDED.studio_captain_id
 RETURNING studio_captain_id,COALESCE(player_id,''),mode,revision`, id).Scan(&a.CaptainID, &a.PlayerID, &a.Mode, &a.Revision)
	return a, err
}

// Bind оставлен для совместимости, но произвольная привязка запрещена.
// Создание и привязка выполняются атомарно только через Bootstrapper.
func (s *Store) Bind(ctx context.Context, id, player string, revision int64) error {
	// Произвольная привязка чужой players-записи запрещена. Только атомарный Bootstrapper.
	return ErrPolicy
}

// Observe принимает только проверенные сервером события. Здесь нет произвольного
// текста/чатов/скрытых координат. Дедуп и лимит памяти атомарны на капитана.
func (s *Store) Observe(ctx context.Context, id string, e MemoryEvent) error {
	if !validID(id) || !validID(e.ID) || len(e.Subject) > 128 || !utf8.ValidString(e.Subject) ||
		(e.Type != "observed" && e.Type != "confirmed" && e.Type != "failure") || e.At.IsZero() || !e.Expires.After(e.At) || e.Expires.Sub(e.At) > 30*24*time.Hour {
		return ErrInput
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var locked string
	if err = tx.QueryRow(ctx, `SELECT studio_captain_id FROM ai_agents WHERE studio_captain_id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO ai_memory_events(agent_id,event_id,event_type,subject_ref,occurred_at,expires_at)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(agent_id,event_id) DO NOTHING`, id, e.ID, e.Type, e.Subject, e.At, e.Expires); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM ai_memory_events WHERE agent_id=$1 AND event_id IN
 (SELECT event_id FROM ai_memory_events WHERE agent_id=$1 ORDER BY occurred_at DESC,event_id DESC OFFSET 128)`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Recall не является ACL публичного чата: только внутренняя память конкретного AI.
func (s *Store) Recall(ctx context.Context, id string, now time.Time) ([]MemoryEvent, error) {
	if !validID(id) || now.IsZero() {
		return nil, ErrInput
	}
	rows, err := s.pool.Query(ctx, `SELECT event_id,event_type,subject_ref,occurred_at,expires_at FROM ai_memory_events
 WHERE agent_id=$1 AND occurred_at<=$2 AND expires_at>$2 ORDER BY occurred_at DESC,event_id DESC LIMIT 128`, id, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemoryEvent{}
	for rows.Next() {
		var e MemoryEvent
		if err = rows.Scan(&e.ID, &e.Type, &e.Subject, &e.At, &e.Expires); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) Prune(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		return ErrInput
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM ai_memory_events WHERE expires_at<=$1`, now)
	return err
}
