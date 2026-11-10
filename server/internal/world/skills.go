package world

import "math"

// SkillDef — умение корабля или модуля. Числа предварительные, сервер их применяет сам.
type SkillDef struct {
	ID, Name, Kind, Icon, Role string
	Energy, Cooldown           float64
	Duration, Range, Radius    float64
	Power                      float64 // доля щита/корпуса, множитель урона или прибавка энергии
	NeedsTarget                bool
}

func skillByID(id string) (SkillDef, bool) {
	s, ok := skillBook[id]
	return s, ok
}

var skillBook = map[string]SkillDef{
	"ftr1_salvo":   {ID: "ftr1_salvo", Name: "Искровой залп", Kind: "strike", Icon: "zap", Role: "strike", Energy: 18, Cooldown: 6, Range: 90, Power: 1.45, NeedsTarget: true},
	"ftr1_break":   {ID: "ftr1_break", Name: "Прорыв", Kind: "dash", Icon: "rocket", Role: "ult", Energy: 35, Cooldown: 28, Range: 110, Duration: 0, Power: 1.2, NeedsTarget: true},
	"ftr2_slug":    {ID: "ftr2_slug", Name: "Тяжёлый снаряд", Kind: "strike", Icon: "zap", Role: "strike", Energy: 22, Cooldown: 8, Range: 100, Power: 1.9, NeedsTarget: true},
	"ftr2_barrage": {ID: "ftr2_barrage", Name: "Осадный шквал", Kind: "nova", Icon: "flame", Role: "ult", Energy: 40, Cooldown: 32, Radius: 80, Power: 1.15},
	"haul_ram":     {ID: "haul_ram", Name: "Таранный импульс", Kind: "strike", Icon: "rocket", Role: "strike", Energy: 16, Cooldown: 7, Range: 55, Power: 1.05, NeedsTarget: true},
	"haul_lock":    {ID: "haul_lock", Name: "Крепление корпуса", Kind: "barrier", Icon: "shield", Role: "ult", Energy: 30, Cooldown: 30, Power: 0.5},
	"mine_charge":  {ID: "mine_charge", Name: "Буровой заряд", Kind: "strike", Icon: "pickaxe", Role: "strike", Energy: 16, Cooldown: 6, Range: 70, Power: 1.25, NeedsTarget: true},
	"mine_pulse":   {ID: "mine_pulse", Name: "Сейсмоимпульс", Kind: "nova", Icon: "pickaxe", Role: "ult", Energy: 34, Cooldown: 30, Radius: 75, Power: 0.9},
	"cor1_shot":    {ID: "cor1_shot", Name: "Охотничий выстрел", Kind: "strike", Icon: "zap", Role: "strike", Energy: 18, Cooldown: 6, Range: 110, Power: 1.5, NeedsTarget: true},
	"cor1_volley":  {ID: "cor1_volley", Name: "Залп корвета", Kind: "nova", Icon: "flame", Role: "ult", Energy: 36, Cooldown: 30, Radius: 70, Power: 1.05},
	"cor2_rake":    {ID: "cor2_rake", Name: "Налёт", Kind: "dash", Icon: "rocket", Role: "strike", Energy: 20, Cooldown: 8, Range: 100, Power: 1.1, NeedsTarget: true},
	"cor2_smoke":   {ID: "cor2_smoke", Name: "Дымовой рывок", Kind: "boost", Icon: "flame", Role: "ult", Energy: 28, Cooldown: 26, Duration: 4},
	"frg1_flak":    {ID: "frg1_flak", Name: "Заградительный огонь", Kind: "strike", Icon: "zap", Role: "strike", Energy: 18, Cooldown: 6, Range: 95, Power: 1.3, NeedsTarget: true},
	"frg1_screen":  {ID: "frg1_screen", Name: "Эскортный щит", Kind: "barrier", Icon: "shield", Role: "ult", Energy: 32, Cooldown: 28, Power: 0.45},
	"frg2_slug":    {ID: "frg2_slug", Name: "Бронебойный", Kind: "strike", Icon: "gem", Role: "strike", Energy: 20, Cooldown: 7, Range: 90, Power: 1.55, NeedsTarget: true},
	"frg2_wall":    {ID: "frg2_wall", Name: "Стена", Kind: "fortify", Icon: "shield", Role: "ult", Energy: 30, Cooldown: 32, Duration: 8},
	"dst_torp":     {ID: "dst_torp", Name: "Торпеда", Kind: "strike", Icon: "rocket", Role: "strike", Energy: 24, Cooldown: 9, Range: 120, Power: 2.1, NeedsTarget: true},
	"dst_rain":     {ID: "dst_rain", Name: "Ракетный дождь", Kind: "nova", Icon: "flame", Role: "ult", Energy: 42, Cooldown: 34, Radius: 90, Power: 1.25},
	"cru_bat":      {ID: "cru_bat", Name: "Батарея", Kind: "strike", Icon: "zap", Role: "strike", Energy: 22, Cooldown: 7, Range: 110, Power: 1.7, NeedsTarget: true},
	"cru_flag":     {ID: "cru_flag", Name: "Флагманский залп", Kind: "nova", Icon: "star", Role: "ult", Energy: 45, Cooldown: 36, Radius: 100, Power: 1.35},
	"sct_mark":     {ID: "sct_mark", Name: "Маркер", Kind: "strike", Icon: "globe", Role: "strike", Energy: 12, Cooldown: 5, Range: 130, Power: 0.85, NeedsTarget: true},
	"sct_eye":      {ID: "sct_eye", Name: "Полный обзор", Kind: "scan", Icon: "globe", Role: "ult", Energy: 25, Cooldown: 24, Radius: 140},
	"sup_jab":      {ID: "sup_jab", Name: "Подавление", Kind: "strike", Icon: "zap", Role: "strike", Energy: 16, Cooldown: 6, Range: 90, Power: 1.1, NeedsTarget: true},
	"sup_field":    {ID: "sup_field", Name: "Поле ремонта", Kind: "mend", Icon: "sparkles", Role: "ult", Energy: 34, Cooldown: 28, Power: 0.28},

	"mod_over":    {ID: "mod_over", Name: "Перегрузка орудий", Kind: "strike", Icon: "zap", Role: "module", Energy: 20, Cooldown: 8, Range: 90, Power: 1.65, NeedsTarget: true},
	"mod_barrier": {ID: "mod_barrier", Name: "Аварийный щит", Kind: "barrier", Icon: "shield", Role: "module", Energy: 22, Cooldown: 16, Power: 0.35},
	"mod_fortify": {ID: "mod_fortify", Name: "Бронеперегрузка", Kind: "fortify", Icon: "gem", Role: "module", Energy: 18, Cooldown: 18, Duration: 6},
	"mod_boost":   {ID: "mod_boost", Name: "Форсаж модуля", Kind: "boost", Icon: "rocket", Role: "module", Energy: 20, Cooldown: 14, Duration: 3.5},
	"mod_vent":    {ID: "mod_vent", Name: "Сброс реактора", Kind: "vent", Icon: "fuel", Role: "module", Energy: 0, Cooldown: 18, Power: 40},
	"mod_scan":    {ID: "mod_scan", Name: "Тактический сканер", Kind: "scan", Icon: "globe", Role: "module", Energy: 15, Cooldown: 12, Radius: 90},
	"mod_drone":   {ID: "mod_drone", Name: "Дрон-удар", Kind: "drone", Icon: "bot", Role: "module", Energy: 22, Cooldown: 12, Range: 100, Power: 0.8, NeedsTarget: true},
	"mod_mend":    {ID: "mod_mend", Name: "Ремонтный луч", Kind: "mend", Icon: "sparkles", Role: "module", Energy: 24, Cooldown: 14, Power: 0.18},
	"mod_brace":   {ID: "mod_brace", Name: "Укрепление трюма", Kind: "barrier", Icon: "package", Role: "module", Energy: 16, Cooldown: 16, Power: 0.25},
	"mod_patch":   {ID: "mod_patch", Name: "Наноремонт", Kind: "mend", Icon: "sparkles", Role: "module", Energy: 20, Cooldown: 12, Power: 0.22},
	"mod_bore":    {ID: "mod_bore", Name: "Промышленный бур", Kind: "strike", Icon: "pickaxe", Role: "module", Energy: 16, Cooldown: 7, Range: 60, Power: 1.2, NeedsTarget: true},
}

