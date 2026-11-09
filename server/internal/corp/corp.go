// Package corp — корпорации минимум (R-64, К-1): создание, открытое вступление, состав, тег, казна.
// Войны, базы, налоги и дипломатия — будущие этапы, здесь не реализуются.
package corp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Стоимость создания корпорации (кредиты, списываются через переданную функцию spend).
const CreateCost int64 = 10000

var (
	ErrBadName   = errors.New("bad_name")
	ErrBadTag    = errors.New("bad_tag")
	ErrTaken     = errors.New("corp_taken")
	ErrNoCorp    = errors.New("no_corp")
	ErrHasCorp   = errors.New("already_in_corp")
	ErrNotMember = errors.New("not_member")
)

var (
	nameRe = regexp.MustCompile(`^[А-Яа-яA-Za-z0-9][А-Яа-яA-Za-z0-9 _-]{1,30}$`)
	tagRe  = regexp.MustCompile(`^[A-Z0-9]{2,4}$`)
)

// Corporation — публичные сведения о корпорации.
type Corporation struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Tag      string    `json:"tag"`
	Owner    string    `json:"owner"`
	Treasury int64     `json:"treasury"`
	Created  time.Time `json:"created_at"`
}

// Member — участник корпорации.
type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Store — хранилище. Все изменения атомарны.
type Store interface {
	// CorporationByPlayer возвращает корпорацию игрока (ok=false — игрок ни в одной).
	CorporationByPlayer(ctx context.Context, pid string) (Corporation, bool, error)
	// Exists проверяет занятость имени или тега до списания взноса.
	Exists(ctx context.Context, name, tag string) (bool, error)
	Create(ctx context.Context, c Corporation, ownerName string) error
	// Join добавляет игрока (open join). ok=false — игрок уже в корпорации или корпорация не найдена по name/id.
	Join(ctx context.Context, corpID, pid, name string) (bool, error)
	Leave(ctx context.Context, pid string) (bool, error)
	Members(ctx context.Context, corpID string) ([]Member, error)
}

// Service — правила корпораций. spend списывает кредиты атомарно (quest.Service.Spend).
type Service struct {
	store Store
	spend func(ctx context.Context, pid string, amount int64) (int64, error)
	now   func() time.Time
}

// NewService создаёт службу. spend может быть nil — тогда создание бесплатное (тесты).
func NewService(store Store, spend func(ctx context.Context, pid string, amount int64) (int64, error)) *Service {
	return &Service{store: store, spend: spend, now: time.Now}
}

func cleanName(s string) string { return strings.Join(strings.Fields(strings.TrimSpace(s)), " ") }

// Create создаёт корпорацию: уникальные имя и тег, цена CreateCost кредитов с баланса основателя.
func (s *Service) Create(ctx context.Context, pid, name, tag string) (Corporation, error) {
	name = cleanName(name)
	tag = strings.ToUpper(strings.TrimSpace(tag))
	if !nameRe.MatchString(name) {
		return Corporation{}, ErrBadName
	}
	if !tagRe.MatchString(tag) {
		return Corporation{}, ErrBadTag
	}
	if _, ok, err := s.store.CorporationByPlayer(ctx, pid); err != nil {
		return Corporation{}, err
	} else if ok {
		return Corporation{}, ErrHasCorp
	}
	taken, err := s.store.Exists(ctx, name, tag)
	if err != nil {
		return Corporation{}, err
	}
	if taken {
		return Corporation{}, ErrTaken
	}
	if s.spend != nil {
		if _, err := s.spend(ctx, pid, CreateCost); err != nil {
			return Corporation{}, fmt.Errorf("списание взноса: %w", err)
		}
	}
	c := Corporation{ID: "co-" + strings.ToLower(tag), Name: name, Tag: tag, Owner: pid, Created: s.now()}
	if err := s.store.Create(ctx, c, pid); err != nil {
		if errors.Is(err, ErrTaken) {
			return Corporation{}, ErrTaken
		}
		return Corporation{}, err
	}
	return c, nil
}

