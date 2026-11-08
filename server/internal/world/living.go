package world

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// Живые локации: враги с лёгким ИИ, месторождения и точки исследования. Всё решает сервер;
// клиентский декор (астероиды, пыль, кометы) сюда не попадает и не синхронизируется.

// Ошибки добычи.
var (
	ErrNoModule = errors.New("no_module")
	ErrFull     = errors.New("hold_full")
	ErrEmpty    = errors.New("deposit_empty")
	ErrNoDep    = errors.New("no_deposit")
	ErrNothing  = errors.New("nothing_to_sell")
	ErrNoStable = errors.New("not_at_station")
	ErrNoShip   = errors.New("no_ship")
	ErrHoldBig  = errors.New("hold_too_big")
)

// Reward — серверная награда, ожидающая выдачи. Ключ уникален для события: повторная выдача не удваивает награду.
type Reward struct {
	Player  string
	Key     string
	Kind    string // kill | discover | sell
	Ref     string
	Credits int64
}

type safeZone struct {
	Pos    Vec
	Radius float64
}

type aiState struct {
	cfg      worldgen.EnemyCfg
	tier     string
	zone     string
	home     Vec
	patrolR  float64
	angle    float64
	target   string
	returned bool
	attackAt float64
}

type pointCfg struct {
	radius, reward float64
}

type deposit struct {
	ID, Name, Zone, Resource, Rarity, Module string
	Pos                                      Vec
	Amount, Max, Regen                       float64
}

func (n *npc) resetAI() {
	if n.ai != nil {
		n.Pos, n.ai.target, n.ai.returned = n.ai.home, "", false
	}
}

func copyHold(h map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for k, v := range h {
		if v > 0 {
			out[k] = v
		}
	}
	return out
}

func holdUsed(h map[string]float64) float64 {
	var s float64
	for _, v := range h {
		s += v
	}
	return s
}

// sanitizeHold отбрасывает некорректные значения и приводит груз к вместимости корабля.
func sanitizeHold(h map[string]float64, capacity float64) map[string]float64 {
	out := map[string]float64{}
	free := capacity
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := h[k]
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || free <= 0 {
			continue
		}
		v = math.Min(v, free)
		out[k] = v
		free -= v
	}
	return out
}

// ApplyLayout добавляет в мир наполнение системы: врагов, месторождения, точки исследования, станции.
// Учебные объекты (дроны, сигналы) и станция «Рубеж» сохраняются. Повторный вызов заменяет прежнее наполнение.
func (w *World) ApplyLayout(l worldgen.Layout, cfg worldgen.Config) error {
	if errs := l.Validate(); len(errs) > 0 {
		return fmt.Errorf("раскладка некорректна: %v", errs)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	keep := w.npcs[:0:0]
	for _, n := range w.npcs {
		if n.ai == nil && n.pt == nil && n.Kind != "station" {
			keep = append(keep, n)
		}
	}
	w.npcs, w.deposits, w.safe, w.layout, w.wcfg = keep, nil, nil, l, cfg
	w.radius = l.Radius
	for _, st := range l.Stations {
		w.safe = append(w.safe, safeZone{Vec{st.Pos[0], st.Pos[1]}, st.SafeRadius})
		if st.ID != StationID {
			w.npcs = append(w.npcs, &npc{ID: st.ID, Name: st.Name, Kind: "station", Stance: "friendly", Pos: Vec{st.Pos[0], st.Pos[1]}})
		}
	}
	for _, sp := range l.Spawns {
		ec, ok := cfg.Enemies[sp.Tier]
		if !ok {
			return fmt.Errorf("враг %s: нет уровня %q", sp.ID, sp.Tier)
		}
		home := Vec{sp.Pos[0], sp.Pos[1]}
		w.npcs = append(w.npcs, &npc{ID: "en-" + sp.ID, Name: ec.Name, Kind: "hostile", Stance: "hostile", Pos: home,
			Hull: ec.Hull, HullMax: ec.Hull, Shield: ec.Shield, ShieldMax: ec.Shield, respawn: ec.Respawn,
			ai: &aiState{cfg: ec, tier: sp.Tier, zone: sp.Zone, home: home, patrolR: sp.Patrol, angle: float64(len(w.npcs)) * 1.7}})
	}
	for _, pt := range l.Points {
		n := &npc{ID: pt.ID, Name: pt.Name, Kind: pt.Kind, Stance: "neutral", Pos: Vec{pt.Pos[0], pt.Pos[1]}, Hidden: pt.Hidden,
			pt: &pointCfg{radius: pt.Radius, reward: pt.Reward}}
		w.npcs = append(w.npcs, n)
	}
	for _, d := range l.Deposits {
		w.deposits = append(w.deposits, &deposit{ID: d.ID, Name: d.Name, Zone: d.Zone, Resource: d.Resource, Rarity: d.Rarity,
			Module: d.Module, Pos: Vec{d.Pos[0], d.Pos[1]}, Amount: d.Amount, Max: d.Max, Regen: d.Regen})
	}
	return nil
}

// Layout возвращает действующую раскладку системы (для клиента).
func (w *World) Layout() (worldgen.Layout, worldgen.Config) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.layout, w.wcfg
}

