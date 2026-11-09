package fitting_test

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
)

func catalog(t *testing.T) *fitting.Catalog {
	t.Helper()
	c, err := fitting.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func stock(t *testing.T, c *fitting.Catalog) (map[string]fitting.Item, map[string]string) {
	t.Helper()
	h, _ := c.Hull("FTR-001")
	items, slots := map[string]fitting.Item{}, map[string]string{}
	for i, m := range h.Starter {
		id := fmt.Sprintf("item-%d", i)
		items[id] = fitting.Item{ID: id, Owner: "owner", Hull: h.ID, Prototype: m.PrototypeID}
		slots[m.SlotID] = id
	}
	return items, slots
}

func TestCatalogMatchesPassportsAndReachesEverySkill(t *testing.T) {
	c := catalog(t)
	p, err := shipcatalog.DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	d := c.Document()
	if len(d.Hulls) != 15 {
		t.Fatal(len(d.Hulls))
	}
	families, sizes, rarities, reached := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range p.Ships {
		h, ok := c.Hull(p.ID)
		if !ok || h.Class != p.ClassID || h.MassBudget != p.ModuleMassBudget || h.ReactorPower != p.ReactorPower || len(h.Slots) != len(p.Slots) || !reflect.DeepEqual(h.Specializations, p.Specializations) || !reflect.DeepEqual(h.Abilities, p.AllowedAbilities) {
			t.Fatalf("passport drift %s", p.ID)
		}
		for i, s := range p.Slots {
			got := h.Slots[i]
			if got.ID != s.ID || got.MaxSize != s.MaxSize || !reflect.DeepEqual(got.Categories, s.Categories) {
				t.Fatalf("slot drift %s/%s", h.ID, s.ID)
			}
		}
		for i, m := range p.StarterKit {
			got, ok := c.Module(m.ID)
			if !ok || got.Mass != m.MassTons || got.PowerDraw != m.PowerMW || got.Size != m.Size || !reflect.DeepEqual(got.Abilities, m.AbilityIDs) || h.Starter[i].SlotID != m.SlotID {
				t.Fatalf("starter drift %s/%s", h.ID, m.ID)
			}
			if len(got.Modifiers) != 0 {
				t.Fatal("starter double-counts legacy base stats")
			}
		}
		for _, m := range d.Modules {
			families[m.Family] = true
			sizes[m.Size] = true
			rarities[m.Rarity] = true
			for _, slot := range h.Slots {
				item := fitting.Item{ID: "probe", Owner: "owner", Prototype: m.ID}
				if _, err := c.Validate("owner", h.ID, 100, 50, map[string]fitting.Item{"probe": item}, map[string]string{slot.ID: "probe"}); err != nil {
					continue
				}
				for _, id := range m.Abilities {
					if slices.Contains(h.Abilities, id) {
						reached[id] = true
					}
				}
			}
		}
	}
	if len(families) != 11 || len(sizes) != 4 || len(rarities) != 5 || len(reached) != 72 {
		t.Fatalf("incomplete families/sizes/rarities/reachability: %d/%d/%d/%d", len(families), len(sizes), len(rarities), len(reached))
	}
	// Public copies cannot poison the shared catalog used for authoritative checks.
	h, _ := c.Hull("FTR-001")
	h.Slots[0].Categories[0] = "broken"
	d.Modules[0].Abilities[0] = "W99"
	again, _ := c.Hull("FTR-001")
	m, _ := c.Module("LAS-S-START")
	if again.Slots[0].Categories[0] != "weapon" || m.Abilities[0] != "W01" {
		t.Fatal("catalog shares mutable slices")
	}
}

func TestCatalogRejectsMalformedRules(t *testing.T) {
	c := catalog(t)
	for _, tt := range []struct {
		name string
		edit func(*fitting.Document)
	}{
		{"duplicate module", func(d *fitting.Document) { d.Modules = append(d.Modules, d.Modules[0]) }},
		{"duplicate hull", func(d *fitting.Document) { d.Hulls = append(d.Hulls, d.Hulls[0]) }},
		{"unknown slot category", func(d *fitting.Document) { d.Hulls[0].Slots[0].Categories = []string{"energy"} }},
		{"unknown ability", func(d *fitting.Document) { d.Modules[0].Abilities = []string{"W99"} }},
		{"cross hull spec", func(d *fitting.Document) { d.Hulls[0].Specializations[0] = "COR-001:hunter" }},
		{"NaN", func(d *fitting.Document) { d.Modules[0].Mass = math.NaN() }},
		{"infinity", func(d *fitting.Document) { d.Hulls[0].ReactorPower = math.Inf(1) }},
		{"negative draw", func(d *fitting.Document) { d.Modules[0].PowerDraw = -1 }},
		{"power from weapon", func(d *fitting.Document) { d.Modules[0].PowerSupply = 999 }},
		{"unknown class", func(d *fitting.Document) { d.Modules[0].Classes = []string{"invented"} }},
		{"unknown rarity", func(d *fitting.Document) { d.Modules[0].Rarity = "mythical" }},
		{"invalid level", func(d *fitting.Document) { d.Modules[0].MinShip = 51 }},
		{"unknown stat", func(d *fitting.Document) { d.Modules[len(d.Modules)-1].Modifiers[0].Stat = "random" }},
		{"duplicate modifier", func(d *fitting.Document) {
			m := &d.Modules[len(d.Modules)-1]
			m.Modifiers = append(m.Modifiers, m.Modifiers[0])
		}},
		{"missing stock prototype", func(d *fitting.Document) { d.Hulls[0].Starter[0].PrototypeID = "MISSING" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := c.Document()
			tt.edit(&d)
			if _, err := fitting.NewCatalog(d); err == nil {
				t.Fatal("invalid rule accepted")
			}
		})
	}
	base, _ := json.Marshal(c.Document())
	var obj map[string]any
	_ = json.Unmarshal(base, &obj)
	obj["unknown"] = true
	bad, _ := json.Marshal(obj)
	for _, raw := range [][]byte{bad, []byte("null"), append(base, []byte(" {}")...)} {
		if _, err := fitting.LoadCatalog(raw); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}

func TestEquipmentRejectsWrongItemsAndSlots(t *testing.T) {
	c := catalog(t)
	for _, tt := range []struct {
		name, code string
		edit       func(map[string]fitting.Item, map[string]string)
	}{
		{"theft", "not_owned", func(items map[string]fitting.Item, slots map[string]string) {
			id := slots["weapon-1"]
			i := items[id]
			i.Owner = "other"
			items[id] = i
		}},
		{"missing", "not_owned", func(items map[string]fitting.Item, slots map[string]string) { delete(items, slots["weapon-1"]) }},
		{"duplicate", "duplicate_item", func(items map[string]fitting.Item, slots map[string]string) { slots["weapon-2"] = slots["weapon-1"] }},
		{"bound stock", "module_bound_ship", func(items map[string]fitting.Item, slots map[string]string) {
			id := slots["weapon-1"]
			i := items[id]
			i.Hull = "COR-001"
			items[id] = i
		}},
		{"wrong category", "module_category", func(items map[string]fitting.Item, slots map[string]string) {
			slots["weapon-2"] = slots["tech-1"]
			delete(slots, "tech-1")
		}},
		{"unknown slot", "unknown_slot", func(items map[string]fitting.Item, slots map[string]string) { slots["client-slot"] = "" }},
		{"bad engineering", "engineering_rank", func(items map[string]fitting.Item, slots map[string]string) {
			id := slots["weapon-1"]
			i := items[id]
			i.Engineering = 6
			items[id] = i
		}},
		{"unknown prototype", "unknown_module", func(items map[string]fitting.Item, slots map[string]string) {
			id := slots["weapon-1"]
			i := items[id]
			i.Prototype = "MISSING"
			items[id] = i
		}},
		{"oversize", "module_size", func(items map[string]fitting.Item, slots map[string]string) {
			id := slots["weapon-1"]
			i := items[id]
			i.Prototype = "LAS-XL-START"
			items[id] = i
		}},
		{"pilot level", "module_level", func(items map[string]fitting.Item, slots map[string]string) {
			id := slots["weapon-1"]
			i := items[id]
			i.Prototype = "LAS-S-WEAPON-00-LEGENDARY"
			items[id] = i
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, slots := stock(t, c)
			tt.edit(items, slots)
			before, _ := json.Marshal([]any{items, slots})
			_, err := c.Validate("owner", "FTR-001", 1, 1, items, slots)
			v, ok := err.(*fitting.Violation)
			if !ok || v.Code != tt.code {
				t.Fatalf("got %v want %s", err, tt.code)
			}
			after, _ := json.Marshal([]any{items, slots})
			if string(before) != string(after) {
				t.Fatal("validator mutated input")
			}
		})
	}
}

func TestFittingBudgetsReactorUniquenessAndModifiers(t *testing.T) {
	c := catalog(t)
	d := c.Document()
	d.Hulls[0].Starter = nil
	d.Hulls[0].MassBudget = 1
	d.Hulls[0].ReactorPower = 5
	d.Hulls[0].Slots = append(d.Hulls[0].Slots, fitting.Slot{ID: "engine-extra", MaxSize: "S", Categories: []string{"engine"}})
	limited, err := fitting.NewCatalog(d)
	if err != nil {
		t.Fatal(err)
	}
	items := map[string]fitting.Item{"gun": {ID: "gun", Owner: "owner", Prototype: "LAS-S-START"}, "shield": {ID: "shield", Owner: "owner", Prototype: "DEF-S-START"}, "reactor": {ID: "reactor", Owner: "owner", Prototype: "REA-S-REACTOR-06-COMMON"}}
	if _, err := limited.Validate("owner", "FTR-001", 1, 1, items, map[string]string{"weapon-1": "gun"}); err != nil {
		t.Fatal(err)
	}
	if _, err := limited.Validate("owner", "FTR-001", 1, 1, items, map[string]string{"weapon-1": "gun", "defense-1": "shield"}); err == nil || err.(*fitting.Violation).Code != "reactor_power" {
		t.Fatal(err)
	}
	d.Hulls[0].MassBudget = 100
	powered, err := fitting.NewCatalog(d)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := powered.Validate("owner", "FTR-001", 1, 1, items, map[string]string{"weapon-1": "gun", "defense-1": "shield", "tech-2": "reactor"}); err != nil || res.PowerDraw != 10 || res.PowerCapacity != 25 {
		t.Fatalf("reactor supply: %+v %v", res, err)
	}
	items["engine1"] = fitting.Item{ID: "engine1", Owner: "owner", Prototype: "ENG-S-START"}
	items["engine2"] = fitting.Item{ID: "engine2", Owner: "owner", Prototype: "ENG-S-START"}
	if _, err := powered.Validate("owner", "FTR-001", 1, 1, items, map[string]string{"tech-1": "engine1", "engine-extra": "engine2"}); err == nil || err.(*fitting.Violation).Code != "module_unique" {
		t.Fatal(err)
	}
	items["large"] = fitting.Item{ID: "large", Owner: "owner", Prototype: "LAS-S-WEAPON-00-LEGENDARY", Engineering: 5}
	if _, err := limited.Validate("owner", "FTR-001", 100, 50, items, map[string]string{"weapon-1": "large"}); err == nil || err.(*fitting.Violation).Code != "mass_budget" {
		t.Fatal(err)
	}
	res, err := c.Validate("owner", "FTR-001", 100, 50, items, map[string]string{"weapon-1": "large"})
	if err != nil {
		t.Fatal(err)
	}
	stats := rpg.Compute(rpg.DefaultConfig(), rpg.Input{Base: map[rpg.StatID]float64{rpg.StatDamage: 100}, Modules: res.Modifiers})
	if math.Abs(stats.Final[rpg.StatDamage]-109.2) > 1e-9 {
		t.Fatalf("quality/engineering/typed stat layering: %v", stats.Final)
	}
	// Unbound purchases can fit compatible hulls; the very same stock item is bound.
	if _, err := c.Validate("owner", "COR-001", 100, 50, items, map[string]string{"weapon-1": "large"}); err != nil {
		t.Fatal(err)
	}
}

func TestFittingChecksClassAndShipLevelIndependently(t *testing.T) {
	c := catalog(t)
	m, _ := c.Module("LAS-S-WEAPON-00-COMMON")
	m.ID = "CLASS-LOCKED"
	m.Classes = []string{"battleship"}
	d := c.Document()
	d.Modules = append(d.Modules, m)
	locked, err := fitting.NewCatalog(d)
	if err != nil {
		t.Fatal(err)
	}
	items := map[string]fitting.Item{"item": {ID: "item", Owner: "owner", Prototype: m.ID}}
	_, err = locked.Validate("owner", "FTR-001", 100, 50, items, map[string]string{"weapon-1": "item"})
	if err == nil || err.(*fitting.Violation).Code != "module_class" {
		t.Fatal(err)
	}
	items["item"] = fitting.Item{ID: "item", Owner: "owner", Prototype: "LAS-S-WEAPON-00-LEGENDARY"}
	_, err = c.Validate("owner", "FTR-001", 100, 1, items, map[string]string{"weapon-1": "item"})
	if err == nil || err.(*fitting.Violation).Code != "module_level" {
		t.Fatal(err)
	}
}
