package corp

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PGStore) Donate(ctx context.Context, pid string, amount int64) (Corporation, AuditEntry, error) {
	if amount <= 0 || amount > 1_000_000_000 {
		return Corporation{}, AuditEntry{}, ErrBadTreasuryAmount
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Corporation{}, AuditEntry{}, err
	}
	defer tx.Rollback(ctx)
	var corpID string
	if err := tx.QueryRow(ctx, `SELECT corp_id FROM corporation_members WHERE player_id = $1 FOR UPDATE`, pid).Scan(&corpID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, AuditEntry{}, ErrNotMember
		}
		return Corporation{}, AuditEntry{}, err
	}
	var credits int64
	if err := tx.QueryRow(ctx, `UPDATE players SET credits = credits - $2 WHERE public_id = $1 AND credits >= $2 RETURNING credits`, pid, amount).Scan(&credits); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, AuditEntry{}, ErrNotEnoughCredits
		}
		return Corporation{}, AuditEntry{}, err
	}
	now := time.Now().UTC()
	var c Corporation
	if err := tx.QueryRow(ctx, `UPDATE corporations SET treasury = treasury + $2 WHERE id = $1 RETURNING id, name, tag, owner_id, treasury, created_at`, corpID, amount).Scan(&c.ID, &c.Name, &c.Tag, &c.Owner, &c.Treasury, &c.Created); err != nil {
		return Corporation{}, AuditEntry{}, err
	}
	a := AuditEntry{ID: auditID(c.ID, pid, now), CorpID: c.ID, PlayerID: pid, Kind: "donation", Amount: amount, Balance: c.Treasury, CreatedAt: now}
	if _, err := tx.Exec(ctx, `INSERT INTO corporation_treasury_audit (id, corp_id, player_id, kind, amount, balance, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, a.ID, a.CorpID, a.PlayerID, a.Kind, a.Amount, a.Balance, a.CreatedAt); err != nil {
		return Corporation{}, AuditEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Corporation{}, AuditEntry{}, err
	}
	_ = credits
	return c, a, nil
}

func (s *PGStore) Journal(ctx context.Context, corpID string, limit int) ([]AuditEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, corp_id, player_id, kind, amount, balance, created_at FROM corporation_treasury_audit WHERE corp_id = $1 ORDER BY created_at DESC LIMIT $2`, corpID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(&a.ID, &a.CorpID, &a.PlayerID, &a.Kind, &a.Amount, &a.Balance, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
