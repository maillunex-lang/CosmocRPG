// Package shipcatalog — каталог кораблей: версии, редактирование параметров в админке и ZIP-патчи (формат Ship_Patch_Pack_v1),
// которые можно готовить через ChatGPT. Все значения проверяются по диапазонам и схеме; применение атомарно, есть откат.
package shipcatalog

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Допустимые диапазоны параметров (защита от опечаток и злоупотреблений; границы сами можно пересмотреть).
type rng struct{ lo, hi float64 }

var shipRanges = map[string]rng{
	"hull": {100, 100000}, "shield": {0, 100000}, "armor": {0, 2000}, "energy": {10, 10000}, "cargo": {0, 100000},
	"speed": {1, 60}, "shield_regen": {0, 1000}, "shield_regen_delay": {0, 60}, "energy_regen": {0, 1000},
	"crew": {1, 20}, "engines": {1, 8},
}

var abilityRanges = map[string]rng{
	"damage": {0, 5000}, "shots": {0, 10}, "range": {0, 500}, "period": {0, 30}, "energy": {0, 1000}, "cooldown": {0, 600},
	"duration": {0, 120}, "speed_mul": {0, 5}, "max_dist": {0, 300}, "min_dist": {0, 100}, "damage_mul": {0, 20}, "radius": {0, 500}, "reveal": {0, 300},
}

