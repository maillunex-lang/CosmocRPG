package world

import (
	"errors"
	"sync"
	"time"
)

// Э-1 (Решение №30): три системы и гиперпереходы.
// Router направляет вызовы к миру игрока по его текущей системе и выполняет прыжки между мирами.
// Миры изолированы: игроки разных систем не видят и не взаимодействуют друг с другом.
// Безопасен для вызова из нескольких горутин.

var (
	ErrNoLink   = errors.New("no_link")   // из текущей системы нет перехода в указанную
	ErrNoGate   = errors.New("no_gate")   // игрок слишком далеко от точки прыжка
	ErrNoSystem = errors.New("no_system") // система не существует
)

// GateRadius — на какой дистанции от точки прыжка допустим гиперпереход.
const GateRadius = 70.0

// Gate — точка гиперперехода между системами.
type Gate struct {
	To  string
	Pos Vec
}

// Router — реестр миров по системам.
type Router struct {
	mu       sync.Mutex
	primary  *World
	order    []string
	worlds   map[string]*World
	gates    map[string][]Gate // система -> точки прыжка
	byPlayer map[string]*World
}

// NewRouter создаёт маршрутизатор; primary — главный мир (стартовая система).
func NewRouter(primary *World) *Router {
	return &Router{primary: primary, worlds: map[string]*World{}, gates: map[string][]Gate{}, byPlayer: map[string]*World{}}
}

// Add регистрирует мир; идентификатор берётся из мира (System().ID).
func (r *Router) Add(w *World) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := w.System().ID
	if id == "" {
		id = "aurora"
	}
	if _, ok := r.worlds[id]; !ok {
		r.order = append(r.order, id)
	}
	r.worlds[id] = w
}

// SetGates задаёт точки прыжка (из конфигурации генератора).
func (r *Router) SetGates(system string, gates []Gate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gates[system] = gates
}

// WorldOf возвращает мир системы (nil, если не зарегистрирована).
func (r *Router) WorldOf(id string) *World {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.worldOf(id)
}

// Primary возвращает главный мир (стартовую систему; здесь живёт агент ИИ-1).
func (r *Router) Primary() *World { return r.primary }

// Systems возвращает описания всех систем в стабильном порядке.
func (r *Router) Systems() []System {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]System, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.worlds[id].System())
	}
	return out
}

// Gates возвращает точки прыжка системы (для клиента и проверок).
func (r *Router) Gates(system string) []Gate {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Gate, len(r.gates[system]))
	copy(out, r.gates[system])
	return out
}

// For возвращает мир игрока (nil, если игрок ни в одном мире).
func (r *Router) For(pid string) *World {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byPlayer[pid]
}

// worldOf — мир системы.
func (r *Router) worldOf(id string) *World {
	if id == "" {
		return r.primary
	}
	if w, ok := r.worlds[id]; ok {
		return w
	}
	return r.primary
}

// Join помещает игрока в мир его системы ("" — главный).
func (r *Router) Join(rec Record) (*Player, bool) {
	w := r.worldOf(rec.System)
	p, ok := w.Join(rec)
	if ok {
		r.mu.Lock()
		r.byPlayer[rec.ID] = w
		r.mu.Unlock()
	}
	return p, ok
}

// Leave убирает игрока из мира (без возврата записи).
func (r *Router) Leave(pid string) {
	if w := r.For(pid); w != nil {
		w.Remove(pid)
	}
	r.mu.Lock()
	delete(r.byPlayer, pid)
	r.mu.Unlock()
}

