package world

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// События Авроры идут по игровому времени, даже если в системе нет игроков.
// Каждое живёт случайные 30–50 минут, следующее выбирается случайно и не повторяет предыдущее.
// Между ними пустого слота нет: сектор не замирает, но и не переключается каждую минуту.

var (
	eventMin = 30 * 60.0
	eventMax = 50 * 60.0
)

// LiveView — то, что клиент показывает в плашке. Объекты события приходят как обычные объекты мира.
type LiveView struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Text  string  `json:"text"`
	Kind  string  `json:"kind"`
	Left  float64 `json:"left"`
}

type eventDef struct {
	id, title, text string
}

// Разные картины, не один маршрут с новой подписью. Главу 1 они не подменяют:
// учебный маяк, буфер «Арго» и остальные сюжетные точки не трогаем и не выдаём за них.
var auroraEvents = []eventDef{
	{"convoy", "Конвой «Северный свет»", "Грузовик сам ходит челноком. Сопровождать его не нужно."},
	{"raid", "Налёт на коридор", "Два пирата охотятся в коридоре и возвращаются, пока окно не закроется."},
	{"bloom", "Рудный всплеск", "Железная жила вскрылась и медленно затягивается, даже если её не копать."},
	{"distress", "Чужой маяк", "Маяк бедствия гуляет широкой петлёй. Это не учебный маяк «Рубежа»."},
	{"patrol", "Патруль Конкордата", "Пара патрульных обходит большой эллипс. Это не эскорт и не задание."},
	{"debris", "Сквозной обломок", "Обломок летит через сектор и отскакивает от края. Это не буфер «Арго»."},
	{"noise", "Спираль помех", "Пятно помех раскручивается и сходится само. К записям «Арго» оно не относится."},
	{"crystal", "Кристаллический карман", "Короткий карман кристаллов. Он не заменяет железную жилу."},
	{"scrap", "Дрейф лома", "Контейнер лома ходит по кругу. Это мусор коридора, не груз экспедиции."},
	{"courier", "Курьерский бот", "Быстрый бот сам меняет четыре точки. Это не корабль «Арго» и не контракт."},
	{"rally", "Пиратский сбор", "Сбор стоит на месте, рядом один охранник. К учебному маяку он не относится."},
	{"sweep", "Досмотр сектора", "Один досмотрщик проходит мимо станции и разворачивается. Допроса нет."},
	{"meteor", "Строй обломков", "Три обломка идут строем через сектор. Это не рейс и не сигнал «Арго»."},
	{"runner", "Бегущий след", "Одиночка уходит к краю и не ищет драки. Догонять не обязательно."},
}

type liveEvent struct {
	id, title, text, kind string
	until, life           float64
	ids                   []string
	center                Vec
	angle                 float64
	dir                   Vec
	leg                   int
	participants          map[string]bool
}

func (w *World) stepEvents(dt float64) {
	if w.system.ID != "aurora" {
		return
	}
	if w.live != nil && w.t < w.live.until {
		w.tickLiveEvent(dt)
		return
	}
	w.endLiveEvent()
	def := w.pickEvent()
	w.startLiveEvent(def, w.rollLife())
	w.tickLiveEvent(dt)
}

func (w *World) rng() *rand.Rand {
	if w.eventRNG == nil {
		var b [8]byte
		seed := time.Now().UnixNano()
		if _, err := crand.Read(b[:]); err == nil {
			seed = int64(binary.LittleEndian.Uint64(b[:]))
		}
		w.eventRNG = rand.New(rand.NewSource(seed))
	}
	return w.eventRNG
}

func (w *World) rollLife() float64 {
	span := eventMax - eventMin
	if span < 0 {
		span = 0
	}
	return eventMin + w.rng().Float64()*span
}

func (w *World) pickEvent() eventDef {
	pool := make([]eventDef, 0, len(auroraEvents))
	for _, d := range auroraEvents {
		if d.id != w.lastEventID {
			pool = append(pool, d)
		}
	}
	if len(pool) == 0 {
		return auroraEvents[0]
	}
	return pool[w.rng().Intn(len(pool))]
}

