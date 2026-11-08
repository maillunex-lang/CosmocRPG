// Package world — авторитетная симуляция одной звёздной системы (Этап 01): корабли игроков,
// движение по вектору, автопилот к цели, снимки состояния. Не знает про сеть и базу данных.
// Все числа и ограничения определяет сервер; клиент присылает только намерения.
package world

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Параметры мира (см. docs/design/STAGE01_ONLINE_SPACE.md).
const (
	Speed         = 14.0  // базовая скорость, ед/с (скорость конкретного корабля берётся из data/ships.json)
	Radius        = 400.0 // радиус системы, ед.
	AutopilotStop = 10.0  // автопилот останавливается на таком расстоянии от цели, ед.
	MaxPlayers    = 50
	Grace         = 10 * time.Second // сколько корабль остаётся в мире после обрыва связи
	StationID     = "st1"
)

// Vec — точка или вектор на плоскости системы.
type Vec struct{ X, Y float64 }

func (v Vec) add(o Vec) Vec     { return Vec{v.X + o.X, v.Y + o.Y} }
func (v Vec) sub(o Vec) Vec     { return Vec{v.X - o.X, v.Y - o.Y} }
func (v Vec) mul(k float64) Vec { return Vec{v.X * k, v.Y * k} }
func (v Vec) length() float64   { return math.Hypot(v.X, v.Y) }
func (v Vec) finite() bool      { return !math.IsNaN(v.X+v.Y) && !math.IsInf(v.X+v.Y, 0) }
func (v Vec) normalized() Vec {
	l := v.length()
	if l < 1e-9 {
		return Vec{}
	}
	return Vec{v.X / l, v.Y / l}
}

// Limit ограничивает длину вектора.
func (v Vec) limit(max float64) Vec {
	if l := v.length(); l > max {
		return v.mul(max / l)
	}
	return v
}

// Player — корабль игрока.
type Player struct {
	ID        string // публичный идентификатор (из хеша токена)
	Name      string
	Pos       Vec
	Dir       Vec
	Speed     float64 // текущая скорость, ед/с
	Move      Vec     // желаемое направление, длина 0..1
	Target    string
	Autopilot bool

	// корабль и бой (Этап «Стриж»): все значения принадлежат серверу
	ShipType     string
	Skin         string
	Spec         string
	Hull, Shield float64
	Energy       float64
	Attacking    bool
	boostUntil   float64
	cooldowns    map[string]float64 // способность -> игровое время окончания перезарядки
	attackAt     float64
	lastHit      float64
	deadUntil    float64
	revealed     map[string]float64 // скрытый объект -> игровое время окончания видимости
	spec         ShipSpec

	gender    string
	pilotSpec string

	connected bool
	lostAt    time.Time // момент обрыва связи
}

// Record — сохраняемое состояние игрока.
type Record struct {
	ID   string
	Name string
	Pos  Vec
	Dir  Vec
	Skin string // облик корабля ("" — по умолчанию)
	Spec string // специализация корабля ("" — по умолчанию)

	// персонаж (создаётся при первом входе)
	Created   bool
	Gender    string // m | f
	PilotSpec string // направление пилота: interceptor | scout | trader | miner
	ShipType  string // тип корабля ("" — по умолчанию)
}

// Object — чужой объект в снимке (корабль другого игрока или станция).
type Object struct {
	ID      string     `json:"id"`
	Kind    string     `json:"kind"`
	Stance  string     `json:"stance"`
	Name    string     `json:"name"`
	Pos     [2]float64 `json:"pos"`
	Dir     [2]float64 `json:"dir"`
	Ship    string     `json:"ship,omitempty"`
	Skin    string     `json:"skin,omitempty"`
	Hull    *float64   `json:"hull,omitempty"`
	HullMax *float64   `json:"hull_max,omitempty"`
	Dead    bool       `json:"dead,omitempty"`
}

// Ship — собственный корабль в снимке.
type Ship struct {
	Name       string        `json:"name"`
	Pos        [2]float64    `json:"pos"`
	Dir        [2]float64    `json:"dir"`
	Speed      float64       `json:"speed"`
	Type       string        `json:"type"`
	Skin       string        `json:"skin"`
	Spec       string        `json:"spec"`
	Hull       float64       `json:"hull"`
	HullMax    float64       `json:"hull_max"`
	Shield     float64       `json:"shield"`
	ShieldMax  float64       `json:"shield_max"`
	Energy     float64       `json:"energy"`
	EnergyMax  float64       `json:"energy_max"`
	Armor      float64       `json:"armor"`
	Cargo      float64       `json:"cargo"`
	PulseRange float64       `json:"pulse_range"`
	Boosting   bool          `json:"boosting"`
	Dead       bool          `json:"dead"`
	Abilities  []AbilityInfo `json:"abilities"`
}

