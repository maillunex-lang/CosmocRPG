package rpg

import (
	"fmt"
	"sort"
)

// Cap — ограничение итогового значения. Max <= 0 — без верхнего предела.
type Cap struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// AttrEffect — вклад одного очка характеристики пилота в характеристику корабля (доля прироста на очко).
type AttrEffect struct {
	Stat     StatID  `json:"stat"`
	PerPoint float64 `json:"per_point"`
}

// MasteryEffect — вклад одного уровня мастерства класса кораблей.
type MasteryEffect struct {
	Stat     StatID  `json:"stat"`
	Op       Op      `json:"op"`
	PerLevel float64 `json:"per_level"`
}

// Config — данные для расчёта (правятся данными, не кодом).
type Config struct {
	// ShipLevelGrowth — доля прироста базового значения на каждый уровень корабля выше первого.
	ShipLevelGrowth map[StatID]float64 `json:"ship_level_growth"`
	// AttributeEffects — как очки характеристик пилота меняют характеристики корабля.
	AttributeEffects map[Attribute][]AttrEffect `json:"attribute_effects"`
	// MasteryEffects — вклад уровней мастерства по классам кораблей.
	MasteryEffects map[string][]MasteryEffect `json:"mastery_effects"`
	// Caps — потолки и полы итоговых значений.
	Caps map[StatID]Cap `json:"caps"`
	// Talents — дерево талантов пилота (для разрешения изученных узлов в вклады).
	Talents TalentTree `json:"talents"`
}

// LearnedTalent — изученный узел и его ранг.
type LearnedTalent struct {
	NodeID string `json:"node_id"`
	Rank   int    `json:"rank"`
}

// Mastery — мастерство класса кораблей.
type Mastery struct {
	Class string `json:"class"`
	Level int    `json:"level"`
}

// Input — вход расчёта: снимок состояния, а не накопленный итог.
type Input struct {
	Base       map[StatID]float64
	ShipLevel  int
	Attributes map[Attribute]int
	Talents    []LearnedTalent
	Mastery    Mastery
	Modules    []Modifier // слой модулей: вклады уже разрешены вызывающим (RPG-07)
	Effects    []Modifier // временные эффекты (RPG-05)
}

// Contribution — строка разбивки итога по источникам.
type Contribution struct {
	Source string  `json:"source"`
	Layer  string  `json:"layer"`
	Op     Op      `json:"op"`
	Value  float64 `json:"value"`
}

// Result — итог расчёта.
type Result struct {
	Final     map[StatID]float64
	Breakdown map[StatID][]Contribution
	Warnings  []string
}

// DefaultConfig — стартовые значения: ориентиры для разработки, не утверждённый баланс (баланс-проход после RPG-1…RPG-3).
func DefaultConfig() Config {
	growth := map[StatID]float64{}
	// мощность вооружения (damage) добавится, когда способности станут данными, а не кодом: сейчас мир применяет рост только к этим четырём
	for _, s := range []StatID{StatHull, StatShield, StatEnergy, StatCargo} {
		growth[s] = 0.03
	}
	return Config{
		ShipLevelGrowth: growth,
		AttributeEffects: map[Attribute][]AttrEffect{
			AttrPiloting:    {{StatSpeed, 0.010}, {StatManeuver, 0.015}},
			AttrTactics:     {{StatDamage, 0.010}, {StatAccuracy, 0.008}, {StatCritChance, 0.004}},
			AttrEngineering: {{StatHull, 0.010}, {StatArmor, 0.010}, {StatResist, 0.004}, {StatRepairCost, -0.008}},
			AttrEnergetics:  {{StatEnergy, 0.012}, {StatEnergyRegen, 0.012}, {StatShield, 0.008}},
			AttrAnalytics:   {{StatSensors, 0.012}, {StatRange, 0.006}, {StatMining, 0.010}, {StatCritDamage, 0.005}},
			AttrLogistics:   {{StatCargo, 0.012}, {StatHaul, 0.010}, {StatFuelUse, -0.008}},
		},
		MasteryEffects: map[string][]MasteryEffect{},
		Caps: map[StatID]Cap{
			StatResist:     {Min: 0, Max: 0.8},
			StatCritChance: {Min: 0, Max: 0.75},
			StatAccuracy:   {Min: 0, Max: 1},
		},
		Talents: TalentTree{},
	}
}