// Jump переносит игрока в другую систему через точку прыжка.
// Возвращает запись и точку появления в новой системе (у зеркальной точки прыжка).
func (r *Router) Jump(pid, to string) (Record, Vec, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	from := r.byPlayer[pid]
	if from == nil {
		return Record{}, Vec{}, ErrNoSystem
	}
	fromID := from.System().ID
	var gate *Gate
	for _, g := range r.gates[fromID] {
		if g.To == to {
			g := g
			gate = &g
			break
		}
	}
	if gate == nil {
		return Record{}, Vec{}, ErrNoLink
	}
	target := r.worldOf(to)
	if target == nil {
		return Record{}, Vec{}, ErrNoSystem
	}
	// игрок должен быть у точки прыжка своей системы
	pos, ok := from.ObjectPos(pid)
	if !ok {
		return Record{}, Vec{}, ErrNoSystem
	}
	if pos.Distance(gate.Pos) > GateRadius {
		return Record{}, Vec{}, ErrNoGate
	}
	// зеркальная точка в целевой системе (переход обратно)
	spawn := Vec{X: 0, Y: -float64(gate.Pos.Y)}
	for _, g := range r.gates[to] {
		if g.To == fromID {
			spawn = g.Pos
			break
		}
	}
	rec, ok := from.Remove(pid)
	if !ok {
		return Record{}, Vec{}, ErrNoSystem
	}
	rec.System = to
	rec.Pos = spawn
	rec.Dir = Vec{X: 0, Y: 1}
	if _, ok := target.Join(rec); !ok {
		return Record{}, Vec{}, ErrNoSystem
	}
	r.byPlayer[pid] = target
	return rec, spawn, nil
}

// Records собирает записи всех игроков всех систем.
func (r *Router) Records() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Record
	for _, id := range r.order {
		out = append(out, r.worlds[id].Records()...)
	}
	return out
}

// Step выполняет шаг симуляции всех систем.
func (r *Router) Step(dt float64, now time.Time) []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Record
	for _, id := range r.order {
		out = append(out, r.worlds[id].Step(dt, now)...)
	}
	return out
}

// DrainEvents собирает события всех систем.
func (r *Router) DrainEvents() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Event
	for _, id := range r.order {
		out = append(out, r.worlds[id].DrainEvents()...)
	}
	return out
}

// DrainRewards собирает награды всех систем.
func (r *Router) DrainRewards() []Reward {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Reward
	for _, id := range r.order {
		out = append(out, r.worlds[id].DrainRewards()...)
	}
	return out
}

// Snapshot возвращает снимок игрока в его системе.
func (r *Router) Snapshot(pid string) (State, bool) {
	w := r.For(pid)
	if w == nil {
		return State{}, false
	}
	return w.Snapshot(pid)
}

// ObjectPos возвращает позицию объекта в системе игрока.
func (r *Router) ObjectPos(pid string) (Vec, bool) {
	w := r.For(pid)
	if w == nil {
		return Vec{}, false
	}
	return w.ObjectPos(pid)
}

// Disconnect отмечает обрыв связи игрока в его системе (возвращает запись для сохранения).
func (r *Router) Disconnect(pid string, now time.Time) (Record, bool) {
	w := r.For(pid)
	if w == nil {
		return Record{}, false
	}
	return w.Disconnect(pid, now)
}

// SetShipLevel задаёт уровень корабля в системе игрока.
func (r *Router) SetShipLevel(pid string, level int) {
	if w := r.For(pid); w != nil {
		w.SetShipLevel(pid, level)
	}
}

// ShipTypeOf возвращает тип корабля игрока.
func (r *Router) ShipTypeOf(pid string) (string, bool) {
	if w := r.For(pid); w != nil {
		return w.ShipTypeOf(pid)
	}
	return "", false
}

// SetCosmetics применяет внешний вид в системе игрока.
func (r *Router) SetCosmetics(pid string, c Cosmetics) bool {
	if w := r.For(pid); w != nil {
		return w.SetCosmetics(pid, c)
	}
	return false
}

// Starter возвращает стартовый корабль (одинаков во всех системах — из главного мира).
func (r *Router) Starter(pilot string) (Starter, bool) { return r.primary.Starter(pilot) }

// Catalog возвращает каталог кораблей (общий для всех систем).
func (r *Router) Catalog() Catalog { return r.primary.Catalog() }

// Count — суммарное число игроков во всех системах.
func (r *Router) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, id := range r.order {
		n += r.worlds[id].Count()
	}
	return n
}

// PlayerCount — суммарное число живых игроков во всех системах.
func (r *Router) PlayerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, id := range r.order {
		n += r.worlds[id].PlayerCount()
	}
	return n
}
