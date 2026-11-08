package shipxp

import (
	"context"
	"sync"
)

// MemStore — хранилище в памяти (тесты).
type MemStore struct {
	mu   sync.Mutex
	xp   map[string]map[string]int64 // игрок -> корабль -> опыт
	keys map[string]bool
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore {
	return &MemStore{xp: map[string]map[string]int64{}, keys: map[string]bool{}}
}

func (m *MemStore) Grant(_ context.Context, pid, ship, key, _ string, amount int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.xp[pid] == nil {
		m.xp[pid] = map[string]int64{}
	}
	k := pid + "|" + key
	if m.keys[k] {
		return false, m.xp[pid][ship], nil
	}
	m.keys[k] = true
	m.xp[pid][ship] += amount
	return true, m.xp[pid][ship], nil
}

func (m *MemStore) All(_ context.Context, pid string) (map[string]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]int64{}
	for k, v := range m.xp[pid] {
		out[k] = v
	}
	return out, nil
}
