// Package shop — VIP-уровни и магазин косметики. Серверная часть: каталог, покупки (идемпотентные), экипировка.
// Пока включён режим «бесплатно»: цена записывается, но не списывается. Боевых преимуществ магазин не даёт.
package shop

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"
)

//go:embed data/shop.json
var defaultJSON []byte

// Типы товаров и их слоты экипировки. Подписка слота не имеет (действует по времени).
var slotOf = map[string]string{"badge": "badge", "engine": "engine", "namecolor": "name", "avatar": "avatar", "frame": "frame", "title": "title"}

// Ошибки.
var (
	ErrNoItem    = errors.New("no_item")
	ErrOwned     = errors.New("already_owned")
	ErrVipNeeded = errors.New("vip_required")
	ErrNotOwned  = errors.New("not_owned")
	ErrBadSlot   = errors.New("bad_slot")
	ErrBadReq    = errors.New("bad_request")
)

// Item — товар каталога.
type Item struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"` // vip | badge | engine | namecolor | avatar | frame | title | subscription
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Price  int    `json:"price"`
	VipMin int    `json:"vip_min,omitempty"`
	Level  int    `json:"level,omitempty"` // для vip
	Days   int    `json:"days,omitempty"`  // для vip и subscription
	Color  string `json:"color,omitempty"`
	Icon   string `json:"icon"`
	Daily  int    `json:"daily,omitempty"` // для subscription: кредиты за первый вход дня
}

// Config — каталог магазина.
type Config struct {
	Free     bool   `json:"free"`
	Currency string `json:"currency"`
	Items    []Item `json:"items"`
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Parse разбирает и проверяет каталог.
func Parse(data []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("shop.json: %w", err)
	}
	seen := map[string]bool{}
	for _, it := range c.Items {
		if it.ID == "" || len(it.ID) > 48 || seen[it.ID] {
			return Config{}, fmt.Errorf("shop.json: пустой, длинный или повторяющийся id %q", it.ID)
		}
		seen[it.ID] = true
		if it.Name == "" || it.Price < 0 || it.Price > 1_000_000 || it.VipMin < 0 || it.VipMin > 7 {
			return Config{}, fmt.Errorf("shop.json: %s: некорректные значения", it.ID)
		}
		switch it.Kind {
		case "vip":
			if it.Level < 1 || it.Level > 7 || it.Days < 1 || it.Days > 365 {
				return Config{}, fmt.Errorf("shop.json: %s: level 1-7, days 1-365", it.ID)
			}
		case "engine", "namecolor", "frame":
			if !colorRe.MatchString(it.Color) {
				return Config{}, fmt.Errorf("shop.json: %s: color вида #rrggbb", it.ID)
			}
		case "avatar", "title":
			if it.Icon == "" {
				return Config{}, fmt.Errorf("shop.json: %s: нужна иконка", it.ID)
			}
		case "subscription":
			if it.Days < 1 || it.Days > 365 || it.Daily < 0 || it.Daily > 100_000 {
				return Config{}, fmt.Errorf("shop.json: %s: days 1-365, daily 0-100000", it.ID)
			}
		case "badge":
		default:
			return Config{}, fmt.Errorf("shop.json: %s: неизвестный вид %q", it.ID, it.Kind)
		}
	}
	return c, nil
}

// State — состояние игрока в магазине.
type State struct {
	VipLevel          int               `json:"vip_level"` // действующий уровень на момент запроса
	VipUntil          *time.Time        `json:"vip_until,omitempty"`
	SubscriptionUntil *time.Time        `json:"subscription_until,omitempty"`
	Owned             []string          `json:"owned"`
	Equipped          map[string]string `json:"equipped"`
}

// SubscriptionActive говорит, действует ли подписка на момент now.
func (s State) SubscriptionActive(now time.Time) bool {
	return s.SubscriptionUntil != nil && s.SubscriptionUntil.After(now)
}

// Store — хранилище. Все изменения атомарны.
type Store interface {
	Load(ctx context.Context, pid string, now time.Time) (State, error)
	// Buy записывает покупку по request_id. applied=false: такой запрос уже был (ничего не меняется).
	Buy(ctx context.Context, pid string, it Item, requestID string, paid int, now time.Time) (st State, applied bool, err error)
	// Equip надевает товар в слот ("" — снять).
	Equip(ctx context.Context, pid, slot, itemID string, now time.Time) (State, error)
}

// Service — бизнес-правила магазина.
type Service struct {
	cfg   Config
	store Store
	items map[string]Item
	now   func() time.Time
}

// NewService создаёт службу с каталогом по умолчанию.
func NewService(store Store) *Service {
	cfg, err := Parse(defaultJSON)
	if err != nil {
		panic(err)
	}
	return NewServiceWith(cfg, store)
}

// NewServiceWith создаёт службу с заданным каталогом.
func NewServiceWith(cfg Config, store Store) *Service {
	s := &Service{cfg: cfg, store: store, items: map[string]Item{}, now: time.Now}
	for _, it := range cfg.Items {
		s.items[it.ID] = it
	}
	return s
}

// Config возвращает каталог.
func (s *Service) Config() Config { return s.cfg }

// State возвращает состояние игрока.
func (s *Service) State(ctx context.Context, pid string) (State, error) {
	return s.store.Load(ctx, pid, s.now())
}

// Item возвращает товар по id.
func (s *Service) Item(id string) (Item, bool) { it, ok := s.items[id]; return it, ok }