var shipKits = map[string][2]string{
	"FTR-001":  {"ftr1_salvo", "ftr1_break"},
	"FTR-002":  {"ftr2_slug", "ftr2_barrage"},
	"TMP-HAUL": {"haul_ram", "haul_lock"},
	"TMP-MINE": {"mine_charge", "mine_pulse"},
	"COR-001":  {"cor1_shot", "cor1_volley"},
	"COR-002":  {"cor2_rake", "cor2_smoke"},
	"FRG-001":  {"frg1_flak", "frg1_screen"},
	"FRG-002":  {"frg2_slug", "frg2_wall"},
	"DST-001":  {"dst_torp", "dst_rain"},
	"CRU-001":  {"cru_bat", "cru_flag"},
	"SCT-001":  {"sct_mark", "sct_eye"},
	"SUP-001":  {"sup_jab", "sup_field"},
}

var moduleSkill = map[string]string{
	"weapon": "mod_over", "shield": "mod_barrier", "armor": "mod_fortify",
	"engine": "mod_boost", "reactor": "mod_vent", "electronics": "mod_scan",
	"drone": "mod_drone", "support": "mod_mend", "cargo": "mod_brace",
	"repair": "mod_patch", "industrial": "mod_bore",
}

// modulePriority — какие два модульных умения видны, если слотов больше двух.
// Особые модули (дрон, ремонт, броня) важнее штатных пушки и двигателя.
var modulePriority = []string{"drone", "support", "repair", "armor", "shield", "weapon", "electronics", "industrial", "cargo", "reactor", "engine"}

