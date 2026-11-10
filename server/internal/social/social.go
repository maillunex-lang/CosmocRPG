package social

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

const maxFriends = 50

var (
	ErrSelf = errors.New("friend_self")
	ErrNone = errors.New("friend_none")
	ErrFull = errors.New("friend_full")
)

type Friend struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Person struct {
	ID, Name string
}

type Store interface {
	Find(ctx context.Context, q string) (Person, bool, error)
	List(ctx context.Context, owner string) ([]Friend, error)
	Add(ctx context.Context, owner string, friend Friend) error
	Remove(ctx context.Context, owner, friendID string) error
}

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }

func (s *Service) List(ctx context.Context, owner string) ([]Friend, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("friend_off")
	}
	return s.store.List(ctx, owner)
}

func (s *Service) Add(ctx context.Context, owner, q string) ([]Friend, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, ErrNone
	}
	who, ok, err := s.store.Find(ctx, q)
	if err != nil {
		return nil, err
	}
	if !ok || who.ID == "" {
		return nil, ErrNone
	}
	if who.ID == owner {
		return nil, ErrSelf
	}
	cur, err := s.store.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	for _, f := range cur {
		if f.ID == who.ID {
			return cur, nil
		}
	}
	if len(cur) >= maxFriends {
		return nil, ErrFull
	}
	if err := s.store.Add(ctx, owner, Friend{ID: who.ID, Name: who.Name}); err != nil {
		return nil, err
	}
	return s.store.List(ctx, owner)
}

func (s *Service) Remove(ctx context.Context, owner, friendID string) ([]Friend, error) {
	if err := s.store.Remove(ctx, owner, strings.TrimSpace(friendID)); err != nil {
		return nil, err
	}
	return s.store.List(ctx, owner)
}

type Mem struct {
	mu      sync.Mutex
	people  map[string]Person
	byName  map[string]string
	friends map[string]map[string]Friend
}

func NewMem() *Mem {
	return &Mem{people: map[string]Person{}, byName: map[string]string{}, friends: map[string]map[string]Friend{}}
}

func (m *Mem) AddPerson(id, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.people[id] = Person{ID: id, Name: name}
	m.byName[strings.ToLower(name)] = id
}

func (m *Mem) Find(_ context.Context, q string) (Person, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.people[q]; ok {
		return p, true, nil
	}
	if id, ok := m.byName[strings.ToLower(q)]; ok {
		return m.people[id], true, nil
	}
	return Person{}, false, nil
}

func (m *Mem) List(_ context.Context, owner string) ([]Friend, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Friend
	for _, f := range m.friends[owner] {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Mem) Add(_ context.Context, owner string, friend Friend) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.friends[owner] == nil {
		m.friends[owner] = map[string]Friend{}
	}
	m.friends[owner][friend.ID] = friend
	return nil
}

func (m *Mem) Remove(_ context.Context, owner, friendID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.friends[owner], friendID)
	return nil
}
