package aiagent

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// World — то, что агенту нужно от мира.
type World interface {
	PlayerCount() int
	AICount() int
	AIIDs() []string
	SpawnAI(world.AISpawn) bool
	DespawnAI(id string) bool
	SetAIViewRadius(r float64)
}

// Reservations lets the new civilization system claim captains so the old FSM
// Manager does not spawn them as simple ships (avoids visible duplicates).
type Reservations interface {
	IsReserved(ctx context.Context, id string) (bool, error)
}

const (
	settingsRefresh = 30 * time.Second
	profilesRefresh = 60 * time.Second
	reuseCooldown   = 10 * time.Minute // недавно ушедший капитан не возвращается сразу, пока есть другие
)

// Status — состояние агента для админки.
type Status struct {
	Enabled    bool           `json:"enabled"`
	Paused     bool           `json:"paused"`
	Players    int            `json:"players"`
	Online     int            `json:"online"`
	Target     int            `json:"target"`
	Profiles   int            `json:"profiles"`
	ByBehavior map[string]int `json:"by_behavior"`
}

// Manager держит нужное число ИИ-капитанов в мире.
type Manager struct {
	w            World
	caps         CaptainSource
	store        SettingsStore
	reservations Reservations // optional; nil = no reservations
	log          *slog.Logger
	now          func() time.Time

	mu            sync.Mutex
	settings      Settings
	settingsAt    time.Time
	profiles      []Profile
	profilesAt    time.Time
	online        map[string]onlineAI // идентификатор -> сведения
	profileByID   map[string]Profile
	leftAt        map[string]time.Time
	emptySince    time.Time
	lastTick      time.Time
	spawnBudget   float64
	despawnBudget float64
	rng           uint32
	lastTarget    int
	loggedOnline  int
	loggedTarget  int
	loggedAt      time.Time
}

type onlineAI struct {
	behavior string
	since    time.Time
}

// NewManager создаёт агента. Настройки берутся из хранилища (нет записи — значения по умолчанию).
func NewManager(w World, caps CaptainSource, store SettingsStore, log *slog.Logger) *Manager {
	return &Manager{w: w, caps: caps, store: store, log: log, now: time.Now, settings: DefaultSettings(),
		online: map[string]onlineAI{}, leftAt: map[string]time.Time{}, rng: 2463534242, spawnBudget: 3}
}

// SetReservations attaches the new-system reservation checker. Safe to call after NewManager.
func (m *Manager) SetReservations(r Reservations) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reservations = r
}

func (m *Manager) rand() float64 {
	m.rng ^= m.rng << 13
	m.rng ^= m.rng >> 17
	m.rng ^= m.rng << 5
	return float64(m.rng%100000) / 100000.0
}

// Settings возвращает действующие настройки.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// SetSettings проверяет, сохраняет и применяет настройки.
func (m *Manager) SetSettings(ctx context.Context, s Settings, admin string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := m.store.Save(ctx, s, admin); err != nil {
		return err
	}
	m.mu.Lock()
	m.settings, m.settingsAt = s, m.now()
	m.mu.Unlock()
	m.w.SetAIViewRadius(s.ViewRadius)
	return nil
}

func (m *Manager) refresh(ctx context.Context) {
	now := m.now()
	if m.store != nil && now.Sub(m.settingsAt) >= settingsRefresh {
		m.settingsAt = now
		if s, ok, err := m.store.Load(ctx); err != nil {
			m.log.Error("ИИ: чтение настроек", "err", err)
		} else if ok && s.Validate() == nil {
			m.settings = s
			m.w.SetAIViewRadius(s.ViewRadius)
		}
	}
	if m.caps != nil && (m.profiles == nil || now.Sub(m.profilesAt) >= profilesRefresh) {
		m.profilesAt = now
		if ps, err := m.caps.Profiles(ctx); err != nil {
			m.log.Error("ИИ: чтение профилей AI Studio", "err", err)
		} else {
			sort.Slice(ps, func(i, j int) bool { return ps[i].ID < ps[j].ID })
			m.profiles = ps
			m.profileByID = map[string]Profile{}
			for _, p := range ps {
				m.profileByID[p.ID] = p
			}
		}
	}
}

