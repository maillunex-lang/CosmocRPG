package quest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Sync(ctx context.Context, defs []Def) error {
	for _, d := range defs {
		body, _ := json.Marshal(map[string]any{
			"text": d.Text, "kind": d.Kind, "target": d.Target, "radius": d.Radius, "goal": d.Goal,
			"reward": map[string]any{"credits": d.Reward.Credits},
		})
		// версия неизменяема: повторная запись той же версии ничего не меняет
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO quest_definitions (id, version, title, body) VALUES ($1, $2, $3, $4)
			ON CONFLICT (id, version) DO NOTHING`, d.ID, d.Version, d.Title, body); err != nil {
			return fmt.Errorf("определение %s v%d: %w", d.ID, d.Version, err)
		}
	}
	return nil
}

func (s *PGStore) Load(ctx context.Context, pid string) (PlayerData, error) {
	var data PlayerData
	if err := s.pool.QueryRow(ctx, `SELECT credits FROM players WHERE public_id = $1`, pid).Scan(&data.Credits); err != nil {
		return data, fmt.Errorf("кредиты игрока: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT quest_id, quest_version, state, progress, goal FROM player_quest_progress WHERE player_id = $1 ORDER BY started_at`, pid)
	if err != nil {
		return data, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Progress
		if err := rows.Scan(&p.QuestID, &p.Version, &p.State, &p.Progress, &p.Goal); err != nil {
			return data, err
		}
		data.Quests = append(data.Quests, p)
	}
	return data, rows.Err()
}

func (s *PGStore) Assign(ctx context.Context, pid string, d Def) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO player_quest_progress (player_id, quest_id, quest_version, state, progress, goal)
		VALUES ($1, $2, $3, 'active', 0, $4) ON CONFLICT (player_id, quest_id) DO NOTHING`, pid, d.ID, d.Version, d.Goal)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PGStore) Complete(ctx context.Context, pid string, d Def) (bool, int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	// переход active -> completed возможен ровно один раз: параллельный вызов получит 0 строк
	tag, err := tx.Exec(ctx, `
		UPDATE player_quest_progress SET state = 'completed', progress = goal, completed_at = now()
		WHERE player_id = $1 AND quest_id = $2 AND quest_version = $3 AND state = 'active'`, pid, d.ID, d.Version)
	if err != nil {
		return false, 0, err
	}
	granted := false
	if tag.RowsAffected() == 1 && d.Reward.Credits > 0 {
		key := fmt.Sprintf("quest:%s:%d", d.ID, d.Version)
		g, err := tx.Exec(ctx, `
			INSERT INTO reward_grants (player_id, grant_key, kind, amount) VALUES ($1, $2, 'credits', $3)
			ON CONFLICT (player_id, grant_key) DO NOTHING`, pid, key, d.Reward.Credits)
		if err != nil {
			return false, 0, err
		}
		if g.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `UPDATE players SET credits = credits + $2 WHERE public_id = $1`, pid, d.Reward.Credits); err != nil {
				return false, 0, err
			}
			granted = true
		}
	}
	var credits int64
	if err := tx.QueryRow(ctx, `SELECT credits FROM players WHERE public_id = $1`, pid).Scan(&credits); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return granted, credits, nil
}

// GrantCredits начисляет кредиты один раз на ключ (UNIQUE player_id + grant_key): повтор ничего не меняет.
func (s *PGStore) GrantCredits(ctx context.Context, pid, key string, amount int64) (bool, int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	granted := false
	if amount > 0 {
		g, err := tx.Exec(ctx, `
			INSERT INTO reward_grants (player_id, grant_key, kind, amount) VALUES ($1, $2, 'credits', $3)
			ON CONFLICT (player_id, grant_key) DO NOTHING`, pid, key, amount)
		if err != nil {
			return false, 0, err
		}
		if g.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `UPDATE players SET credits = credits + $2 WHERE public_id = $1`, pid, amount); err != nil {
				return false, 0, err
			}
			granted = true
		}
	}
	var credits int64
	if err := tx.QueryRow(ctx, `SELECT credits FROM players WHERE public_id = $1`, pid).Scan(&credits); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return granted, credits, nil
}

func (m *MemStore) GrantCredits(_ context.Context, pid, key string, amount int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := pid + "|" + key
	if amount <= 0 || m.grants[k] {
		return false, m.credits[pid], nil
	}
	m.grants[k] = true
	m.credits[pid] += amount
	return true, m.credits[pid], nil
}

// MemStore — хранилище в памяти для тестов; повторяет семантику PGStore.
type MemStore struct {
	mu      sync.Mutex
	credits map[string]int64
	quests  map[string]map[string]*Progress
	grants  map[string]bool
	Defs    []Def
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore {
	return &MemStore{credits: map[string]int64{}, quests: map[string]map[string]*Progress{}, grants: map[string]bool{}}
}

func (m *MemStore) Sync(_ context.Context, defs []Def) error { m.Defs = defs; return nil }

func (m *MemStore) Load(_ context.Context, pid string) (PlayerData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := PlayerData{Credits: m.credits[pid]}
	for _, p := range m.quests[pid] {
		d.Quests = append(d.Quests, *p)
	}
	return d, nil
}

func (m *MemStore) Assign(_ context.Context, pid string, d Def) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.quests[pid] == nil {
		m.quests[pid] = map[string]*Progress{}
	}
	if _, ok := m.quests[pid][d.ID]; ok {
		return false, nil
	}
	m.quests[pid][d.ID] = &Progress{QuestID: d.ID, Version: d.Version, State: "active", Goal: d.Goal}
	return true, nil
}

func (m *MemStore) Complete(_ context.Context, pid string, d Def) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.quests[pid][d.ID]
	if p == nil || p.State != "active" || p.Version != d.Version {
		return false, m.credits[pid], nil
	}
	p.State, p.Progress = "completed", p.Goal
	key := pid + "|" + fmt.Sprintf("quest:%s:%d", d.ID, d.Version)
	if m.grants[key] {
		return false, m.credits[pid], nil
	}
	m.grants[key] = true
	m.credits[pid] += d.Reward.Credits
	return true, m.credits[pid], nil
}
