package worldgen

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sort"
	"strings"
)

func hypot(a, b float64) float64     { return math.Hypot(a, b) }
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// Правила размещения.
const (
	HubFreeRadius   = 70.0 // вокруг главной станции нет ничего, кроме неё самой: место для полётов и боёв
	MaxZoneCoverage = 0.40 // зоны и тела вместе занимают не больше этой доли площади системы
	ZoneGap         = 22.0 // зазор между несвязанными зонами
)

// Body — планета или луна.
type Body struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Kind   string     `json:"kind"`  // planet | moon
	Style  string     `json:"style"` // rocky | gas | ice
	Pos    [2]float64 `json:"pos"`
	Radius float64    `json:"radius"`
	Ring   bool       `json:"ring"`
	Parent string     `json:"parent,omitempty"`
	Color  [3]float64 `json:"color"`
}

// Station — станция (торговый объект с безопасной зоной).
type Station struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Pos        [2]float64 `json:"pos"`
	SafeRadius float64    `json:"safe_radius"`
	Hub        bool       `json:"hub"`
}

// Zone — зона активности: pve, mining, trade, explore. Зоны одной системы могут частично пересекаться.
type Zone struct {
	ID     string     `json:"id"`
	Type   string     `json:"type"`
	Name   string     `json:"name"`
	Pos    [2]float64 `json:"pos"`
	Radius float64    `json:"radius"`
	Danger int        `json:"danger"`
}

// Deposit — месторождение (серверный интерактивный объект).
type Deposit struct {
	ID       string     `json:"id"`
	Zone     string     `json:"zone"`
	Name     string     `json:"name"`
	Resource string     `json:"resource"`
	Rarity   string     `json:"rarity"`
	Module   string     `json:"module"`
	Pos      [2]float64 `json:"pos"`
	Amount   float64    `json:"amount"`
	Max      float64    `json:"max"`
	Regen    float64    `json:"regen_per_min"`
}

// Spawn — точка появления врага PvE-зоны.
type Spawn struct {
	ID     string     `json:"id"`
	Zone   string     `json:"zone"`
	Tier   string     `json:"tier"`
	Pos    [2]float64 `json:"pos"`
	Patrol float64    `json:"patrol"`
}

// Point — точка исследования.
type Point struct {
	ID     string     `json:"id"`
	Kind   string     `json:"kind"` // anomaly | signal | derelict
	Name   string     `json:"name"`
	Pos    [2]float64 `json:"pos"`
	Radius float64    `json:"radius"`
	Reward float64    `json:"reward"`
	Hidden bool       `json:"hidden"`
}

// Lane — торговый маршрут между станциями.
type Lane struct {
	A string `json:"a"`
	B string `json:"b"`
}

// DecorParams — параметры клиентского декора (сервер объекты декора не хранит и не синхронизирует).
type DecorParams struct {
	Seed    int64   `json:"seed"`
	Density float64 `json:"density"`
	Busy    bool    `json:"busy"`
}

// Layout — раскладка системы.
type Layout struct {
	System   string      `json:"system"`
	Name     string      `json:"name"`
	Seed     int64       `json:"seed"`
	Radius   float64     `json:"radius"`
	Hub      [2]float64  `json:"hub"`
	Bodies   []Body      `json:"bodies"`
	Stations []Station   `json:"stations"`
	Zones    []Zone      `json:"zones"`
	Deposits []Deposit   `json:"deposits"`
	Spawns   []Spawn     `json:"spawns"`
	Points   []Point     `json:"points"`
	Lanes    []Lane      `json:"lanes"`
	Decor    DecorParams `json:"decor"`
}

type circle struct{ x, y, r float64 }

func (c circle) overlaps(o circle, gap float64) bool { return hypot(c.x-o.x, c.y-o.y) < c.r+o.r+gap }

type gen struct {
	r        *rand.Rand
	p        SystemParams
	blockers []circle // занятые области (кроме разрешённых пересечений)
	hub      circle
}

func (g *gen) free(c circle, gap float64) bool {
	if hypot(c.x, c.y)+c.r > g.p.Radius*0.96 {
		return false
	}
	if hypot(c.x-g.hub.x, c.y-g.hub.y) < HubFreeRadius+c.r {
		return false
	}
	for _, b := range g.blockers {
		if c.overlaps(b, gap) {
			return false
		}
	}
	return true
}