// Tick выполняет один шаг: сверяет число ИИ с целью и добавляет или убирает капитанов с ограничением скорости.
func (m *Manager) Tick(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refresh(ctx)
	now := m.now()
	dt := 0.0
	if !m.lastTick.IsZero() {
		dt = now.Sub(m.lastTick).Seconds()
	}
	m.lastTick = now
	s := m.settings

	// синхронизация с миром (кто-то мог уйти из мира иным путём)
	inWorld := map[string]bool{}
	for _, id := range m.w.AIIDs() {
		inWorld[id] = true
	}
	for id := range m.online {
		if !inWorld[id] {
			delete(m.online, id)
		}
	}

	live := m.w.PlayerCount()
	if live > 0 {
		m.emptySince = time.Time{}
	} else if m.emptySince.IsZero() {
		m.emptySince = now
	}
	target := s.Target(live)
	if live == 0 && s.Enabled && !m.emptySince.IsZero() && now.Sub(m.emptySince) < time.Duration(s.EmptyGraceSec)*time.Second {
		target = len(m.online) // короткая задержка после ухода последнего игрока
	}
	m.lastTarget = target
	defer m.logChange(now, live)
	if s.Paused {
		return
	}
	m.spawnBudget = minf(m.spawnBudget+float64(s.SpawnPerMin)*dt/60, float64(maxi(s.SpawnPerMin, 1)))
	m.despawnBudget = minf(m.despawnBudget+float64(s.DespawnPerMin)*dt/60, float64(maxi(s.DespawnPerMin, 1)))

	online := len(m.online)
	switch {
	case online < target:
		n := mini(target-online, int(m.spawnBudget))
		for i := 0; i < n; i++ {
			if m.spawnOne(s, now) {
				m.spawnBudget--
			}
		}
	case online > target:
		n := online - target
		if live == 0 { // без живых игроков ИИ нет онлайн: после задержки снимаются все сразу
			m.despawnBudget = float64(n)
		}
		n = mini(n, int(m.despawnBudget))
		for i := 0; i < n; i++ {
			if m.despawnOldest(now) {
				m.despawnBudget--
			}
		}
	}
}

// logChange записывает в журнал изменение числа ИИ или цели (не чаще раза в 10 секунд).
func (m *Manager) logChange(now time.Time, live int) {
	online := len(m.online)
	if (online == m.loggedOnline && m.lastTarget == m.loggedTarget) || now.Sub(m.loggedAt) < 10*time.Second {
		return
	}
	m.loggedOnline, m.loggedTarget, m.loggedAt = online, m.lastTarget, now
	m.log.Info("ИИ-игроки", "онлайн", online, "цель", m.lastTarget, "игроков", live)
}

func (m *Manager) countByBehavior() map[string]int {
	out := map[string]int{}
	for _, a := range m.online {
		out[a.behavior]++
	}
	return out
}

// pickBehavior выбирает поведение с наибольшим отставанием от заданных долей.
func (m *Manager) pickBehavior(s Settings, total int) string {
	list := s.shareList()
	if len(list) == 0 {
		return "support"
	}
	sum := 0
	for _, k := range list {
		sum += s.Shares[k]
	}
	cur := m.countByBehavior()
	best, bestDef := list[0], -1e9
	for _, k := range list {
		def := float64(total+1)*float64(s.Shares[k])/float64(sum) - float64(cur[k]) + m.rand()*0.01
		if def > bestDef {
			best, bestDef = k, def
		}
	}
	return best
}

func (m *Manager) isReserved(id string) bool {
	if m.reservations == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ok, err := m.reservations.IsReserved(ctx, id)
	if err != nil {
		m.log.Error("ИИ: проверка резервации", "id", id, "err", err)
		return false // fail-open for old system; new system is responsible for its own actors
	}
	return ok
}

