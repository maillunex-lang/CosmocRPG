package location

import (
	"context"
	"sync"
	"time"
)

// MemStore — журнал в памяти для тестов. Повторяет идемпотентность PGStore.
type MemStore struct {
	mu   sync.Mutex
	rows map[string][]Visit
	fail bool
}

// NewMemStore создаёт пустой журнал.
func NewMemStore() *MemStore { return &MemStore{rows: map[string][]Visit{}} }

// Fail заставляет Record вернуть ошибку.
func (m *MemStore) Fail(on bool) {
	m.mu.Lock()
	m.fail = on
	m.mu.Unlock()
}

func (m *MemStore) Record(_ context.Context, v Visit) error {
	if v.Player == "" || v.System == "" || v.ID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return ErrClosed
	}
	for _, old := range m.rows[v.Player] {
		if old.System == v.System && old.ID == v.ID {
			return nil
		}
	}
	if v.At.IsZero() {
		v.At = time.Now()
	}
	m.rows[v.Player] = append(m.rows[v.Player], v)
	return nil
}

func (m *MemStore) List(_ context.Context, player string) ([]Visit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.rows[player]
	out := make([]Visit, len(src))
	copy(out, src)
	return out, nil
}
