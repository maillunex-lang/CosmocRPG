package shipcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Synthetic fixture tests the contract independently from the authored balance.
func passportFixture(t *testing.T) PassportCatalog {
	t.Helper()
	identities, err := DefaultIdentityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	base, _ := world.New().Catalog().Spec(world.DefaultShip)
	c := PassportCatalog{Version: 1}
	for _, identity := range identities.Ships {
		sp := ShipPassport{
			ID: identity.ID, Name: identity.Name, ClassID: identity.ClassID, Description: "Synthetic passport for validation tests.", Base: base,
			PreliminaryFields: []string{"base", "description", "mass_tons", "resistances", "acceleration_units_s2", "turn_rate_deg_s", "signature", "sensor_range_units", "reactor_power_mw", "module_mass_budget_tons", "slots", "allowed_abilities", "starter_kit", "economy", "npc", "visual"},
			MassTons:          20, Resistances: map[string]float64{"kinetic": 0, "thermal": 0, "energy": 0, "explosive": 0, "electromagnetic": 0},
			Acceleration: 5, TurnRate: 90, Signature: 10, Sensors: 100, ReactorPower: 30, ModuleMassBudget: 10,
			Slots: []PassportSlot{
				{"w1", "weapon", "S", []string{"weapon"}}, {"d1", "defensive", "S", []string{"shield", "armor"}},
				{"t1", "technical", "S", []string{"engine"}}, {"t2", "technical", "S", []string{"reactor"}},
				{"t3", "technical", "S", []string{"electronics"}}, {"s1", "special", "S", []string{"drone"}},
			},
			AllowedAbilities: []string{"W01", "M01", "D01", "E01"},
			StarterKit: []StarterModule{
				{"LAS-S", "w1", "LAS", "weapon", "S", 1, 5, []string{"W01"}}, {"DEF-S", "d1", "DEF", "shield", "S", 1, 5, []string{"D01"}},
				{"ENG-S", "t1", "ENG", "engine", "S", 1, 5, []string{"M01"}}, {"REA-S", "t2", "REA", "reactor", "S", 1, 0, []string{}},
				{"ELE-S", "t3", "ELE", "electronics", "S", 1, 5, []string{"E01"}},
			},
			Economy: PassportEconomy{"station_shipyard", 10000, 1, 1, 100, 0.5}, NPC: PassportNPC{"assault", 1, 3, 0},
			Visual: PassportVisual{PlaceholderID: "fixture-" + identity.ID, Silhouette: "dart", ForwardAxis: "-Z", UpAxis: "+Y", CollisionHalfExtents: [3]float64{3, 1, 5}, Mounts: []PassportMount{
				{"weapon-1", "weapon", [3]float64{0, 0, -5}}, {"engine-1", "engine", [3]float64{-1, 0, 5}}, {"engine-2", "engine", [3]float64{1, 0, 5}}, {"vfx-1", "vfx", [3]float64{0, 1, 0}},
			}},
		}
		sp.Base.ID = identity.ID
		// Avoid fixture sharing of maps/slices across ships.
		for _, spec := range identity.Specializations {
			sp.Specializations = append(sp.Specializations, spec.ID)
		}
		c.Ships = append(c.Ships, sp)
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var isolated PassportCatalog
	if err := json.Unmarshal(b, &isolated); err != nil {
		t.Fatal(err)
	}
	return isolated
}

func marshalPassports(t *testing.T, c PassportCatalog) []byte {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPassportSchemaAndRoundtrip(t *testing.T) {
	fixture := passportFixture(t)
	got, err := LoadPassportCatalog(marshalPassports(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, fixture) {
		t.Fatal("passport fields lost on load")
	}
	again, err := LoadPassportCatalog(marshalPassports(t, got))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) {
		t.Fatal("roundtrip changed passport")
	}
}

func TestPassportRejectsInvalidData(t *testing.T) {
	cases := []struct {
		name   string
		change func(*ShipPassport)
		want   string
	}{
		{"wrong identity", func(s *ShipPassport) { s.Name = "wrong" }, "identity mismatch"},
		{"unknown model", func(s *ShipPassport) { s.ID = "BAD-001" }, "unknown or duplicate"},
		{"wrong base", func(s *ShipPassport) { s.Base.ID = "FTR-002" }, "identity mismatch"},
		{"wrong class", func(s *ShipPassport) { s.ClassID = "carrier" }, "identity mismatch"},
		{"wrong specialization", func(s *ShipPassport) { s.Specializations[0] = "CAR-001:swarm_commander" }, "unknown specialization"},
		{"missing specialization", func(s *ShipPassport) { s.Specializations = s.Specializations[:1] }, ""},
		{"duplicate specialization", func(s *ShipPassport) { s.Specializations[1] = s.Specializations[0] }, ""},
		{"unknown base ability spec", func(s *ShipPassport) {
			a := s.Base.Abilities["boost"]
			a.Spec = "missing"
			s.Base.Abilities["boost"] = a
		}, "unknown base specialization"},
		{"duplicate legacy spec", func(s *ShipPassport) { s.Base.Specs = append(s.Base.Specs, s.Base.Specs[0]) }, ""},
		{"empty base role", func(s *ShipPassport) { s.Base.Role = "" }, ""},
		{"negative hull", func(s *ShipPassport) { s.Base.Hull = -1 }, ""},
		{"base invalid boost", func(s *ShipPassport) {
			ab := s.Base.Abilities["boost"]
			ab.Duration = 0
			s.Base.Abilities["boost"] = ab
		}, "base:"},
		{"zero mass", func(s *ShipPassport) { s.MassTons = 0 }, ""},
		{"resistance cap", func(s *ShipPassport) { s.Resistances["thermal"] = 0.81 }, ""},
		{"missing resistance", func(s *ShipPassport) { delete(s.Resistances, "kinetic") }, ""},
		{"missing preliminary marker", func(s *ShipPassport) { s.PreliminaryFields = s.PreliminaryFields[:len(s.PreliminaryFields)-1] }, ""},
		{"duplicate slot", func(s *ShipPassport) { s.Slots[1].ID = s.Slots[0].ID }, "duplicate slot"},
		{"missing group", func(s *ShipPassport) { s.Slots = s.Slots[:5] }, "missing slot group"},
		{"bad category for group", func(s *ShipPassport) { s.Slots[0].Categories = []string{"shield"} }, "incompatible with slot group"},
		{"missing starter slot", func(s *ShipPassport) { s.StarterKit[0].SlotID = "absent" }, "unknown starter slot"},
		{"duplicate occupied slot", func(s *ShipPassport) { s.StarterKit[1].SlotID = s.StarterKit[0].SlotID }, "duplicate starter"},
		{"duplicate starter id", func(s *ShipPassport) { s.StarterKit[1].ID = s.StarterKit[0].ID }, "duplicate starter"},
		{"bad size", func(s *ShipPassport) { s.StarterKit[0].Size = "XL" }, "starter size exceeds"},
		{"bad category", func(s *ShipPassport) { s.StarterKit[0].Category = "engine"; s.StarterKit[0].Family = "ENG" }, "starter category incompatible"},
		{"bad family", func(s *ShipPassport) { s.StarterKit[0].Family = "ENG" }, "starter family incompatible"},
		{"mass budget", func(s *ShipPassport) { s.ModuleMassBudget = 1 }, "starter mass exceeds"},
		{"power budget", func(s *ShipPassport) { s.ReactorPower = 1 }, "starter power exceeds"},
		{"locked starter skill", func(s *ShipPassport) { s.StarterKit[0].AbilityIDs = []string{"W02"} }, "starter ability not allowed"},
		{"unknown ability", func(s *ShipPassport) { s.AllowedAbilities = append(s.AllowedAbilities, "W17") }, ""},
		{"ultimate in normal abilities", func(s *ShipPassport) { s.AllowedAbilities = append(s.AllowedAbilities, "ULT-01") }, ""},
		{"missing engine", func(s *ShipPassport) { s.StarterKit = append(s.StarterKit[:2], s.StarterKit[3:]...) }, "missing starter system"},
		{"no shield module", func(s *ShipPassport) { s.StarterKit = append(s.StarterKit[:1], s.StarterKit[2:]...) }, "missing starter shield"},
		{"bad insurance", func(s *ShipPassport) { s.Economy.InsurancePayout = 1.1 }, ""},
		{"pilot level cap", func(s *ShipPassport) { s.Economy.PilotLevel = 101 }, ""},
		{"drone cap", func(s *ShipPassport) { s.NPC.MaxDrones = 33 }, ""},
		{"axis", func(s *ShipPassport) { s.Visual.ForwardAxis = "+Z" }, ""},
		{"collision", func(s *ShipPassport) { s.Visual.CollisionHalfExtents[0] = 100 }, "collision exceeds"},
		{"mount outside", func(s *ShipPassport) { s.Visual.Mounts[0].Position[0] = 100 }, "outside ship dimensions"},
		{"duplicate mount", func(s *ShipPassport) { s.Visual.Mounts[1].ID = s.Visual.Mounts[0].ID }, "duplicate mount"},
		{"missing engine mount", func(s *ShipPassport) { s.Visual.Mounts = s.Visual.Mounts[:3]; s.Visual.Mounts[2].Kind = "vfx" }, "missing weapon/engine/vfx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := passportFixture(t)
			tc.change(&fixture.Ships[0])
			got, err := LoadPassportCatalog(marshalPassports(t, fixture))
			if err == nil {
				t.Fatal("invalid passport accepted")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.Ships) != 0 {
				t.Fatal("partial data on failure")
			}
		})
	}
}

func TestPassportCatalogCompletenessAndUnknownFields(t *testing.T) {
	fixture := passportFixture(t)
	for _, change := range []func(*PassportCatalog){
		func(c *PassportCatalog) { c.Version = 2 }, func(c *PassportCatalog) { c.Ships = c.Ships[:14] },
		func(c *PassportCatalog) { c.Ships[1] = c.Ships[0] },
		func(c *PassportCatalog) { c.Ships[1].Visual.PlaceholderID = c.Ships[0].Visual.PlaceholderID },
		func(c *PassportCatalog) { c.Ships[8].PreliminaryFields = c.Ships[8].PreliminaryFields[1:] },
	} {
		var c PassportCatalog
		if err := json.Unmarshal(marshalPassports(t, fixture), &c); err != nil {
			t.Fatal(err)
		}
		change(&c)
		if _, err := LoadPassportCatalog(marshalPassports(t, c)); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
	raw := string(marshalPassports(t, fixture))
	for _, data := range []string{`null`, `{`, raw + ` {}`, strings.Replace(raw, `"version":1`, `"version":1,"script":"execute"`, 1), strings.Replace(raw, `"mass_tons":20`, `"mass_tons":20,"unknown":1`, 1)} {
		if _, err := LoadPassportCatalog([]byte(data)); err == nil {
			t.Fatal("unknown field or invalid JSON accepted")
		}
	}
}

func TestPassportConcurrentLoad(t *testing.T) {
	raw := marshalPassports(t, passportFixture(t))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := LoadPassportCatalog(raw); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestCheckLegacyPassportBasesRejectsStatAndAbilityDrift(t *testing.T) {
	fixture := passportFixture(t)
	legacy := world.Doc{Ships: []world.ShipSpec{fixture.Ships[0].Base}}
	var c PassportCatalog
	if err := json.Unmarshal(marshalPassports(t, fixture), &c); err != nil {
		t.Fatal(err)
	}
	if err := CheckLegacyPassportBases(c, legacy); err != nil {
		t.Fatal(err)
	}
	c.Ships[0].Base.Energy++
	if err := CheckLegacyPassportBases(c, legacy); err == nil {
		t.Fatal("energy drift accepted")
	}
	c.Ships[0].Base.Energy--
	ab := c.Ships[0].Base.Abilities["boost"]
	ab.Cooldown++
	c.Ships[0].Base.Abilities["boost"] = ab
	if err := CheckLegacyPassportBases(c, legacy); err == nil {
		t.Fatal("legacy ability drift accepted")
	}
	if err := CheckLegacyPassportBases(PassportCatalog{}, legacy); err == nil {
		t.Fatal("missing legacy ship accepted")
	}
}