// ValidateShip проверяет паспорт корабля: диапазоны, перечисления и обязательные способности.
func ValidateShip(s world.ShipSpec) []string {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	if len(s.ID) < 3 || len(s.ID) > 24 {
		add("id: 3-24 символа")
	}
	if s.Name == "" || len([]rune(s.Name)) > 60 {
		add("name: 1-60 символов")
	}
	if len([]rune(s.Class)) > 60 || len([]rune(s.Role)) > 60 || len([]rune(s.Weapons)) > 120 {
		add("class, role, weapons: слишком длинный текст")
	}
	check := func(name string, v float64, r rng) {
		if v < r.lo || v > r.hi || v != v {
			add("%s: значение %v вне диапазона [%v, %v]", name, v, r.lo, r.hi)
		}
	}
	check("hull", s.Hull, shipRanges["hull"])
	check("shield", s.Shield, shipRanges["shield"])
	check("armor", s.Armor, shipRanges["armor"])
	check("energy", s.Energy, shipRanges["energy"])
	check("cargo", s.Cargo, shipRanges["cargo"])
	check("speed", s.Speed, shipRanges["speed"])
	check("shield_regen", s.ShieldRegen, shipRanges["shield_regen"])
	check("shield_regen_delay", s.ShieldRegenDelay, shipRanges["shield_regen_delay"])
	check("energy_regen", s.EnergyRegen, shipRanges["energy_regen"])
	check("crew", float64(s.Crew), shipRanges["crew"])
	check("engines", float64(s.Engines), shipRanges["engines"])
	for i, d := range s.DimensionsM {
		check(fmt.Sprintf("dimensions_m[%d]", i), d, rng{0.5, 500})
	}
	if len(s.Skins) == 0 || len(s.Skins) > 8 {
		add("skins: от 1 до 8 обликов")
	}
	if len(s.Specs) == 0 || len(s.Specs) > 8 {
		add("specs: от 1 до 8 специализаций")
	}
	for _, k := range append(append([]string{}, s.Skins...), s.Specs...) {
		if len(k) == 0 || len(k) > 24 {
			add("облик или специализация %q: 1-24 символа", k)
		}
	}
	for _, sp := range s.Specs {
		if sp == s.DefaultSpec {
			goto okSpec
		}
	}
	add("default_spec: должна быть из списка specs")
okSpec:
	for _, need := range []string{"pulse", "boost"} {
		if _, ok := s.Abilities[need]; !ok {
			add("abilities.%s: обязательная способность", need)
		}
	}
	for _, name := range sortedKeys(s.Abilities) {
		a := s.Abilities[name]
		if len(name) > 24 {
			add("abilities.%s: слишком длинное имя", name)
		}
		for f, v := range map[string]float64{"damage": a.Damage, "shots": float64(a.Shots), "range": a.Range, "period": a.Period, "energy": a.Energy,
			"cooldown": a.Cooldown, "duration": a.Duration, "speed_mul": a.SpeedMul, "max_dist": a.MaxDist, "min_dist": a.MinDist,
			"damage_mul": a.DamageMul, "radius": a.Radius, "reveal": a.Reveal} {
			check("abilities."+name+"."+f, v, abilityRanges[f])
		}
		if name == "pulse" && (a.Period <= 0 || a.Shots < 1) {
			add("abilities.pulse: нужны period > 0 и shots >= 1")
		}
		if name == "boost" && (a.SpeedMul < 1 || a.Duration <= 0) {
			add("abilities.boost: нужны speed_mul >= 1 и duration > 0")
		}
	}
	sort.Strings(errs)
	return errs
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SchemaJSON возвращает JSON Schema паспорта корабля с теми же диапазонами (кладётся в ZIP-патчи для ChatGPT).
func SchemaJSON() []byte {
	num := func(r rng) map[string]any { return map[string]any{"type": "number", "minimum": r.lo, "maximum": r.hi} }
	intg := func(r rng) map[string]any { return map[string]any{"type": "integer", "minimum": r.lo, "maximum": r.hi} }
	str := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	strs := map[string]any{"type": "array", "items": str(24), "minItems": 1, "maxItems": 8, "uniqueItems": true}
	ability := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"name": str(60), "spec": map[string]any{"type": "string", "maxLength": 24},
		"damage": num(abilityRanges["damage"]), "shots": intg(abilityRanges["shots"]), "range": num(abilityRanges["range"]), "period": num(abilityRanges["period"]),
		"energy": num(abilityRanges["energy"]), "cooldown": num(abilityRanges["cooldown"]), "duration": num(abilityRanges["duration"]),
		"speed_mul": num(abilityRanges["speed_mul"]), "max_dist": num(abilityRanges["max_dist"]), "min_dist": num(abilityRanges["min_dist"]),
		"damage_mul": num(abilityRanges["damage_mul"]), "radius": num(abilityRanges["radius"]), "reveal": num(abilityRanges["reveal"]),
	}}
	props := map[string]any{
		"id": map[string]any{"type": "string", "pattern": "^[A-Z]{3}-[A-Z0-9]{3,12}$"}, "name": str(60), "class": map[string]any{"type": "string", "maxLength": 60},
		"role": map[string]any{"type": "string", "maxLength": 60}, "temporary": map[string]any{"type": "boolean"}, "specs": strs, "default_spec": str(24), "skins": strs,
		"dimensions_m": map[string]any{"type": "array", "items": num(rng{0.5, 500}), "minItems": 3, "maxItems": 3},
		"crew":         intg(shipRanges["crew"]), "engines": intg(shipRanges["engines"]), "weapons": map[string]any{"type": "string", "maxLength": 120},
		"hull": num(shipRanges["hull"]), "shield": num(shipRanges["shield"]), "armor": num(shipRanges["armor"]), "energy": num(shipRanges["energy"]),
		"cargo": num(shipRanges["cargo"]), "speed": num(shipRanges["speed"]), "shield_regen": num(shipRanges["shield_regen"]),
		"shield_regen_delay": num(shipRanges["shield_regen_delay"]), "energy_regen": num(shipRanges["energy_regen"]),
		"abilities": map[string]any{"type": "object", "minProperties": 2, "additionalProperties": ability},
	}
	req := []string{"id", "name", "specs", "default_spec", "skins", "hull", "shield", "armor", "energy", "cargo", "speed", "abilities"}
	b, _ := json.MarshalIndent(map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Паспорт корабля (Ship_Patch_Pack_v1)", "type": "object",
		"properties": props, "required": req, "additionalProperties": false,
	}, "", "  ")
	return append(b, '\n')
}