func (w *World) liveView() *LiveView {
	if w.live == nil {
		return nil
	}
	left := w.live.until - w.t
	if left < 0 {
		left = 0
	}
	return &LiveView{ID: w.live.id, Title: w.live.title, Text: w.live.text, Kind: w.live.kind, Left: left}
}

func (w *World) eventCenter() Vec {
	ang := w.rng().Float64() * 2 * math.Pi
	r := w.radius * 0.42
	if r < 280 {
		r = 280
	}
	if w.radius > 0 && r > w.radius*0.62 {
		r = w.radius * 0.62
	}
	return w.awayFromStation(Vec{math.Cos(ang) * r, math.Sin(ang) * r}, 220)
}

func (w *World) awayFromStation(c Vec, min float64) Vec {
	st := Vec{w.station.Pos[0], w.station.Pos[1]}
	if dist(c, st) >= min {
		return c
	}
	away := c.sub(st)
	if away.length() < 1 {
		away = Vec{1, 0}
	}
	return st.add(away.normalized().mul(min))
}

func (w *World) clampPos(p Vec) Vec {
	lim := w.radius * 0.9
	if lim < 50 {
		lim = 50
	}
	if p.length() > lim {
		return p.normalized().mul(lim)
	}
	return p
}

func (w *World) reach() float64 {
	r := w.radius * 0.28
	if r < 180 {
		r = 180
	}
	if w.radius > 0 && r > w.radius*0.34 {
		r = w.radius * 0.34
	}
	return r
}

