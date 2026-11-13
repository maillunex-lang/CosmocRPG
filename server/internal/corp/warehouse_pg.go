package corp

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PGStore) Warehouse(ctx context.Context, pid string, limit int) (Corporation, []WarehouseItem, []WarehouseAudit, error) {
	var c Corporation
	if err := s.pool.QueryRow(ctx, `SELECT c.id,c.name,c.tag,c.owner_id,c.treasury,c.created_at FROM corporations c JOIN corporation_members m ON m.corp_id=c.id WHERE m.player_id=$1`, pid).Scan(&c.ID, &c.Name, &c.Tag, &c.Owner, &c.Treasury, &c.Created); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, nil, nil, ErrNotMember
		}
		return Corporation{}, nil, nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT resource,amount FROM corporation_warehouse WHERE corp_id=$1 AND amount>0 ORDER BY resource`, c.ID)
	if err != nil {
		return Corporation{}, nil, nil, err
	}
	defer rows.Close()
	items := []WarehouseItem{}
	for rows.Next() {
		var x WarehouseItem
		if err := rows.Scan(&x.Resource, &x.Amount); err != nil {
			return Corporation{}, nil, nil, err
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		return Corporation{}, nil, nil, err
	}
	logs, err := s.warehouseJournal(ctx, c.ID, limit)
	return c, items, logs, err
}

func (s *PGStore) warehouseJournal(ctx context.Context, corpID string, limit int) ([]WarehouseAudit, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,corp_id,player_id,kind,resource,amount,created_at FROM corporation_warehouse_audit WHERE corp_id=$1 ORDER BY created_at DESC LIMIT $2`, corpID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WarehouseAudit{}
	for rows.Next() {
		var x WarehouseAudit
		if err := rows.Scan(&x.ID, &x.CorpID, &x.PlayerID, &x.Kind, &x.Resource, &x.Amount, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *PGStore) Deposit(ctx context.Context, pid, resource string, amount float64) (Corporation, error) {
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Corporation{}, err
	}
	defer tx.Rollback(ctx)
	var c Corporation
	if err := tx.QueryRow(ctx, `SELECT c.id,c.name,c.tag,c.owner_id,c.treasury,c.created_at FROM corporations c JOIN corporation_members m ON m.corp_id=c.id WHERE m.player_id=$1 FOR UPDATE`, pid).Scan(&c.ID, &c.Name, &c.Tag, &c.Owner, &c.Treasury, &c.Created); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, ErrNotMember
		}
		return Corporation{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO corporation_warehouse(corp_id,resource,amount) VALUES($1,$2,$3) ON CONFLICT(corp_id,resource) DO UPDATE SET amount=corporation_warehouse.amount+EXCLUDED.amount`, c.ID, resource, amount); err != nil {
		return Corporation{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO corporation_warehouse_audit(id,corp_id,player_id,kind,resource,amount,created_at) VALUES($1,$2,$3,'deposit',$4,$5,$6)`, warehouseAuditID(c.ID, pid, "deposit", now), c.ID, pid, resource, amount, now); err != nil {
		return Corporation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Corporation{}, err
	}
	return c, nil
}

func (s *PGStore) Withdraw(ctx context.Context, pid, resource string, amount float64) (Corporation, error) {
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Corporation{}, err
	}
	defer tx.Rollback(ctx)
	var c Corporation
	var role string
	if err := tx.QueryRow(ctx, `SELECT c.id,c.name,c.tag,c.owner_id,c.treasury,c.created_at,m.role FROM corporations c JOIN corporation_members m ON m.corp_id=c.id WHERE m.player_id=$1 FOR UPDATE`, pid).Scan(&c.ID, &c.Name, &c.Tag, &c.Owner, &c.Treasury, &c.Created, &role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, ErrNotMember
		}
		return Corporation{}, err
	}
	if role != "owner" {
		return Corporation{}, ErrWarehouseAccess
	}
	var left float64
	if err := tx.QueryRow(ctx, `UPDATE corporation_warehouse SET amount=amount-$3 WHERE corp_id=$1 AND resource=$2 AND amount >= $3 RETURNING amount`, c.ID, resource, amount).Scan(&left); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, ErrWarehouseEmpty
		}
		return Corporation{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO corporation_warehouse_audit(id,corp_id,player_id,kind,resource,amount,created_at) VALUES($1,$2,$3,'withdraw',$4,$5,$6)`, warehouseAuditID(c.ID, pid, "withdraw", now), c.ID, pid, resource, amount, now); err != nil {
		return Corporation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Corporation{}, err
	}
	_ = left
	return c, nil
}

func (s *PGStore) LogWarehouseAudit(ctx context.Context, corpID, pid, kind, resource string, amount float64) error {
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `INSERT INTO corporation_warehouse_audit(id,corp_id,player_id,kind,resource,amount,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, warehouseAuditID(corpID, pid, kind, now), corpID, pid, kind, resource, amount, now)
	return err
}

func (s *PGStore) ContributeResource(ctx context.Context, pid, resource string, amount float64) (Corporation, error) {
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Corporation{}, err
	}
	defer tx.Rollback(ctx)
	var c Corporation
	if err := tx.QueryRow(ctx, `SELECT c.id,c.name,c.tag,c.owner_id,c.treasury,c.created_at FROM corporations c JOIN corporation_members m ON m.corp_id=c.id WHERE m.player_id=$1 FOR UPDATE`, pid).Scan(&c.ID, &c.Name, &c.Tag, &c.Owner, &c.Treasury, &c.Created); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Corporation{}, ErrNotMember
		}
		return Corporation{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE corporation_warehouse SET amount=amount-$3 WHERE corp_id=$1 AND resource=$2 AND amount >= $3`, c.ID, resource, amount); err != nil {
		return Corporation{}, err
	}
	var left float64
	if err := tx.QueryRow(ctx, `SELECT amount FROM corporation_warehouse WHERE corp_id=$1 AND resource=$2`, c.ID, resource).Scan(&left); err != nil || left < -1e-6 {
		return Corporation{}, ErrWarehouseEmpty
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO corporation_warehouse_audit(id,corp_id,player_id,kind,resource,amount,created_at) VALUES($1,$2,$3,'task_contribution',$4,$5,$6)`, warehouseAuditID(c.ID, pid, "task_contribution", now), c.ID, pid, resource, amount, now); err != nil {
		return Corporation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Corporation{}, err
	}
	return c, nil
}
