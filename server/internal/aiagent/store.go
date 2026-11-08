package aiagent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — настройки в PostgreSQL (таблица ai_settings, одна строка).
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Load(ctx context.Context) (Settings, bool, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT data FROM ai_settings WHERE id = 1`).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Settings{}, false, nil
		}
		return Settings{}, false, err
	}
	out := DefaultSettings()
	if err := json.Unmarshal(raw, &out); err != nil {
		return Settings{}, false, err
	}
	return out, true, nil
}

func (s *PGStore) Save(ctx context.Context, st Settings, admin string) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO ai_settings (id, data, updated_by) VALUES (1, $1::jsonb, $2)
		ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now(), updated_by = EXCLUDED.updated_by`, string(raw), admin)
	return err
}

// MemStore — настройки в памяти (тесты).
type MemStore struct {
	mu sync.Mutex
	s  *Settings
}

func (m *MemStore) Load(context.Context) (Settings, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		return Settings{}, false, nil
	}
	return *m.s, true, nil
}

func (m *MemStore) Save(_ context.Context, s Settings, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := s
	m.s = &cp
	return nil
}