// Buy покупает товар. Идемпотентна по requestID. В режиме «бесплатно» оплаченная сумма 0.
func (s *Service) Buy(ctx context.Context, pid, itemID, requestID string) (State, error) {
	if len(requestID) < 8 || len(requestID) > 64 {
		return State{}, ErrBadReq
	}
	it, ok := s.items[itemID]
	if !ok {
		return State{}, ErrNoItem
	}
	now := s.now()
	cur, err := s.store.Load(ctx, pid, now)
	if err != nil {
		return State{}, err
	}
	if cur.VipLevel < it.VipMin {
		return State{}, ErrVipNeeded
	}
	paid := it.Price
	if s.cfg.Free {
		paid = 0
	}
	st, _, err := s.store.Buy(ctx, pid, it, requestID, paid, now)
	return st, err
}

// Equip надевает купленный товар в слот; VIP-товары требуют действующего уровня.
func (s *Service) Equip(ctx context.Context, pid, slot, itemID string) (State, error) {
	now := s.now()
	if itemID != "" {
		it, ok := s.items[itemID]
		if !ok {
			return State{}, ErrNoItem
		}
		if slotOf[it.Kind] != slot {
			return State{}, ErrBadSlot
		}
		cur, err := s.store.Load(ctx, pid, now)
		if err != nil {
			return State{}, err
		}
		if cur.VipLevel < it.VipMin {
			return State{}, ErrVipNeeded
		}
	} else {
		valid := false
		for _, sl := range slotOf {
			valid = valid || sl == slot
		}
		if !valid {
			return State{}, ErrBadSlot
		}
	}
	return s.store.Equip(ctx, pid, slot, itemID, now)
}

// Cosmetics вычисляет внешний вид по состоянию: значок, цвет ника, цвет двигателя, профиль и VIP-уровень.
// Товары, требующие более высокого VIP, чем действует сейчас (VIP истёк), не показываются.
type Cosmetics struct {
	Vip       int    `json:"vip,omitempty"`
	Badge     string `json:"badge,omitempty"`
	NameColor string `json:"name_color,omitempty"`
	Engine    string `json:"engine,omitempty"`
	Avatar    string `json:"avatar,omitempty"` // иконка аватарки
	Frame     string `json:"frame,omitempty"`  // цвет рамки профиля
	Title     string `json:"title,omitempty"`  // титул (текст)
}

func (s *Service) Cosmetics(st State) Cosmetics {
	c := Cosmetics{Vip: st.VipLevel}
	for slot, id := range st.Equipped {
		it, ok := s.items[id]
		if !ok || it.VipMin > st.VipLevel || slotOf[it.Kind] != slot {
			continue
		}
		switch slot {
		case "badge":
			c.Badge = it.Icon
		case "name":
			c.NameColor = it.Color
		case "engine":
			c.Engine = it.Color
		case "avatar":
			c.Avatar = it.Icon
		case "frame":
			c.Frame = it.Color
		case "title":
			c.Title = it.Name
		}
	}
	return c
}

// ---- хранилище в памяти (тесты) ----

type memPlayer struct {
	level    int
	until    time.Time
	subUntil time.Time
	owned    map[string]bool
	equipped map[string]string
	requests map[string]bool
}

// MemStore — хранилище в памяти; повторяет семантику PGStore.
type MemStore struct {
	mu sync.Mutex
	pl map[string]*memPlayer
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore { return &MemStore{pl: map[string]*memPlayer{}} }

func (m *MemStore) get(pid string) *memPlayer {
	p := m.pl[pid]
	if p == nil {
		p = &memPlayer{owned: map[string]bool{}, equipped: map[string]string{}, requests: map[string]bool{}}
		m.pl[pid] = p
	}
	return p
}

func (p *memPlayer) state(now time.Time) State {
	st := State{Owned: []string{}, Equipped: map[string]string{}}
	if p.level > 0 && p.until.After(now) {
		st.VipLevel = p.level
		u := p.until
		st.VipUntil = &u
	}
	if p.subUntil.After(now) {
		su := p.subUntil
		st.SubscriptionUntil = &su
	}
	for id := range p.owned {
		st.Owned = append(st.Owned, id)
	}
	sort.Strings(st.Owned)
	for k, v := range p.equipped {
		st.Equipped[k] = v
	}
	return st
}

func (m *MemStore) Load(_ context.Context, pid string, now time.Time) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.get(pid).state(now), nil
}

func (m *MemStore) Buy(_ context.Context, pid string, it Item, req string, _ int, now time.Time) (State, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.get(pid)
	if p.requests[req] {
		return p.state(now), false, nil
	}
	if it.Kind == "vip" {
		active := p.level > 0 && p.until.After(now)
		base, level := now, it.Level
		if active {
			base, level = p.until, max(it.Level, p.level)
		}
		p.until, p.level = base.Add(time.Duration(it.Days)*24*time.Hour), level
	} else if it.Kind == "subscription" {
		base := now
		if p.subUntil.After(now) {
			base = p.subUntil
		}
		p.subUntil = base.Add(time.Duration(it.Days) * 24 * time.Hour)
	} else {
		if p.owned[it.ID] {
			return p.state(now), false, ErrOwned
		}
		p.owned[it.ID] = true
	}
	p.requests[req] = true
	return p.state(now), true, nil
}

func (m *MemStore) Equip(_ context.Context, pid, slot, itemID string, now time.Time) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.get(pid)
	if itemID == "" {
		delete(p.equipped, slot)
		return p.state(now), nil
	}
	if !p.owned[itemID] {
		return State{}, ErrNotOwned
	}
	p.equipped[slot] = itemID
	return p.state(now), nil
}
