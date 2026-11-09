package fitting

import (
	"fmt"
	"sort"

	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
)

type Item struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	Hull         string `json:"hull"`
	Prototype    string `json:"prototype"`
	Engineering  int    `json:"engineering"`
	EquippedHull string `json:"equipped_hull,omitempty"`
	EquippedSlot string `json:"equipped_slot,omitempty"`
}
type Violation struct {
	Code  string `json:"code"`
	Field string `json:"field"`
}

func (v *Violation) Error() string     { return v.Code + ":" + v.Field }
func invalid(code, field string) error { return &Violation{Code: code, Field: field} }

type Result struct {
	Mass          float64             `json:"mass_tons"`
	MassBudget    float64             `json:"mass_budget_tons"`
	PowerDraw     float64             `json:"power_draw_mw"`
	PowerCapacity float64             `json:"power_capacity_mw"`
	Abilities     map[string][]string `json:"ability_sources"`
	Modifiers     []rpg.Modifier      `json:"modifiers"`
}

// Validate resolves every value from the server catalog. It never accepts item
// stats, owner, level or prototype authority from an equipment request.
func (c *Catalog) Validate(owner, hullID string, pilotLevel, shipLevel int, items map[string]Item, slots map[string]string) (Result, error) {
	out := Result{Abilities: map[string][]string{}, Modifiers: []rpg.Modifier{}}
	h, ok := c.hulls[hullID]
	if !ok {
		return Result{}, invalid("unknown_hull", hullID)
	}
	if (!ValidID(owner) || len(owner) > 64) || pilotLevel < 1 || pilotLevel > 100 || shipLevel < 1 || shipLevel > 50 {
		return Result{}, invalid("invalid_progress", owner)
	}
	out.MassBudget, out.PowerCapacity = h.MassBudget, h.ReactorPower
	bySlot := map[string]Slot{}
	for _, s := range h.Slots {
		bySlot[s.ID] = s
	}
	ids := make([]string, 0, len(slots))
	for id := range slots {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	used, groups := map[string]bool{}, map[string]bool{}
	for _, slotID := range ids {
		s, ok := bySlot[slotID]
		if !ok {
			return Result{}, invalid("unknown_slot", slotID)
		}
		id := slots[slotID]
		if id == "" {
			continue
		}
		item, ok := items[id]
		if !ok || !ValidID(id) || item.ID != id || item.Owner != owner {
			return Result{}, invalid("not_owned", id)
		}
		if item.Hull != "" && item.Hull != hullID {
			return Result{}, invalid("module_bound_ship", id)
		}
		if item.EquippedHull != "" && item.EquippedHull != hullID {
			return Result{}, invalid("item_in_use", id)
		}
		if used[id] {
			return Result{}, invalid("duplicate_item", id)
		}
		used[id] = true
		m, ok := c.modules[item.Prototype]
		if !ok {
			return Result{}, invalid("unknown_module", item.Prototype)
		}
		if item.Engineering < 0 || item.Engineering > 5 {
			return Result{}, invalid("engineering_rank", id)
		}
		if size(m.Size) > size(s.MaxSize) {
			return Result{}, invalid("module_size", slotID)
		}
		if !contains(s.Categories, m.Category) {
			return Result{}, invalid("module_category", slotID)
		}
		if len(m.Classes) > 0 && !contains(m.Classes, h.Class) {
			return Result{}, invalid("module_class", id)
		}
		if pilotLevel < m.MinPilot || shipLevel < m.MinShip {
			return Result{}, invalid("module_level", id)
		}
		if m.UniqueGroup != "" {
			if groups[m.UniqueGroup] {
				return Result{}, invalid("module_unique", m.UniqueGroup)
			}
			groups[m.UniqueGroup] = true
		}
		out.Mass += m.Mass * (1 + 0.02*float64(item.Engineering))
		out.PowerDraw += m.PowerDraw * (1 + 0.05*float64(item.Engineering))
		out.PowerCapacity += m.PowerSupply
		for _, ability := range m.Abilities {
			if contains(h.Abilities, ability) {
				out.Abilities[ability] = append(out.Abilities[ability], id)
			}
		}
		for _, mod := range m.Modifiers {
			mod.Source = "module:" + id
			mod.Layer = rpg.LayerModules
			mod.Value *= Quality(m.Rarity) * (1 + 0.03*float64(item.Engineering))
			out.Modifiers = append(out.Modifiers, mod)
		}
	}
	if !finite(out.Mass) || out.Mass > out.MassBudget+1e-9 {
		return Result{}, invalid("mass_budget", fmt.Sprint(out.MassBudget))
	}
	if !finite(out.PowerDraw) || !finite(out.PowerCapacity) || out.PowerDraw > out.PowerCapacity+1e-9 {
		return Result{}, invalid("reactor_power", fmt.Sprint(out.PowerCapacity))
	}
	return out, nil
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
