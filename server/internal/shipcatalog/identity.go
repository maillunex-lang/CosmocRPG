package shipcatalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// IdentityRegistry is the Ships RPG target identity contract. It is deliberately
// separate from runtime passports until an explicit, compatible catalog upgrade.
type IdentityRegistry struct {
	Version int            `json:"version"`
	Classes []string       `json:"classes"`
	Ships   []ShipIdentity `json:"ships"`
}

type ShipIdentity struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	ClassID         string                   `json:"class_id"`
	Specializations []SpecializationIdentity `json:"specializations"`
}

type SpecializationIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

//go:embed data/rpg_identity.json
var identityJSON []byte

//go:embed data/rpg_identity.schema.json
var identitySchemaJSON []byte

var identitySchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(identitySchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("identity schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	const url = "https://cosmocrpg.invalid/schemas/rpg-identity-v1.json"
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
})

// DefaultIdentityRegistry returns a fresh, validated target registry. Changes to
// the result cannot affect subsequent callers or the active runtime catalog.
func DefaultIdentityRegistry() (IdentityRegistry, error) {
	return LoadIdentityRegistry(identityJSON)
}

// LoadIdentityRegistry checks JSON Schema and semantic ID/reference constraints.
// It never publishes a version, executes effects, or changes player state.
func LoadIdentityRegistry(data []byte) (IdentityRegistry, error) {
	fail := func(err error) (IdentityRegistry, error) {
		return IdentityRegistry{}, fmt.Errorf("rpg identity: %w", err)
	}
	schema, err := identitySchema()
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
	var r IdentityRegistry
	if err := json.Unmarshal(data, &r); err != nil {
		return fail(err)
	}
	classes := make(map[string]bool, len(r.Classes))
	used := map[string]bool{}
	for _, c := range r.Classes {
		classes[c] = true
	}
	ships, specs := map[string]bool{}, map[string]bool{}
	for _, ship := range r.Ships {
		if ships[ship.ID] {
			return fail(fmt.Errorf("duplicate ship %s", ship.ID))
		}
		ships[ship.ID] = true
		if !classes[ship.ClassID] {
			return fail(fmt.Errorf("ship %s: unknown class %s", ship.ID, ship.ClassID))
		}
		used[ship.ClassID] = true
		for _, spec := range ship.Specializations {
			if !strings.HasPrefix(spec.ID, ship.ID+":") {
				return fail(fmt.Errorf("specialization %s: wrong ship %s", spec.ID, ship.ID))
			}
			if specs[spec.ID] {
				return fail(fmt.Errorf("duplicate specialization %s", spec.ID))
			}
			specs[spec.ID] = true
		}
	}
	for _, class := range r.Classes {
		if !used[class] {
			return fail(fmt.Errorf("unused class %s", class))
		}
	}
	return r, nil
}
