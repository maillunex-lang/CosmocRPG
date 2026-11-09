package shipcatalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestIdentityRegistryLegacyCompatibility(t *testing.T) {
	r, err := DefaultIdentityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Ships) != 15 || len(r.Classes) != 12 {
		t.Fatalf("models/classes: %d/%d", len(r.Ships), len(r.Classes))
	}
	byID := map[string]ShipIdentity{}
	count := 0
	for _, ship := range r.Ships {
		byID[ship.ID] = ship
		count += len(ship.Specializations)
	}
	if count != 30 {
		t.Fatalf("specializations: %d", count)
	}
	before := world.DefaultDoc()
	for _, old := range before.Ships {
		if _, ok := byID[old.ID]; !ok {
			t.Errorf("legacy ship missing: %s", old.ID)
		}
	}
	for _, id := range []string{"BSH-001", "CAR-001", "EWR-001"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("new ship missing: %s", id)
		}
	}
	if !reflect.DeepEqual(before, world.DefaultDoc()) {
		t.Fatal("registry changed legacy runtime data")
	}
	// Mutating a returned registry must not change the embedded contract.
	r.Ships[0].Specializations[0].Name = "changed"
	r.Classes[0] = "changed"
	again, err := DefaultIdentityRegistry()
	if err != nil || again.Ships[0].Specializations[0].Name == "changed" || again.Classes[0] == "changed" {
		t.Fatalf("shared data: %v", err)
	}
}

func TestIdentityRegistryRejectsInvalidContracts(t *testing.T) {
	cases := []struct {
		name   string
		change func(*IdentityRegistry)
		want   string
	}{
		{"version", func(r *IdentityRegistry) { r.Version = 2 }, ""},
		{"missing model", func(r *IdentityRegistry) { r.Ships = r.Ships[:14] }, ""},
		{"duplicate model", func(r *IdentityRegistry) { r.Ships[1] = r.Ships[0] }, "duplicate ship"},
		{"duplicate class", func(r *IdentityRegistry) { r.Classes[1] = r.Classes[0] }, ""},
		{"unknown class", func(r *IdentityRegistry) { r.Ships[0].ClassID = "unknown" }, "unknown class"},
		{"unused class", func(r *IdentityRegistry) { r.Ships[8].ClassID = "fighter" }, "unused class"},
		{"missing specialization", func(r *IdentityRegistry) { r.Ships[0].Specializations = r.Ships[0].Specializations[:1] }, ""},
		{"duplicate specialization", func(r *IdentityRegistry) { r.Ships[0].Specializations[1] = r.Ships[0].Specializations[0] }, "duplicate specialization"},
		{"wrong owner", func(r *IdentityRegistry) { r.Ships[0].Specializations[0].ID = "COR-001:hunter" }, "wrong ship"},
		{"bad model id", func(r *IdentityRegistry) { r.Ships[0].ID = "bad" }, ""},
		{"empty name", func(r *IdentityRegistry) { r.Ships[0].Name = "" }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := DefaultIdentityRegistry()
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&r)
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			result, err := LoadIdentityRegistry(data)
			if err == nil {
				t.Fatal("invalid contract accepted")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wrong error: %v", err)
			}
			if len(result.Ships) != 0 {
				t.Fatal("partial result on failure")
			}
		})
	}
	for _, data := range []string{`{`, `null`, string(identityJSON) + ` {}`, strings.Replace(string(identityJSON), `"version": 1`, `"version": 1, "code": "execute"`, 1)} {
		if _, err := LoadIdentityRegistry([]byte(data)); err == nil {
			t.Fatalf("invalid JSON/schema accepted: %.80s", data)
		}
	}
}