func (w *World) startLiveEvent(def eventDef, life float64) {
	w.endLiveEvent()
	if life < 1 {
		life = 1
	}
	ang := w.rng().Float64() * 2 * math.Pi
	ev := &liveEvent{
		id: def.id, title: def.title, text: def.text, kind: def.id,
		until: w.t + life, life: life, center: w.eventCenter(),
		dir: Vec{math.Cos(ang), math.Sin(ang)}, participants: map[string]bool{},
	}
	add := func(n *npc) {
		w.npcs = append(w.npcs, n)
		ev.ids = append(ev.ids, n.ID)
	}
	addDep := func(d *deposit) {
		w.deposits = append(w.deposits, d)
		ev.ids = append(ev.ids, d.ID)
	}
	c := ev.center
	switch def.id {
	case "convoy":
		add(&npc{ID: "dyn-convoy", Name: "Грузовик «Северный свет»", Kind: "convoy", Stance: "neutral", Pos: c, Heading: ev.dir})
	case "raid":
		cfg := worldgen.EnemyCfg{Speed: 9, ChaseSpeed: 14, Aggro: 180, Leash: 320, AttackRange: 26, Damage: 10, Period: 1.4, Reward: 20}
		for i, name := range []string{"Налётчик коридора", "Второй налётчик"} {
			pos := Vec{c.X + float64(i)*40, c.Y}
			add(&npc{
				ID: raidID(i), Name: name, Kind: "hostile", Stance: "hostile", Pos: pos, Heading: ev.dir,
				Hull: 800, HullMax: 800, Shield: 150, ShieldMax: 150, respawn: 150, ephemeral: true,
				ai: &aiState{cfg: cfg, home: c, patrolR: 70, angle: float64(i) * math.Pi},
			})
		}
	case "bloom":
		addDep(&deposit{
			ID: "dyn-bloom", Name: "Вскрытая железная жила", Zone: "dyn", Resource: "iron_ore", Rarity: "common",
			Pos: c, Amount: 72, Max: 72,
		})
	case "distress":
		add(&npc{ID: "dyn-distress", Name: "Маяк бедствия", Kind: "signal", Stance: "neutral", Pos: c, Heading: ev.dir, pt: &pointCfg{radius: 28}})
	case "patrol":
		for i, name := range []string{"Патруль Конкордата", "Второй патрульный"} {
			ang := float64(i) * math.Pi
			pos := Vec{c.X + math.Cos(ang)*80, c.Y + math.Sin(ang)*48}
			add(&npc{ID: patrolID(i), Name: name, Kind: "patrol", Stance: "friendly", Pos: pos, Heading: ev.dir})
		}
	case "debris":
		add(&npc{ID: "dyn-debris", Name: "Сквозной обломок", Kind: "derelict", Stance: "neutral", Pos: c, Heading: ev.dir, pt: &pointCfg{radius: 26}})
	case "noise":
		add(&npc{ID: "dyn-noise", Name: "Спираль помех", Kind: "anomaly", Stance: "neutral", Pos: c, pt: &pointCfg{radius: 30}})
	case "crystal":
		addDep(&deposit{
			ID: "dyn-crystal", Name: "Кристаллический карман", Zone: "dyn", Resource: "crystals", Rarity: "rare",
			Pos: c, Amount: 18, Max: 18,
		})
	case "scrap":
		addDep(&deposit{
			ID: "dyn-scrap", Name: "Дрейфующий лом", Zone: "dyn", Resource: "scrap", Rarity: "common",
			Pos: c, Amount: 40, Max: 40,
		})
	case "courier":
		add(&npc{ID: "dyn-courier", Name: "Курьерский бот", Kind: "convoy", Stance: "neutral", Pos: c, Heading: ev.dir})
	case "rally":
		add(&npc{ID: "dyn-rally", Name: "Пиратский сбор", Kind: "signal", Stance: "neutral", Pos: c, pt: &pointCfg{radius: 30}})
		cfg := worldgen.EnemyCfg{Speed: 8, ChaseSpeed: 13, Aggro: 150, Leash: 220, AttackRange: 24, Damage: 8, Period: 1.5, Reward: 20}
		add(&npc{
			ID: "dyn-rally-0", Name: "Охранник сбора", Kind: "hostile", Stance: "hostile",
			Pos: Vec{c.X + 36, c.Y}, Heading: ev.dir,
			Hull: 700, HullMax: 700, Shield: 120, ShieldMax: 120, respawn: 150, ephemeral: true,
			ai: &aiState{cfg: cfg, home: c, patrolR: 48, angle: 0.4},
		})
	case "sweep":
		st := Vec{w.station.Pos[0], w.station.Pos[1]}
		perp := Vec{-ev.dir.Y, ev.dir.X}
		add(&npc{ID: "dyn-sweep", Name: "Досмотрщик Конкордата", Kind: "patrol", Stance: "friendly", Pos: w.clampPos(st.add(ev.dir.mul(420)).add(perp.mul(200))), Heading: ev.dir})
	case "meteor":
		perp := Vec{-ev.dir.Y, ev.dir.X}
		for i, name := range []string{"Головной обломок", "Средний обломок", "Замыкающий обломок"} {
			pos := c.add(perp.mul(float64(i-1) * 48))
			add(&npc{ID: meteorID(i), Name: name, Kind: "derelict", Stance: "neutral", Pos: w.clampPos(pos), Heading: ev.dir, pt: &pointCfg{radius: 22}})
		}
	case "runner":
		cfg := worldgen.EnemyCfg{Speed: 0, ChaseSpeed: 0, Aggro: 0, Leash: 1, AttackRange: 0, Damage: 0, Period: 10, Reward: 25}
		add(&npc{
			ID: "dyn-runner", Name: "Бегущий след", Kind: "hostile", Stance: "hostile", Pos: c, Heading: ev.dir,
			Hull: 420, HullMax: 420, Shield: 80, ShieldMax: 80, respawn: 1e9, ephemeral: true,
			ai: &aiState{cfg: cfg, home: c, patrolR: 1},
		})
	}
	w.lastEventID = def.id
	w.live = ev
}

