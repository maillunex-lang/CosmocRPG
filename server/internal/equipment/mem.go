package equipment

import (
	"context"
	"sync"
)

// MemStore models the same owner-existence/CAS rules as PostgreSQL. Owners are
// registered by the test/application harness, never by an equipment request.
type MemStore struct {
	mu     sync.Mutex
	owners map[string]bool
	states map[string]StarterState
}

func NewMemStore(owners ...string) *MemStore {
	m := &MemStore{owners: map[string]bool{}, states: map[string]StarterState{}}
	for _, owner := range owners {
		m.owners[owner] = true
	}
	return m
}

func (m *MemStore) Ensure(ctx context.Context, owner string) (StarterState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return StarterState{}, err
	}
	if !validOwner(owner) {
		return StarterState{}, ErrInvalidState
	}
	if !m.owners[owner] {
		return StarterState{}, ErrNoOwner
	}
	if s, ok := m.states[owner]; ok {
		if !valid(s, owner) {
			return StarterState{}, ErrInvalidState
		}
		return s, nil
	}
	s := initial(owner)
	m.states[owner] = s
	return s, nil
}

func (m *MemStore) Load(ctx context.Context, owner string) (StarterState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.load(ctx, owner)
}

func (m *MemStore) load(ctx context.Context, owner string) (StarterState, error) {
	if err := ctx.Err(); err != nil {
		return StarterState{}, err
	}
	if !validOwner(owner) {
		return StarterState{}, ErrInvalidState
	}
	s, ok := m.states[owner]
	if !ok {
		return StarterState{}, ErrNotInitialized
	}
	if !valid(s, owner) {
		return StarterState{}, ErrInvalidState
	}
	return s, nil
}

func (m *MemStore) SetEngine(ctx context.Context, owner string, revision int64, equipped bool) (StarterState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.load(ctx, owner)
	if err != nil {
		return StarterState{}, err
	}
	s, err = next(s, revision, equipped)
	if err != nil {
		return StarterState{}, err
	}
	m.states[owner] = s
	return s, nil
}

var _ StarterStore = (*MemStore)(nil)
