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
	ErrNoModule   = errors.New("no_module")
	ErrFull       = errors.New("hold_full")
	ErrEmpty      = errors.New("deposit_empty")
	ErrNoDep      = errors.New("no_deposit")
	ErrNothing    = errors.New("nothing_to_sell")
	ErrNoStable   = errors.New("not_at_station")
	ErrHoldFull   = errors.New("hold_full")
	ErrNoResource = errors.New("no_item")
	ErrNoShip     = errors.New("no_ship")
	ErrHoldBig    = errors.New("hold_too_big")
)

// Reward — серверная награда, ожидающая выдачи. Ключ уникален для события: повторная выдача не удваивает награду.
type Reward struct {
	Player  string
	Key     string
	Kind    string // kill | discover | sell | mine
	Ref     string
	Credits int64
	Amount  float64 // mine: сколько единиц ресурса добыто
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
// Скрытые сигналы сохраняются. Повторный вызов заменяет прежнее наполнение.
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
	w.live = nil
	w.radius = l.Radius
	// Э-1: идентичность системы берётся из раскладки (нефиксированная «Кси»)
	w.system = System{ID: l.System, Name: l.Name}
	for _, sp := range cfg.Systems {
		if sp.ID == l.System {
			w.system.Background = sp.Background
			w.system.Faction = sp.Faction
			break
		}
	}
	for _, st := range l.Stations {
		w.safe = append(w.safe, safeZone{Vec{st.Pos[0], st.Pos[1]}, st.SafeRadius})
		if st.ID == StationID || st.Hub {
			w.station.ID = st.ID
			w.station.Name = st.Name
			w.station.Pos = st.Pos
			continue
		}
		w.npcs = append(w.npcs, &npc{ID: st.ID, Name: st.Name, Kind: "station", Stance: "friendly", Pos: Vec{st.Pos[0], st.Pos[1]}})
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
	w.seedLocalPilotsLocked()
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
	if w.lane != nil {
		return false
	}
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
	who := w.lootMate(p)
	w.rewards = append(w.rewards, Reward{Player: who.ID, Key: fmt.Sprintf("kill:%s:%d:%d", n.ID, n.kills, time.Now().UnixNano()),
		Kind: "kill", Ref: n.ID, Credits: int64(n.ai.cfg.Reward)})
	w.emit(Event{Type: "loot", From: n.ID, To: who.ID, Amount: n.ai.cfg.Reward})
	w.spawnWreck(n, who.ID)
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
	w.advanceLane()
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
		w.stepStations(p)
	}
	w.collectWrecks()
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
	if w.lane != nil && n.room > w.lane.open {
		return
	}
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
			w.damagePlayerLocked(tgt.ID, cfg.Damage, n.ID)
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

func (w *World) lootMate(p *Player) *Player {
	if p == nil || p.partyID == "" {
		return p
	}
	ids := make([]string, 0, 4)
	for id, o := range w.players {
		if o.partyID == p.partyID && !o.ai && !o.wing && o.deadUntil <= w.t {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return p
	}
	sort.Strings(ids)
	n := int(time.Now().UnixNano() % int64(len(ids)))
	if n < 0 {
		n = -n
	}
	if o := w.players[ids[n]]; o != nil {
		return o
	}
	return p
}

func (w *World) depositByID(id string) *deposit {
	for _, d := range w.deposits {
		if d.ID == id {
			return d
		}
	}
	return nil
}

func mineCfg(p *Player) (AbilityCfg, bool) {
	if p == nil || p.spec.Cargo <= 0 {
		return AbilityCfg{}, false
	}
	if cfg, ok := p.spec.Abilities["mine"]; ok && cfg.Range > 0 && cfg.Damage > 0 {
		return cfg, true
	}
	// Корабль без добывающего модуля всё равно собирает: реже и меньше за один захват.
	return AbilityCfg{Name: "Ручной сбор", Range: 18, Period: 2.4, Damage: 1}, true
}

// StartMining начинает добычу из месторождения. Нужна способность mine (модуль добычи), дальность и свободное место в грузе.
func (w *World) StartMining(id, depID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	if p.actionBusy() {
		return ErrEquipmentBusy
	}
	if p.deadUntil > w.t {
		return ErrDead
	}
	cfg, ok := mineCfg(p)
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
	if holdUsed(p.hold) >= p.spec.Cargo || cargoRoom(p, d.Resource) < 0.05 {
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
	if p.mining == "" || p.actionBusy() || p.cargoRevision == math.MaxInt64 {
		return
	}
	cfg, ok := mineCfg(p)
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
	free := cargoRoom(p, d.Resource)
	amt := math.Min(math.Min(cfg.Damage, d.Amount), free)
	if amt > 0 {
		d.Amount -= amt
		p.hold[d.Resource] += amt
		p.cargoRevision++
		w.emit(Event{Type: "mine", From: p.ID, To: d.ID, Amount: amt})
		w.rewards = append(w.rewards, Reward{Player: p.ID, Key: fmt.Sprintf("mine:%d", time.Now().UnixNano()),
			Kind: "mine", Ref: d.Resource, Amount: amt})
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
		w.arrived = append(w.arrived, Arrival{Player: p.ID, System: w.system.ID, ID: n.ID, Name: n.Name, Kind: n.Kind, Pos: n.Pos})
		w.emit(Event{Type: "discover", From: p.ID, To: n.ID, Amount: n.pt.reward})
		if n.pt.reward > 0 {
			w.rewards = append(w.rewards, Reward{Player: p.ID, Key: "discover:" + n.ID, Kind: "discover", Ref: n.ID, Credits: int64(n.pt.reward)})
		}
	}
}

func (w *World) stepStations(p *Player) {
	seen := map[string]bool{}
	try := func(id, name string, pos Vec) {
		if id == "" || seen[id] || p.discovered["station:"+id] {
			return
		}
		if dist(p.Pos, pos) > 40 {
			return
		}
		seen[id] = true
		p.discovered["station:"+id] = true
		w.arrived = append(w.arrived, Arrival{Player: p.ID, System: w.system.ID, ID: id, Name: name, Kind: "station", Pos: pos})
	}
	try(w.station.ID, w.station.Name, Vec{w.station.Pos[0], w.station.Pos[1]})
	for _, n := range w.npcs {
		if n.Kind == "station" {
			try(n.ID, n.Name, n.Pos)
		}
	}
}

// DrainArrivals забирает новые прибытия и очищает очередь.
func (w *World) DrainArrivals() []Arrival {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.arrived
	w.arrived = nil
	return out
}

// AtKind сообщает, стоит ли живой игрок у объекта этого вида (аномалия, обломок, сигнал, станция).
func (w *World) AtKind(id, kind string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok || p.deadUntil > w.t {
		return false
	}
	if kind == "station" && dist(p.Pos, Vec{w.station.Pos[0], w.station.Pos[1]}) <= 40 {
		return true
	}
	for _, n := range w.npcs {
		if n.deadUntil > w.t {
			continue
		}
		if n.Kind != kind && n.ID != kind {
			continue
		}
		if n.Hidden && p.revealed[n.ID] <= w.t {
			continue
		}
		reach := 40.0
		if n.pt != nil && n.pt.radius > 0 {
			reach = n.pt.radius
		}
		if dist(p.Pos, n.Pos) <= reach {
			return true
		}
	}
	return false
}

// TakeCargo списывает ресурс у станции. false — не списано, груз не изменён.
func (w *World) TakeCargo(id, resource string, amount float64) (Record, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return Record{}, false
	}
	if p.actionBusy() || p.cargoRevision == math.MaxInt64 || p.deadUntil > w.t || !w.inSafe(p.Pos) {
		return Record{}, false
	}
	if p.hold[resource]+1e-6 < amount {
		return Record{}, false
	}
	p.hold[resource] -= amount
	if p.hold[resource] < 1e-6 {
		delete(p.hold, resource)
	}
	p.cargoRevision++
	return p.record(), true
}

// GiveCargo возвращает ресурс, если списание задания не удалось сохранить. false — груз не изменён.
func (w *World) GiveCargo(id, resource string, amount float64) (Record, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok || amount <= 0 || p.actionBusy() || p.cargoRevision == math.MaxInt64 {
		return Record{}, false
	}
	if holdUsed(p.hold)+amount > p.spec.Cargo+1e-6 {
		return Record{}, false
	}
	if p.hold == nil {
		p.hold = map[string]float64{}
	}
	p.hold[resource] += amount
	p.cargoRevision++
	return p.record(), true
}
func (w *World) SellCargo(id string) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return 0, ErrNoPlayer
	}
	if p.actionBusy() {
		return 0, ErrEquipmentBusy
	}
	if p.cargoRevision == math.MaxInt64 {
		return 0, ErrEquipmentBusy
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
	p.cargoRevision++
	if credits > 0 {
		w.rewards = append(w.rewards, Reward{Player: p.ID, Key: fmt.Sprintf("sell:%d", time.Now().UnixNano()), Kind: "sell", Credits: credits})
	}
	w.emit(Event{Type: "sell", From: p.ID, Amount: float64(credits)})
	return credits, nil
}

// --- Станция: ремонт, покупка ресурсов, улучшение (Э-2, Решение №30) ---

// RepairCost — цена полного ремонта у станции: 2 кредита за недостающую единицу корпуса и щита.
func (w *World) RepairCost(id string) (int64, error) {
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
	missing := (p.spec.Hull - p.Hull) + (p.spec.Shield - p.Shield)
	if missing <= 0 {
		return 0, nil
	}
	return int64(math.Ceil(missing * 2.0)), nil
}

// Repair восстанавливает корпус и щит до максимума (после списания кредитов на уровне hub).
func (w *World) Repair(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	if !w.inSafe(p.Pos) {
		return ErrNoStable
	}
	p.Hull, p.Shield = p.spec.Hull, p.spec.Shield
	w.emit(Event{Type: "repair", From: id})
	return nil
}

// ResourcePrice возвращает цену покупки ресурса у станции (продажная × 1.5, минимум 1).
func (w *World) ResourcePrice(name string) (float64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, ok := w.wcfg.Resources[name]
	if !ok || r.Price <= 0 {
		return 0, false
	}
	return math.Max(r.Price*1.5, 1.0), true
}

// AddCargo добавляет ресурс в трюм с проверкой вместимости (покупка у станции, Э-2).
func (w *World) AddCargo(id, res string, amt float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	if p.actionBusy() {
		return ErrEquipmentBusy
	}
	if p.cargoRevision == math.MaxInt64 {
		return ErrEquipmentBusy
	}
	if p.deadUntil > w.t {
		return ErrDead
	}
	if !w.inSafe(p.Pos) {
		return ErrNoStable
	}
	if _, ok := w.wcfg.Resources[res]; !ok {
		return ErrNoResource
	}
	free := cargoRoom(p, res)
	if amt > free+0.001 {
		return ErrHoldFull
	}
	p.hold[res] += amt
	p.cargoRevision++
	return nil
}

// ShipLevelOf возвращает уровень корабля игрока (покупка улучшения, Э-2).
func (w *World) ShipLevelOf(id string) (int, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return 0, false
	}
	return p.shipLevel, true
}
