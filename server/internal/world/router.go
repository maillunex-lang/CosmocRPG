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
	To  string     `json:"to"`
	Pos [2]float64 `json:"pos"`
}

// Router — реестр миров по системам.
type Router struct {
	mu       sync.Mutex
	primary  *World
	order    []string
	worlds   map[string]*World
	gates    map[string][]Gate // система -> точки прыжка
	byPlayer map[string]*World
	parties  map[string]*party
	byMember map[string]string
	invites  map[string]partyInvite
}

// NewRouter создаёт маршрутизатор; primary — главный мир (стартовая система).
func NewRouter(primary *World) *Router {
	return &Router{
		primary: primary, worlds: map[string]*World{}, gates: map[string][]Gate{}, byPlayer: map[string]*World{},
		parties: map[string]*party{}, byMember: map[string]string{}, invites: map[string]partyInvite{},
	}
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

// Links возвращает уникальные рёбра гиперпереходов между системами (для звёздной карты).
func (r *Router) Links() [][2]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]bool{}
	var out [][2]string
	for from, gates := range r.gates {
		for _, g := range gates {
			key := from + "|" + g.To
			rev := g.To + "|" + from
			if seen[key] || seen[rev] {
				continue
			}
			seen[key] = true
			out = append(out, [2]string{from, g.To})
		}
	}
	return out
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
		if skipInstanceSystem(id) || r.worlds[id] == nil {
			continue
		}
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
	r.mu.Lock()
	w := r.byPlayer[rec.ID]
	if w == nil {
		w = r.worldOf(rec.System)
	}
	r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if w := r.byPlayer[pid]; w != nil {
		if _, err := w.removePlayer(pid, nil); err == ErrEquipmentBusy {
			return
		}
	}
	delete(r.byPlayer, pid)
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
	// зеркальная точка в целевой системе (переход обратно)
	gateVec := Vec{X: gate.Pos[0], Y: gate.Pos[1]}
	spawn := Vec{X: 0, Y: -gate.Pos[1]}
	for _, g := range r.gates[to] {
		if g.To == fromID {
			spawn = Vec{X: g.Pos[0], Y: g.Pos[1]}
			break
		}
	}
	rec, err := from.removePlayer(pid, &gateVec)
	if err != nil {
		if err == ErrNoPlayer {
			err = ErrNoSystem
		}
		return Record{}, Vec{}, err
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

// Place переносит игрока в другую систему без точки прыжка. Нужен ИИ, чтобы
// капитан с уже пройденным сюжетом не появлялся у всех на станции Авроры.
func (r *Router) Place(pid, system string, pos Vec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	target := r.worlds[system]
	if target == nil {
		return ErrNoSystem
	}
	from := r.byPlayer[pid]
	if from == nil {
		return ErrNoSystem
	}
	if from == target {
		return nil
	}
	rec, err := from.removePlayer(pid, nil)
	if err != nil {
		return err
	}
	rec.System = system
	rec.Pos = pos
	if _, ok := target.Join(rec); !ok {
		return ErrNoSystem
	}
	r.byPlayer[pid] = target
	return nil
}

// SetPos двигает только корабль ИИ в его текущей системе.
func (r *Router) SetPos(pid string, pos Vec) bool {
	w := r.For(pid)
	if w == nil {
		return false
	}
	return w.MoveAI(pid, pos)
}

// SystemID — система, в которой игрок сейчас. Пусто, если его нет в мире.
func (r *Router) SystemID(pid string) string {
	w := r.For(pid)
	if w == nil {
		return ""
	}
	id := w.System().ID
	if id == "" {
		return "aurora"
	}
	return id
}

// Records собирает записи всех игроков всех систем.
func (r *Router) Records() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Record
	for _, id := range r.order {
		w := r.worlds[id]
		if w == nil {
			continue
		}
		recs := w.Records()
		for i := range recs {
			if home, ok := r.homePosLocked(recs[i].ID); ok {
				recs[i].Pos = home
			}
		}
		out = append(out, recs...)
	}
	return out
}

// Step выполняет шаг симуляции всех систем.
func (r *Router) Step(dt float64, now time.Time) []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.syncPartyLocked()
	var out []Record
	for _, id := range r.order {
		w := r.worlds[id]
		if w == nil {
			continue
		}
		recs := w.Step(dt, now)
		for i := range recs {
			if home, ok := r.homePosLocked(recs[i].ID); ok {
				recs[i].Pos = home
			}
			if r.byPlayer[recs[i].ID] == w {
				delete(r.byPlayer, recs[i].ID)
			}
		}
		out = append(out, recs...)
	}
	r.tickPartiesLocked()
	return out
}

func (r *Router) syncPartyLocked() {
	for _, w := range r.worlds {
		if w != nil {
			w.clearPartyTags()
		}
	}
	for _, p := range r.parties {
		if p == nil {
			continue
		}
		for _, m := range p.members {
			if m.pending {
				continue
			}
			w := r.byPlayer[m.id]
			if w == nil {
				continue
			}
			lead := ""
			if m.id != p.leader && !m.wing && !m.ai && !m.followOff && r.byPlayer[p.leader] == w {
				lead = p.leader
			}
			w.tagParty(m.id, p.id, lead)
		}
	}
}

func (w *World) clearPartyTags() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range w.players {
		p.partyID = ""
		p.followLeader = ""
	}
}

func (w *World) tagParty(id, party, leader string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return
	}
	p.partyID = party
	p.followLeader = leader
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

// DrainArrivals собирает первые прибытия всех систем.
func (r *Router) DrainArrivals() []Arrival {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Arrival
	for _, id := range r.order {
		out = append(out, r.worlds[id].DrainArrivals()...)
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
	st, ok := w.Snapshot(pid)
	if !ok {
		return State{}, false
	}
	if view, has := r.PartySnapshot(pid); has {
		st.Party = &view
	}
	return st, true
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
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureParty()
	if pidParty := r.byMember[pid]; pidParty != "" {
		if p := r.parties[pidParty]; p != nil && p.inst != "" {
			if p.leader == pid {
				r.disbandLocked(p, false)
			} else {
				r.extractLocked(p, pid)
				r.dropMemberLocked(p, pid)
			}
		}
	}
	w := r.byPlayer[pid]
	if w == nil {
		return Record{}, false
	}
	rec, ok := w.Disconnect(pid, now)
	if !ok {
		return Record{}, false
	}
	if home, okHome := r.homePosLocked(pid); okHome {
		rec.Pos = home
	}
	return rec, true
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
