package world

import (
	"errors"
	"math"
	"strings"
)

// Ошибки применения способностей: коды совпадают с кодами сообщений протокола.
var (
	ErrNoPlayer  = errors.New("игрока нет")
	ErrUnknown   = errors.New("unknown_ability")
	ErrCooldown  = errors.New("cooldown")
	ErrEnergy    = errors.New("no_energy")
	ErrNoTarget  = errors.New("no_target")
	ErrRange     = errors.New("out_of_range")
	ErrWrongSpec = errors.New("wrong_spec")
	ErrDead      = errors.New("dead")
	ErrBadSkin   = errors.New("bad_skin")
	ErrBadSpec   = errors.New("bad_spec")
	ErrBadTarget = errors.New("bad_target")
	ErrBadOption = errors.New("bad_option")
)

func (w *World) initShip(p *Player, shipType, skin, spec string) {
	sp, ok := w.catalog.Spec(shipType)
	if !ok {
		sp, _ = w.catalog.Spec(DefaultShip)
	}
	p.spec, p.ShipType = sp, sp.ID
	p.Skin = sp.Skins[0]
	for _, s := range sp.Skins {
		if s == skin {
			p.Skin = skin
		}
	}
	p.Spec = sp.DefaultSpec
	for _, s := range sp.Specs {
		if s == spec {
			p.Spec = spec
		}
	}
	p.Hull, p.Shield, p.Energy = sp.Hull, sp.Shield, sp.Energy
	p.shipLevel = 1 // уровень экземпляра выставляет SetShipLevel (хаб читает его из хранилища)
	p.cooldowns = map[string]float64{}
	p.revealed = map[string]float64{}
	p.initStarterEngine()
}

func (w *World) emit(e Event) { w.events = append(w.events, e) }

// DrainEvents возвращает накопленные события и очищает очередь.
func (w *World) DrainEvents() []Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.events
	w.events = nil
	return out
}

// SetSkin выбирает облик корабля (характеристики у всех обликов одинаковые).
func (w *World) SetSkin(id, skin string) (Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return Record{}, ErrNoPlayer
	}
	for _, s := range p.spec.Skins {
		if s == skin {
			p.Skin = skin
			return p.record(), nil
		}
	}
	return Record{}, ErrBadSkin
}

// SetSpec выбирает специализацию.
func (w *World) SetSpec(id, spec string) (Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return Record{}, ErrNoPlayer
	}
	for _, s := range p.spec.Specs {
		if s == spec {
			p.Spec = spec
			return p.record(), nil
		}
	}
	return Record{}, ErrBadSpec
}

// SetAttack включает или выключает автоатаку «Сдвоенный импульс» по выбранной цели.
func (w *World) SetAttack(id string, on bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	if on {
		if p.Target == "" {
			return ErrNoTarget
		}
		if w.npcByID(p.Target, nil) == nil {
			return ErrBadTarget // игроков атаковать нельзя: PvP не включён
		}
	}
	p.Attacking = on
	return nil
}

func (w *World) npcByID(id string, visibleOnlyFor *Player) *npc {
	for _, n := range w.npcs {
		if n.ID == id && n.Kind == "hostile" {
			return n
		}
	}
	return nil
}

func (w *World) cooldownLeft(p *Player, ab string) float64 {
	return math.Max(0, p.cooldowns[ab]-w.t)
}

func (w *World) available(p *Player, ab string) (AbilityCfg, error) {
	// Catalog presence does not imply an implemented handler. Fail before any
	// resource checks so imported/future keys cannot report a successful cast.
	switch ab {
	case "boost", "intercept", "scan":
	default:
		return AbilityCfg{}, ErrUnknown
	}
	if ab == "boost" {
		if err := p.boostAccess(); err != nil {
			return AbilityCfg{}, err
		}
	}
	cfg, ok := p.spec.Abilities[ab]
	if !ok {
		return cfg, ErrUnknown
	}
	if cfg.Spec != "" && cfg.Spec != p.Spec {
		return cfg, ErrWrongSpec
	}
	if p.deadUntil > w.t {
		return cfg, ErrDead
	}
	if w.cooldownLeft(p, ab) > 0 {
		return cfg, ErrCooldown
	}
	if p.Energy < cfg.Energy {
		return cfg, ErrEnergy
	}
	return cfg, nil
}

