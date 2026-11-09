// Package worldgen — детерминированная генерация наполнения звёздных систем (зоны PvE, добыча, торговля, исследование,
// планеты и декор) и конфигурация мира (враги, ресурсы, визуальные параметры). Серверные интерактивные объекты
// и клиентский декор разделены: декор клиент строит сам по seed и параметрам, синхронизации не требует.
package worldgen

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

//go:embed data/world.json
var defaultJSON []byte

// SystemParams — параметры системы, из которых по seed строится раскладка.
// Jump — точка гиперперехода в другую систему (Э-1, Решение №30).
type Jump struct {
	To  string     `json:"to"`
	Pos [2]float64 `json:"pos"`
}

type SystemParams struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Background     string     `json:"background"`
	Seed           int64      `json:"seed"`
	Radius         float64    `json:"radius"`
	Specialization string     `json:"specialization"`          // mixed | mining | trade | pirate | explore
	Faction        string     `json:"faction"`                 // фракция системы (Галактика R30)
	Hub            [2]float64 `json:"hub"`                     // главная станция
	Difficulty     int        `json:"difficulty"`              // 1..3: состав PvE
	Planets        int        `json:"planets,omitempty"`       // число планет (по умолчанию 1)
	Belts          int        `json:"belts,omitempty"`         // астероидных поясов
	PirateZones    int        `json:"pirate_zones,omitempty"`  // пиратских территорий
	Stations       int        `json:"stations,omitempty"`      // дополнительных торговых станций
	Jumps          []Jump     `json:"jumps,omitempty"`         // гиперпереходы в другие системы (Э-1)
	Anomalies      int        `json:"anomalies,omitempty"`     // аномалий
	Derelicts      int        `json:"derelicts,omitempty"`     // заброшенных станций
	Caches         int        `json:"caches,omitempty"`        // охраняемых тайников
	DebrisFields   int        `json:"debris_fields,omitempty"` // полей обломков
	Features       []string   `json:"features"`                // asteroid_belt, gas_giant, pirate_territory, anomalies, busy_lanes, debris_field, derelict, guarded_cache
}

// EnemyCfg — параметры типа врага (лёгкая серверная логика, без уникальных ИИ-капитанов).
type EnemyCfg struct {
	Name        string  `json:"name"`
	Hull        float64 `json:"hull"`
	Shield      float64 `json:"shield"`
	Armor       float64 `json:"armor"`
	Speed       float64 `json:"speed"`
	ChaseSpeed  float64 `json:"chase_speed"`
	Aggro       float64 `json:"aggro"`
	Leash       float64 `json:"leash"`
	AttackRange float64 `json:"attack_range"`
	Damage      float64 `json:"damage"`
	Period      float64 `json:"period"`
	Respawn     float64 `json:"respawn"`
	Reward      float64 `json:"reward"`
}

// ResourceCfg — тип ресурса.
type ResourceCfg struct {
	Name        string  `json:"name"`
	Module      string  `json:"module"` // требуемый добывающий модуль
	Rarity      string  `json:"rarity"`
	Amount      float64 `json:"amount"`
	RegenPerMin float64 `json:"regen_per_min"`
	Price       float64 `json:"price"` // кредитов за единицу при продаже на станции
	Yield       float64 `json:"yield"` // справочно: выход за цикл по умолчанию (фактический задаёт модуль корабля)
}

// Preset — ограничения качества графики (клиент берёт значения по выбранному пресету).
type Preset struct {
	ChunkRadius       int  `json:"chunk_radius"`
	AsteroidsPerChunk int  `json:"asteroids_per_chunk"`
	DebrisPerChunk    int  `json:"debris_per_chunk"`
	DustParticles     int  `json:"dust_particles"`
	Comets            bool `json:"comets"`
	Ships             int  `json:"ships"`
}

// Decor — множители и флаги декоративных элементов.
type Decor struct {
	Asteroids float64 `json:"asteroids"`
	Debris    float64 `json:"debris"`
	Comets    float64 `json:"comets"`
	Ships     float64 `json:"ships"`
	Planets   bool    `json:"planets"`
	AnomalyFX bool    `json:"anomaly_fx"`
}

// Visual — графические параметры окружения, редактируются в админке.
type Visual struct {
	Twinkle    float64           `json:"twinkle"`
	Dust       float64           `json:"dust"`
	StarLayers int               `json:"star_layers"`
	Decor      Decor             `json:"decor"`
	Presets    map[string]Preset `json:"presets"`
}

// Config — конфигурация мира (версионируется и применяется на лету).
type Config struct {
	Systems   []SystemParams         `json:"systems"`
	Enemies   map[string]EnemyCfg    `json:"enemies"`
	Resources map[string]ResourceCfg `json:"resources"`
	Visual    Visual                 `json:"visual"`
}

// DefaultConfig возвращает встроенную конфигурацию.
func DefaultConfig() Config {
	var c Config
	if err := json.Unmarshal(defaultJSON, &c); err != nil {
		panic(err)
	}
	return c
}

// Tiers — известные уровни врагов (по возрастанию силы).
var Tiers = []string{"normal", "reinforced", "elite", "miniboss", "worldboss"}

var specs = map[string]bool{"mixed": true, "mining": true, "trade": true, "pirate": true, "explore": true}
var featureSet = map[string]bool{"asteroid_belt": true, "gas_giant": true, "pirate_territory": true, "anomalies": true, "busy_lanes": true, "debris_field": true, "derelict": true, "guarded_cache": true, "boss_lair": true}
var rarities = map[string]bool{"common": true, "uncommon": true, "rare": true, "epic": true}

