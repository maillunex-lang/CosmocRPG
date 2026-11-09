package equipment

import (
	"context"
	"math"
	"reflect"
	"strings"
	"sync"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

// GeneralAccount is registered by an application/test harness, never a request.
type GeneralAccount struct {
	Progress fitting.Progress
	ShipXP   map[string]int64
}
type specReceipt struct {
	Hull, Spec string
	Amount     int64
}
type generalOwner struct {
	account   GeneralAccount
	pilot     fitting.PilotData
	items     map[string]fitting.Item
	slots     map[string]map[string]string
	profiles  map[string]fitting.ShipData
	revisions map[string]int64
	grants    map[string]specReceipt
}
type GeneralMemStore struct {
	mu      sync.Mutex
	catalog *fitting.Catalog
	owners  map[string]*generalOwner
}

func NewGeneralMemStore(c *fitting.Catalog, accounts map[string]GeneralAccount) *GeneralMemStore {
	m := &GeneralMemStore{catalog: c, owners: map[string]*generalOwner{}}
	for id, a := range accounts {
		attrs := map[string]int{}
		for k, v := range a.Progress.Attributes {
			attrs[k] = v
		}
		a.Progress.Attributes = attrs
		xps := map[string]int64{}
		for k, v := range a.ShipXP {
			xps[k] = v
		}
		a.ShipXP = xps
		m.owners[id] = &generalOwner{account: a, pilot: fitting.NewPilotData(), items: map[string]fitting.Item{}, slots: map[string]map[string]string{}, profiles: map[string]fitting.ShipData{}, revisions: map[string]int64{}, grants: map[string]specReceipt{}}
	}
	return m
}
func (m *GeneralMemStore) frame(owner, hull string) (fitting.State, error) {
	o := m.owners[owner]
	if o == nil {
		return fitting.State{}, fitting.ErrNoOwner
	}
	profile, ok := o.profiles[hull]
	if !ok {
		return fitting.State{}, fitting.ErrNotInitialized
	}
	s := fitting.State{Owner: owner, Hull: hull, Revision: o.revisions[hull], Items: o.items, Slots: o.slots[hull], Pilot: o.pilot, Profile: profile, Progress: o.account.Progress}.Clone()
	s.Progress.ShipXP = o.account.ShipXP[hull]
	for id, item := range s.Items {
		item.EquippedHull, item.EquippedSlot = "", ""
		s.Items[id] = item
	}
	for ship, slots := range o.slots {
		for slot, id := range slots {
			item := s.Items[id]
			item.EquippedHull, item.EquippedSlot = ship, slot
			s.Items[id] = item
		}
	}
	s.Profile.Builds[s.Profile.ActiveBuild].Slots = copySlots(s.Slots)
	if _, err := m.catalog.ValidateState(s); err != nil {
		return fitting.State{}, err
	}
	return s, nil
}
func (m *GeneralMemStore) Ensure(ctx context.Context, owner, hull, legacySpec string) (fitting.State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fitting.State{}, err
	}
	if !m.valid(owner, hull) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	o := m.owners[owner]
	if o == nil {
		return fitting.State{}, fitting.ErrNoOwner
	}
	h, ok := m.catalog.Hull(hull)
	if !ok {
		return fitting.State{}, fitting.ErrInvalidState
	}
	if _, ok := o.profiles[hull]; ok {
		return m.frame(owner, hull)
	}
	// Work on a complete copy; even initialization failures have no side effects.
	shadow := *o
	shadow.items = map[string]fitting.Item{}
	for k, v := range o.items {
		shadow.items[k] = v
	}
	shadow.slots = map[string]map[string]string{}
	for k, v := range o.slots {
		shadow.slots[k] = copySlots(v)
	}
	shadow.profiles = map[string]fitting.ShipData{}
	for k, v := range o.profiles {
		shadow.profiles[k] = v
	}
	shadow.revisions = map[string]int64{}
	for k, v := range o.revisions {
		shadow.revisions[k] = v
	}
	shadow.pilot = fitting.State{Pilot: o.pilot}.Clone().Pilot
	_, loadoutExisted := o.slots[hull]
	if loadoutExisted && hull == fitting.ShipStrizh {
		id := fitting.NativeItemID(owner, hull, "tech-1", "ENG-S-START")
		item, ok := o.items[id]
		if !ok || item.Prototype != "ENG-S-START" || item.Hull != hull {
			return fitting.State{}, fitting.ErrInvalidState
		}
	}
	if shadow.slots[hull] == nil {
		shadow.slots[hull] = map[string]string{}
	}
	learned := false
	for _, mount := range h.Starter {
		id := fitting.NativeItemID(owner, hull, mount.SlotID, mount.PrototypeID)
		if old, ok := shadow.items[id]; ok {
			if old.Owner != owner || old.Hull != hull || old.Prototype != mount.PrototypeID {
				return fitting.State{}, fitting.ErrInvalidState
			}
		} else {
			shadow.items[id] = fitting.Item{ID: id, Owner: owner, Hull: hull, Prototype: mount.PrototypeID}
			if shadow.slots[hull][mount.SlotID] == "" {
				shadow.slots[hull][mount.SlotID] = id
			}
		}
		module, _ := m.catalog.Module(mount.PrototypeID)
		for _, id := range module.Abilities {
			if shadow.pilot.Learned[id] == 0 {
				shadow.pilot.Learned[id] = 1
				learned = true
			}
		}
	}
	if loadoutExisted {
		if shadow.revisions[hull] == math.MaxInt64 {
			return fitting.State{}, fitting.ErrConflict
		}
		shadow.revisions[hull]++
	}
	if learned {
		if shadow.pilot.Revision == math.MaxInt64 {
			return fitting.State{}, fitting.ErrConflict
		}
		shadow.pilot.Revision++
	}
	shadow.profiles[hull] = m.catalog.NewProfile(hull, legacySpec, shadow.slots[hull])
	m.owners[owner] = &shadow
	state, err := m.frame(owner, hull)
	if err != nil {
		m.owners[owner] = o
		return fitting.State{}, err
	}
	if err := ctx.Err(); err != nil {
		m.owners[owner] = o
		return fitting.State{}, err
	}
	return state, nil
}
func (m *GeneralMemStore) Load(ctx context.Context, owner, hull string) (fitting.State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fitting.State{}, err
	}
	if !m.valid(owner, hull) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	return m.frame(owner, hull)
}
func (m *GeneralMemStore) Update(ctx context.Context, owner, hull string, revision, pilotRevision int64, fn func(*fitting.State) error) (fitting.State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fitting.State{}, err
	}
	if !m.valid(owner, hull) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	if fn == nil {
		return fitting.State{}, fitting.ErrInvalidState
	}
	before, err := m.frame(owner, hull)
	if err != nil {
		return fitting.State{}, err
	}
	if revision < 0 || pilotRevision < 0 || before.Revision != revision || before.Pilot.Revision != pilotRevision {
		return fitting.State{}, fitting.ErrConflict
	}
	after := before.Clone()
	if err := fn(&after); err != nil {
		return fitting.State{}, err
	}
	if !immutableGeneral(before, after) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	if _, err := m.catalog.ValidateState(after); err != nil {
		return fitting.State{}, err
	}
	if reflect.DeepEqual(before, after) {
		return before, nil
	}
	if before.Revision == math.MaxInt64 {
		return fitting.State{}, fitting.ErrConflict
	}
	if !reflect.DeepEqual(before.Pilot, after.Pilot) {
		if before.Pilot.Revision == math.MaxInt64 {
			return fitting.State{}, fitting.ErrConflict
		}
		after.Pilot.Revision++
	}
	if err := ctx.Err(); err != nil {
		return fitting.State{}, err
	}
	o := m.owners[owner]
	after.Profile.Builds[after.Profile.ActiveBuild].Slots = copySlots(after.Slots)
	o.items = after.Items
	o.slots[hull] = after.Slots
	o.profiles[hull] = after.Profile
	o.pilot = after.Pilot
	o.revisions[hull]++
	o.account.Progress.Credits = after.Progress.Credits
	return m.frame(owner, hull)
}
func (m *GeneralMemStore) GrantSpecializationXP(ctx context.Context, owner, hull, spec, key string, amount int64) (bool, fitting.State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, fitting.State{}, err
	}
	if !m.valid(owner, hull) || len(key) < 1 || len(key) > 128 || strings.ContainsRune(key, 0) || amount <= 0 || amount > 1e9 {
		return false, fitting.State{}, fitting.ErrInvalidState
	}
	s, err := m.frame(owner, hull)
	if err != nil {
		return false, fitting.State{}, err
	}
	current, ok := s.Profile.SpecializationXP[spec]
	if !ok {
		return false, fitting.State{}, fitting.ErrInvalidState
	}
	o := m.owners[owner]
	receipt := specReceipt{Hull: hull, Spec: spec, Amount: amount}
	if old, ok := o.grants[key]; ok {
		if old != receipt {
			return false, fitting.State{}, fitting.ErrInvalidState
		}
		return false, s, nil
	}
	next := current + amount
	if capXP := fitting.DefaultSpecializationCurve.TotalXP(20); next > capXP {
		next = capXP
	}
	if next > current {
		if s.Revision == math.MaxInt64 {
			return false, fitting.State{}, fitting.ErrConflict
		}
		s.Profile.SpecializationXP[spec] = next
		o.profiles[hull] = s.Profile
		o.revisions[hull]++
	}
	o.grants[key] = receipt
	out, err := m.frame(owner, hull)
	return true, out, err
}

// SeedLegacyStarter imports an existing fixed-engine harness state, never an RPC.
func (m *GeneralMemStore) SeedLegacyStarter(s StarterState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o := m.owners[s.OwnerID]
	if o == nil {
		return fitting.ErrNoOwner
	}
	if !s.Valid(s.OwnerID) || o.slots[s.ShipType] != nil {
		return fitting.ErrInvalidState
	}
	o.items[s.InstanceID] = fitting.Item{ID: s.InstanceID, Owner: s.OwnerID, Hull: s.ShipType, Prototype: s.PrototypeID}
	o.slots[s.ShipType] = map[string]string{}
	if s.Equipped {
		o.slots[s.ShipType][s.SlotID] = s.InstanceID
	}
	o.revisions[s.ShipType] = s.Revision
	return nil
}

var _ GeneralStore = (*GeneralMemStore)(nil)

func (m *GeneralMemStore) valid(owner, hull string) bool {
	return m.catalog.HasHull(hull) && fitting.ValidID(owner) && len(owner) <= 64
}
