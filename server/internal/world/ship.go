package world

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// Конфигурация кораблей и способностей: data/ships.json. Все числа балансируемые; сервер единственный источник результатов.

//go:embed data/ships.json
var shipsJSON []byte

// AbilityCfg — параметры способности (используются только нужные для конкретной).
type AbilityCfg struct {
	Name      string  `json:"name"`
	Spec      string  `json:"spec"` // "" — общая; иначе только для этой специализации
	Damage    float64 `json:"damage"`
	Shots     int     `json:"shots"`
	Range     float64 `json:"range"`
	Period    float64 `json:"period"`
	Energy    float64 `json:"energy"`
	Cooldown  float64 `json:"cooldown"`
	Duration  float64 `json:"duration"`
	SpeedMul  float64 `json:"speed_mul"`
	MaxDist   float64 `json:"max_dist"`
	MinDist   float64 `json:"min_dist"`
	DamageMul float64 `json:"damage_mul"`
	Radius    float64 `json:"radius"`
	Reveal    float64 `json:"reveal"`
}

// ShipSpec — паспорт корабля.
type ShipSpec struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Class            string                `json:"class"`
	Role             string                `json:"role"`
	Temporary        bool                  `json:"temporary"` // временная модель-заглушка: не считается полноценным кораблём каталога
	Specs            []string              `json:"specs"`
	DefaultSpec      string                `json:"default_spec"`
	Skins            []string              `json:"skins"`
	DimensionsM      [3]float64            `json:"dimensions_m"`
	Crew             int                   `json:"crew"`
	Engines          int                   `json:"engines"`
	Weapons          string                `json:"weapons"`
	Hull             float64               `json:"hull"`
	Shield           float64               `json:"shield"`
	Armor            float64               `json:"armor"`
	Energy           float64               `json:"energy"`
	Cargo            float64               `json:"cargo"`
	Speed            float64               `json:"speed"`
	ShieldRegen      float64               `json:"shield_regen"`
	ShieldRegenDelay float64               `json:"shield_regen_delay"`
	EnergyRegen      float64               `json:"energy_regen"`
	Abilities        map[string]AbilityCfg `json:"abilities"`
}

// Starter — стартовый корабль для направления пилота (отдельные копии корабля под специализации не создаются).
type Starter struct {
	Ship string `json:"ship"`
	Spec string `json:"spec"`
	Skin string `json:"skin"`
}

// PilotSpecs — стартовые направления пилота (не классы кораблей).
var PilotSpecs = []string{"interceptor", "scout", "trader", "miner"}

// Catalog — каталог кораблей и стартовые корабли направлений пилота.
type Catalog struct {
	Ships    map[string]ShipSpec
	Starters map[string]Starter
}

// Clone — глубокая копия каталога: изменения копии не затрагивают оригинал (карты и срезы не разделяются).
func (c Catalog) Clone() Catalog {
	out := Catalog{Ships: map[string]ShipSpec{}, Starters: map[string]Starter{}}
	for id, sp := range c.Ships {
		ab := make(map[string]AbilityCfg, len(sp.Abilities))
		for k, v := range sp.Abilities {
			ab[k] = v
		}
		sp.Abilities = ab
		sp.Specs = append([]string{}, sp.Specs...)
		sp.Skins = append([]string{}, sp.Skins...)
		out.Ships[id] = sp
	}
	for k, v := range c.Starters {
		out.Starters[k] = v
	}
	return out
}

// Doc — документ каталога для хранения и ZIP-патчей (корабли по идентификатору).
type Doc struct {
	Ships    []ShipSpec         `json:"ships"`
	Starters map[string]Starter `json:"starters"`
}

// Doc возвращает документ каталога (корабли упорядочены по идентификатору).
func (c Catalog) Doc() Doc {
	d := Doc{Starters: c.Starters}
	for _, id := range sortedKeys(c.Ships) {
		d.Ships = append(d.Ships, c.Ships[id])
	}
	return d
}

