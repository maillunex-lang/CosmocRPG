package world

// StarterLoadout is trusted server storage state for the bound Strizh engine.
// It travels between worlds, but is never saved by the legacy player autosave.
type StarterLoadout struct {
	OwnerID, ShipType, InstanceID, PrototypeID, SlotID string
	Equipped                                           bool
	Revision                                           int64
}

func (s StarterLoadout) Valid(owner string) bool {
	return owner != "" && s.OwnerID == owner && s.ShipType == DefaultShip &&
		s.InstanceID == starterEngineID(owner) && s.PrototypeID == StrizhEnginePrototype &&
		s.SlotID == StrizhEngineSlot && s.Revision >= 0
}

func copyStarter(s *StarterLoadout) *StarterLoadout {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

// restoreStarter never lets an older in-flight bootstrap overwrite a newer
// committed revision already applied to the still-live ship.
func (p *Player) restoreStarter(s *StarterLoadout) bool {
	if s == nil {
		return true
	}
	if !s.Valid(p.ID) {
		return false
	}
	if old := p.starterLoadout; old != nil {
		if s.Revision < old.Revision {
			return true
		}
		if s.Revision == old.Revision && *s != *old {
			return false
		}
	}
	p.starterLoadout = copyStarter(s)
	return true
}

// RestoreStarter applies a storage read before switching to Strizh. Other hulls
// keep their legacy abilities and retain this cached Strizh loadout for later.
func (w *World) RestoreStarter(id string, s StarterLoadout) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil || p.actionBusy() || !p.restoreStarter(&s) {
		return false
	}
	p.refreshStarterEngine()
	return true
}
