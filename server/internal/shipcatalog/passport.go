package shipcatalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// PassportCatalog is a proposed v2 data contract, not an active world catalog.
// Base preserves the complete legacy passport; the new fields do not add stats
// to it until equipment/effect integration explicitly defines those layers.
type PassportCatalog struct {
	Version int            `json:"version"`
	Ships   []ShipPassport `json:"ships"`
}

type ShipPassport struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	ClassID           string             `json:"class_id"`
	Description       string             `json:"description"`
	Base              world.ShipSpec     `json:"base"`
	PreliminaryFields []string           `json:"preliminary_fields"`
	MassTons          float64            `json:"mass_tons"`
	Resistances       map[string]float64 `json:"resistances"`
	Acceleration      float64            `json:"acceleration_units_s2"`
	TurnRate          float64            `json:"turn_rate_deg_s"`
	Signature         float64            `json:"signature"`
	Sensors           float64            `json:"sensor_range_units"`
	ReactorPower      float64            `json:"reactor_power_mw"`
	ModuleMassBudget  float64            `json:"module_mass_budget_tons"`
	Slots             []PassportSlot     `json:"slots"`
	Specializations   []string           `json:"specializations"`
	AllowedAbilities  []string           `json:"allowed_abilities"`
	StarterKit        []StarterModule    `json:"starter_kit"`
	Economy           PassportEconomy    `json:"economy"`
	NPC               PassportNPC        `json:"npc"`
	Visual            PassportVisual     `json:"visual"`
}

type PassportSlot struct {
	ID         string   `json:"id"`
	Group      string   `json:"group"`
	MaxSize    string   `json:"max_size"`
	Categories []string `json:"categories"`
}

// StarterModule is a self-contained proposed starter blueprint. Family is a
// classification, not a reference to a module ID that does not yet exist.
type StarterModule struct {
	ID         string   `json:"id"`
	SlotID     string   `json:"slot_id"`
	Family     string   `json:"family"`
	Category   string   `json:"category"`
	Size       string   `json:"size"`
	MassTons   float64  `json:"mass_tons"`
	PowerMW    float64  `json:"power_mw"`
	AbilityIDs []string `json:"ability_ids"`
}

type PassportEconomy struct {
	Acquisition      string  `json:"acquisition"`
	PriceCredits     int64   `json:"price_credits"`
	PilotLevel       int     `json:"pilot_level"`
	RepairPerHull    float64 `json:"repair_credits_per_hull"`
	InsurancePremium int64   `json:"insurance_premium_credits"`
	InsurancePayout  float64 `json:"insurance_payout_fraction"`
}

type PassportNPC struct {
	Role            string  `json:"role"`
	ThreatTier      int     `json:"threat_tier"`
	AbilityInterval float64 `json:"ability_interval_s"`
	MaxDrones       int     `json:"max_drones"`
}

type PassportVisual struct {
	PlaceholderID        string          `json:"placeholder_id"`
	Silhouette           string          `json:"silhouette"`
	ForwardAxis          string          `json:"forward_axis"`
	UpAxis               string          `json:"up_axis"`
	CollisionHalfExtents [3]float64      `json:"collision_half_extents_m"`
	Mounts               []PassportMount `json:"mounts"`
}

type PassportMount struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Position [3]float64 `json:"position_m"`
}

//go:embed data/rpg_passports.schema.json
var passportSchemaJSON []byte

var passportSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	const root = "https://cosmocrpg.invalid/schemas/"
	for name, data := range map[string][]byte{"rpg-passports-v1.json": passportSchemaJSON, "ship-patch-v1.json": SchemaJSON()} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if err := c.AddResource(root+name, doc); err != nil {
			return nil, err
		}
	}
	return c.Compile(root + "rpg-passports-v1.json")
})