// UseAbility применяет способность (boost, intercept, scan). Все условия проверяются здесь.
func (w *World) UseAbility(id, ab string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	if ab == StrizhBoostAbility {
		if p.ShipType != DefaultShip {
			return ErrUnknown
		}
		ab = "boost" // One cooldown/resource path for both protocol names.
	}
	cfg, err := w.available(p, ab)
	if err != nil {
		return err
	}
	switch ab {
	case "boost":
		p.Energy -= cfg.Energy
		p.cooldowns[ab] = w.t + cfg.Cooldown
		p.boostUntil = w.t + cfg.Duration
		w.emit(Event{Type: "boost", From: p.ID, Amount: cfg.Duration})
	case "intercept":
		n := w.npcByID(p.Target, nil)
		if n == nil || n.deadUntil > w.t {
			return ErrNoTarget
		}
		d := dist(p.Pos, n.Pos)
		if d > cfg.Range {
			return ErrRange
		}
		p.Energy -= cfg.Energy
		p.cooldowns[ab] = w.t + cfg.Cooldown
		// рывок к цели: сближение не ближе MinDist и не дальше MaxDist за один рывок
		if d > cfg.MinDist {
			step := math.Min(cfg.MaxDist, d-cfg.MinDist)
			dir := n.Pos.sub(p.Pos).normalized()
			p.Pos = p.Pos.add(dir.mul(step))
			p.Dir = dir
		}
		pl := p.spec.Abilities["pulse"]
		dmg := pl.Damage * float64(max(pl.Shots, 1)) * cfg.DamageMul
		w.hitNPC(p, n, dmg)
		w.emit(Event{Type: "intercept", From: p.ID, To: n.ID, Amount: dmg})
	case "scan":
		p.Energy -= cfg.Energy
		p.cooldowns[ab] = w.t + cfg.Cooldown
		found := 0
		for _, n := range w.npcs {
			if n.Hidden && dist(p.Pos, n.Pos) <= cfg.Radius {
				p.revealed[n.ID] = w.t + cfg.Reveal
				found++
			}
		}
		w.emit(Event{Type: "scan", From: p.ID, Radius: cfg.Radius, Amount: float64(found)})
	}
	return nil
}

func (w *World) hitNPC(p *Player, n *npc, dmg float64) {
	toS, toH := applyDamage(&n.Shield, &n.Hull, 0, dmg)
	if toS > 0 {
		w.emit(Event{Type: "hit_shield", From: p.ID, To: n.ID, Amount: toS})
	}
	if toH > 0 {
		w.emit(Event{Type: "hit_hull", From: p.ID, To: n.ID, Amount: toH})
	}
	if n.Hull <= 0 {
		n.deadUntil = w.t + n.respawn
		w.emit(Event{Type: "death", To: n.ID})
		w.onNPCKilled(p, n)
		for _, q := range w.players {
			if q.Target == n.ID {
				q.Target, q.Attacking, q.Autopilot = "", false, false
			}
		}
	}
}

// DamagePlayer наносит игроку урон от серверного источника (будущие угрозы). Игроки друг друга не атакуют.
func (w *World) DamagePlayer(id string, dmg float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.damagePlayerLocked(id, dmg, "")
}

// damagePlayerLocked наносит урон; attacker — идентификатор нападающего (пусто: безымянный источник). Если у игрока включён
// автоответ и он не занят другой живой целью, он начинает атаковать нападающего (когда тот в дальности, стреляет автоатака).
func (w *World) damagePlayerLocked(id string, dmg float64, attacker string) {
	p, ok := w.players[id]
	if !ok || p.deadUntil > w.t {
		return
	}
	toS, toH := applyDamage(&p.Shield, &p.Hull, p.spec.Armor, dmg)
	p.lastHit = w.t
	if toS > 0 {
		w.emit(Event{Type: "hit_shield", To: p.ID, Amount: toS})
	}
	if toH > 0 {
		w.emit(Event{Type: "hit_hull", To: p.ID, Amount: toH})
	}
	if p.Hull <= 0 {
		w.emit(Event{Type: "death", To: p.ID})
		p.deadUntil = w.t + 3
		p.Attacking, p.Autopilot, p.Move, p.mining = false, false, Vec{}, ""
		return
	}
	w.retaliate(p, attacker)
}

// retaliate: автоответ на атаку (настройка игрока auto_retaliate, по умолчанию включена).
func (w *World) retaliate(p *Player, attacker string) {
	if attacker == "" || p.noRetaliate {
		return
	}
	n := w.npcByID(attacker, nil)
	if n == nil || n.deadUntil > w.t {
		return
	}
	if p.Attacking && p.Target != attacker {
		if cur := w.npcByID(p.Target, nil); cur != nil && cur.deadUntil <= w.t {
			return // уже воюет с другой живой целью: не переключаем
		}
	}
	p.Target, p.Attacking, p.Autopilot, p.mining = attacker, true, false, ""
}

// SetOption меняет настройку игрока. Сейчас: auto_retaliate (автоответ на атаку).
func (w *World) SetOption(id, name string, value bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ErrNoPlayer
	}
	switch name {
	case "auto_retaliate":
		p.noRetaliate = !value
		return nil
	}
	return ErrBadOption
}