func (m *Manager) spawnOne(s Settings, now time.Time) bool {
	if len(m.profiles) == 0 {
		return false
	}
	behavior := m.pickBehavior(s, len(m.online))
	disabled := s.DisabledSet()
	var fresh, cooled []Profile
	for _, p := range m.profiles {
		if _, on := m.online[p.ID]; on || disabled[p.ID] || BehaviorFor(p.Profession) != behavior || m.isReserved(p.ID) {
			continue
		}
		if t, ok := m.leftAt[p.ID]; ok && now.Sub(t) < reuseCooldown {
			cooled = append(cooled, p)
			continue
		}
		fresh = append(fresh, p)
	}
	pool := fresh
	if len(pool) == 0 {
		pool = cooled
	}
	if len(pool) == 0 { // профилей этого поведения нет: берём любое свободное и не отключённое
		for _, p := range m.profiles {
			if _, on := m.online[p.ID]; !on && !disabled[p.ID] && !m.isReserved(p.ID) {
				pool = append(pool, p)
			}
		}
		if len(pool) == 0 {
			return false
		}
	}
	p := pool[int(m.rand()*float64(len(pool)))%len(pool)]
	b := BehaviorFor(p.Profession)
	if !m.w.SpawnAI(world.AISpawn{ID: p.ID, Name: p.Name, Profession: p.Profession, Behavior: b, ShipType: ShipFor(b, p.ID), Seed: seedOf(p.ID)}) {
		return false
	}
	m.online[p.ID] = onlineAI{behavior: b, since: now}
	return true
}

func (m *Manager) despawnOldest(now time.Time) bool {
	var oldest string
	var at time.Time
	for id, a := range m.online {
		if oldest == "" || a.since.Before(at) || (a.since.Equal(at) && id < oldest) {
			oldest, at = id, a.since
		}
	}
	if oldest == "" {
		return false
	}
	m.w.DespawnAI(oldest)
	delete(m.online, oldest)
	m.leftAt[oldest] = now
	return true
}

// Status возвращает состояние для админки.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Status{Enabled: m.settings.Enabled, Paused: m.settings.Paused, Players: m.w.PlayerCount(), Online: len(m.online), Target: m.lastTarget,
		Profiles: len(m.profiles), ByBehavior: m.countByBehavior()}
}

// OnlineCaptain — капитан онлайн для админки и диалога.
type OnlineCaptain struct {
	ID, Name, Profession, Behavior string
	Since                          time.Time
}

// IsOnline говорит, что капитан сейчас в мире.
func (m *Manager) IsOnline(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.online[id]
	return ok
}

// NameOf возвращает имя капитана по идентификатору (пустая строка, если профиль неизвестен).
func (m *Manager) NameOf(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.profileByID[id]; ok {
		return p.Name
	}
	return id
}

// RandomOnline выбирает случайного онлайн-капитана, кроме exclude (может быть ""). Нужен диалогу (ИИ-2).
func (m *Manager) RandomOnline(exclude string, r float64) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.online))
	for id := range m.online {
		if id != exclude {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return "", false
	}
	sort.Strings(ids)
	return ids[int(r*float64(len(ids)))%len(ids)], true
}

// Lookup находит онлайн-капитана по идентификатору или имени (личные сообщения, Directory чата).
func (m *Manager) Lookup(q string) (OnlineCaptain, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, a := range m.online {
		p, ok := m.profileByID[id]
		if !ok {
			continue
		}
		if id == q || strings.EqualFold(p.Name, q) {
			return OnlineCaptain{ID: id, Name: p.Name, Profession: p.Profession, Behavior: a.behavior, Since: a.since}, true
		}
	}
	return OnlineCaptain{}, false
}

// Captains возвращает все профили студии с состоянием онлайн (для админки ИИ-3 и диалога).
func (m *Manager) Captains() []OnlineCaptain {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]OnlineCaptain, 0, len(m.profiles))
	for _, p := range m.profiles {
		c := OnlineCaptain{ID: p.ID, Name: p.Name, Profession: p.Profession, Behavior: BehaviorFor(p.Profession)}
		if a, on := m.online[p.ID]; on {
			c.Since = a.since
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// KeepSettings подхватывает настройки и профили студии и не ставит корабли в мир.
// Старый цикл Run больше не запускается: простые корабли мешали цивилизации.
func (m *Manager) KeepSettings(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 2 * time.Second
	}
	load := func() {
		m.mu.Lock()
		m.refresh(ctx)
		m.mu.Unlock()
	}
	load()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			load()
		}
	}
}

// Run выполняет шаги с заданным интервалом, пока контекст не отменён.
func (m *Manager) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Tick(ctx)
		}
	}
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func mini(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PlayersOnline — сколько живых игроков сейчас в мире (для самостоятельных реплик ИИ: без читателей писать не нужно).
func (m *Manager) PlayersOnline() int { return m.w.PlayerCount() }