// place ищет свободное место в кольце [lo, hi] долей радиуса системы.
func (g *gen) place(radius, lo, hi, gap float64) (circle, bool) {
	for i := 0; i < 600; i++ {
		a := g.r.Float64() * 2 * math.Pi
		d := (lo + g.r.Float64()*(hi-lo)) * g.p.Radius
		c := circle{math.Cos(a) * d, math.Sin(a) * d, radius}
		if g.free(c, gap) {
			return c, true
		}
	}
	return circle{}, false
}

// placeNear ищет место вплотную к другому кругу с заданным перекрытием (0..1 от меньшего радиуса).
func (g *gen) placeNear(o circle, radius, overlap float64, ignore int) (circle, bool) {
	for i := 0; i < 600; i++ {
		a := g.r.Float64() * 2 * math.Pi
		d := (o.r + radius) * (1 - overlap*0.5)
		c := circle{o.x + math.Cos(a)*d, o.y + math.Sin(a)*d, radius}
		if hypot(c.x, c.y)+c.r > g.p.Radius*0.96 || hypot(c.x-g.hub.x, c.y-g.hub.y) < HubFreeRadius+c.r {
			continue
		}
		ok := true
		for j, b := range g.blockers {
			if j != ignore && c.overlaps(b, ZoneGap) {
				ok = false
			}
		}
		if ok {
			return c, true
		}
	}
	return circle{}, false
}

