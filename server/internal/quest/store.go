package quest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
)

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Sync(ctx context.Context, defs []Def) error {
	for _, d := range defs {
		body, _ := json.Marshal(map[string]any{
			"text": d.Text, "kind": d.Kind, "target": d.Target, "system": d.System, "marker": d.Marker, "radius": d.Radius, "goal": d.Goal,
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

func (m *MemStore) SpendCredits(_ context.Context, pid string, amount int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.credits[pid]
	if !ok {
		return false, 0, nil
	}
	if amount > 0 && cur < amount {
		return false, cur, nil
	}
	m.credits[pid] = cur - amount
	return true, m.credits[pid], nil
}

// SpendCredits списывает кредиты атомарно: баланс не может уйти в минус.
func (s *PGStore) SpendCredits(ctx context.Context, pid string, amount int64) (bool, int64, error) {
	if amount <= 0 {
		var cur int64
		if err := s.pool.QueryRow(ctx, `SELECT credits FROM players WHERE public_id = $1`, pid).Scan(&cur); err != nil {
			return false, 0, err
		}
		return true, cur, nil
	}
	var credits int64
	if err := s.pool.QueryRow(ctx, `
		UPDATE players SET credits = credits - $2, updated_at = now()
		WHERE public_id = $1 AND credits >= $2
		RETURNING credits`, pid, amount).Scan(&credits); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := s.pool.QueryRow(ctx, `SELECT credits FROM players WHERE public_id = $1`, pid).Scan(&credits); err != nil {
				return false, 0, err
			}
			return false, credits, nil
		}
		return false, 0, err
	}
	return true, credits, nil
}

// MemStore — хранилище в памяти для тестов; повторяет семантику PGStore.
type MemStore struct {
	purchases     map[string]memoryPurchase
	purchaseCargo map[string]*economy.Cargo
	mu            sync.Mutex
	credits       map[string]int64
	quests        map[string]map[string]*Progress
	grants        map[string]bool
	events        map[string]eventReceipt
	Defs          []Def
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

// Advance не позволяет переполнить progress и не меняет уже завершённые задания.
func (s *PGStore) Advance(ctx context.Context, pid string, d Def, n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	var progress int
	err := s.pool.QueryRow(ctx, `UPDATE player_quest_progress
 SET progress = progress + LEAST($4::bigint, (goal - progress)::bigint)
 WHERE player_id=$1 AND quest_id=$2 AND quest_version=$3 AND state='active'
 RETURNING progress`, pid, d.ID, d.Version, n).Scan(&progress)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return progress, err
}

func (m *MemStore) Advance(_ context.Context, pid string, d Def, n int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.quests[pid][d.ID]
	if p == nil || p.State != "active" || p.Version != d.Version || n <= 0 {
		return 0, nil
	}
	if n >= p.Goal-p.Progress {
		p.Progress = p.Goal
	} else {
		p.Progress += n
	}
	return p.Progress, nil
}

// Сохранённый результат не переносится на квесты, появившиеся позже события.
type eventReceipt struct {
	hash     [32]byte
	progress map[string]int
}

var ErrEventConflict = errors.New("quest_event_payload_conflict")

func (s *PGStore) AdvanceOnce(ctx context.Context, pid, key string, hash [32]byte, defs []Def, n int) (map[string]int, error) {
	if n <= 0 || key == "" || len(key) > 128 {
		return nil, ErrEventConflict
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO quest_event_receipts(player_id,event_key,request_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, pid, key, hash[:])
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	if tag.RowsAffected() == 0 {
		var previous []byte
		var raw []byte
		if err = tx.QueryRow(ctx, `SELECT request_hash,progresses FROM quest_event_receipts WHERE player_id=$1 AND event_key=$2`, pid, key).Scan(&previous, &raw); err != nil {
			return nil, err
		}
		if string(previous) != string(hash[:]) {
			return nil, ErrEventConflict
		}
		if err = json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	ordered := append([]Def(nil), defs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	for _, d := range ordered {
		var progress int
		err = tx.QueryRow(ctx, `UPDATE player_quest_progress SET progress=progress+LEAST($4::bigint,(goal-progress)::bigint)
   WHERE player_id=$1 AND quest_id=$2 AND quest_version=$3 AND state='active' RETURNING progress`, pid, d.ID, d.Version, n).Scan(&progress)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[d.ID] = progress
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE quest_event_receipts SET progresses=$3 WHERE player_id=$1 AND event_key=$2`, pid, key, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
func (m *MemStore) AdvanceOnce(_ context.Context, pid, key string, hash [32]byte, defs []Def, n int) (map[string]int, error) {
	if n <= 0 || key == "" || len(key) > 128 {
		return nil, ErrEventConflict
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.events == nil {
		m.events = map[string]eventReceipt{}
	}
	index := pid + "|" + key
	if old, ok := m.events[index]; ok {
		if old.hash != hash {
			return nil, ErrEventConflict
		}
		out := map[string]int{}
		for k, v := range old.progress {
			out[k] = v
		}
		return out, nil
	}
	out := map[string]int{}
	for _, d := range defs {
		p := m.quests[pid][d.ID]
		if p == nil || p.Version != d.Version || p.State != "active" {
			continue
		}
		if n >= p.Goal-p.Progress {
			p.Progress = p.Goal
		} else {
			p.Progress += n
		}
		out[d.ID] = p.Progress
	}
	m.events[index] = eventReceipt{hash, out}
	copy := map[string]int{}
	for k, v := range out {
		copy[k] = v
	}
	return copy, nil
}