// DrainRewards возвращает накопленные награды и очищает очередь.
func (w *World) DrainRewards() []Reward {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.rewards
	w.rewards = nil
	return out
}

func (w *World) inSafe(p Vec) bool {
	for _, s := range w.safe {
		if dist(p, s.Pos) <= s.Radius {
			return true
		}
	}
	if dist(p, Vec{w.station.Pos[0], w.station.Pos[1]}) <= 40 {
		return true
	}
	return false
}

func (w *World) onNPCKilled(p *Player, n *npc) {
	n.kills++
	if n.ai == nil || n.ai.cfg.Reward <= 0 {
		return
	}
	w.rewards = append(w.rewards, Reward{Player: p.ID, Key: fmt.Sprintf("kill:%s:%d:%d", n.ID, n.kills, time.Now().UnixNano()),
		Kind: "kill", Ref: n.ID, Credits: int64(n.ai.cfg.Reward)})
	w.emit(Event{Type: "loot", From: n.ID, To: p.ID, Amount: n.ai.cfg.Reward})
	n.ai.target = ""
}

func (w *World) depositObjects() []Object {
	out := make([]Object, 0, len(w.deposits))
	for _, d := range w.deposits {
		a, m := math.Round(d.Amount), d.Max
		out = append(out, Object{ID: d.ID, Kind: "deposit", Stance: "neutral", Name: d.Name, Pos: [2]float64{d.Pos.X, d.Pos.Y},
			Resource: d.Resource, Rarity: d.Rarity, Amount: &a, AmountMax: &m, Zone: d.Zone})
	}
	return out
}

// stepLiving — шаг ИИ врагов, регенерации месторождений, добычи и исследования.
func (w *World) stepLiving(dt float64) {
	for _, d := range w.deposits {
		if d.Amount < d.Max {
			d.Amount = math.Min(d.Max, d.Amount+d.Regen/60*dt)
		}
	}
	for _, n := range w.npcs {
		if n.ai != nil && n.deadUntil <= w.t {
			w.stepEnemy(n, dt)
		}
	}
	ids := make([]string, 0, len(w.players))
	for id := range w.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := w.players[id]
		if p.deadUntil > w.t {
			continue
		}
		w.stepMining(p)
		w.stepDiscovery(p)
	}
}

func moveToward(from, to Vec, step float64) Vec {
	d := to.sub(from)
	l := d.length()
	if l <= step || l == 0 {
		return to
	}
	return from.add(d.mul(step / l))
}

func (w *World) stepEnemy(n *npc, dt float64) {
	a := n.ai
	cfg := a.cfg
	var tgt *Player
	if a.target != "" {
		if p, ok := w.players[a.target]; ok && p.deadUntil <= w.t && !w.inSafe(p.Pos) && dist(p.Pos, a.home) <= cfg.Leash && dist(n.Pos, a.home) <= cfg.Leash {
			tgt = p
		} else {
			a.target, a.returned = "", true
		}
	}
	if tgt == nil && !a.returned {
		best := math.MaxFloat64
		ids := make([]string, 0, len(w.players))
		for id := range w.players {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			p := w.players[id]
			d := dist(p.Pos, n.Pos)
			if p.deadUntil <= w.t && d <= cfg.Aggro && d < best && !w.inSafe(p.Pos) && dist(p.Pos, a.home) <= cfg.Leash {
				tgt, best = p, d
			}
		}
		if tgt != nil {
			a.target = tgt.ID
		}
	}
	switch {
	case tgt != nil:
		d := dist(tgt.Pos, n.Pos)
		if d > cfg.AttackRange*0.8 {
			n.Pos = moveToward(n.Pos, tgt.Pos, cfg.ChaseSpeed*dt)
		}
		if d <= cfg.AttackRange && w.t >= a.attackAt {
			a.attackAt = w.t + cfg.Period
			w.emit(Event{Type: "fire", From: n.ID, To: tgt.ID})
			w.damagePlayerLocked(tgt.ID, cfg.Damage)
		}
	case a.returned:
		n.Pos = moveToward(n.Pos, a.home, cfg.ChaseSpeed*dt)
		if dist(n.Pos, a.home) < 1 {
			a.returned = false
			n.Hull, n.Shield = n.HullMax, n.ShieldMax
		}
	default: // патруль по окружности вокруг дома
		a.angle += dt * cfg.Speed / math.Max(a.patrolR, 1)
		wp := Vec{a.home.X + math.Cos(a.angle)*a.patrolR, a.home.Y + math.Sin(a.angle)*a.patrolR}
		n.Pos = moveToward(n.Pos, wp, cfg.Speed*dt)
	}
}

