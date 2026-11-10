package world

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ИИ-капитаны (R-42, этап ИИ-1). Это корабли мира, а не игроки: у них нет сессии и записи в базе, они не добывают и не продают
// настоящие ресурсы, не получают награды и не влияют на экономику, а только ходят по правилам своей профессии и делают мир живым.
// Появлением и исчезновением управляет пакет aiagent; здесь только простая детерминированная симуляция.

const (
	// AIViewRadius — на каком расстоянии от игрока видны ИИ-корабли (область интереса: полный список не рассылается).
	AIViewRadius = 450.0
	// AICruise — доля максимальной скорости корабля, с которой ИИ летит по маршруту.
	AICruise = 0.55
)

// Поведение ИИ-капитана по профессии.
const (
	AIMiner   = "miner"   // летает между месторождениями и станцией
	AITrader  = "trader"  // курсирует между станциями
	AIScout   = "scout"   // облетает точки исследования и месторождения
	AIPatrol  = "patrol"  // патрулирует вокруг станций
	AISupport = "support" // держится рядом со станциями
)

const (
	aiTravel = iota
	aiWork
)

// AISpawn — описание появляющегося ИИ-капитана.
type AISpawn struct {
	ID, Name, Profession string
	Behavior             string // AIMiner, AITrader, AIScout, AIPatrol, AISupport
	ShipType             string // тип корабля из каталога; неизвестный заменяется кораблём по умолчанию
	Skin                 string // облик; пусто или неизвестный — по зерну
	Seed                 uint32 // зерно поведения: одинаковое зерно даёт одинаковый маршрут
}

type aiShip struct {
	ID, Name, Profession, Behavior string
	ShipType, Skin                 string
	Pos, Dir                       Vec
	speed                          float64
	state                          int
	dest                           Vec
	phase                          int // чередование работы и «сдачи» у станции
	workUntil                      float64
	rng                            uint32
	home                           Vec
	homeR                          float64
	level                          int
}

func (a *aiShip) rand() float64 {
	if a.rng == 0 {
		a.rng = 2463534242
	}
	a.rng ^= a.rng << 13
	a.rng ^= a.rng >> 17
	a.rng ^= a.rng << 5
	return float64(a.rng%100000) / 100000.0
}

func (w *World) aiStations() []Vec {
	out := []Vec{{w.station.Pos[0], w.station.Pos[1]}}
	for _, n := range w.npcs {
		if n.Kind == "station" {
			out = append(out, n.Pos)
		}
	}
	return out
}

// SpawnAI добавляет ИИ-капитана на станцию. false: такой идентификатор уже есть.
func (w *World) SpawnAI(s AISpawn) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if s.ID == "" {
		return false
	}
	if _, dup := w.aiByID[s.ID]; dup {
		return false
	}
	if _, clash := w.players[s.ID]; clash {
		return false
	}
	for _, p := range w.players {
		if p.ai && p.aiCaptain == s.ID {
			return false
		}
	}
	sp, ok := w.catalog.Spec(s.ShipType)
	if !ok {
		sp, _ = w.catalog.Spec(DefaultShip)
	}
	a := &aiShip{ID: s.ID, Name: s.Name, Profession: s.Profession, Behavior: s.Behavior, ShipType: sp.ID, speed: sp.Speed, rng: s.Seed*2654435761 + 12345}
	a.Skin = sp.Skins[0]
	for _, k := range sp.Skins {
		if k == s.Skin {
			a.Skin = k
		}
	}
	if s.Skin == "" && len(sp.Skins) > 0 {
		a.Skin = sp.Skins[int(a.rand()*float64(len(sp.Skins)))%len(sp.Skins)]
	}
	sts := w.aiStations()
	base := sts[int(a.rand()*float64(len(sts)))%len(sts)]
	ang := a.rand() * 2 * math.Pi
	a.Pos = base.add(Vec{math.Cos(ang), math.Sin(ang)}.mul(6 + a.rand()*14))
	a.Dir = Vec{math.Cos(ang + 1), math.Sin(ang + 1)}
	a.state, a.workUntil = aiWork, w.t+1+a.rand()*6
	if w.aiByID == nil {
		w.aiByID = map[string]*aiShip{}
	}
	w.aiByID[a.ID] = a
	w.ais = append(w.ais, a)
	return true
}

// DespawnAI убирает ИИ-капитана. false: такого нет.
func (w *World) DespawnAI(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.aiByID[id]; !ok {
		return false
	}
	delete(w.aiByID, id)
	for i, a := range w.ais {
		if a.ID == id {
			w.ais = append(w.ais[:i], w.ais[i+1:]...)
			break
		}
	}
	return true
}

