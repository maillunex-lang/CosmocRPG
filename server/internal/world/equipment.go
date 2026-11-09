package world

import "errors"

var (
	ErrEquipmentUnavailable = errors.New("equipment_unavailable")
	ErrEquipmentConflict    = errors.New("equipment_revision_conflict")
	ErrEquipmentBusy        = errors.New("equipment_busy")
	ErrEquipmentCombat      = errors.New("equipment_in_combat")
	ErrEquipmentHull        = errors.New("equipment_unsupported_ship")
)

// Alpha service radius at the actual primary station, not any safe-zone.
const EquipmentDockRadius = 20.0
const equipmentCombatGrace = 10.0

type EquipmentStatus struct {
	Revision  *int64 `json:"revision,omitempty"`
	CanChange bool   `json:"can_change"`
	Reason    string `json:"reason,omitempty"`
}

func (w *World) equipmentCheck(p *Player) error {
	if p.ShipType != DefaultShip {
		return ErrEquipmentHull
	}
	if !p.persistentEquipment || p.starterLoadout == nil {
		return ErrEquipmentUnavailable
	}
	if p.deadUntil > w.t {
		return ErrDead
	}
	if p.Attacking || p.combatUntil > w.t {
		return ErrEquipmentCombat
	}
	if dist(p.Pos, Vec{w.station.Pos[0], w.station.Pos[1]}) > EquipmentDockRadius {
		return ErrNoStable
	}
	if p.Move.length() > 0.05 || p.Speed > 0.1 || p.Autopilot || p.mining != "" || p.boostUntil > w.t {
		return ErrEquipmentBusy
	}
	return nil
}

func (w *World) equipmentStatus(p *Player) *EquipmentStatus {
	if !p.persistentEquipment {
		return nil
	}
	s := &EquipmentStatus{}
	if p.starterLoadout != nil && p.ShipType == DefaultShip {
		rev := p.starterLoadout.Revision
		s.Revision = &rev
	}
	if err := w.equipmentCheck(p); err != nil {
		s.Reason = err.Error()
	} else {
		s.CanChange = true
	}
	return s
}

// ChangeStarter serializes dock/combat checks, the bounded storage callback and
// application under the simulation lock. No movement/damage/jump/hull change
// can slip between validation and commit. The callback must NOT call World or
// Router methods; the WS coordinator supplies its own short DB deadlines.
// On storage failure it returns an authoritative reload, if available. If the
// outcome cannot be established we disable boost until an explicit reload.
func (w *World) ChangeStarter(id string, revision int64, equipped bool, commit func() (StarterLoadout, error)) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return ErrNoPlayer
	}
	if err := w.equipmentCheck(p); err != nil {
		return err
	}
	if revision < 0 || revision != p.starterLoadout.Revision {
		return ErrEquipmentConflict
	}
	if commit == nil {
		return ErrEquipmentUnavailable
	}
	expectedRevision := revision
	if equipped != p.starterLoadout.Equipped {
		expectedRevision++
	}
	state, err := commit()
	if err == nil && (state.Revision != expectedRevision || state.Equipped != equipped) {
		state = StarterLoadout{}
	}
	if state.Valid(id) && p.restoreStarter(&state) {
		p.initStarterEngine()
		if !state.Equipped {
			p.boostUntil = 0
		}
		return err
	}
	// Never report success or retain an enabled ability with unknown DB outcome.
	p.starterLoadout = nil
	p.initStarterEngine()
	p.boostUntil = 0
	return ErrEquipmentUnavailable
}