// State — снимок состояния для одного игрока.
type State struct {
	Time      float64  `json:"time"`
	Ship      Ship     `json:"ship"`
	Objects   []Object `json:"objects"`
	TargetID  string   `json:"target_id"`
	Autopilot bool     `json:"autopilot"`
	Attacking bool     `json:"attacking"`
}

// System — описание системы.
type System struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Background string `json:"background"`
}

// World — состояние системы. Безопасен для вызова из нескольких горутин.
type World struct {
	mu      sync.Mutex
	players map[string]*Player
	t       float64
	system  System
	station Object
	catalog Catalog
	npcs    []*npc
	events  []Event
}

// New создаёт мир с одной системой и станцией.
func New() *World {
	return &World{
		players: map[string]*Player{},
		catalog: defaultCatalog.Clone(),
		npcs:    defaultNPCs(),
		system:  System{ID: "xi", Name: "Система Кси", Background: "xi"},
		station: Object{ID: StationID, Kind: "station", Stance: "friendly", Name: "Станция «Рубеж»",
			Pos: [2]float64{-12, 14}, Dir: [2]float64{0, -1}},
	}
}

// System возвращает описание системы.
func (w *World) System() System { return w.system }

// Count возвращает число кораблей в мире.
func (w *World) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.players)
}

// Join добавляет корабль игрока. false: мир переполнен. Если корабль уже в мире (повторный вход
// с тем же токеном), он остаётся на месте и снова считается подключённым.
func (w *World) Join(r Record) (*Player, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[r.ID]; ok {
		p.connected = true
		p.Name = r.Name
		p.Move = Vec{}
		return p, true
	}
	if len(w.players) >= MaxPlayers {
		return nil, false
	}
	pos := r.Pos
	if !pos.finite() || pos.length() > Radius {
		pos = Vec{}
	}
	dir := r.Dir.normalized()
	if dir == (Vec{}) {
		dir = Vec{1, 0}
	}
	p := &Player{ID: r.ID, Name: r.Name, Pos: pos, Dir: dir, connected: true}
	p.gender, p.pilotSpec = r.Gender, r.PilotSpec
	w.initShip(p, r.ShipType, r.Skin, r.Spec)
	w.players[r.ID] = p
	return p, true
}

// Disconnect помечает связь потерянной; корабль исчезнет через Grace.
func (w *World) Disconnect(id string, now time.Time) (Record, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return Record{}, false
	}
	p.connected = false
	p.lostAt = now
	p.Move = Vec{}
	p.Autopilot = false
	return p.record(), true
}

func (p *Player) record() Record {
	return Record{ID: p.ID, Name: p.Name, Pos: p.Pos, Dir: p.Dir, Skin: p.Skin, Spec: p.Spec,
		Created: true, Gender: p.gender, PilotSpec: p.pilotSpec, ShipType: p.ShipType}
}

// Records возвращает состояние всех кораблей (для периодического сохранения).
func (w *World) Records() []Record {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Record, 0, len(w.players))
	for _, p := range w.players {
		out = append(out, p.record())
	}
	return out
}

// SetMove задаёт желаемое направление (ручное управление отключает автопилот).
func (w *World) SetMove(id string, v Vec) bool {
	if !v.finite() {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return false
	}
	p.Move = v.limit(1)
	if p.Move.length() > 0.05 {
		p.Autopilot = false
	}
	return true
}

// SetTarget выбирает цель. Пустая строка снимает цель. false: такой цели нет.
func (w *World) SetTarget(id, target string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return false
	}
	if target == "" {
		p.Target = ""
		p.Autopilot = false
		return true
	}
	if target == id {
		return false
	}
	if _, _, ok := w.objectPos(target); !ok {
		return false
	}
	p.Target = target
	return true
}

// SetAutopilot включает или выключает автопилот к выбранной цели. false: цели нет.
func (w *World) SetAutopilot(id string, on bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return false
	}
	if on {
		if p.Target == "" {
			return false
		}
		p.Move = Vec{}
	}
	p.Autopilot = on
	return true
}

// objectPos ищет объект мира по идентификатору (вызывать под блокировкой).
func (w *World) objectPos(id string) (Vec, string, bool) {
	if id == w.station.ID {
		return Vec{w.station.Pos[0], w.station.Pos[1]}, w.station.Name, true
	}
	for _, n := range w.npcs {
		if n.ID == id && (!n.Hidden || n.Kind == "hostile") && n.deadUntil <= w.t {
			return n.Pos, n.Name, true
		}
	}
	if p, ok := w.players[id]; ok {
		return p.Pos, p.Name, true
	}
	return Vec{}, "", false
}