func (g *gen) rnd(lo, hi float64) float64 { return lo + g.r.Float64()*(hi-lo) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func pos(c circle) [2]float64  { return [2]float64{round1(c.x), round1(c.y)} }

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

var beltNames = []string{"Пояс", "Каменное кольцо", "Рой Ферро", "Поле Дюрана"}
var pirateNames = []string{"Пиратская территория", "Логово «Чёрный ворон»", "Владения рейдеров", "Заслон «Шакал»"}
var anomalyNames = []string{"Гравитационная аномалия", "Пульсирующий разлом", "Дрейфующее эхо", "Тихая воронка"}

func (g *gen) pick(list []string) string { return list[g.r.Intn(len(list))] }

var planetNames = []string{"I", "II", "III", "IV", "V"}

// Диапазоны расстояний от центра системы, ед. мира: ближние зоны рядом с главной станцией, дальние — на краю карты.
// Границы обрезаются по размеру системы, поэтому одна и та же логика работает и для малых, и для больших систем.
type band struct{ lo, hi float64 }

var (
	bandNear = band{150, 330}
	bandMid  = band{300, 650}
	bandFar  = band{550, 1100}
)

// placeAbs ищет свободное место на расстоянии [lo, hi] от центра системы.
func (g *gen) placeAbs(radius float64, b band, gap float64) (circle, bool) {
	hi := math.Min(b.hi, g.p.Radius*0.9)
	lo := math.Min(b.lo, hi*0.85)
	return g.place(radius, lo/g.p.Radius, hi/g.p.Radius, gap)
}

// count возвращает число объектов: явное значение параметра, иначе 1 при наличии особенности и 0 без неё.
func count(explicit int, feature bool) int {
	if explicit > 0 {
		return explicit
	}
	if feature {
		return 1
	}
	return 0
}

func (g *gen) bandFor(i int, firstNear bool) band {
	switch {
	case i == 0 && firstNear:
		return bandNear
	case i <= 1:
		return bandMid
	}
	return bandFar
}

// Generate строит раскладку системы детерминированно по seed и параметрам: один и тот же вход даёт один и тот же результат.
func Generate(p SystemParams, cfg Config) Layout {
	g := &gen{r: rand.New(rand.NewSource(p.Seed)), p: p}
	g.hub = circle{p.Hub[0], p.Hub[1], 12}
	l := Layout{System: p.ID, Name: p.Name, Seed: p.Seed, Radius: p.Radius, Hub: p.Hub,
		Bodies: []Body{}, Stations: []Station{}, Zones: []Zone{}, Deposits: []Deposit{}, Spawns: []Spawn{}, Points: []Point{}, Lanes: []Lane{}}
	l.Stations = append(l.Stations, Station{ID: "st1", Name: "Станция «Рубеж»", Pos: p.Hub, SafeRadius: 40, Hub: true})
	l.Zones = append(l.Zones, Zone{ID: "z-trade-1", Type: "trade", Name: "Торговый узел «Рубеж»", Pos: p.Hub, Radius: 55, Danger: 0})
	dens := map[string]float64{"mining": 1.3, "pirate": 1.0, "trade": 0.8, "explore": 1.1, "mixed": 1.0}[p.Specialization]
	trade := has(p.Features, "busy_lanes") || p.Specialization == "trade"
	l.Decor = DecorParams{Seed: p.Seed*31 + 7, Density: dens, Busy: trade}
	gas := has(p.Features, "gas_giant")

	// планеты: первая крупная (газовый гигант, если задан), остальные меньше; луны у первой
	nPlanets := count(p.Planets, true)
	for i := 0; i < nPlanets; i++ {
		b := bandMid
		if i > 0 {
			b = bandFar
		}
		pr := g.rnd(34, 52)
		if i > 0 {
			pr = g.rnd(24, 40)
		}
		pc, ok := g.placeAbs(pr, band{b.lo * 0.6, b.hi}, ZoneGap)
		if !ok {
			continue
		}
		style := []string{"rocky", "ice"}[g.r.Intn(2)]
		isGas := gas && i == 0
		if isGas {
			style = "gas"
		}
		col := [3]float64{0.35 + g.r.Float64()*0.3, 0.45 + g.r.Float64()*0.3, 0.7 + g.r.Float64()*0.3}
		id := fmt.Sprintf("pl-%d", i+1)
		name := p.Name + " " + planetNames[i%len(planetNames)]
		l.Bodies = append(l.Bodies, Body{ID: id, Name: name, Kind: "planet", Style: style, Pos: pos(pc), Radius: round1(pc.r), Ring: isGas || g.r.Intn(3) == 0, Color: col})
		g.blockers = append(g.blockers, circle{pc.x, pc.y, pc.r + 6})
		for m := 0; m < g.r.Intn(3); m++ {
			a := g.r.Float64() * 2 * math.Pi
			d := pc.r + g.rnd(18, 34)
			mr := g.rnd(5, 9)
			mc := circle{pc.x + math.Cos(a)*d, pc.y + math.Sin(a)*d, mr}
			if hypot(mc.x, mc.y)+mr < p.Radius*0.96 && hypot(mc.x-g.hub.x, mc.y-g.hub.y) > HubFreeRadius+mr {
				l.Bodies = append(l.Bodies, Body{ID: fmt.Sprintf("mn-%d-%d", i+1, m+1), Name: fmt.Sprintf("%s%s", name, string(rune('а'+m))), Kind: "moon", Style: "rocky", Pos: pos(mc), Radius: round1(mr), Parent: id, Color: [3]float64{0.6, 0.6, 0.62}})
				g.blockers = append(g.blockers, circle{mc.x, mc.y, mr + 3})
			}
		}
		if isGas { // газовые залежи у газового гиганта
			gr := math.Min(pc.r+52, p.Radius*0.96-hypot(pc.x, pc.y))
			if gr >= pc.r+30 {
				zc := circle{pc.x, pc.y, gr}
				l.Zones = append(l.Zones, Zone{ID: "z-gas-1", Type: "mining", Name: "Газовое облако", Pos: pos(zc), Radius: round1(zc.r), Danger: 1})
				g.blockers = append(g.blockers, circle{zc.x, zc.y, zc.r})
				res := cfg.Resources["gas"]
				for k := 0; k < 3; k++ {
					a := float64(k)*2*math.Pi/3 + g.rnd(0, 0.8)
					d := pc.r + g.rnd(24, math.Min(40, gr-pc.r-2))
					l.Deposits = append(l.Deposits, mkDeposit(l, "gas", res, "z-gas-1", "Газовая залежь", circle{pc.x + math.Cos(a)*d, pc.y + math.Sin(a)*d, 0}, g.rnd(0.8, 1.2)))
				}
			}
		}
	}

	// астероидные пояса: зоны добычи (первый рядом с хабом)
	var belts []circle
	for i := 0; i < count(p.Belts, has(p.Features, "asteroid_belt") || p.Specialization == "mining"); i++ {
		bc, ok := g.placeAbs(g.rnd(55, 75), g.bandFor(i, true), ZoneGap)
		if !ok {
			continue
		}
		belts = append(belts, bc)
		id := fmt.Sprintf("z-belt-%d", len(belts))
		l.Zones = append(l.Zones, Zone{ID: id, Type: "mining", Name: g.pick(beltNames) + " " + p.Name + " " + planetNames[(len(belts)-1)%len(planetNames)], Pos: pos(bc), Radius: round1(bc.r), Danger: 1 + min(i, 1)})
		g.blockers = append(g.blockers, circle{bc.x, bc.y, bc.r})
	}

	// пиратские территории: первая может частично перекрывать пояс
	type pz struct {
		c    circle
		id   string
		diff int
	}
	var pirates []pz
	for i := 0; i < count(p.PirateZones, has(p.Features, "pirate_territory") || p.Specialization == "pirate"); i++ {
		pr := g.rnd(88, 115)
		var pc circle
		ok := false
		if i == 0 && len(belts) > 0 && g.r.Intn(100) < 65 {
			idx := len(g.blockers) - 1
			for j := range g.blockers { // последний блокер пояса: найдём его по координатам
				if g.blockers[j].x == belts[0].x && g.blockers[j].y == belts[0].y {
					idx = j
				}
			}
			saved := g.blockers[idx]
			g.blockers[idx] = circle{-1e9, -1e9, 0}
			pc, ok = g.placeNear(belts[0], pr, 0.25, idx)
			g.blockers[idx] = saved
			if ok && hypot(pc.x-g.hub.x, pc.y-g.hub.y) < math.Min(0.5*p.Radius, 260) {
				ok = false
			}
		}
		if !ok {
			pc, ok = g.placeAbs(pr, g.bandFor(i+1, false), ZoneGap)
		}
		if !ok {
			continue
		}
		diff := min(p.Difficulty+i/1, 3)
		id := fmt.Sprintf("z-pve-%d", len(pirates)+1)
		pirates = append(pirates, pz{pc, id, diff})
		l.Zones = append(l.Zones, Zone{ID: id, Type: "pve", Name: g.pick(pirateNames), Pos: pos(pc), Radius: round1(pc.r), Danger: diff})
		g.blockers = append(g.blockers, circle{pc.x, pc.y, pc.r})
	}

	// месторождения поясов (состав разнообразен, дальние пояса богаче)
	for bi, belt := range belts {
		id := fmt.Sprintf("z-belt-%d", bi+1)
		mixes := [][]string{
			{"iron_ore", "iron_ore", "scrap", "crystals", "iron_ore"},
			{"iron_ore", "scrap", "scrap", "iron_ore"},
			{"iron_ore", "crystals", "iron_ore", "gas", "scrap"},
		}
		mix := mixes[g.r.Intn(len(mixes))]
		if bi > 0 {
			mix = append(mix, "crystals", "crystals")
		}
		for _, rid := range mix {
			a := g.rnd(0, 2*math.Pi)
			d := g.rnd(0.15, 0.75) * belt.r
			k := g.rnd(0.8, 1.2) * (1 + 0.25*float64(min(bi, 2)))
			l.Deposits = append(l.Deposits, mkDeposit(l, rid, cfg.Resources[rid], id, "Месторождение", circle{belt.x + math.Cos(a)*d, belt.y + math.Sin(a)*d, 0}, k))
		}
	}
	// враги пиратских территорий (лёгкая серверная логика, без ИИ-капитанов)
	for pi, pir := range pirates {
		comp := []string{"normal", "normal", "normal"}
		if pir.diff >= 2 {
			comp = append(comp, "reinforced", "elite")
		}
		if pir.diff >= 3 {
			comp = append(comp, "miniboss")
		}
		for _, tier := range comp {
			a := g.rnd(0, 2*math.Pi)
			d := g.rnd(0.1, 0.6) * pir.c.r
			l.Spawns = append(l.Spawns, Spawn{ID: fmt.Sprintf("sp-%d", len(l.Spawns)+1), Zone: pir.id, Tier: tier,
				Pos: pos(circle{pir.c.x + math.Cos(a)*d, pir.c.y + math.Sin(a)*d, 0}), Patrol: round1(g.rnd(14, 26))})
		}
		// богатое месторождение на пересечении пояса и пиратской территории (риск нападения при добыче)
		if pi == 0 && len(belts) > 0 && hypot(pir.c.x-belts[0].x, pir.c.y-belts[0].y) < pir.c.r+belts[0].r {
			mx, my := (pir.c.x+belts[0].x)/2, (pir.c.y+belts[0].y)/2
			a := g.rnd(0, 2*math.Pi)
			c := circle{mx + math.Cos(a)*8, my + math.Sin(a)*8, 0}
			l.Deposits = append(l.Deposits, mkDeposit(l, "crystals", cfg.Resources["crystals"], pir.id, "Богатое месторождение (под охраной)", c, 1.6))
		}
	}
	// логово босса: далеко от хаба, мировой босс и охрана
	if has(p.Features, "boss_lair") {
		if bc, ok := g.placeAbs(g.rnd(70, 90), bandFar, ZoneGap); ok {
			id := "z-pve-boss"
			l.Zones = append(l.Zones, Zone{ID: id, Type: "pve", Name: "Логово «Левиафан»", Pos: pos(bc), Radius: round1(bc.r), Danger: 3})
			g.blockers = append(g.blockers, circle{bc.x, bc.y, bc.r})
			for k, tier := range []string{"worldboss", "elite", "elite"} {
				a := g.rnd(0, 2*math.Pi)
				d := 0.0
				if k > 0 {
					d = g.rnd(0.3, 0.6) * bc.r
				}
				l.Spawns = append(l.Spawns, Spawn{ID: fmt.Sprintf("sp-%d", len(l.Spawns)+1), Zone: id, Tier: tier, Pos: pos(circle{bc.x + math.Cos(a)*d, bc.y + math.Sin(a)*d, 0}), Patrol: round1(g.rnd(10, 18))})
			}
			l.Deposits = append(l.Deposits, mkDeposit(l, "crystals", cfg.Resources["crystals"], id, "Сокровищница логова (под охраной)", circle{bc.x + 10, bc.y, 0}, 2.2))
		}
	}
	// поля обломков: зоны лома без пиратов
	for i := 0; i < count(p.DebrisFields, has(p.Features, "debris_field")); i++ {
		if dc, ok := g.placeAbs(g.rnd(35, 50), g.bandFor(i, true), ZoneGap); ok {
			id := fmt.Sprintf("z-debris-%d", i+1)
			l.Zones = append(l.Zones, Zone{ID: id, Type: "mining", Name: "Поле обломков", Pos: pos(dc), Radius: round1(dc.r), Danger: 0})
			g.blockers = append(g.blockers, circle{dc.x, dc.y, dc.r})
			for k := 0; k < 2+g.r.Intn(2); k++ {
				a, d := g.rnd(0, 2*math.Pi), g.rnd(0.1, 0.7)*dc.r
				l.Deposits = append(l.Deposits, mkDeposit(l, "scrap", cfg.Resources["scrap"], id, "Груда лома", circle{dc.x + math.Cos(a)*d, dc.y + math.Sin(a)*d, 0}, g.rnd(0.6, 1.0)))
			}
		}
	}
	// заброшенные станции: точки исследования
	for i := 0; i < count(p.Derelicts, has(p.Features, "derelict")); i++ {
		if dc, ok := g.placeAbs(26, g.bandFor(i+1, false), ZoneGap); ok {
			n := i + 1
			l.Zones = append(l.Zones, Zone{ID: fmt.Sprintf("z-explore-derelict-%d", n), Type: "explore", Name: "Заброшенная станция", Pos: pos(dc), Radius: round1(dc.r), Danger: 1})
			l.Points = append(l.Points, Point{ID: fmt.Sprintf("dr-%d", n), Kind: "derelict", Name: "Заброшенная станция", Pos: pos(circle{dc.x, dc.y, 0}), Radius: 16, Reward: 220 + 80*float64(i)})
			g.blockers = append(g.blockers, circle{dc.x, dc.y, dc.r})
		}
	}
	// охраняемые тайники: малые PvE-зоны с элитным стражем
	for i := 0; i < count(p.Caches, has(p.Features, "guarded_cache")); i++ {
		if cc, ok := g.placeAbs(30, g.bandFor(i+1, false), ZoneGap); ok {
			id := fmt.Sprintf("z-pve-cache-%d", i+1)
			l.Zones = append(l.Zones, Zone{ID: id, Type: "pve", Name: "Охраняемый тайник", Pos: pos(cc), Radius: round1(cc.r), Danger: min(p.Difficulty+1, 3)})
			l.Spawns = append(l.Spawns, Spawn{ID: fmt.Sprintf("sp-%d", len(l.Spawns)+1), Zone: id, Tier: "elite", Pos: pos(circle{cc.x, cc.y, 0}), Patrol: 12})
			l.Deposits = append(l.Deposits, mkDeposit(l, "crystals", cfg.Resources["crystals"], id, "Тайник (под охраной)", circle{cc.x + 6, cc.y, 0}, 1.4))
			g.blockers = append(g.blockers, circle{cc.x, cc.y, cc.r})
		}
	}
	// торговля: дополнительные станции и маршруты
	nStations := count(p.Stations, trade)
	prev := "st1"
	for i := 0; i < nStations; i++ {
		b := bandMid
		if i > 0 {
			b = bandFar
		}
		if sc, ok := g.placeAbs(46, b, ZoneGap); ok {
			id := fmt.Sprintf("st%d", i+2)
			names := []string{"Торговая станция «Дальний порт»", "Станция «Каменный причал»", "Форпост «Северный ветер»", "Станция «Янтарная гавань»"}
			l.Stations = append(l.Stations, Station{ID: id, Name: names[i%len(names)], Pos: pos(sc), SafeRadius: 35})
			l.Zones = append(l.Zones, Zone{ID: fmt.Sprintf("z-trade-%d", i+2), Type: "trade", Name: "Торговый узел " + names[i%len(names)][strings.Index(names[i%len(names)], " ")+1:], Pos: pos(sc), Radius: 45, Danger: 0})
			g.blockers = append(g.blockers, circle{sc.x, sc.y, 45})
			l.Lanes = append(l.Lanes, Lane{A: prev, B: id})
			prev = id
		}
	}
	// исследование: аномалии и скрытые сигналы далеко от центра
	nAn := count(p.Anomalies, has(p.Features, "anomalies") || p.Specialization == "explore")
	if p.Specialization == "explore" && p.Anomalies == 0 {
		nAn = 2
	}
	for i := 0; i < nAn; i++ {
		if ac, ok := g.placeAbs(30, g.bandFor(i+1, false), ZoneGap); ok {
			id := fmt.Sprintf("an-%d", i+1)
			l.Zones = append(l.Zones, Zone{ID: "z-explore-" + id, Type: "explore", Name: g.pick(anomalyNames), Pos: pos(ac), Radius: round1(ac.r), Danger: 1})
			l.Points = append(l.Points, Point{ID: id, Kind: "anomaly", Name: "Гравитационная аномалия", Pos: pos(circle{ac.x, ac.y, 0}), Radius: 18, Reward: 150 + 40*float64(i)})
			g.blockers = append(g.blockers, circle{ac.x, ac.y, ac.r})
		}
	}
	if nAn > 0 {
		for i := 0; i < 1+nAn/2; i++ {
			if sc, ok := g.placeAbs(14, band{400, 1100}, ZoneGap); ok {
				l.Points = append(l.Points, Point{ID: fmt.Sprintf("sg-%d", i+1), Kind: "signal", Name: "Секретный сигнал", Pos: pos(sc), Radius: 14, Reward: 120 + 30*float64(i), Hidden: true})
				g.blockers = append(g.blockers, circle{sc.x, sc.y, 20})
			}
		}
	}
	return l
}

func mkDeposit(l Layout, rid string, res ResourceCfg, zone, name string, c circle, k float64) Deposit {
	amount := math.Round(res.Amount * k)
	return Deposit{ID: fmt.Sprintf("dep-%d", len(l.Deposits)+1), Zone: zone, Name: name + " (" + res.Name + ")", Resource: rid, Rarity: res.Rarity, Module: res.Module,
		Pos: pos(c), Amount: amount, Max: amount, Regen: res.RegenPerMin}
}

// Validate проверяет раскладку по правилам размещения. Пустой список — раскладка корректна.
func (l Layout) Validate() []string {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	zones := map[string]Zone{}
	var area float64
	for _, z := range l.Zones {
		zones[z.ID] = z
	}
	in := func(x, y, r float64) bool { return hypot(x, y)+r <= l.Radius*0.97 }
	for _, z := range l.Zones {
		if !in(z.Pos[0], z.Pos[1], z.Radius) {
			add("зона %s выходит за границы системы", z.ID)
		}
		if z.Type == "trade" {
			continue
		}
		area += math.Pi * z.Radius * z.Radius
		if hypot(z.Pos[0]-l.Hub[0], z.Pos[1]-l.Hub[1]) < HubFreeRadius+z.Radius*0.0 && z.Type != "trade" {
			add("зона %s закрывает пространство вокруг главной станции", z.ID)
		}
	}
	for _, b := range l.Bodies {
		if !in(b.Pos[0], b.Pos[1], b.Radius) {
			add("тело %s выходит за границы системы", b.ID)
		}
		if hypot(b.Pos[0]-l.Hub[0], b.Pos[1]-l.Hub[1]) < HubFreeRadius+b.Radius {
			add("тело %s слишком близко к главной станции", b.ID)
		}
		if b.Kind == "planet" {
			area += math.Pi * b.Radius * b.Radius
		}
	}
	if cov := area / (math.Pi * l.Radius * l.Radius); cov > MaxZoneCoverage {
		add("зоны и тела занимают %.0f%% площади (максимум %.0f%%)", cov*100, MaxZoneCoverage*100)
	}
	// пересечения: разрешено только pve с mining
	for i := range l.Zones {
		for j := i + 1; j < len(l.Zones); j++ {
			a, b := l.Zones[i], l.Zones[j]
			if a.Type == "trade" || b.Type == "trade" {
				continue
			}
			if hypot(a.Pos[0]-b.Pos[0], a.Pos[1]-b.Pos[1]) < a.Radius+b.Radius {
				ok := (a.Type == "pve" && b.Type == "mining") || (a.Type == "mining" && b.Type == "pve")
				if !ok {
					add("зоны %s и %s пересекаются недопустимо", a.ID, b.ID)
				}
			}
		}
	}
	for _, d := range l.Deposits {
		z, ok := zones[d.Zone]
		if !ok {
			add("месторождение %s: нет зоны %s", d.ID, d.Zone)
			continue
		}
		if hypot(d.Pos[0]-z.Pos[0], d.Pos[1]-z.Pos[1]) > z.Radius*1.1 {
			add("месторождение %s вне своей зоны", d.ID)
		}
		if hypot(d.Pos[0]-l.Hub[0], d.Pos[1]-l.Hub[1]) < HubFreeRadius {
			add("месторождение %s рядом с главной станцией", d.ID)
		}
		if d.Amount <= 0 || d.Max < d.Amount {
			add("месторождение %s: некорректное количество", d.ID)
		}
	}
	for _, s := range l.Spawns {
		z, ok := zones[s.Zone]
		if !ok || z.Type != "pve" {
			add("враг %s: нет PvE-зоны %s", s.ID, s.Zone)
			continue
		}
		if hypot(s.Pos[0]-z.Pos[0], s.Pos[1]-z.Pos[1]) > z.Radius {
			add("враг %s вне своей зоны", s.ID)
		}
		if hypot(s.Pos[0]-l.Hub[0], s.Pos[1]-l.Hub[1]) < 40+HubFreeRadius {
			add("враг %s рядом со станцией", s.ID)
		}
	}
	for _, p := range l.Points {
		if !in(p.Pos[0], p.Pos[1], p.Radius) || hypot(p.Pos[0]-l.Hub[0], p.Pos[1]-l.Hub[1]) < HubFreeRadius {
			add("точка %s размещена неверно", p.ID)
		}
	}
	for _, st := range l.Stations {
		if !in(st.Pos[0], st.Pos[1], st.SafeRadius) {
			add("станция %s выходит за границы", st.ID)
		}
	}
	ids := map[string]bool{}
	check := func(id string) {
		if ids[id] {
			add("повторяющийся идентификатор %s", id)
		}
		ids[id] = true
	}
	for _, x := range l.Stations {
		check(x.ID)
	}
	for _, x := range l.Zones {
		check(x.ID)
	}
	for _, x := range l.Deposits {
		check(x.ID)
	}
	for _, x := range l.Spawns {
		check(x.ID)
	}
	for _, x := range l.Points {
		check(x.ID)
	}
	for _, x := range l.Bodies {
		check(x.ID)
	}
	sort.Strings(errs)
	return errs
}