func pickModuleCats(have []string) []string {
	got := map[string]bool{}
	for _, c := range have {
		got[c] = true
	}
	var out []string
	for _, c := range modulePriority {
		if !got[c] {
			continue
		}
		out = append(out, c)
		if len(out) == 2 {
			break
		}
	}
	return out
}

var defaultModuleCategories = map[string][]string{
	"FTR-001":  {"weapon", "shield"},
	"FTR-002":  {"weapon", "armor"},
	"TMP-HAUL": {"cargo", "engine"},
	"TMP-MINE": {"electronics", "support"},
	"COR-001":  {"weapon", "electronics"},
	"COR-002":  {"engine", "weapon"},
	"FRG-001":  {"shield", "support"},
	"FRG-002":  {"armor", "shield"},
	"DST-001":  {"weapon", "drone"},
	"CRU-001":  {"weapon", "shield"},
	"SCT-001":  {"electronics", "engine"},
	"SUP-001":  {"support", "shield"},
}

// SetModuleCategories запоминает категории надетых модулей. Пустой список после известной примерки убирает модульные умения.
func (w *World) SetModuleCategories(id string, cats []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return
	}
	p.modulesKnown = true
	p.moduleCategories = append([]string(nil), cats...)
}

func (p *Player) moduleCats() []string {
	if p.modulesKnown {
		return p.moduleCategories
	}
	return defaultModuleCategories[p.ShipType]
}

func (w *World) loadout(p *Player) []SkillDef {
	kit := shipKits[p.ShipType]
	var strike, ult SkillDef
	seen := map[string]bool{}
	if sk, ok := skillByID(kit[0]); ok {
		strike = sk
		seen[sk.ID] = true
	}
	if sk, ok := skillByID(kit[1]); ok {
		ult = sk
		seen[sk.ID] = true
	}
	var mods []SkillDef
	for _, cat := range pickModuleCats(p.moduleCats()) {
		id := moduleSkill[cat]
		if id == "" || seen[id] {
			continue
		}
		sk, ok := skillByID(id)
		if !ok {
			continue
		}
		seen[id] = true
		mods = append(mods, sk)
	}
	var out []SkillDef
	if strike.ID != "" {
		out = append(out, strike)
	}
	out = append(out, mods...)
	if ult.ID != "" {
		out = append(out, ult)
	}
	return out
}

func (w *World) skillGranted(p *Player, id string) bool {
	for _, sk := range w.loadout(p) {
		if sk.ID == id {
			return true
		}
	}
	return false
}

