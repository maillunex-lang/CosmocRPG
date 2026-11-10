// Package world — авторитетная симуляция одной звёздной системы (Этап 01): корабли игроков,
// движение по вектору, автопилот к цели, снимки состояния. Не знает про сеть и базу данных.
// Все числа и ограничения определяет сервер; клиент присылает только намерения.
package world

import (
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
	"math"
	"sort"
	"sync"
	"time"
)

// Параметры мира (см. docs/design/STAGE01_ONLINE_SPACE.md).
const (
	Speed         = 14.0  // базовая скорость, ед/с (скорость конкретного корабля берётся из data/ships.json)
	Radius        = 400.0 // радиус системы по умолчанию (без раскладки), ед.
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

// Distance — расстояние между точками (Э-1: проверка близости к точке прыжка).
func (v Vec) Distance(o Vec) float64 { return math.Hypot(v.X-o.X, v.Y-o.Y) }
func (v Vec) finite() bool           { return !math.IsNaN(v.X+v.Y) && !math.IsInf(v.X+v.Y, 0) }
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
	ID          string // публичный идентификатор (из хеша токена)
	Name        string
	Pos         Vec
	Dir         Vec
	Speed       float64 // текущая скорость, ед/с
	Move        Vec
	Target      string
	Autopilot   bool
	Waypoint    Vec
	HasWaypoint bool

	// корабль и бой (Этап «Стриж»): все значения принадлежат серверу
	ShipType            string
	Skin                string
	Spec                string
	Hull, Shield        float64
	Energy              float64
	Attacking           bool
	noRetaliate         bool // автоответ на атаку выключен игроком (по умолчанию включён)
	boostUntil          float64
	cooldowns           map[string]float64 // способность -> игровое время окончания перезарядки
	attackAt            float64
	lastHit             float64
	combatUntil         float64 // recent incoming/outgoing combat; prevents refitting after attack is stopped
	deadUntil           float64
	revealed            map[string]float64 // скрытый объект -> игровое время окончания видимости
	spec                ShipSpec
	starterEngine       *starterEngineItem
	starterLoadout      *StarterLoadout
	equipmentLease      *equipmentLease
	purchaseLease       *purchaseLease
	cargoRevision       int64
	cargoUncertain      bool
	persistentEquipment bool
	boostRank           int
	shipLevel           int // уровень корабля 1–50: масштабирует корпус, щит, энергию и трюм (rpg.Compute)

	gender    string
	pilotSpec string

	cosmetics Cosmetics

	// груз и живые локации
	hold       map[string]float64
	mining     string
	mineAt     float64
	mineSince  float64
	discovered map[string]bool

	ai        bool
	aiCaptain string
	connected bool
	lostAt    time.Time // момент обрыва связи
}

// Record — сохраняемое состояние игрока.
type Record struct {
	CargoRevision  int64  `json:"-"`
	SkipCargo      bool   `json:"-"`
	CargoConfirmed bool   `json:"-"`
	AI             bool   `json:"-"` // доверенная серверная метка
	AICaptain      string `json:"-"`
	ID             string
	Name           string
	Pos            Vec
	Dir            Vec
	Skin           string // облик корабля ("" — по умолчанию)
	Spec           string // специализация корабля ("" — по умолчанию)

	// персонаж (создаётся при первом входе)
	Created   bool
	Gender    string // m | f
	PilotSpec string // направление пилота: interceptor | scout | trader | miner
	ShipType  string // тип корабля ("" — по умолчанию)

	Hold map[string]float64 // груз (ресурс -> количество)

	CombatRemaining     float64         `json:"-"` // only world-to-world combat lock transport
	PersistentEquipment bool            `json:"-"` // require authoritative loadout for migrated hulls
	StarterLoadout      *StarterLoadout `json:"-"` // runtime transport only; equipment has its own store

	System string // текущая звёздная система (Э-1); "" — главная (aurora)
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
	AI      bool       `json:"ai,omitempty"` // ИИ-капитан (метка иконкой, R-42)

	// косметика (VIP, значок, цвета): только внешний вид
	Vip       int    `json:"vip,omitempty"`
	Badge     string `json:"badge,omitempty"`
	NameColor string `json:"name_color,omitempty"`
	Engine    string `json:"engine,omitempty"`

	// живые локации
	Tier      string   `json:"tier,omitempty"`
	Resource  string   `json:"resource,omitempty"`
	Rarity    string   `json:"rarity,omitempty"`
	Amount    *float64 `json:"amount,omitempty"`
	AmountMax *float64 `json:"amount_max,omitempty"`
	Zone      string   `json:"zone,omitempty"`
	Temporary bool     `json:"temporary,omitempty"`
}

