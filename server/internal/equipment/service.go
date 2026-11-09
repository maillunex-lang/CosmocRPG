package equipment

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

// Command has no owner, item stats, prices, XP, or credits fields. The server
// derives all authority/costs from the authenticated state and catalog.
type Command struct {
	Op        string `json:"op"`
	Prototype string `json:"prototype,omitempty"`
	Item      string `json:"item,omitempty"`
	Slot      string `json:"slot,omitempty"`
	Family    string `json:"family,omitempty"`
}
type Service struct {
	store   GeneralStore
	catalog *fitting.Catalog
}

func NewService(store GeneralStore, catalog *fitting.Catalog) *Service {
	return &Service{store: store, catalog: catalog}
}
func (s *Service) Catalog() *fitting.Catalog { return s.catalog }
func (s *Service) Ensure(ctx context.Context, owner, hull, legacySpec string) (fitting.State, error) {
	return s.store.Ensure(ctx, owner, hull, legacySpec)
}
func (s *Service) State(ctx context.Context, owner, hull string) (fitting.State, error) {
	return s.store.Load(ctx, owner, hull)
}
func (s *Service) Execute(ctx context.Context, owner, hull string, revision, pilotRevision int64, cmd Command, check func(fitting.State) error) (fitting.State, error) {
	return s.store.Update(ctx, owner, hull, revision, pilotRevision, func(state *fitting.State) error {
		if err := s.apply(state, cmd); err != nil {
			return err
		}
		if _, err := s.catalog.ValidateState(*state); err != nil {
			return err
		}
		if check != nil {
			return check(*state)
		}
		return nil
	})
}
func rejected(code, field string) error { return &fitting.Violation{Code: code, Field: field} }
func charge(s *fitting.State, amount int64) error {
	if amount < 0 {
		return fitting.ErrInvalidState
	}
	if s.Progress.Credits < amount {
		return fitting.ErrCredits
	}
	s.Progress.Credits -= amount
	return nil
}
func EngineeringCost(m fitting.Module, next int) int64 {
	base := m.Price / 2
	if base < 100 {
		base = 100
	}
	return base * int64(next*next)
}
func ResearchCost(next int) int64 { return 500 * int64(next*next) }
func (s *Service) apply(state *fitting.State, cmd Command) error {
	pl, sl := state.Progress.Levels()
	switch cmd.Op {
	case "buy":
		if cmd.Item != "" || cmd.Slot != "" || cmd.Family != "" {
			return rejected("bad_request", cmd.Op)
		}
		m, ok := s.catalog.Module(cmd.Prototype)
		if !ok || m.Price <= 0 {
			return rejected("not_for_sale", cmd.Prototype)
		}
		if pl < m.MinPilot {
			return rejected("pilot_level", cmd.Prototype)
		}
		if state.Pilot.Research[m.Family] < fitting.ResearchRequired(m.Rarity) {
			return rejected("research_required", m.Family)
		}
		// Reserve room for native kits of hulls not visited yet.
		purchased := 0
		for _, i := range state.Items {
			if i.Hull == "" {
				purchased++
			}
		}
		if len(state.Items) >= fitting.MaxItems || purchased >= fitting.MaxItems-128 {
			return rejected("inventory_full", cmd.Prototype)
		}
		if err := charge(state, m.Price); err != nil {
			return err
		}
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return err
		}
		id := "mod_" + hex.EncodeToString(raw[:])
		state.Items[id] = fitting.Item{ID: id, Owner: state.Owner, Prototype: m.ID}
	case "equip", "unequip":
		if cmd.Prototype != "" || cmd.Family != "" {
			return rejected("bad_request", cmd.Op)
		}
		h, _ := s.catalog.Hull(state.Hull)
		found := false
		for _, slot := range h.Slots {
			if slot.ID == cmd.Slot {
				found = true
				break
			}
		}
		if !found {
			return rejected("unknown_slot", cmd.Slot)
		}
		if cmd.Op == "unequip" {
			if cmd.Item != "" {
				return rejected("bad_request", cmd.Op)
			}
			delete(state.Slots, cmd.Slot)
		} else {
			if cmd.Item == "" {
				return rejected("not_owned", cmd.Item)
			}
			state.Slots[cmd.Slot] = cmd.Item
		}
	case "engineer":
		if cmd.Prototype != "" || cmd.Slot != "" || cmd.Family != "" {
			return rejected("bad_request", cmd.Op)
		}
		item, ok := state.Items[cmd.Item]
		if !ok || item.Owner != state.Owner {
			return rejected("not_owned", cmd.Item)
		}
		if item.EquippedHull != "" && item.EquippedHull != state.Hull {
			return rejected("item_in_use", cmd.Item)
		}
		m, ok := s.catalog.Module(item.Prototype)
		if !ok {
			return rejected("unknown_module", item.Prototype)
		}
		next := item.Engineering + 1
		if next > 5 {
			return rejected("engineering_max", cmd.Item)
		}
		if pl < next*5 || sl < next*2-1 {
			return rejected("engineering_level", cmd.Item)
		}
		if err := charge(state, EngineeringCost(m, next)); err != nil {
			return err
		}
		item.Engineering = next
		state.Items[cmd.Item] = item
	case "research":
		if cmd.Prototype != "" || cmd.Item != "" || cmd.Slot != "" {
			return rejected("bad_request", cmd.Op)
		}
		if !fitting.KnownFamily(cmd.Family) {
			return rejected("unknown_research", cmd.Family)
		}
		next := state.Pilot.Research[cmd.Family] + 1
		if next > 3 {
			return rejected("research_max", cmd.Family)
		}
		if pl < map[int]int{1: 5, 2: 20, 3: 40}[next] {
			return rejected("pilot_level", cmd.Family)
		}
		if err := charge(state, ResearchCost(next)); err != nil {
			return err
		}
		state.Pilot.Research[cmd.Family] = next
	default:
		return rejected("unknown_fitting_command", cmd.Op)
	}
	return nil
}