// AICount — сколько ИИ-капитанов сейчас в мире.
func (w *World) AICount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.ais)
}

// AIIDs — идентификаторы ИИ-капитанов в мире (по возрастанию).
func (w *World) AIIDs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.ais))
	for _, a := range w.ais {
		out = append(out, a.ID)
	}
	sort.Strings(out)
	return out
}

// PlayerCount — сколько живых игроков сейчас на связи (ИИ не считаются).
func (w *World) PlayerCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, p := range w.players {
		if p.connected && !p.ai {
			n++
		}
	}
	return n
}

// SetAIViewRadius меняет радиус видимости ИИ-кораблей (0 — по умолчанию).
func (w *World) SetAIViewRadius(r float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.aiView = r
}

// aiObjects возвращает всех ИИ-пилотов системы. Раньше дальние пропадали из снимка
// и на карте мигали: то есть, то нет.
func (w *World) aiObjects(_ Vec) []Object {
	var out []Object
	for _, a := range w.ais {
		out = append(out, Object{ID: a.ID, Kind: "ai", Stance: "neutral", Name: a.Name, Pos: [2]float64{a.Pos.X, a.Pos.Y},
			Dir: [2]float64{a.Dir.X, a.Dir.Y}, Ship: a.ShipType, Skin: a.Skin, AI: true, Level: a.level})
	}
	return out
}

// stepAIs двигает ИИ-капитанов (под блокировкой).
func (w *World) stepAIs(dt float64) {
	for _, a := range w.ais {
		if a.state == aiWork {
			if w.t >= a.workUntil {
				w.aiPickDest(a)
			}
			continue
		}
		d := a.dest.sub(a.Pos)
		left := d.length()
		if left < 2.5 {
			a.state = aiWork
			a.workUntil = w.t + w.aiDwell(a)
			a.phase++
			continue
		}
		a.Dir = turnToward(a.Dir, d, TurnRate*dt)
		step := math.Min(left, a.speed*AICruise*dt)
		a.Pos = a.Pos.add(d.normalized().mul(step))
		if a.Pos.length() > w.radius {
			a.Pos = a.Pos.normalized().mul(w.radius)
		}
	}
}

// aiDwell — сколько ИИ задерживается в точке назначения: на работе дольше, у станции короче.
func (w *World) aiDwell(a *aiShip) float64 {
	switch a.Behavior {
	case AIMiner:
		if a.phase%2 == 0 { // только что прилетел к месторождению
			return 20 + a.rand()*30
		}
		return 6 + a.rand()*8
	case AIScout:
		return 5 + a.rand()*10
	case AIPatrol:
		return 3 + a.rand()*7
	}
	return 8 + a.rand()*14
}

func nearest(from Vec, pts []Vec) Vec {
	best, bd := pts[0], math.Inf(1)
	for _, p := range pts {
		if d := dist(from, p); d < bd {
			best, bd = p, d
		}
	}
	return best
}

// aiPickDest выбирает следующую точку маршрута по поведению.
func (w *World) aiPickDest(a *aiShip) {
	stations := w.aiStations()
	var deps []Vec
	for _, d := range w.deposits {
		deps = append(deps, d.Pos)
	}
	var points []Vec
	for _, n := range w.npcs {
		if n.pt != nil && !n.Hidden {
			points = append(points, n.Pos)
		}
	}
	pick := func(pts []Vec) Vec { return pts[int(a.rand()*float64(len(pts)))%len(pts)] }
	around := func(c Vec, r float64) Vec {
		ang := a.rand() * 2 * math.Pi
		return c.add(Vec{math.Cos(ang), math.Sin(ang)}.mul(r * (0.4 + 0.6*a.rand())))
	}
	var dest Vec
	if a.homeR > 0 {
		dest = around(a.home, a.homeR*0.72)
	} else {
		switch a.Behavior {
		case AIMiner:
			if a.phase%2 == 1 || len(deps) == 0 { // после работы — на станцию
				dest = around(nearest(a.Pos, stations), 12)
			} else {
				dest = around(pick(deps), 5)
			}
		case AITrader:
			dest = around(pick(stations), 12)
			if len(stations) == 1 && len(points) > 0 && a.phase%2 == 1 {
				dest = around(pick(points), 15)
			}
		case AIScout:
			pool := append(append([]Vec{}, points...), deps...)
			if len(pool) == 0 {
				pool = stations
			}
			dest = around(pick(pool), 10)
		case AIPatrol:
			dest = around(pick(stations), 70+a.rand()*60)
		default: // support и неизвестные
			dest = around(pick(stations), 45)
		}
	}
	if dest.length() > w.radius {
		dest = dest.normalized().mul(w.radius * 0.95)
	}
	a.dest, a.state = dest, aiTravel
}