// Validate проверяет конфигурацию: диапазоны, ссылки и возможность построить раскладки всех систем без нарушений правил размещения.
func (c Config) Validate() []string {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	between := func(name string, v, lo, hi float64) {
		if v < lo || v > hi || v != v {
			add("%s: значение %v вне диапазона [%v, %v]", name, v, lo, hi)
		}
	}
	if len(c.Systems) == 0 || len(c.Systems) > 64 {
		add("systems: от 1 до 64 систем")
	}
	seen := map[string]bool{}
	for _, s := range c.Systems {
		if s.ID == "" || len(s.ID) > 24 || seen[s.ID] {
			add("system %q: пустой, длинный или повторяющийся идентификатор", s.ID)
		}
		seen[s.ID] = true
		if s.Name == "" || len([]rune(s.Name)) > 60 {
			add("system %s: name 1-60 символов", s.ID)
		}
		between("system "+s.ID+".radius", s.Radius, 200, 5000)
		between("system "+s.ID+".difficulty", float64(s.Difficulty), 1, 3)
		for name, n := range map[string]int{"planets": s.Planets, "belts": s.Belts, "pirate_zones": s.PirateZones, "stations": s.Stations, "anomalies": s.Anomalies, "derelicts": s.Derelicts, "caches": s.Caches, "debris_fields": s.DebrisFields} {
			between("system "+s.ID+"."+name, float64(n), 0, 6)
		}
		if !specs[s.Specialization] {
			add("system %s: неизвестная специализация %q", s.ID, s.Specialization)
		}
		for _, f := range s.Features {
			if !featureSet[f] {
				add("system %s: неизвестная особенность %q", s.ID, f)
			}
		}
		if hypot(s.Hub[0], s.Hub[1]) > s.Radius*0.5 {
			add("system %s: главная станция слишком далеко от центра", s.ID)
		}
	}
	for _, t := range Tiers {
		e, ok := c.Enemies[t]
		if !ok {
			add("enemies: нет уровня %q", t)
			continue
		}
		p := "enemies." + t + "."
		if e.Name == "" {
			add("%sname: пусто", p)
		}
		between(p+"hull", e.Hull, 50, 1e6)
		between(p+"shield", e.Shield, 0, 1e6)
		between(p+"armor", e.Armor, 0, 5000)
		between(p+"speed", e.Speed, 0.5, 40)
		between(p+"chase_speed", e.ChaseSpeed, e.Speed, 60)
		between(p+"aggro", e.Aggro, 5, 400)
		between(p+"leash", e.Leash, e.Aggro, 800)
		between(p+"attack_range", e.AttackRange, 5, 200)
		between(p+"damage", e.Damage, 0, 5000)
		between(p+"period", e.Period, 0.2, 30)
		between(p+"respawn", e.Respawn, 5, 86400)
		between(p+"reward", e.Reward, 0, 1e6)
	}
	for _, id := range sortedKeys(c.Resources) {
		r := c.Resources[id]
		p := "resources." + id + "."
		if r.Name == "" || r.Module == "" {
			add("%sname и module обязательны", p)
		}
		if !rarities[r.Rarity] {
			add("%srarity: common, uncommon, rare, epic", p)
		}
		between(p+"amount", r.Amount, 10, 1e6)
		between(p+"regen_per_min", r.RegenPerMin, 0, 10000)
		between(p+"yield", r.Yield, 1, 1000)
		between(p+"price", r.Price, 0, 100000)
	}
	v := c.Visual
	between("visual.twinkle", v.Twinkle, 0, 1)
	between("visual.dust", v.Dust, 0, 1)
	between("visual.star_layers", float64(v.StarLayers), 0, 2)
	between("visual.decor.asteroids", v.Decor.Asteroids, 0, 3)
	between("visual.decor.debris", v.Decor.Debris, 0, 3)
	between("visual.decor.comets", v.Decor.Comets, 0, 3)
	between("visual.decor.ships", v.Decor.Ships, 0, 3)
	for _, q := range []string{"low", "medium", "high"} {
		p, ok := v.Presets[q]
		if !ok {
			add("visual.presets: нет пресета %q", q)
			continue
		}
		n := "visual.presets." + q + "."
		between(n+"chunk_radius", float64(p.ChunkRadius), 1, 3)
		between(n+"asteroids_per_chunk", float64(p.AsteroidsPerChunk), 0, 16)
		between(n+"debris_per_chunk", float64(p.DebrisPerChunk), 0, 8)
		between(n+"dust_particles", float64(p.DustParticles), 0, 128)
		between(n+"ships", float64(p.Ships), 0, 8)
	}
	if len(errs) == 0 {
		for _, s := range c.Systems {
			l := Generate(s, c)
			for _, e := range l.Validate() {
				add("system %s: %s", s.ID, e)
			}
		}
	}
	sort.Strings(errs)
	return errs
}

// Parse разбирает конфигурацию (неизвестные поля отклоняются) и проверяет её.
func Parse(data []byte) (Config, error) {
	var c Config
	dec := json.NewDecoder(bytesReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("конфигурация мира: %w", err)
	}
	if errs := c.Validate(); len(errs) > 0 {
		return Config{}, errors.New(errs[0])
	}
	return c, nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
