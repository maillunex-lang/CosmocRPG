package corp

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — корпорации в PostgreSQL (миграция 0019).
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) CorporationByPlayer(ctx context.Context, pid string) (Corporation, bool, error) {
	var c Corporation
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.name, c.tag, c.owner_id, c.treasury, c.created_at
		FROM corporations c JOIN corporation_members m ON m.corp_id = c.id
		WHERE m.player_id = $1`, pid).Scan(&c.ID, &c.Name, &c.Tag, &c.Owner, &c.Treasury, &c.Created)
	if errors.Is(err, pgx.ErrNoRows) {
		return Corporation{}, false, nil
	}
	if err != nil {
		return Corporation{}, false, err
	}
	return c, true, nil
}

func (s *PGStore) Exists(ctx context.Context, name, tag string) (bool, error) {
	var one int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM corporations WHERE lower(name) = lower($1) OR tag = $2 LIMIT 1`, name, tag).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *PGStore) Create(ctx context.Context, c Corporation, ownerName string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO corporations (id, name, tag, owner_id, treasury, created_at) VALUES ($1,$2,$3,$4,0,$5)`,
		c.ID, c.Name, c.Tag, c.Owner, c.Created); err != nil {
		return ErrTaken
	}
	if _, err := tx.Exec(ctx, `INSERT INTO corporation_members (corp_id, player_id, role) VALUES ($1,$2,'owner')`, c.ID, c.Owner); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) Join(ctx context.Context, _ string, pid, nameOrTag string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO corporation_members (corp_id, player_id, role)
		SELECT id, $1, 'member' FROM corporations WHERE lower(name) = lower($2) OR tag = upper($3)
		ON CONFLICT DO NOTHING`, pid, nameOrTag, nameOrTag)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PGStore) Leave(ctx context.Context, pid string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM corporation_members WHERE player_id = $1`, pid)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PGStore) Members(ctx context.Context, corpID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.player_id, p.name, m.role FROM corporation_members m JOIN players p ON p.public_id = m.player_id
		WHERE m.corp_id = $1 ORDER BY (m.role = 'owner') DESC, p.name`, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

var _ = time.Now // резерв