// LoadPassportCatalog validates schema and all currently available references.
// Ability IDs refer to the 72 IDs in the master specification; effect/module
// implementation and unlock reachability remain separate SR-012/013 tasks.
func LoadPassportCatalog(data []byte) (PassportCatalog, error) {
	fail := func(err error) (PassportCatalog, error) {
		return PassportCatalog{}, fmt.Errorf("rpg passports: %w", err)
	}
	schema, err := passportSchema()
	if err != nil {
		return fail(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fail(err)
	}
	if err := schema.Validate(doc); err != nil {
		return fail(err)
	}
	var out PassportCatalog
	if err := json.Unmarshal(data, &out); err != nil {
		return fail(err)
	}
	identities, err := DefaultIdentityRegistry()
	if err != nil {
		return fail(err)
	}
	byID := map[string]ShipIdentity{}
	for _, ship := range identities.Ships {
		byID[ship.ID] = ship
	}
	legacyIDs := map[string]bool{}
	for _, ship := range world.DefaultDoc().Ships {
		legacyIDs[ship.ID] = true
	}
	seen, visuals := map[string]bool{}, map[string]bool{}
	for _, ship := range out.Ships {
		identity, ok := byID[ship.ID]
		if !ok || seen[ship.ID] {
			return fail(fmt.Errorf("unknown or duplicate ship %s", ship.ID))
		}
		seen[ship.ID] = true
		if !legacyIDs[ship.ID] && !containsCategory(ship.PreliminaryFields, "base") {
			return fail(fmt.Errorf("new base must be preliminary: %s", ship.ID))
		}
		if ship.Name != identity.Name || ship.ClassID != identity.ClassID || ship.Base.ID != ship.ID {
			return fail(fmt.Errorf("identity mismatch for %s", ship.ID))
		}
		expected := map[string]bool{}
		for _, spec := range identity.Specializations {
			expected[spec.ID] = true
		}
		for _, id := range ship.Specializations {
			if !expected[id] {
				return fail(fmt.Errorf("unknown specialization %s for %s", id, ship.ID))
			}
			delete(expected, id)
		}
		if len(expected) != 0 {
			return fail(fmt.Errorf("missing specializations for %s", ship.ID))
		}
		if errs := ValidateShip(ship.Base); len(errs) > 0 {
			return fail(fmt.Errorf("%s base: %v", ship.ID, errs))
		}

		for id, ability := range ship.Base.Abilities {
			if ability.Spec != "" && !containsCategory(ship.Base.Specs, ability.Spec) {
				return fail(fmt.Errorf("%s: unknown base specialization for ability %s", ship.ID, id))
			}
		}
		if err := validatePassportEquipment(ship); err != nil {
			return fail(fmt.Errorf("%s: %w", ship.ID, err))
		}
		if visuals[ship.Visual.PlaceholderID] {
			return fail(fmt.Errorf("duplicate placeholder %s", ship.Visual.PlaceholderID))
		}
		visuals[ship.Visual.PlaceholderID] = true
		if err := validatePassportVisual(ship); err != nil {
			return fail(fmt.Errorf("%s: %w", ship.ID, err))
		}
	}
	return out, nil
}

// CheckLegacyPassportBases prevents a seed-data update from silently changing
// any old field or ability value. It never overwrites active/admin catalogs.
func CheckLegacyPassportBases(c PassportCatalog, legacy world.Doc) error {
	byID := map[string]world.ShipSpec{}
	for _, ship := range c.Ships {
		byID[ship.ID] = ship.Base
	}
	for _, old := range legacy.Ships {
		if got, ok := byID[old.ID]; !ok || !reflect.DeepEqual(got, old) {
			return fmt.Errorf("legacy passport changed: %s", old.ID)
		}
	}
	return nil
}

func validatePassportEquipment(ship ShipPassport) error {
	slots := map[string]PassportSlot{}
	groups := map[string]bool{}
	allowedGroups := map[string]map[string]bool{
		"weapon": {"weapon": true}, "defensive": {"shield": true, "armor": true},
		"technical": {"engine": true, "reactor": true, "electronics": true, "repair": true, "support": true, "cargo": true},
		"special":   {"drone": true, "industrial": true, "support": true, "electronics": true, "cargo": true},
	}
	for _, slot := range ship.Slots {
		if _, ok := slots[slot.ID]; ok {
			return fmt.Errorf("duplicate slot %s", slot.ID)
		}
		for _, category := range slot.Categories {
			if !allowedGroups[slot.Group][category] {
				return fmt.Errorf("category %s incompatible with slot group %s", category, slot.Group)
			}
		}
		slots[slot.ID] = slot
		groups[slot.Group] = true
	}
	for _, group := range []string{"weapon", "defensive", "technical", "special"} {
		if !groups[group] {
			return fmt.Errorf("missing slot group %s", group)
		}
	}
	sizes := map[string]int{"S": 1, "M": 2, "L": 3, "XL": 4}
	families := map[string]string{"weapon": "LAS KIN MSL", "engine": "ENG", "shield": "DEF", "armor": "DEF", "reactor": "REA", "electronics": "ELE", "repair": "SUP", "support": "SUP", "drone": "DRN", "industrial": "IND", "cargo": "CRG"}
	allowed := map[string]bool{}
	for _, id := range ship.AllowedAbilities {
		allowed[id] = true
	}
	occupied, modules, systems := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var mass, power float64
	for _, module := range ship.StarterKit {
		slot, ok := slots[module.SlotID]
		if !ok {
			return fmt.Errorf("unknown starter slot %s", module.SlotID)
		}
		if occupied[module.SlotID] || modules[module.ID] {
			return fmt.Errorf("duplicate starter slot or module %s", module.ID)
		}
		occupied[module.SlotID] = true
		modules[module.ID] = true
		if sizes[module.Size] > sizes[slot.MaxSize] {
			return fmt.Errorf("starter size exceeds slot %s", slot.ID)
		}
		if !containsCategory(slot.Categories, module.Category) {
			return fmt.Errorf("starter category incompatible with slot %s", slot.ID)
		}
		if !containsCategory(strings.Fields(families[module.Category]), module.Family) {
			return fmt.Errorf("starter family incompatible with category %s", module.Category)
		}
		for _, id := range module.AbilityIDs {
			if !allowed[id] {
				return fmt.Errorf("starter ability not allowed: %s", id)
			}
		}
		mass += module.MassTons
		power += module.PowerMW
		systems[module.Category] = true
	}
	if mass > ship.ModuleMassBudget+1e-9 {
		return fmt.Errorf("starter mass exceeds budget")
	}
	if power > ship.ReactorPower+1e-9 {
		return fmt.Errorf("starter power exceeds reactor budget")
	}
	for _, category := range []string{"weapon", "engine", "reactor", "electronics"} {
		if !systems[category] {
			return fmt.Errorf("missing starter system %s", category)
		}
	}
	if ship.Base.Shield > 0 && !systems["shield"] {
		return fmt.Errorf("missing starter shield")
	}
	return nil
}

func containsCategory(list []string, value string) bool {
	for _, x := range list {
		if x == value {
			return true
		}
	}
	return false
}

func validatePassportVisual(ship ShipPassport) error {
	// Base dimensions are [length, width, height]. Visual coordinates are Godot
	// metres: +Y up, -Z forward. World 2D units do not inherit these dimensions.
	half := [3]float64{ship.Base.DimensionsM[1] / 2, ship.Base.DimensionsM[2] / 2, ship.Base.DimensionsM[0] / 2}
	for axis, extent := range ship.Visual.CollisionHalfExtents {
		if extent > half[axis] {
			return fmt.Errorf("collision exceeds ship dimensions on axis %d", axis)
		}
	}
	mounts := map[string]bool{}
	kinds := map[string]int{}
	for _, mount := range ship.Visual.Mounts {
		if mounts[mount.ID] {
			return fmt.Errorf("duplicate mount %s", mount.ID)
		}
		mounts[mount.ID] = true
		kinds[mount.Kind]++
		for axis, pos := range mount.Position {
			if pos < -half[axis] || pos > half[axis] {
				return fmt.Errorf("mount %s outside ship dimensions", mount.ID)
			}
		}
	}
	if kinds["weapon"] < 1 || kinds["engine"] < ship.Base.Engines || kinds["vfx"] < 1 {
		return fmt.Errorf("missing weapon/engine/vfx mounts")
	}
	return nil
}

//go:embed data/rpg_passports.json
var passportsJSON []byte

// DefaultPassportCatalog returns an independently decoded draft catalog. It
// verifies every legacy base against the existing seed and has no publish hook.
func DefaultPassportCatalog() (PassportCatalog, error) {
	c, err := LoadPassportCatalog(passportsJSON)
	if err != nil {
		return PassportCatalog{}, err
	}
	if err := CheckLegacyPassportBases(c, world.DefaultDoc()); err != nil {
		return PassportCatalog{}, err
	}
	return c, nil
}
