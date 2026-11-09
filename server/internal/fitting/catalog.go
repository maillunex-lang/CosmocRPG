// Package fitting defines server-owned equipment rules without depending on
// World, transport or storage. Reactor power is never combat energy.
package fitting

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Slot struct {
	ID         string   `json:"id"`
	MaxSize    string   `json:"max_size"`
	Categories []string `json:"categories"`
}
type Mount struct {
	PrototypeID string `json:"prototype_id"`
	SlotID      string `json:"slot_id"`
}
type Hull struct {
	ID              string   `json:"id"`
	Class           string   `json:"class"`
	MassBudget      float64  `json:"mass_budget_tons"`
	ReactorPower    float64  `json:"reactor_power_mw"`
	Slots           []Slot   `json:"slots"`
	Starter         []Mount  `json:"starter"`
	Abilities       []string `json:"abilities"`
	Specializations []string `json:"specializations"`
}
type Module struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Family      string         `json:"family"`
	Category    string         `json:"category"`
	Size        string         `json:"size"`
	Rarity      string         `json:"rarity"`
	Mass        float64        `json:"mass_tons"`
	PowerDraw   float64        `json:"power_draw_mw"`
	PowerSupply float64        `json:"power_supply_mw"`
	MinPilot    int            `json:"min_pilot"`
	MinShip     int            `json:"min_ship"`
	Price       int64          `json:"price"`
	Classes     []string       `json:"classes"`
	UniqueGroup string         `json:"unique_group"`
	Abilities   []string       `json:"abilities"`
	Modifiers   []rpg.Modifier `json:"modifiers"`
}
type Document struct {
	Version     int      `json:"version"`
	Preliminary bool     `json:"preliminary"`
	Hulls       []Hull   `json:"hulls"`
	Modules     []Module `json:"modules"`
}
type Catalog struct {
	doc     Document
	hulls   map[string]Hull
	modules map[string]Module
}

//go:embed data/equipment.json
var catalogJSON []byte

//go:embed data/equipment.schema.json
var schemaJSON []byte

var defaultCatalog = sync.OnceValues(func() (*Catalog, error) { return LoadCatalog(catalogJSON) })