func (w *World) anyNPC(id string) *npc {
	for _, n := range w.npcs {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func raidID(i int) string {
	if i == 0 {
		return "dyn-raid-0"
	}
	return "dyn-raid-1"
}

func patrolID(i int) string {
	if i == 0 {
		return "dyn-patrol-0"
	}
	return "dyn-patrol-1"
}

func meteorID(i int) string {
	switch i {
	case 0:
		return "dyn-meteor-0"
	case 1:
		return "dyn-meteor-1"
	default:
		return "dyn-meteor-2"
	}
}

func (w *World) endLiveEvent() {
	if w.live == nil {
		return
	}
	drop := map[string]bool{}
	for _, id := range w.live.ids {
		drop[id] = true
	}
	nextN := make([]*npc, 0, len(w.npcs))
	for _, n := range w.npcs {
		if !drop[n.ID] {
			nextN = append(nextN, n)
		}
	}
	w.npcs = nextN
	nextD := make([]*deposit, 0, len(w.deposits))
	for _, d := range w.deposits {
		if !drop[d.ID] {
			nextD = append(nextD, d)
		}
	}
	w.deposits = nextD
	for _, p := range w.players {
		if drop[p.mining] {
			p.mining = ""
		}
		if drop[p.Target] {
			p.Target = ""
			p.Attacking = false
		}
	}
	if w.live != nil {
		credits := eventRewardCredits(w.live.id)
		for player := range w.live.participants {
			if credits > 0 {
				w.rewards = append(w.rewards, Reward{Player: player, Key: fmt.Sprintf("event:%s:%d", w.live.id, int64(w.live.until)), Kind: "event", Ref: w.live.id, Credits: credits})
			}
		}
	}
	w.live = nil
}

func eventRewardCredits(id string) int64 {
	switch id {
	case "raid", "rally":
		return 180
	case "bloom", "crystal", "scrap":
		return 120
	case "distress", "noise", "debris":
		return 150
	default:
		return 80
	}
}

func (w *World) tickLiveEvent(dt float64) {
	ev := w.live
	if ev == nil || dt <= 0 {
		return
	}
	switch ev.id {
	case "convoy":
		reach := w.reach()
		w.advanceLeg(ev, w.anyNPC("dyn-convoy"), []Vec{
			ev.center.add(ev.dir.mul(reach)),
			ev.center.add(ev.dir.mul(-reach)),
		}, 9, dt)
	case "raid":
		w.reviveIfDue("dyn-raid-0")
		w.reviveIfDue("dyn-raid-1")
	case "bloom":
		w.drainDeposit("dyn-bloom", 0.45, ev.life, dt)
	case "distress":
		ev.angle += dt * 0.05
		w.driftNPC("dyn-distress", ev, Vec{math.Cos(ev.angle) * 220, math.Sin(ev.angle*1.6) * 140})
	case "patrol":
		ev.angle += dt * 0.05
		for i := 0; i < 2; i++ {
			n := w.anyNPC(patrolID(i))
			if n == nil {
				continue
			}
			ang := ev.angle + float64(i)*math.Pi
			w.moveNPC(n, w.clampPos(Vec{ev.center.X + math.Cos(ang)*240, ev.center.Y + math.Sin(ang)*130}))
		}
	case "debris":
		w.coast("dyn-debris", ev, 11, dt)
	case "noise":
		ev.angle += dt
		phase := math.Mod(ev.angle, 240)
		turn := phase / 240 * 4 * math.Pi
		r := 40 + 220*(phase/240)
		w.driftNPC("dyn-noise", ev, Vec{math.Cos(turn) * r, math.Sin(turn) * r})
	case "crystal":
		w.drainDeposit("dyn-crystal", 0.2, ev.life, dt)
	case "scrap":
		ev.angle += dt * 0.012
		if d := w.depositByID("dyn-scrap"); d != nil {
			d.Pos = w.clampPos(ev.center.add(Vec{math.Cos(ev.angle) * 110, math.Sin(ev.angle) * 110}))
		}
		w.drainDeposit("dyn-scrap", 0.3, ev.life, dt)
	case "courier":
		reach := w.reach()
		pts := make([]Vec, 4)
		for i := 0; i < 4; i++ {
			a := math.Atan2(ev.dir.Y, ev.dir.X) + float64(i)*math.Pi/2
			pts[i] = ev.center.add(Vec{math.Cos(a) * reach, math.Sin(a) * reach})
		}
		w.advanceLeg(ev, w.anyNPC("dyn-courier"), pts, 22, dt)
	case "rally":
		w.reviveIfDue("dyn-rally-0")
	case "sweep":
		st := Vec{w.station.Pos[0], w.station.Pos[1]}
		perp := Vec{-ev.dir.Y, ev.dir.X}
		w.advanceLeg(ev, w.anyNPC("dyn-sweep"), []Vec{
			st.add(ev.dir.mul(460)).add(perp.mul(200)),
			st.add(ev.dir.mul(-460)).add(perp.mul(200)),
		}, 15, dt)
	case "meteor":
		w.coastFormation(ev, []string{"dyn-meteor-0", "dyn-meteor-1", "dyn-meteor-2"}, 13, dt)
	case "runner":
		n := w.anyNPC("dyn-runner")
		if n == nil || n.deadUntil > w.t {
			return
		}
		w.coast("dyn-runner", ev, 16, dt)
	}
	// Участие считается сервером по близости к центру события. Награда выдаётся один раз при завершении.
	for id, player := range w.players {
		if player != nil && dist(player.Pos, ev.center) <= 180 {
			ev.participants[id] = true
		}
	}
}

func (w *World) reviveIfDue(id string) {
	n := w.anyNPC(id)
	if n == nil || n.Kind != "hostile" {
		return
	}
	if n.deadUntil > 0 && n.deadUntil <= w.t {
		n.deadUntil = 0
		n.Hull, n.Shield = n.HullMax, n.ShieldMax
		n.resetAI()
	}
}

func (w *World) drainDeposit(id string, frac, life, dt float64) {
	d := w.depositByID(id)
	if d == nil || life <= 0 || frac <= 0 {
		return
	}
	d.Amount = math.Max(0, d.Amount-d.Max*frac/life*dt)
}

func (w *World) driftNPC(id string, ev *liveEvent, off Vec) {
	n := w.anyNPC(id)
	if n == nil {
		return
	}
	w.moveNPC(n, w.clampPos(ev.center.add(off)))
}

func (w *World) moveNPC(n *npc, next Vec) {
	if n == nil {
		return
	}
	if d := next.sub(n.Pos); d.length() > 0.01 {
		n.Heading = d.normalized()
	}
	n.Pos = next
}

func (w *World) advanceLeg(ev *liveEvent, n *npc, points []Vec, speed, dt float64) {
	if n == nil || len(points) == 0 {
		return
	}
	if ev.leg < 0 || ev.leg >= len(points) {
		ev.leg = 0
	}
	goal := w.clampPos(points[ev.leg])
	step := speed * dt
	if dist(n.Pos, goal) <= math.Max(step, 6) {
		ev.leg = (ev.leg + 1) % len(points)
		goal = w.clampPos(points[ev.leg])
	}
	nxt := moveToward(n.Pos, goal, step)
	w.moveNPC(n, nxt)
}

func (w *World) coast(id string, ev *liveEvent, speed, dt float64) {
	n := w.anyNPC(id)
	if n == nil {
		return
	}
	dir := ev.dir
	if dir.length() < 0.01 {
		dir = Vec{1, 0}
	}
	dir = dir.normalized()
	nxt := n.Pos.add(dir.mul(speed * dt))
	lim := w.radius * 0.86
	if lim < 80 {
		lim = 80
	}
	if nxt.length() > lim {
		normal := nxt.normalized()
		dot := dir.X*normal.X + dir.Y*normal.Y
		dir = Vec{dir.X - 2*dot*normal.X, dir.Y - 2*dot*normal.Y}
		if dir.length() < 0.01 {
			dir = Vec{-normal.Y, normal.X}
		}
		ev.dir = dir.normalized()
		nxt = normal.mul(lim * 0.98)
	} else {
		ev.dir = dir
	}
	w.moveNPC(n, nxt)
}

func (w *World) coastFormation(ev *liveEvent, ids []string, speed, dt float64) {
	if len(ids) == 0 {
		return
	}
	w.coast(ids[0], ev, speed, dt)
	lead := w.anyNPC(ids[0])
	if lead == nil {
		return
	}
	perp := Vec{-ev.dir.Y, ev.dir.X}
	for i := 1; i < len(ids); i++ {
		n := w.anyNPC(ids[i])
		if n == nil {
			continue
		}
		w.moveNPC(n, w.clampPos(lead.Pos.add(perp.mul(float64(i)*48))))
	}
}