func (w *World) depositByID(id string) *deposit {
	for _, d := range w.deposits {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// StartMining начинает добычу из месторождения. Нужна способность mine (модуль добычи), дальность и свободное место в грузе.
func (w *World) StartMining(id, depID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	if p.deadUntil > w.t {
		return ErrDead
	}
	cfg, ok := p.spec.Abilities["mine"]
	if !ok {
		return ErrNoModule
	}
	d := w.depositByID(depID)
	if d == nil {
		return ErrNoDep
	}
	if dist(p.Pos, d.Pos) > cfg.Range {
		return ErrRange
	}
	if d.Amount < 1 {
		return ErrEmpty
	}
	if holdUsed(p.hold) >= p.spec.Cargo {
		return ErrFull
	}
	p.mining, p.mineAt, p.mineSince = depID, w.t+cfg.Period, w.t
	w.emit(Event{Type: "mine_start", From: p.ID, To: depID})
	return nil
}

// StopMining прекращает добычу.
func (w *World) StopMining(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[id]; ok && p.mining != "" {
		p.mining = ""
		w.emit(Event{Type: "mine_stop", From: p.ID})
	}
}

func (w *World) stepMining(p *Player) {
	if p.mining == "" {
		return
	}
	cfg, ok := p.spec.Abilities["mine"]
	d := w.depositByID(p.mining)
	stop := func() { p.mining = ""; w.emit(Event{Type: "mine_stop", From: p.ID}) }
	// добыча прерывается уходом из дальности, уроном и пропажей месторождения
	if !ok || d == nil || dist(p.Pos, d.Pos) > cfg.Range*1.15 || p.lastHit > p.mineSince {
		stop()
		return
	}
	if w.t < p.mineAt {
		return
	}
	p.mineAt = w.t + cfg.Period
	free := p.spec.Cargo - holdUsed(p.hold)
	amt := math.Min(math.Min(cfg.Damage, d.Amount), free)
	if amt > 0 {
		d.Amount -= amt
		p.hold[d.Resource] += amt
		w.emit(Event{Type: "mine", From: p.ID, To: d.ID, Amount: amt})
	}
	if free-amt < 0.01 || d.Amount < 0.01 {
		stop()
	}
}

func (w *World) stepDiscovery(p *Player) {
	for _, n := range w.npcs {
		if n.pt == nil || p.discovered[n.ID] {
			continue
		}
		if n.Hidden && p.revealed[n.ID] <= w.t {
			continue
		}
		if dist(p.Pos, n.Pos) > n.pt.radius {
			continue
		}
		p.discovered[n.ID] = true
		w.emit(Event{Type: "discover", From: p.ID, To: n.ID, Amount: n.pt.reward})
		if n.pt.reward > 0 {
			w.rewards = append(w.rewards, Reward{Player: p.ID, Key: "discover:" + n.ID, Kind: "discover", Ref: n.ID, Credits: int64(n.pt.reward)})
		}
	}
}

// SellCargo продаёт весь груз на станции (в зоне станции). Цены задаёт конфигурация ресурсов.
func (w *World) SellCargo(id string) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return 0, ErrNoPlayer
	}
	if p.deadUntil > w.t {
		return 0, ErrDead
	}
	if !w.inSafe(p.Pos) {
		return 0, ErrNoStable
	}
	var total float64
	for res, amt := range p.hold {
		total += amt * w.wcfg.Resources[res].Price
	}
	if holdUsed(p.hold) <= 0 {
		return 0, ErrNothing
	}
	credits := int64(math.Round(total))
	p.hold, p.mining = map[string]float64{}, ""
	if credits > 0 {
		w.rewards = append(w.rewards, Reward{Player: p.ID, Key: fmt.Sprintf("sell:%d", time.Now().UnixNano()), Kind: "sell", Credits: credits})
	}
	w.emit(Event{Type: "sell", From: p.ID, Amount: float64(credits)})
	return credits, nil
}