// LoadCatalog разбирает и проверяет конфигурацию кораблей.
func LoadCatalog(data []byte) (Catalog, error) {
	var raw Doc
	if err := json.Unmarshal(data, &raw); err != nil {
		return Catalog{}, fmt.Errorf("ships.json: %w", err)
	}
	c := Catalog{Ships: map[string]ShipSpec{}, Starters: raw.Starters}
	if c.Starters == nil {
		c.Starters = map[string]Starter{}
	}
	for _, s := range raw.Ships {
		if s.ID == "" || s.Hull <= 0 || s.Shield < 0 || s.Energy <= 0 || s.Speed <= 0 || len(s.Skins) == 0 || len(s.Specs) == 0 {
			return Catalog{}, fmt.Errorf("ships.json: некорректный корабль %q", s.ID)
		}
		if _, dup := c.Ships[s.ID]; dup {
			return Catalog{}, fmt.Errorf("ships.json: повтор %s", s.ID)
		}
		okSpec := false
		for _, sp := range s.Specs {
			okSpec = okSpec || sp == s.DefaultSpec
		}
		if !okSpec {
			return Catalog{}, fmt.Errorf("ships.json: %s: специализация по умолчанию не из списка", s.ID)
		}
		for _, k := range []string{"pulse", "boost"} {
			if _, ok := s.Abilities[k]; !ok {
				return Catalog{}, fmt.Errorf("ships.json: %s: нет способности %s", s.ID, k)
			}
		}
		c.Ships[s.ID] = s
	}
	if _, ok := c.Ships[DefaultShip]; !ok {
		return Catalog{}, fmt.Errorf("ships.json: нет корабля по умолчанию %s", DefaultShip)
	}
	for _, ps := range PilotSpecs {
		st, ok := c.Starters[ps]
		if !ok {
			return Catalog{}, fmt.Errorf("ships.json: нет стартового корабля для направления %q", ps)
		}
		sp, ok := c.Ships[st.Ship]
		if !ok {
			return Catalog{}, fmt.Errorf("ships.json: стартовый корабль %q направления %q не найден", st.Ship, ps)
		}
		if !contains(sp.Specs, st.Spec) || !contains(sp.Skins, st.Skin) {
			return Catalog{}, fmt.Errorf("ships.json: стартовый выбор направления %q не подходит кораблю %s", ps, st.Ship)
		}
	}
	return c, nil
}

// DefaultShip — корабль новых игроков.
const DefaultShip = "FTR-001"

var defaultCatalog = func() Catalog {
	c, err := LoadCatalog(shipsJSON)
	if err != nil {
		panic(err)
	}
	return c
}()

// Спека возвращает паспорт корабля.
func (c Catalog) Spec(id string) (ShipSpec, bool) { s, ok := c.Ships[id]; return s, ok }

// AbilityInfo — способность в снимке состояния.
type AbilityInfo struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Energy       float64 `json:"energy_cost"`
	Cooldown     float64 `json:"cooldown"`
	CooldownLeft float64 `json:"cooldown_left"`
	NeedsTarget  bool    `json:"needs_target"`
	Active       bool    `json:"active"`
}

// Event — боевое или визуальное событие для клиентов (сервер решает, клиент только показывает).
type Event struct {
	Type   string  `json:"type"` // fire | hit_shield | hit_hull | death | respawn | boost | intercept | scan | dock
	From   string  `json:"from,omitempty"`
	To     string  `json:"to,omitempty"`
	Amount float64 `json:"amount,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

// Training — неигровые объекты мира: учебные дроны (цели с запасом прочности) и скрытые сигналы (видны после сканирования).
type npc struct {
	ID, Name, Kind, Stance string
	Pos                    Vec
	Hull, HullMax          float64
	Shield, ShieldMax      float64
	Hidden                 bool
	deadUntil              float64
	respawn                float64
}

func defaultNPCs() []*npc {
	return []*npc{
		{ID: "dr1", Name: "Учебный дрон-1", Kind: "hostile", Stance: "hostile", Pos: Vec{40, -30}, Hull: 1500, HullMax: 1500, Shield: 300, ShieldMax: 300, respawn: 10},
		{ID: "dr2", Name: "Учебный дрон-2", Kind: "hostile", Stance: "hostile", Pos: Vec{-45, -20}, Hull: 1500, HullMax: 1500, Shield: 300, ShieldMax: 300, respawn: 10},
		{ID: "sig1", Name: "Слабый сигнал", Kind: "signal", Stance: "neutral", Pos: Vec{90, 60}, Hidden: true},
		{ID: "sig2", Name: "Дрейфующий контейнер", Kind: "signal", Stance: "neutral", Pos: Vec{-70, 85}, Hidden: true},
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func dist(a, b Vec) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// applyDamage наносит урон: сначала щит, затем корпус с учётом брони. Возвращает (по щиту, по корпусу).
func applyDamage(shield, hull *float64, armor, dmg float64) (float64, float64) {
	toShield := math.Min(*shield, dmg)
	*shield -= toShield
	rest := dmg - toShield
	if rest <= 0 {
		return toShield, 0
	}
	eff := rest / (1 + armor/400) // броня снижает урон по корпусу
	toHull := math.Min(*hull, eff)
	*hull -= toHull
	return toShield, toHull
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