// stepCombat — часть шага симуляции: восстановление, автоатака, форсаж, возрождение, дроны.
func (w *World) stepCombat(p *Player, dt float64) {
	sp := p.spec
	if p.deadUntil > 0 && p.deadUntil <= w.t {
		p.deadUntil = 0
		p.Hull, p.Shield, p.Energy = sp.Hull, sp.Shield, sp.Energy
		p.Pos = Vec{w.station.Pos[0] + 14, w.station.Pos[1] - 10}
		w.emit(Event{Type: "respawn", To: p.ID})
	}
	if p.deadUntil > w.t {
		return
	}
	p.Energy = math.Min(sp.Energy, p.Energy+sp.EnergyRegen*dt)
	if w.t-p.lastHit >= sp.ShieldRegenDelay {
		p.Shield = math.Min(sp.Shield, p.Shield+sp.ShieldRegen*dt)
	}
	for id, until := range p.revealed {
		if until <= w.t {
			delete(p.revealed, id)
		}
	}
	if !p.Attacking {
		return
	}
	n := w.npcByID(p.Target, nil)
	pl := sp.Abilities["pulse"]
	if n == nil || n.deadUntil > w.t {
		p.Attacking = false
		return
	}
	if dist(p.Pos, n.Pos) > pl.Range || p.Energy < pl.Energy || w.t < p.attackAt {
		return
	}
	p.attackAt = w.t + pl.Period
	p.Energy -= pl.Energy
	w.emit(Event{Type: "fire", From: p.ID, To: n.ID})
	w.hitNPC(p, n, pl.Damage*float64(max(pl.Shots, 1)))
}

func (w *World) stepNPCs() {
	for _, n := range w.npcs {
		if n.Kind == "hostile" && n.deadUntil > 0 && n.deadUntil <= w.t {
			n.deadUntil, n.Hull, n.Shield = 0, n.HullMax, n.ShieldMax
			n.resetAI()
			w.emit(Event{Type: "respawn", To: n.ID})
		}
	}
}

// speedFor возвращает множитель скорости игрока (форсаж).
func (w *World) speedFor(p *Player) float64 {
	if p.boostUntil > w.t {
		return p.spec.Abilities["boost"].SpeedMul
	}
	return 1
}

// abilityList формирует список способностей игрока для клиента.
func (w *World) abilityList(p *Player) []AbilityInfo {
	var out []AbilityInfo
	order := []string{"boost", "intercept", "scan"}
	for _, id := range order {
		cfg, ok := p.spec.Abilities[id]
		if !ok || (cfg.Spec != "" && cfg.Spec != p.Spec) {
			continue
		}
		info := AbilityInfo{ID: id, Name: cfg.Name, Energy: cfg.Energy, Cooldown: cfg.Cooldown, CooldownLeft: w.cooldownLeft(p, id), NeedsTarget: id == "intercept", Active: id == "boost" && p.boostUntil > w.t, Enabled: true}
		if id == "boost" && p.ShipType == DefaultShip {
			info.CatalogID = StrizhBoostAbility
			info.Rank = p.boostRank
			if p.starterEngine != nil {
				info.ModuleID = p.starterEngine.InstanceID
			}
			if err := p.boostAccess(); err != nil {
				info.Enabled = false
				info.UnavailableReason = err.Error()
			}
		}
		out = append(out, info)
	}
	return out
}

// SpecInfo возвращает паспорт корабля игрока (для ангара).
func (w *World) SpecInfo(id string) (ShipSpec, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return ShipSpec{}, false
	}
	return p.spec, true
}

// ValidSkin проверяет облик (для протокола).
func ValidSkin(s string) bool {
	return len(s) > 0 && len(s) <= 8 && !strings.ContainsAny(s, " \t\n")
}

// Starter возвращает стартовый корабль направления пилота.
func (w *World) Starter(pilot string) (Starter, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	st, ok := w.catalog.Starters[pilot]
	return st, ok
}

// Catalog возвращает действующий каталог кораблей.
func (w *World) Catalog() Catalog {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.catalog.Clone()
}

// SetCatalog применяет новый каталог на лету: у игроков обновляются паспорта кораблей (значения ограничиваются новыми максимумами).
func (w *World) SetCatalog(c Catalog) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.catalog = c.Clone()
	c = w.catalog
	for _, p := range w.players {
		sp, ok := c.Spec(p.ShipType)
		if !ok {
			sp, _ = c.Spec(DefaultShip)
			p.ShipType = sp.ID
			p.initStarterEngine()
		}
		p.spec = sp
		p.Hull, p.Shield, p.Energy = math.Min(p.Hull, sp.Hull), math.Min(p.Shield, sp.Shield), math.Min(p.Energy, sp.Energy)
		if !contains(sp.Skins, p.Skin) {
			p.Skin = sp.Skins[0]
		}
		if !contains(sp.Specs, p.Spec) {
			p.Spec = sp.DefaultSpec
		}
	}
}