// Step продвигает симуляцию на dt секунд. now нужен для удаления потерянных кораблей;
// возвращает записи удалённых кораблей (их нужно сохранить).
func (w *World) Step(dt float64, now time.Time) []Record {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.t += dt
	var removed []Record
	for id, p := range w.players {
		if !p.connected && now.Sub(p.lostAt) >= Grace {
			removed = append(removed, p.record())
			delete(w.players, id)
			continue
		}
		w.stepCombat(p, dt)
		if p.deadUntil > w.t {
			p.Speed, p.Move = 0, Vec{}
			continue
		}
		move := p.Move
		if p.Autopilot {
			tp, _, ok := w.objectPos(p.Target)
			if !ok {
				p.Autopilot = false
				p.Target = ""
			} else {
				d := tp.sub(p.Pos)
				if d.length() > AutopilotStop {
					move = d.normalized()
				} else {
					move = Vec{}
				}
			}
		}
		vel := move.limit(1).mul(p.spec.Speed * w.speedFor(p))
		step := vel.mul(dt)
		if p.Autopilot && move != (Vec{}) {
			// не перелетать точку остановки
			if tp, _, ok := w.objectPos(p.Target); ok {
				if rem := tp.sub(p.Pos).length() - AutopilotStop; rem < step.length() && rem > 0 {
					step = step.normalized().mul(rem)
				}
			}
		}
		np := p.Pos.add(step)
		if np.length() > Radius {
			np = np.normalized().mul(Radius)
		}
		moved := np.sub(p.Pos)
		p.Pos = np
		if dt > 0 {
			p.Speed = moved.length() / dt
		}
		if moved.length() > 1e-9 {
			p.Dir = moved.normalized()
		}
	}
	w.stepNPCs()
	sort.Slice(removed, func(i, j int) bool { return removed[i].ID < removed[j].ID })
	return removed
}

// Snapshot строит снимок состояния для игрока. false: игрока нет в мире.
func (w *World) Snapshot(id string) (State, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	me, ok := w.players[id]
	if !ok {
		return State{}, false
	}
	st := State{
		Time: w.t,
		Ship: Ship{Name: me.Name, Pos: [2]float64{me.Pos.X, me.Pos.Y}, Dir: [2]float64{me.Dir.X, me.Dir.Y}, Speed: me.Speed,
			Type: me.ShipType, Skin: me.Skin, Spec: me.Spec, Hull: me.Hull, HullMax: me.spec.Hull, Shield: me.Shield, ShieldMax: me.spec.Shield,
			Energy: me.Energy, EnergyMax: me.spec.Energy, Armor: me.spec.Armor, Cargo: me.spec.Cargo, PulseRange: me.spec.Abilities["pulse"].Range, Boosting: me.boostUntil > w.t,
			Dead: me.deadUntil > w.t, Abilities: w.abilityList(me)},
		TargetID:  me.Target,
		Autopilot: me.Autopilot,
		Attacking: me.Attacking,
		Objects:   []Object{w.station},
	}
	for _, n := range w.npcs {
		if n.Hidden && me.revealed[n.ID] <= w.t {
			continue // скрытые объекты видны только после сканирования
		}
		o := Object{ID: n.ID, Kind: n.Kind, Stance: n.Stance, Name: n.Name, Pos: [2]float64{n.Pos.X, n.Pos.Y}, Dir: [2]float64{0, -1}, Dead: n.deadUntil > w.t}
		if n.Kind == "hostile" {
			h, hm := n.Hull, n.HullMax
			o.Hull, o.HullMax = &h, &hm
		}
		st.Objects = append(st.Objects, o)
	}
	ids := make([]string, 0, len(w.players))
	for pid := range w.players {
		if pid != id {
			ids = append(ids, pid)
		}
	}
	sort.Strings(ids)
	for _, pid := range ids {
		p := w.players[pid]
		st.Objects = append(st.Objects, Object{ID: p.ID, Kind: "player", Stance: "neutral", Name: p.Name,
			Pos: [2]float64{p.Pos.X, p.Pos.Y}, Dir: [2]float64{p.Dir.X, p.Dir.Y}, Ship: p.ShipType, Skin: p.Skin, Dead: p.deadUntil > w.t})
	}
	return st, true
}

// Time возвращает игровое время, с.
func (w *World) Time() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.t
}

// Dist возвращает расстояние между точками.
func Dist(a, b Vec) float64 { return a.sub(b).length() }

// ObjectPos возвращает положение объекта мира (станции или корабля) по идентификатору.
func (w *World) ObjectPos(id string) (Vec, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, _, ok := w.objectPos(id)
	return p, ok
}