// Compute вычисляет итоговые характеристики с нуля. Чистая функция: не меняет вход, одинаковый вход даёт одинаковый итог.
//
// Правила (АР-3): слои применяются в порядке «уровень корабля → характеристики пилота → таланты → мастерство → модули → эффекты»;
// внутри слоя сначала все add, затем одно умножение на (1 + сумма mul); каждый источник (source) учитывается один раз;
// в конце применяются потолки; результат не зависит от порядка элементов во входных списках.
func Compute(cfg Config, in Input) Result {
	res := Result{Final: map[StatID]float64{}, Breakdown: map[StatID][]Contribution{}}
	mods := collectModifiers(cfg, in, &res)

	// Источник учитывается один раз: повтор (source, stat) отбрасывается.
	sort.SliceStable(mods, func(i, j int) bool { return lessMod(mods[i], mods[j]) })
	seen := map[string]bool{}
	var uniq []Modifier
	for _, m := range mods {
		k := fmt.Sprintf("%s|%s|%d|%s", m.Source, m.Stat, m.Layer, m.Op)
		if seen[k] {
			res.Warnings = append(res.Warnings, "повторный источник отброшен: "+m.Source)
			continue
		}
		seen[k] = true
		uniq = append(uniq, m)
	}

	for _, stat := range AllStats {
		v, ok := in.Base[stat]
		if !ok {
			continue
		}
		for layer := LayerShipLevel; layer <= LayerEffects; layer++ {
			var add, mul float64
			for _, m := range uniq {
				if m.Stat != stat || m.Layer != layer {
					continue
				}
				res.Breakdown[stat] = append(res.Breakdown[stat], Contribution{Source: m.Source, Layer: layer.String(), Op: m.Op, Value: m.Value})
				if m.Op == OpAdd {
					add += m.Value
				} else {
					mul += m.Value
				}
			}
			v = (v + add) * (1 + mul)
		}
		c := cfg.Caps[stat]
		if v < c.Min {
			v = c.Min
		}
		if c.Max > 0 && v > c.Max {
			v = c.Max
		}
		res.Final[stat] = v
	}
	return res
}

func lessMod(a, b Modifier) bool {
	if a.Layer != b.Layer {
		return a.Layer < b.Layer
	}
	if a.Stat != b.Stat {
		return a.Stat < b.Stat
	}
	if a.Source != b.Source {
		return a.Source < b.Source
	}
	if a.Op != b.Op {
		return a.Op < b.Op
	}
	return a.Value < b.Value
}

// collectModifiers собирает вклады всех слоёв из входа и данных; неизвестные и некорректные отбрасываются с предупреждением.
func collectModifiers(cfg Config, in Input, res *Result) []Modifier {
	var mods []Modifier

	if lvl := in.ShipLevel; lvl > 1 {
		for _, stat := range AllStats {
			if g := cfg.ShipLevelGrowth[stat]; g != 0 {
				mods = append(mods, Modifier{Source: "ship_level", Layer: LayerShipLevel, Stat: stat, Op: OpMul, Value: g * float64(lvl-1)})
			}
		}
	}

	for _, attr := range AllAttributes {
		pts := in.Attributes[attr]
		if pts <= 0 {
			continue
		}
		for _, e := range cfg.AttributeEffects[attr] {
			mods = append(mods, Modifier{Source: "attr:" + string(attr), Layer: LayerAttributes, Stat: e.Stat, Op: OpMul, Value: e.PerPoint * float64(pts)})
		}
	}

	byID := map[string]TalentNode{}
	for _, n := range cfg.Talents.Nodes {
		byID[n.ID] = n
	}
	for _, lt := range in.Talents {
		n, ok := byID[lt.NodeID]
		if !ok {
			res.Warnings = append(res.Warnings, "неизвестный талант: "+lt.NodeID)
			continue
		}
		rank := lt.Rank
		if rank > n.MaxRank {
			res.Warnings = append(res.Warnings, "ранг таланта выше максимального: "+lt.NodeID)
			rank = n.MaxRank
		}
		if rank <= 0 {
			continue
		}
		for _, e := range n.Effects {
			mods = append(mods, Modifier{Source: "talent:" + n.ID, Layer: LayerTalents, Stat: e.Stat, Op: e.Op, Value: e.PerRank * float64(rank)})
		}
	}

	if in.Mastery.Level > 0 {
		for _, e := range cfg.MasteryEffects[in.Mastery.Class] {
			mods = append(mods, Modifier{Source: "mastery:" + in.Mastery.Class, Layer: LayerMastery, Stat: e.Stat, Op: e.Op, Value: e.PerLevel * float64(in.Mastery.Level)})
		}
	}

	for _, m := range in.Modules {
		m.Layer = LayerModules
		mods = append(mods, m)
	}
	for _, m := range in.Effects {
		m.Layer = LayerEffects
		mods = append(mods, m)
	}

	valid := mods[:0]
	for _, m := range mods {
		if !ValidStat(m.Stat) || !ValidOp(m.Op) || m.Source == "" {
			res.Warnings = append(res.Warnings, "некорректный вклад отброшен: "+m.Source)
			continue
		}
		valid = append(valid, m)
	}
	return valid
}
