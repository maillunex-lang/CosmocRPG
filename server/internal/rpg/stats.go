// Package rpg — RPG-фундамент (RPG-0): чистый расчёт характеристик корабля из слоёв, кривые опыта и схема талантов.
// Пакет ни с чем не связан (ни с миром, ни с БД): подключение к игре — этапы RPG-1…RPG-6.
package rpg

// StatID — характеристика корабля (19 штук в четырёх группах, SHIPS_SPEC §5).
type StatID string

const (
	// Защита.
	StatHull   StatID = "hull"   // прочность корпуса
	StatShield StatID = "shield" // прочность щитов
	StatArmor  StatID = "armor"  // броня
	StatResist StatID = "resist" // сопротивления (доля поглощаемого урона)
	// Атака.
	StatDamage     StatID = "damage"      // мощность вооружения
	StatAccuracy   StatID = "accuracy"    // точность
	StatRange      StatID = "range"       // дальность
	StatCritChance StatID = "crit_chance" // вероятность критического попадания
	StatCritDamage StatID = "crit_damage" // критический урон
	// Системы.
	StatSpeed       StatID = "speed"        // скорость
	StatManeuver    StatID = "maneuver"     // манёвренность
	StatEnergy      StatID = "energy"       // запас энергии
	StatEnergyRegen StatID = "energy_regen" // восстановление энергии
	StatSensors     StatID = "sensors"      // сенсоры
	// Экономика.
	StatCargo      StatID = "cargo"       // вместимость трюма
	StatMining     StatID = "mining"      // эффективность добычи
	StatFuelUse    StatID = "fuel_use"    // расход топлива
	StatRepairCost StatID = "repair_cost" // стоимость ремонта
	StatHaul       StatID = "haul"        // эффективность перевозки
)

// AllStats — все характеристики корабля в стабильном порядке.
var AllStats = []StatID{
	StatHull, StatShield, StatArmor, StatResist,
	StatDamage, StatAccuracy, StatRange, StatCritChance, StatCritDamage,
	StatSpeed, StatManeuver, StatEnergy, StatEnergyRegen, StatSensors,
	StatCargo, StatMining, StatFuelUse, StatRepairCost, StatHaul,
}

var statSet = func() map[StatID]bool {
	m := make(map[StatID]bool, len(AllStats))
	for _, s := range AllStats {
		m[s] = true
	}
	return m
}()

// ValidStat сообщает, известна ли характеристика.
func ValidStat(s StatID) bool { return statSet[s] }

// Attribute — базовая характеристика пилота (6 штук, RPG-04).
type Attribute string

const (
	AttrPiloting    Attribute = "piloting"    // Пилотирование
	AttrTactics     Attribute = "tactics"     // Тактика
	AttrEngineering Attribute = "engineering" // Инженерия
	AttrEnergetics  Attribute = "energetics"  // Энергетика
	AttrAnalytics   Attribute = "analytics"   // Аналитика
	AttrLogistics   Attribute = "logistics"   // Логистика
)

// AllAttributes — характеристики пилота в стабильном порядке.
var AllAttributes = []Attribute{AttrPiloting, AttrTactics, AttrEngineering, AttrEnergetics, AttrAnalytics, AttrLogistics}

// Op — способ применения вклада.
type Op string

const (
	// OpAdd прибавляет значение к характеристике.
	OpAdd Op = "add"
	// OpMul — доля прироста: 0.05 означает +5 %. Доли одного слоя складываются и применяются один раз.
	OpMul Op = "mul"
)

// ValidOp сообщает, известна ли операция.
func ValidOp(o Op) bool { return o == OpAdd || o == OpMul }

// Layer — слой расчёта; порядок слоёв фиксирован (АР-3).
type Layer int

const (
	LayerShipLevel Layer = iota + 1
	LayerAttributes
	LayerTalents
	LayerMastery
	LayerModules
	LayerEffects
)

var layerNames = map[Layer]string{
	LayerShipLevel: "ship_level", LayerAttributes: "attributes", LayerTalents: "talents",
	LayerMastery: "mastery", LayerModules: "modules", LayerEffects: "effects",
}

func (l Layer) String() string { return layerNames[l] }

// Modifier — вклад одного источника в одну характеристику.
// Source — стабильный идентификатор источника ("talent:N42", "module:slot2", "effect:boost"); каждый источник применяется один раз.
type Modifier struct {
	Source string  `json:"source"`
	Layer  Layer   `json:"layer"`
	Stat   StatID  `json:"stat"`
	Op     Op      `json:"op"`
	Value  float64 `json:"value"`
}
