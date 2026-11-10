package world

import (
	"errors"
	"math"
)

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
	Pending   bool   `json:"pending,omitempty"`
}

func (w *World) equipmentCheck(p *Player) error {
	if p.actionBusy() {
		return ErrEquipmentBusy
	}
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

// CanRefit — смена модулей у станции, на любом корпусе. Штатный двигатель Стрижа сюда не входит.
func (w *World) CanRefit(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return ErrNoPlayer
	}
	if p.actionBusy() {
		return ErrEquipmentBusy
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
	s := &EquipmentStatus{Pending: p.equipmentLease != nil}
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

// equipmentLease reserves only this actor. Storage never holds the world lock.
type equipmentLease struct {
	player           *Player
	expectedRevision int64
	equipped         bool
}

// ChangeStarter accepts a dock-only operation before dispatching storage. Hits
// after acceptance still apply immediately; the accepted change precedes them.
// Its actor cannot move/transfer/cast while pending. Other actors and simulation
// continue normally. commit must obey its IO deadlines and must not recurse into
// ChangeStarter for the same actor.
func (w *World) ChangeStarter(id string, revision int64, equipped bool, commit func() (StarterLoadout, error)) error {
	w.mu.Lock()
	p := w.players[id]
	if p == nil {
		w.mu.Unlock()
		return ErrNoPlayer
	}
	if err := w.equipmentCheck(p); err != nil {
		w.mu.Unlock()
		return err
	}
	if revision < 0 || revision != p.starterLoadout.Revision {
		w.mu.Unlock()
		return ErrEquipmentConflict
	}
	if commit == nil {
		w.mu.Unlock()
		return ErrEquipmentUnavailable
	}
	expected := revision
	if equipped != p.starterLoadout.Equipped {
		if revision == math.MaxInt64 {
			w.mu.Unlock()
			return ErrEquipmentConflict
		}
		expected++
	}
	lease := &equipmentLease{player: p, expectedRevision: expected, equipped: equipped}
	p.equipmentLease = lease
	w.mu.Unlock()
	// Also release/fail closed if an unexpected callback panic unwinds the caller.
	defer w.abandonEquipment(lease)
	state, err := commit()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.players[id] != p || p.equipmentLease != lease {
		return ErrEquipmentUnavailable
	}
	p.equipmentLease = nil
	if err == nil && (state.Revision != lease.expectedRevision || state.Equipped != lease.equipped) {
		state = StarterLoadout{}
	}
	if state.Valid(id) && p.restoreStarter(&state) {
		p.refreshStarterEngine()
		return err
	}
	p.starterLoadout = nil
	p.refreshStarterEngine()
	p.boostUntil = 0
	return ErrEquipmentUnavailable
}

func (w *World) abandonEquipment(lease *equipmentLease) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := lease.player
	if w.players[p.ID] != p || p.equipmentLease != lease {
		return
	}
	p.equipmentLease = nil
	p.starterLoadout = nil
	p.refreshStarterEngine()
	p.boostUntil = 0
}