func DefaultCatalog() (*Catalog, error) { return defaultCatalog() }
func LoadCatalog(data []byte) (*Catalog, error) {
	sdoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://cosmocrpg.invalid/equipment.json", sdoc); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile("https://cosmocrpg.invalid/equipment.json")
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := schema.Validate(doc); err != nil {
		return nil, fmt.Errorf("equipment schema: %w", err)
	}
	var d Document
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("trailing equipment JSON")
	}
	return NewCatalog(d)
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_-]{0,95}$`)

func ValidID(s string) bool { return idPattern.MatchString(s) }
func SkillID(s string) bool {
	if len(s) != 3 || s[1] < '0' || s[1] > '9' || s[2] < '0' || s[2] > '9' {
		return false
	}
	n := int(s[1]-'0')*10 + int(s[2]-'0')
	return n > 0 && n <= map[byte]int{'W': 16, 'M': 10, 'D': 10, 'E': 10, 'S': 8, 'R': 6, 'I': 6, 'C': 6}[s[0]]
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func size(s string) int     { return map[string]int{"S": 1, "M": 2, "L": 3, "XL": 4}[s] }
func Quality(s string) float64 {
	return map[string]float64{"common": 1, "improved": 1.10, "rare": 1.25, "epic": 1.40, "legendary": 1.60}[s]
}
func KnownFamily(id string) bool {
	for _, xs := range families {
		if slices.Contains(xs, id) {
			return true
		}
	}
	return false
}
func ResearchRequired(rarity string) int {
	return map[string]int{"rare": 1, "epic": 2, "legendary": 3}[rarity]
}

var categories = map[string]bool{"weapon": true, "engine": true, "shield": true, "armor": true, "reactor": true, "electronics": true, "repair": true, "support": true, "drone": true, "industrial": true, "cargo": true}
var families = map[string][]string{"weapon": {"LAS", "KIN", "MSL"}, "engine": {"ENG"}, "shield": {"DEF"}, "armor": {"DEF"}, "reactor": {"REA"}, "electronics": {"ELE"}, "repair": {"SUP"}, "support": {"SUP"}, "drone": {"DRN"}, "industrial": {"IND"}, "cargo": {"CRG"}}

func NewCatalog(d Document) (*Catalog, error) {
	fail := func(f string, args ...any) (*Catalog, error) {
		return nil, fmt.Errorf("equipment catalog: "+f, args...)
	}
	// Own all slices; caller mutation cannot change an already validated catalog.
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	if d.Version != 1 || !d.Preliminary || len(d.Hulls) == 0 || len(d.Modules) == 0 {
		return fail("invalid version/preliminary/empty registry")
	}
	c := &Catalog{doc: d, hulls: map[string]Hull{}, modules: map[string]Module{}}
	classes := map[string]bool{}
	for _, h := range d.Hulls {
		if !ValidID(h.ID) || !ValidID(h.Class) || c.hulls[h.ID].ID != "" || !finite(h.MassBudget) || h.MassBudget <= 0 || !finite(h.ReactorPower) || h.ReactorPower <= 0 {
			return fail("invalid hull %s", h.ID)
		}
		classes[h.Class] = true
		seen := map[string]bool{}
		for _, s := range h.Slots {
			if !ValidID(s.ID) || seen[s.ID] || size(s.MaxSize) == 0 || len(s.Categories) == 0 {
				return fail("invalid slot %s/%s", h.ID, s.ID)
			}
			seen[s.ID] = true
			cats := map[string]bool{}
			for _, cat := range s.Categories {
				if !categories[cat] || cats[cat] {
					return fail("invalid category %s", cat)
				}
				cats[cat] = true
			}
		}
		if len(seen) == 0 {
			return fail("hull %s has no slots", h.ID)
		}
		for _, ids := range [][]string{h.Abilities, h.Specializations} {
			unique := map[string]bool{}
			for _, id := range ids {
				if !ValidID(id) || unique[id] {
					return fail("duplicate/invalid hull reference %s", id)
				}
				unique[id] = true
			}
		}
		if len(h.Specializations) != 2 {
			return fail("hull %s needs two specializations", h.ID)
		}
		for _, id := range h.Abilities {
			if !SkillID(id) {
				return fail("unknown ability %s", id)
			}
		}
		for _, id := range h.Specializations {
			if !strings.HasPrefix(id, h.ID+":") {
				return fail("cross-hull specialization %s", id)
			}
		}
		c.hulls[h.ID] = h
	}
	for _, m := range d.Modules {
		if !ValidID(m.ID) || m.Name == "" || c.modules[m.ID].ID != "" || !categories[m.Category] || !slices.Contains(families[m.Category], m.Family) || size(m.Size) == 0 || Quality(m.Rarity) == 0 {
			return fail("invalid module %s", m.ID)
		}
		if !finite(m.Mass) || m.Mass < 0 || !finite(m.PowerDraw) || m.PowerDraw < 0 || !finite(m.PowerSupply) || m.PowerSupply < 0 || m.PowerSupply > 0 && m.Category != "reactor" || m.MinPilot < 1 || m.MinPilot > 100 || m.MinShip < 1 || m.MinShip > 50 || m.Price < 0 || m.Price > 1e12 {
			return fail("invalid module limits %s", m.ID)
		}
		if m.UniqueGroup != "" && !ValidID(m.UniqueGroup) {
			return fail("invalid unique group %s", m.ID)
		}
		for _, cl := range m.Classes {
			if !classes[cl] {
				return fail("unknown class %s", cl)
			}
		}
		seen := map[string]bool{}
		for _, id := range m.Abilities {
			if !SkillID(id) || seen[id] {
				return fail("bad ability %s", id)
			}
			seen[id] = true
		}
		modifierKeys := map[string]bool{}
		for _, mod := range m.Modifiers {
			key := string(mod.Stat) + ":" + string(mod.Op)
			if !rpg.ValidStat(mod.Stat) || !rpg.ValidOp(mod.Op) || !finite(mod.Value) || mod.Value < -0.9 || mod.Value > 1e6 || mod.Source != "" || mod.Layer != rpg.LayerModules || modifierKeys[key] {
				return fail("invalid modifier %s", m.ID)
			}
			modifierKeys[key] = true
		}
		c.modules[m.ID] = m
	}
	for _, h := range d.Hulls {
		items := map[string]Item{}
		slots := map[string]string{}
		for i, m := range h.Starter {
			key := fmt.Sprintf("stock-%d", i)
			items[key] = Item{ID: key, Owner: "catalog", Hull: h.ID, Prototype: m.PrototypeID}
			if slots[m.SlotID] != "" {
				return fail("duplicate starter slot %s", h.ID)
			}
			slots[m.SlotID] = key
		}
		if _, err := c.Validate("catalog", h.ID, 1, 1, items, slots); err != nil {
			return fail("starter %s: %v", h.ID, err)
		}
	}
	return c, nil
}
func clone[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}
func (c *Catalog) Hull(id string) (Hull, bool)     { h, ok := c.hulls[id]; return clone(h), ok }
func (c *Catalog) Module(id string) (Module, bool) { m, ok := c.modules[id]; return clone(m), ok }
func (c *Catalog) Document() Document              { return clone(c.doc) }

func (c *Catalog) HasHull(id string) bool { _, ok := c.hulls[id]; return ok }
