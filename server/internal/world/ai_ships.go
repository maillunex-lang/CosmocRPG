package world

import (
	"math"
	"sort"
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
		if p.connected {
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

// aiObjects возвращает ИИ-корабли в радиусе видимости от точки (под блокировкой).
func (w *World) aiObjects(from Vec) []Object {
	r := w.aiView
	if r <= 0 {
		r = AIViewRadius
	}
	var out []Object
	for _, a := range w.ais {
		if dist(a.Pos, from) > r {
			continue
		}
		out = append(out, Object{ID: a.ID, Kind: "ai", Stance: "neutral", Name: a.Name, Pos: [2]float64{a.Pos.X, a.Pos.Y},
			Dir: [2]float64{a.Dir.X, a.Dir.Y}, Ship: a.ShipType, Skin: a.Skin, AI: true})
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
	if dest.length() > w.radius {
		dest = dest.normalized().mul(w.radius * 0.95)
	}
	a.dest, a.state = dest, aiTravel
}
