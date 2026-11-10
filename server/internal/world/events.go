package world

import (
	"math"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// События Авроры идут по игровому времени, даже если в системе нет игроков.
// Один слот — одно событие. Следующий слот убирает прежнее и ставит новое.

const eventSpan = 75.0

// LiveView — то, что клиент показывает в плашке. Объекты события приходят как обычные объекты мира.
type LiveView struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Text  string  `json:"text"`
	Kind  string  `json:"kind"`
	Left  float64 `json:"left"`
}

type eventDef struct {
	id, title, text, kind string
}

// Семь разных картин. Суть главы 1 они не подменяют: буфер «Арго» и учебный маяк не трогаем.
var auroraEvents = []eventDef{
	{"convoy", "Конвой «Северный свет»", "Грузовик сам идёт к воротам Меридиана. Его не надо сопровождать.", "convoy"},
	{"raid", "Налёт на коридор", "Два пирата вышли поохотиться. Если их не трогать, они уйдут сами.", "raid"},
	{"bloom", "Рудный всплеск", "Железная жила вскрылась и снова затягивается, даже если её никто не копает.", "bloom"},
	{"distress", "Сигнал бедствия", "Короткий маяк дрейфует и погаснет по таймеру. Это не учебный маяк.", "distress"},
	{"patrol", "Патруль Конкордата", "Пара патрульных обходит сектор. Они не квест и не эскорт.", "patrol"},
	{"debris", "Дрейф обломков", "Поле обломков смещается. Это не буфер рейса «Арго».", "debris"},
	{"noise", "Всплеск помех", "Пятно помех гуляет по сектору и расходится само.", "noise"},
}

type liveEvent struct {
	slot       int
	id, title  string
	text, kind string
	until      float64
	ids        []string
	center     Vec
	angle      float64
}

func (w *World) stepEvents(dt float64) {
	if w.system.ID != "aurora" {
		return
	}
	slot := int(math.Floor(w.t / eventSpan))
	if slot < 0 {
		slot = 0
	}
	if w.live == nil || w.live.slot != slot {
		w.endLiveEvent()
		w.startLiveEvent(auroraEvents[slot%len(auroraEvents)], slot)
	}
	w.tickLiveEvent(dt)
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

func (w *World) eventCenter(slot int) Vec {
	ang := float64(slot%len(auroraEvents)) * (2 * math.Pi / float64(len(auroraEvents)))
	r := 520.0
	if w.radius > 0 && r > w.radius*0.72 {
		r = w.radius * 0.72
	}
	c := Vec{math.Cos(ang) * r, math.Sin(ang) * r}
	st := Vec{w.station.Pos[0], w.station.Pos[1]}
	if dist(c, st) < 180 {
		away := c.sub(st)
		if away.length() < 1 {
			away = Vec{1, 0}
		}
		c = st.add(away.normalized().mul(180))
	}
	return c
}

func (w *World) startLiveEvent(def eventDef, slot int) {
	c := w.eventCenter(slot)
	ev := &liveEvent{
		slot: slot, id: def.id, title: def.title, text: def.text, kind: def.kind,
		until: float64(slot+1) * eventSpan, center: c,
	}
	add := func(n *npc) {
		w.npcs = append(w.npcs, n)
		ev.ids = append(ev.ids, n.ID)
	}
	switch def.kind {
	case "convoy":
		add(&npc{ID: "dyn-convoy", Name: "Грузовик «Северный свет»", Kind: "convoy", Stance: "neutral", Pos: c, Heading: Vec{1, 0}})
	case "raid":
		cfg := worldgen.EnemyCfg{Speed: 10, ChaseSpeed: 14, Aggro: 160, Leash: 280, AttackRange: 26, Damage: 10, Period: 1.4, Reward: 35}
		for i, name := range []string{"Налётчик коридора", "Второй налётчик"} {
			pos := Vec{c.X + float64(i)*36, c.Y}
			add(&npc{
				ID: raidID(i), Name: name, Kind: "hostile", Stance: "hostile", Pos: pos,
				Hull: 800, HullMax: 800, Shield: 150, ShieldMax: 150, respawn: 1e9, ephemeral: true,
				ai: &aiState{cfg: cfg, home: c, patrolR: 42, angle: float64(i) * math.Pi},
			})
		}
	case "bloom":
		w.deposits = append(w.deposits, &deposit{
			ID: "dyn-bloom", Name: "Вскрытая железная жила", Zone: "dyn", Resource: "iron_ore", Rarity: "common",
			Pos: c, Amount: 48, Max: 48,
		})
		ev.ids = append(ev.ids, "dyn-bloom")
	case "distress":
		add(&npc{ID: "dyn-distress", Name: "Маяк бедствия", Kind: "signal", Stance: "neutral", Pos: c, pt: &pointCfg{radius: 28}})
	case "patrol":
		for i, name := range []string{"Патруль Конкордата", "Второй патрульный"} {
			ang := float64(i) * math.Pi
			pos := Vec{c.X + math.Cos(ang)*46, c.Y + math.Sin(ang)*46}
			add(&npc{ID: patrolID(i), Name: name, Kind: "patrol", Stance: "friendly", Pos: pos})
		}
	case "debris":
		add(&npc{ID: "dyn-debris", Name: "Дрейфующие обломки", Kind: "derelict", Stance: "neutral", Pos: c, pt: &pointCfg{radius: 26}})
	case "noise":
		add(&npc{ID: "dyn-noise", Name: "Блуждающие помехи", Kind: "anomaly", Stance: "neutral", Pos: c, pt: &pointCfg{radius: 30}})
	}
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
	w.live = nil
}

func (w *World) tickLiveEvent(dt float64) {
	ev := w.live
	if ev == nil || dt <= 0 {
		return
	}
	switch ev.kind {
	case "convoy":
		n := w.anyNPC("dyn-convoy")
		if n == nil {
			return
		}
		goal := Vec{w.radius * 0.78, w.radius * 0.12}
		if goal.length() < 1 {
			goal = Vec{900, 120}
		}
		step := 22 * dt
		dir := goal.sub(n.Pos)
		if dir.length() > 1 {
			n.Heading = dir.normalized()
			n.Pos = moveToward(n.Pos, goal, step)
		}
	case "bloom":
		d := w.depositByID("dyn-bloom")
		if d == nil {
			return
		}
		d.Amount = math.Max(0, d.Amount-0.35*dt)
	case "patrol":
		ev.angle += dt * 0.35
		for i := 0; i < 2; i++ {
			n := w.anyNPC(patrolID(i))
			if n == nil {
				continue
			}
			ang := ev.angle + float64(i)*math.Pi
			next := Vec{ev.center.X + math.Cos(ang)*46, ev.center.Y + math.Sin(ang)*46}
			if d := next.sub(n.Pos); d.length() > 0.01 {
				n.Heading = d.normalized()
			}
			n.Pos = next
		}
	case "distress", "debris", "noise":
		id := map[string]string{"distress": "dyn-distress", "debris": "dyn-debris", "noise": "dyn-noise"}[ev.kind]
		n := w.anyNPC(id)
		if n == nil {
			return
		}
		ev.angle += dt * 0.15
		drift := Vec{math.Cos(ev.angle) * 18, math.Sin(ev.angle*0.7) * 14}
		n.Pos = ev.center.add(drift)
		if drift.length() > 0.01 {
			n.Heading = drift.normalized()
		}
	}
}
