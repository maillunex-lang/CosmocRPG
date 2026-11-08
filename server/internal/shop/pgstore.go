package shop

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func load(ctx context.Context, q queryer, pid string, now time.Time) (State, error) {
	var level int
	var until *time.Time
	var eq []byte
	if err := q.QueryRow(ctx, `SELECT vip_level, vip_until, equipped FROM players WHERE public_id = $1`, pid).Scan(&level, &until, &eq); err != nil {
		return State{}, err
	}
	st := State{Owned: []string{}, Equipped: map[string]string{}}
	_ = json.Unmarshal(eq, &st.Equipped)
	if st.Equipped == nil {
		st.Equipped = map[string]string{}
	}
	if level > 0 && until != nil && until.After(now) {
		st.VipLevel, st.VipUntil = level, until
	}
	rows, err := q.Query(ctx, `SELECT item_id FROM player_items WHERE player_id = $1 ORDER BY item_id`, pid)
	if err != nil {
		return State{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return State{}, err
		}
		st.Owned = append(st.Owned, id)
	}
	return st, rows.Err()
}

func (s *PGStore) Load(ctx context.Context, pid string, now time.Time) (State, error) {
	return load(ctx, s.pool, pid, now)
}

func (s *PGStore) Buy(ctx context.Context, pid string, it Item, req string, paid int, now time.Time) (State, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return State{}, false, err
	}
	defer tx.Rollback(ctx)
	// блокируем строку игрока: параллельные покупки одного игрока выполняются по очереди
	var curLevel int
	var curUntil *time.Time
	if err := tx.QueryRow(ctx, `SELECT vip_level, vip_until FROM players WHERE public_id = $1 FOR UPDATE`, pid).Scan(&curLevel, &curUntil); err != nil {
		return State{}, false, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO shop_purchases (player_id, item_id, request_id, price, paid) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, request_id) DO NOTHING`, pid, it.ID, req, it.Price, paid)
	if err != nil {
		return State{}, false, err
	}
	if tag.RowsAffected() == 0 { // повтор запроса: ничего не меняем
		st, err := load(ctx, tx, pid, now)
		return st, false, err
	}
	if it.Kind == "vip" {
		active := curLevel > 0 && curUntil != nil && curUntil.After(now)
		base, level := now, it.Level
		if active {
			base, level = *curUntil, max(it.Level, curLevel)
		}
		until := base.Add(time.Duration(it.Days) * 24 * time.Hour)
		if _, err := tx.Exec(ctx, `UPDATE players SET vip_level = $2, vip_until = $3 WHERE public_id = $1`, pid, level, until); err != nil {
			return State{}, false, err
		}
	} else {
		t, err := tx.Exec(ctx, `INSERT INTO player_items (player_id, item_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, pid, it.ID)
		if err != nil {
			return State{}, false, err
		}
		if t.RowsAffected() == 0 {
			return State{}, false, ErrOwned // откат: запись о покупке не сохраняется
		}
	}
	st, err := load(ctx, tx, pid, now)
	if err != nil {
		return State{}, false, err
	}
	return st, true, tx.Commit(ctx)
}

func (s *PGStore) Equip(ctx context.Context, pid, slot, itemID string, now time.Time) (State, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback(ctx)
	if itemID == "" {
		_, err = tx.Exec(ctx, `UPDATE players SET equipped = equipped - $2::text WHERE public_id = $1`, pid, slot)
	} else {
		var one int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM player_items WHERE player_id = $1 AND item_id = $2`, pid, itemID).Scan(&one); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return State{}, ErrNotOwned
			}
			return State{}, err
		}
		_, err = tx.Exec(ctx, `UPDATE players SET equipped = equipped || jsonb_build_object($2::text, $3::text) WHERE public_id = $1`, pid, slot, itemID)
	}
	if err != nil {
		return State{}, err
	}
	st, err := load(ctx, tx, pid, now)
	if err != nil {
		return State{}, err
	}
	return st, tx.Commit(ctx)
}