var localPilotNames = []string{
	"Север", "Иней", "Волна", "Кедр", "Искра", "Буран", "Ласточка", "Гранит",
	"Огонёк", "Туман", "Янтарь", "Сокол", "Нива", "Прибой", "Кварц", "Заря",
}

func hullForLevel(level, role int) string {
	low := []string{"FTR-001", "SCT-001", "TMP-MINE", "SUP-001"}
	mid := []string{"FTR-002", "COR-001", "SCT-001", "TMP-MINE"}
	high := []string{"COR-002", "FRG-001", "COR-001", "TMP-HAUL"}
	top := []string{"DST-001", "FRG-002", "CRU-001", "COR-002"}
	table := low
	switch {
	case level >= 16:
		table = top
	case level >= 10:
		table = high
	case level >= 5:
		table = mid
	}
	return table[role%len(table)]
}

// seedLocalPilotsLocked ставит по два пилота в каждую зону. Уровень растёт с опасностью зоны
// и сложностью системы. Повторный вызов заменяет прежний состав, не трогая капитанов студии.
func (w *World) seedLocalPilotsLocked() {
	if w.aiByID == nil {
		w.aiByID = map[string]*aiShip{}
	}
	var kept []*aiShip
	for _, a := range w.ais {
		if strings.HasPrefix(a.ID, "loc-") {
			delete(w.aiByID, a.ID)
			continue
		}
		kept = append(kept, a)
	}
	w.ais = kept
	if w.lane != nil || len(w.layout.Zones) == 0 {
		return
	}
	diff := 1
	for _, sp := range w.wcfg.Systems {
		if sp.ID == w.system.ID && sp.Difficulty > 0 {
			diff = sp.Difficulty
		}
	}
	sys := w.system.ID
	if sys == "" {
		sys = "sys"
	}
	n := 0
	for zi, z := range w.layout.Zones {
		if z.Radius <= 1 {
			continue
		}
		for k := 0; k < 2; k++ {
			danger := z.Danger
			if danger < 0 {
				danger = 0
			}
			level := 1 + danger*3 + (diff-1)*4
			if level > 30 {
				level = 30
			}
			behavior := AIPatrol
			if k == 0 {
				switch z.Type {
				case "mining":
					behavior = AIMiner
				case "explore":
					behavior = AIScout
				case "trade":
					behavior = AITrader
				default:
					behavior = AIPatrol
				}
			} else {
				behavior = AISupport
			}
			ang := float64((zi*37+k*113)%360) * math.Pi / 180
			rad := z.Radius * (0.28 + 0.22*float64(k))
			pos := Vec{z.Pos[0] + math.Cos(ang)*rad, z.Pos[1] + math.Sin(ang)*rad}
			name := fmt.Sprintf("%s · ур. %d", localPilotNames[(zi*2+k)%len(localPilotNames)], level)
			w.placeLocalPilotLocked(fmt.Sprintf("loc-%s-%d", sys, n), name, behavior, hullForLevel(level, k), pos, Vec{z.Pos[0], z.Pos[1]}, z.Radius, level, uint32(n+1))
			n++
		}
	}
}

func (w *World) placeLocalPilotLocked(id, name, behavior, ship string, pos, home Vec, homeR float64, level int, seed uint32) {
	if id == "" || w.aiByID[id] != nil || w.players[id] != nil {
		return
	}
	sp, ok := w.catalog.Spec(ship)
	if !ok {
		sp, _ = w.catalog.Spec(DefaultShip)
	}
	a := &aiShip{ID: id, Name: name, Profession: behavior, Behavior: behavior, ShipType: sp.ID, speed: sp.Speed, rng: seed*2654435761 + 12345, home: home, homeR: homeR, level: level}
	if len(sp.Skins) > 0 {
		a.Skin = sp.Skins[int(a.rand()*float64(len(sp.Skins)))%len(sp.Skins)]
	}
	if w.radius > 0 && pos.length() > w.radius*0.96 {
		pos = pos.normalized().mul(w.radius * 0.96)
	}
	a.Pos = pos
	a.Dir = Vec{1, 0}
	a.state, a.workUntil = aiWork, w.t+1+a.rand()*4
	w.aiByID[a.ID] = a
	w.ais = append(w.ais, a)
}
