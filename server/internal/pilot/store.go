package pilot

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func decodeAttrs(raw []byte) map[string]int {
	out := map[string]int{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// Grant: запись в журнал и увеличение опыта в одной транзакции; повтор ключа ничего не меняет.
func (s *PGStore) Grant(ctx context.Context, pid, key, source string, amount int64) (bool, int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	granted := false
	if amount > 0 {
		g, err := tx.Exec(ctx, `
			INSERT INTO xp_grants (player_id, grant_key, source, amount) VALUES ($1, $2, $3, $4)
			ON CONFLICT (player_id, grant_key) DO NOTHING`, pid, key, source, amount)
		if err != nil {
			return false, 0, err
		}
		if g.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `UPDATE players SET pilot_xp = pilot_xp + $2 WHERE public_id = $1`, pid, amount); err != nil {
				return false, 0, err
			}
			granted = true
		}
	}
	var xp int64
	if err := tx.QueryRow(ctx, `SELECT pilot_xp FROM players WHERE public_id = $1`, pid).Scan(&xp); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, 0, ErrNoPlayer
		}
		return false, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return granted, xp, nil
}

// Load читает опыт и очки.
func (s *PGStore) Load(ctx context.Context, pid string) (int64, map[string]int, error) {
	var xp int64
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT pilot_xp, pilot_attrs FROM players WHERE public_id = $1`, pid).Scan(&xp, &raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, ErrNoPlayer
		}
		return 0, nil, err
	}
	return xp, decodeAttrs(raw), nil
}

// Update меняет очки под блокировкой строки игрока.
func (s *PGStore) Update(ctx context.Context, pid string, fn func(int64, map[string]int) (map[string]int, error)) (int64, map[string]int, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback(ctx)
	var xp int64
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT pilot_xp, pilot_attrs FROM players WHERE public_id = $1 FOR UPDATE`, pid).Scan(&xp, &raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, ErrNoPlayer
		}
		return 0, nil, err
	}
	next, err := fn(xp, decodeAttrs(raw))
	if err != nil {
		return 0, nil, err
	}
	enc, err := json.Marshal(next)
	if err != nil {
		return 0, nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE players SET pilot_attrs = $2::jsonb WHERE public_id = $1`, pid, string(enc)); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return xp, next, nil
}

// MemStore — хранилище в памяти (тесты).
type MemStore struct {
	mu    sync.Mutex
	xp    map[string]int64
	attrs map[string]map[string]int
	keys  map[string]bool
}

// NewMemStore создаёт пустое хранилище; игроки создаются при первом обращении.
func NewMemStore() *MemStore {
	return &MemStore{xp: map[string]int64{}, attrs: map[string]map[string]int{}, keys: map[string]bool{}}
}

func (m *MemStore) Grant(_ context.Context, pid, key, _ string, amount int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := pid + "|" + key
	if amount > 0 && !m.keys[k] {
		m.keys[k] = true
		m.xp[pid] += amount
		return true, m.xp[pid], nil
	}
	return false, m.xp[pid], nil
}

func (m *MemStore) Load(_ context.Context, pid string) (int64, map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := map[string]int{}
	for k, v := range m.attrs[pid] {
		cp[k] = v
	}
	return m.xp[pid], cp, nil
}

func (m *MemStore) Update(_ context.Context, pid string, fn func(int64, map[string]int) (map[string]int, error)) (int64, map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := map[string]int{}
	for k, v := range m.attrs[pid] {
		cur[k] = v
	}
	next, err := fn(m.xp[pid], cur)
	if err != nil {
		return 0, nil, err
	}
	m.attrs[pid] = next
	return m.xp[pid], next, nil
}