// Ship — собственный корабль в снимке.
type Ship struct {
	Name       string           `json:"name"`
	Pos        [2]float64       `json:"pos"`
	Dir        [2]float64       `json:"dir"`
	Speed      float64          `json:"speed"`
	Type       string           `json:"type"`
	Skin       string           `json:"skin"`
	Spec       string           `json:"spec"`
	Hull       float64          `json:"hull"`
	HullMax    float64          `json:"hull_max"`
	Shield     float64          `json:"shield"`
	ShieldMax  float64          `json:"shield_max"`
	Energy     float64          `json:"energy"`
	EnergyMax  float64          `json:"energy_max"`
	Armor      float64          `json:"armor"`
	Cargo      float64          `json:"cargo"`
	PulseRange float64          `json:"pulse_range"`
	Boosting   bool             `json:"boosting"`
	Dead       bool             `json:"dead"`
	Abilities  []AbilityInfo    `json:"abilities"`
	Modules    []ModuleInfo     `json:"modules,omitempty"`
	Equipment  *EquipmentStatus `json:"equipment,omitempty"`
	Level      int              `json:"level"` // уровень корабля (RPG-2)

	Hold      map[string]float64 `json:"hold"`
	CargoUsed float64            `json:"cargo_used"`
	Mining    string             `json:"mining,omitempty"`
	CanMine   bool               `json:"can_mine"`
	MineRange float64            `json:"mine_range,omitempty"`
	InSafe    bool               `json:"in_safe"`

	Cosmetics Cosmetics `json:"cosmetics"`
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
	Faction    string `json:"faction,omitempty"` // владение фракции (Галактика R30)
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
	arrived []Arrival

	// живые локации
	deposits []*deposit
	safe     []safeZone
	rewards  []Reward
	ais      []*aiShip          // ИИ-капитаны (R-42): не игроки, не сохраняются, не влияют на экономику
	aiByID   map[string]*aiShip // они же по идентификатору
	aiView   float64            // радиус видимости ИИ-кораблей для игрока (область интереса)
	layout   worldgen.Layout
	wcfg     worldgen.Config
	radius   float64 // радиус системы: из раскладки, иначе Radius
}

