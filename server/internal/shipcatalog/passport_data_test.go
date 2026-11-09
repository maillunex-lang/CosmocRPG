package shipcatalog

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestDefaultPassportsPreserveEveryLegacyField(t *testing.T) {
	before := world.DefaultDoc()
	c, err := DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ships) != 15 {
		t.Fatalf("models: %d", len(c.Ships))
	}
	if err := CheckLegacyPassportBases(c, before); err != nil {
		t.Fatal(err)
	}
	classes, specs, silhouettes := map[string]bool{}, map[string]bool{}, map[string]bool{}
	byID := map[string]ShipPassport{}
	for _, ship := range c.Ships {
		byID[ship.ID] = ship
		classes[ship.ClassID] = true
		for _, id := range ship.Specializations {
			specs[id] = true
		}
		if silhouettes[ship.Visual.Silhouette] {
			t.Errorf("repeated silhouette: %s", ship.Visual.Silhouette)
		}
		silhouettes[ship.Visual.Silhouette] = true
		weapons := 0
		for _, slot := range ship.Slots {
			if slot.Group == "weapon" {
				weapons++
			}
		}
		mounts := 0
		for _, mount := range ship.Visual.Mounts {
			if mount.Kind == "weapon" {
				mounts++
			}
		}
		if mounts < weapons {
			t.Errorf("%s: missing weapon mount", ship.ID)
		}
	}
	if len(classes) != 12 || len(specs) != 30 || len(silhouettes) != 15 {
		t.Fatalf("classes/specs/silhouettes: %d/%d/%d", len(classes), len(specs), len(silhouettes))
	}
	for _, old := range before.Ships {
		if !reflect.DeepEqual(byID[old.ID].Base, old) {
			t.Errorf("legacy changed: %s", old.ID)
		}
		if containsCategory(byID[old.ID].PreliminaryFields, "base") {
			t.Errorf("legacy base relabelled: %s", old.ID)
		}
	}
	for _, id := range []string{"BSH-001", "CAR-001", "EWR-001"} {
		if !containsCategory(byID[id].PreliminaryFields, "base") {
			t.Errorf("new base not provisional: %s", id)
		}
	}
	if !reflect.DeepEqual(before, world.DefaultDoc()) {
		t.Fatal("runtime seed was mutated")
	}
	if len(world.New().Catalog().Ships) != 12 {
		t.Fatal("draft catalog published implicitly")
	}
	// All fields, including old local specialization keys and abilities, survive.
	again, err := LoadPassportCatalog(marshalPassports(t, c))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, again) {
		t.Fatal("authored data lost on roundtrip")
	}
}

func TestDefaultPassportsCoverAllSpecifiedAbilityIDs(t *testing.T) {
	c, err := DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	permitted := map[string]bool{}
	for _, ship := range c.Ships {
		for _, id := range ship.AllowedAbilities {
			permitted[id] = true
		}
	}
	expected := 0
	for prefix, n := range map[string]int{"W": 16, "M": 10, "D": 10, "E": 10, "S": 8, "R": 6, "I": 6, "C": 6} {
		for i := 1; i <= n; i++ {
			expected++
			id := fmt.Sprintf("%s%02d", prefix, i)
			if !permitted[id] {
				t.Errorf("no permitted ship for %s", id)
			}
		}
	}
	if expected != 72 || len(permitted) != 72 {
		t.Fatalf("ability permissions: %d/%d", len(permitted), expected)
	}
	// This checks hull permissions, not module unlock reachability (SR-013).
}

func TestDefaultPassportCopiesDoNotShareNestedState(t *testing.T) {
	c, err := DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	original := marshalPassports(t, c)
	c.Ships[0].Base.Abilities["boost"] = world.AbilityCfg{}
	c.Ships[0].Resistances["kinetic"] = 0.8
	c.Ships[0].Slots[0].Categories[0] = "shield"
	c.Ships[0].StarterKit[0].AbilityIDs[0] = "W16"
	c.Ships[0].Specializations[0] = "changed"
	again, err := DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var want PassportCatalog
	if err := json.Unmarshal(original, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, want) {
		t.Fatal("default catalog shared mutable data")
	}
}

func TestStarterPowerBudgetIsIndependentOfCombatEnergy(t *testing.T) {
	c, err := DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ship := &c.Ships[0]
	var mass, power float64
	for _, module := range ship.StarterKit {
		mass += module.MassTons
		power += module.PowerMW
	}
	ship.Base.Energy = 10 // Valid legacy-schema value, deliberately below MW total.
	ship.ModuleMassBudget = mass
	ship.ReactorPower = power
	if _, err := LoadPassportCatalog(marshalPassports(t, c)); err != nil {
		t.Fatalf("exact budgets or separate energy rejected: %v", err)
	}
	ship.ReactorPower = power - 0.01
	if _, err := LoadPassportCatalog(marshalPassports(t, c)); err == nil {
		t.Fatal("power overdraw accepted")
	}
	ship.ReactorPower = power
	ship.ModuleMassBudget = mass - 0.01
	if _, err := LoadPassportCatalog(marshalPassports(t, c)); err == nil {
		t.Fatal("mass overdraw accepted")
	}
}

func TestStrizhStarterBlueprintMatchesBoundRuntimeContract(t *testing.T) {
	c, err := DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ship := range c.Ships {
		if ship.ID != world.DefaultShip {
			continue
		}
		for _, module := range ship.StarterKit {
			if module.ID == world.StrizhEnginePrototype {
				found = true
				if module.SlotID != world.StrizhEngineSlot || module.Category != "engine" || module.Family != "ENG" || !containsCategory(module.AbilityIDs, world.StrizhBoostAbility) {
					t.Fatalf("draft starter and bound runtime contract diverged: %+v", module)
				}
			}
		}
	}
	if !found {
		t.Fatal("runtime starter prototype has no validated passport blueprint")
	}
}