func (w *World) availableSkill(p *Player, sk SkillDef) (AbilityCfg, error) {
	cfg := AbilityCfg{Name: sk.Name, Energy: sk.Energy, Cooldown: sk.Cooldown, Range: sk.Range, Radius: sk.Radius, Duration: sk.Duration}
	if !w.skillGranted(p, sk.ID) {
		return cfg, ErrUnknown
	}
	if p.deadUntil > w.t {
		return cfg, ErrDead
	}
	if w.cooldownLeft(p, sk.ID) > 0 {
		return cfg, ErrCooldown
	}
	if p.Energy < sk.Energy {
		return cfg, ErrEnergy
	}
	return cfg, nil
}

func (w *World) strikeDamage(p *Player, mul float64) float64 {
	pl := p.spec.Abilities["pulse"]
	base := pl.Damage
	if base <= 0 {
		base = 14
	}
	shots := pl.Shots
	if shots < 1 {
		shots = 1
	}
	return base * float64(shots) * mul
}

func (w *World) castSkill(p *Player, sk SkillDef) error {
	switch sk.Kind {
	case "strike", "dash", "drone":
		n := w.npcByID(p.Target, nil)
		if n == nil || n.deadUntil > w.t {
			return ErrNoTarget
		}
		d := dist(p.Pos, n.Pos)
		if sk.Range > 0 && d > sk.Range {
			return ErrRange
		}
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		if sk.Kind == "dash" && d > 18 {
			step := math.Min(70, d-18)
			dir := n.Pos.sub(p.Pos).normalized()
			p.Pos = p.Pos.add(dir.mul(step))
			p.Dir = dir
		}
		dmg := w.strikeDamage(p, sk.Power)
		w.hitNPC(p, n, dmg)
		kind := "skill_strike"
		if sk.Kind == "dash" {
			kind = "skill_dash"
		}
		if sk.Kind == "drone" {
			kind = "skill_drone"
		}
		w.emit(Event{Type: kind, From: p.ID, To: n.ID, Amount: dmg})
	case "nova":
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		dmg := w.strikeDamage(p, sk.Power)
		for _, n := range w.npcs {
			if n.Kind == "hostile" && n.deadUntil <= w.t && dist(p.Pos, n.Pos) <= sk.Radius {
				w.hitNPC(p, n, dmg)
			}
		}
		w.emit(Event{Type: "skill_nova", From: p.ID, Amount: dmg, Radius: sk.Radius})
	case "barrier":
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		p.Shield = math.Min(p.spec.Shield, p.Shield+p.spec.Shield*sk.Power)
		w.emit(Event{Type: "skill_barrier", From: p.ID, Amount: sk.Power})
	case "mend":
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		p.Hull = math.Min(p.spec.Hull, p.Hull+p.spec.Hull*sk.Power)
		w.emit(Event{Type: "skill_mend", From: p.ID, Amount: sk.Power})
	case "fortify":
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		p.guardUntil = w.t + sk.Duration
		p.guardMul = 0.65
		w.emit(Event{Type: "skill_fortify", From: p.ID, Amount: sk.Duration})
	case "boost":
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		p.boostUntil = w.t + sk.Duration
		w.emit(Event{Type: "skill_boost", From: p.ID, Amount: sk.Duration})
	case "vent":
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		p.Energy = math.Min(p.spec.Energy, p.Energy+sk.Power)
		w.emit(Event{Type: "skill_vent", From: p.ID, Amount: sk.Power})
	case "scan":
		p.Energy -= sk.Energy
		p.cooldowns[sk.ID] = w.t + sk.Cooldown
		found := 0
		for _, n := range w.npcs {
			if n.Hidden && dist(p.Pos, n.Pos) <= sk.Radius {
				p.revealed[n.ID] = w.t + 20
				found++
			}
		}
		w.emit(Event{Type: "skill_scan", From: p.ID, Radius: sk.Radius, Amount: float64(found)})
	default:
		return ErrUnknown
	}
	return nil
}

func (w *World) appendLoadout(p *Player, out []AbilityInfo) []AbilityInfo {
	for _, sk := range w.loadout(p) {
		info := AbilityInfo{ID: sk.ID, Name: sk.Name, Energy: sk.Energy, Cooldown: sk.Cooldown, CooldownLeft: w.cooldownLeft(p, sk.ID), NeedsTarget: sk.NeedsTarget, Range: sk.Range, Icon: sk.Icon, Role: sk.Role, Enabled: true}
		if p.actionBusy() {
			info.Enabled = false
			info.UnavailableReason = ErrEquipmentBusy.Error()
		}
		out = append(out, info)
	}
	return out
}