// New создаёт мир с одной системой и станцией.
func New() *World {
	return &World{
		radius:  Radius,
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
	if r.StarterLoadout != nil && !r.StarterLoadout.Valid(r.ID) {
		return nil, false
	}
	if p, ok := w.players[r.ID]; ok {
		if p.ai != r.AI || p.aiCaptain != r.AICaptain {
			return nil, false
		}
		if p.equipmentLease != nil || p.purchaseLease != nil {
			return nil, false
		}
		if p.cargoUncertain {
			if !r.CargoConfirmed {
				return nil, false
			}
			if r.CargoRevision > p.cargoRevision {
				p.hold = copyHold(r.Hold)
				p.cargoRevision = r.CargoRevision
			}
			p.cargoUncertain = false
		}
		if !p.restoreStarter(r.StarterLoadout) {
			return nil, false
		}
		p.persistentEquipment = p.persistentEquipment || r.PersistentEquipment
		if r.StarterLoadout != nil || p.persistentEquipment {
			p.refreshStarterEngine()
		}
		p.connected = true
		p.Name = r.Name
		p.Move = Vec{}
		return p, true
	}
	humans, ais := 0, len(w.ais)
	for _, existing := range w.players {
		if existing.ai {
			ais++
		} else {
			humans++
		}
	}
	if (!r.AI && humans >= MaxPlayers) || (r.AI && ais >= 200) {
		return nil, false
	}
	if r.AI && (r.AICaptain == "" || w.aiByID[r.AICaptain] != nil) {
		return nil, false
	}
	pos := r.Pos
	if !pos.finite() || pos.length() > w.radius {
		pos = Vec{}
	}
	dir := r.Dir.normalized()
	if dir == (Vec{}) {
		dir = Vec{1, 0}
	}
	p := &Player{ID: r.ID, Name: r.Name, Pos: pos, Dir: dir, connected: true, ai: r.AI, aiCaptain: r.AICaptain}
	p.gender, p.pilotSpec = r.Gender, r.PilotSpec
	p.starterLoadout = copyStarter(r.StarterLoadout)
	p.persistentEquipment = r.PersistentEquipment
	p.combatUntil = w.t + math.Min(equipmentCombatGrace, math.Max(0, r.CombatRemaining))
	w.initShip(p, r.ShipType, r.Skin, r.Spec)
	p.hold = sanitizeHold(r.Hold, p.spec.Cargo)
	p.cargoRevision = r.CargoRevision
	p.discovered = map[string]bool{}
	w.players[r.ID] = p
	return p, true
}

// Disconnect помечает связь потерянной; корабль исчезнет через Grace.
// Remove безвозвратно убирает игрока из мира и возвращает его запись (гиперпереход, Э-1).
func (w *World) Remove(id string) (Record, bool) {
	r, err := w.removePlayer(id, nil)
	return r, err == nil
}

// removePlayer checks reservation and optional gate distance in one critical
// section. A transfer cannot slip between reservation and an earlier check.
func (w *World) removePlayer(id string, gate *Vec) (Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return Record{}, ErrNoPlayer
	}
	if p.actionBusy() {
		return Record{}, ErrEquipmentBusy
	}
	if gate != nil && p.Pos.Distance(*gate) > GateRadius {
		return Record{}, ErrNoGate
	}
	rec := p.record()
	rec.System = w.System().ID
	rec.CombatRemaining = math.Max(0, p.combatUntil-w.t)
	delete(w.players, id)
	return rec, nil
}

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
	return Record{CargoRevision: p.cargoRevision, SkipCargo: p.cargoUncertain || p.purchaseLease != nil, AI: p.ai, AICaptain: p.aiCaptain, ID: p.ID, Name: p.Name, Pos: p.Pos, Dir: p.Dir, Skin: p.Skin, Spec: p.Spec,
		PersistentEquipment: p.persistentEquipment, StarterLoadout: copyStarter(p.starterLoadout), Created: true, Gender: p.gender, PilotSpec: p.pilotSpec, ShipType: p.ShipType, Hold: copyHold(p.hold)}
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
	if p.actionBusy() {
		return false
	}
	p.Move = v.limit(1)
	if p.Move.length() > 0.05 {
		p.Autopilot = false
		p.HasWaypoint = false
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
		p.HasWaypoint = false
		return true
	}
	if target == id {
		return false
	}
	if _, _, ok := w.objectPos(target); !ok {
		return false
	}
	p.Target = target
	p.HasWaypoint = false
	return true
}