// Info возвращает корпорацию игрока и её состав.
func (s *Service) Info(ctx context.Context, pid string) (Corporation, []Member, error) {
	c, ok, err := s.store.CorporationByPlayer(ctx, pid)
	if err != nil {
		return Corporation{}, nil, err
	}
	if !ok {
		return Corporation{}, nil, ErrNoCorp
	}
	mem, err := s.store.Members(ctx, c.ID)
	return c, mem, err
}

// Join — открытое вступление по имени или тегу корпорации.
func (s *Service) Join(ctx context.Context, pid, nameOrTag, playerName string) (Corporation, error) {
	if _, ok, err := s.store.CorporationByPlayer(ctx, pid); err != nil {
		return Corporation{}, err
	} else if ok {
		return Corporation{}, ErrHasCorp
	}
	joined, err := s.store.Join(ctx, "", pid, nameOrTag) // store находит корпорацию по имени/тегу
	if err != nil {
		return Corporation{}, err
	}
	if !joined {
		return Corporation{}, ErrNoCorp
	}
	c, _, err := s.store.CorporationByPlayer(ctx, pid)
	return c, err
}

// Leave выходит из корпорации (основатель пока может выйти наравне с членами; передача прав — позже).
func (s *Service) Leave(ctx context.Context, pid string) error {
	left, err := s.store.Leave(ctx, pid)
	if err != nil {
		return err
	}
	if !left {
		return ErrNotMember
	}
	return nil
}

// ---- хранилище в памяти (тесты) ----

type memCorp struct {
	Corporation
	members map[string]Member
}

// MemStore — хранилище в памяти; повторяет ограничения PGStore.
type MemStore struct {
	mu     sync.Mutex
	byID   map[string]*memCorp
	byName map[string]string
	byTag  map[string]string
	player map[string]string
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore {
	return &MemStore{byID: map[string]*memCorp{}, byName: map[string]string{}, byTag: map[string]string{}, player: map[string]string{}}
}

func (m *MemStore) CorporationByPlayer(_ context.Context, pid string) (Corporation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.player[pid]
	if !ok {
		return Corporation{}, false, nil
	}
	return m.byID[id].Corporation, true, nil
}

func (m *MemStore) Exists(_ context.Context, name, tag string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byName[strings.ToLower(name)]; ok {
		return true, nil
	}
	if _, ok := m.byTag[tag]; ok {
		return true, nil
	}
	return false, nil
}

func (m *MemStore) Create(_ context.Context, c Corporation, ownerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byName[strings.ToLower(c.Name)]; ok {
		return ErrTaken
	}
	if _, ok := m.byTag[c.Tag]; ok {
		return ErrTaken
	}
	m.byID[c.ID] = &memCorp{Corporation: c, members: map[string]Member{c.Owner: {ID: c.Owner, Name: ownerName, Role: "owner"}}}
	m.byName[strings.ToLower(c.Name)] = c.ID
	m.byTag[c.Tag] = c.ID
	m.player[c.Owner] = c.ID
	return nil
}

// Join ищет корпорацию по имени (регистр не важен) или тегу (верхний регистр).
func (m *MemStore) Join(_ context.Context, _ string, pid, nameOrTag string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.player[pid]; ok {
		return false, nil
	}
	id, ok := m.byName[strings.ToLower(cleanName(nameOrTag))]
	if !ok {
		id, ok = m.byTag[strings.ToUpper(strings.TrimSpace(nameOrTag))]
	}
	if !ok {
		return false, nil
	}
	c := m.byID[id]
	c.members[pid] = Member{ID: pid, Role: "member"}
	m.player[pid] = id
	return true, nil
}

func (m *MemStore) Leave(_ context.Context, pid string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.player[pid]
	if !ok {
		return false, nil
	}
	delete(m.byID[id].members, pid)
	delete(m.player, pid)
	return true, nil
}

func (m *MemStore) Members(_ context.Context, corpID string) ([]Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Member
	for _, mem := range m.byID[corpID].members {
		out = append(out, mem)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role == "owner"
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