// SetWaypoint ведёт автопилот в произвольную точку системы, а не к объекту.
func (w *World) SetWaypoint(id string, pos Vec) bool {
	if !pos.finite() {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok || p.actionBusy() || p.deadUntil > w.t {
		return false
	}
	if l := pos.length(); w.radius > 0 && l > w.radius {
		pos = pos.normalized().mul(w.radius * 0.98)
	}
	p.Waypoint = pos
	p.HasWaypoint = true
	p.Target = ""
	p.Move = Vec{}
	p.Autopilot = true
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
	if on && p.equipmentLease != nil {
		return false
	}
	if on {
		if p.Target == "" && !p.HasWaypoint {
			return false
		}
		p.Move = Vec{}
	} else {
		p.HasWaypoint = false
	}
	p.Autopilot = on
	return true
}

// autopilotDest — точка маршрута: произвольная метка или выбранный объект. Вызывать под блокировкой.
func (w *World) autopilotDest(p *Player) (Vec, bool) {
	if p.HasWaypoint {
		return p.Waypoint, true
	}
	if p.Target == "" {
		return Vec{}, false
	}
	pos, _, ok := w.objectPos(p.Target)
	return pos, ok
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
	for _, d := range w.deposits {
		if d.ID == id {
			return d.Pos, d.Name, true
		}
	}
	if p, ok := w.players[id]; ok {
		return p.Pos, p.Name, true
	}
	if a, ok := w.aiByID[id]; ok {
		return a.Pos, a.Name, true
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
		if !p.connected && p.equipmentLease == nil && p.purchaseLease == nil && now.Sub(p.lostAt) >= Grace {
			removed = append(removed, p.record())
			delete(w.players, id)
			continue
		}
		w.stepCombat(p, dt)
		if p.deadUntil > w.t {
			p.Speed, p.Move = 0, Vec{}
			continue
		}
		if p.actionBusy() {
			p.Speed = 0
			continue // keep only this actor docked; damage/combat above still run
		}
		move := p.Move
		if p.Autopilot {
			dest, ok := w.autopilotDest(p)
			if !ok {
				p.Autopilot = false
				p.Target = ""
				p.HasWaypoint = false
			} else {
				d := dest.sub(p.Pos)
				if d.length() > AutopilotStop {
					move = d.normalized()
				} else {
					move = Vec{}
					if p.HasWaypoint {
						p.HasWaypoint = false
						p.Autopilot = false
					}
				}
			}
		}
		vel := move.limit(1).mul(p.spec.Speed * w.speedFor(p))
		step := vel.mul(dt)
		if p.Autopilot && move != (Vec{}) {
			if dest, ok := w.autopilotDest(p); ok {
				if rem := dest.sub(p.Pos).length() - AutopilotStop; rem < step.length() && rem > 0 {
					step = step.normalized().mul(rem)
				}
			}
		}
		np := p.Pos.add(step)
		if np.length() > w.radius {
			np = np.normalized().mul(w.radius)
		}
		moved := np.sub(p.Pos)
		p.Pos = np
		if dt > 0 {
			p.Speed = moved.length() / dt
		}
		if moved.length() > 1e-9 {
			p.Dir = moved.normalized()
		}
		w.faceAttackTarget(p, dt)
	}
	w.stepNPCs()
	w.stepLiving(dt)
	w.stepAIs(dt)
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
			Dead: me.deadUntil > w.t, Abilities: w.abilityList(me), Modules: me.moduleList(), Equipment: w.equipmentStatus(me), Level: maxInt(me.shipLevel, 1), Hold: copyHold(me.hold), CargoUsed: holdUsed(me.hold), Mining: me.mining, Cosmetics: me.cosmetics,
			CanMine: me.spec.Abilities["mine"].Range > 0, MineRange: me.spec.Abilities["mine"].Range, InSafe: w.inSafe(me.Pos)},
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
	st.Objects = append(st.Objects, w.depositObjects()...)
	ids := make([]string, 0, len(w.players))
	for pid := range w.players {
		if pid != id {
			ids = append(ids, pid)
		}
	}
	sort.Strings(ids)
	for _, pid := range ids {
		p := w.players[pid]
		st.Objects = append(st.Objects, Object{AI: p.ai, ID: p.ID, Kind: "player", Stance: "neutral", Name: p.Name,
			Pos: [2]float64{p.Pos.X, p.Pos.Y}, Dir: [2]float64{p.Dir.X, p.Dir.Y}, Ship: p.ShipType, Skin: p.Skin, Dead: p.deadUntil > w.t,
			Vip: p.cosmetics.Vip, Badge: p.cosmetics.Badge, NameColor: p.cosmetics.NameColor, Engine: p.cosmetics.Engine})
	}
	st.Objects = append(st.Objects, w.aiObjects(me.Pos)...)
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
