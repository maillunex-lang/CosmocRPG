package world

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const (
	StrizhEnginePrototype = "ENG-S-START"
	StrizhEngineSlot      = "tech-1"
	StrizhBoostAbility    = "M01"
)

var (
	ErrModuleUnavailable = errors.New("module_unavailable")
	ErrAbilityNotLearned = errors.New("ability_not_learned")
)

// starterEngineItem represents the existing, bound native starter system. There
// is exactly one per Strizh owner; it cannot be transferred or sold.
type starterEngineItem struct {
	InstanceID, OwnerID, PrototypeID, SlotID string
	Equipped                                 bool
}

// ModuleInfo is presentation data for the current ship, never client authority.
type ModuleInfo struct {
	InstanceID  string `json:"instance_id"`
	PrototypeID string `json:"prototype_id"`
	SlotID      string `json:"slot_id"`
	Name        string `json:"name"`
	Owned       bool   `json:"owned"`
	Equipped    bool   `json:"equipped"`
	AbilityID   string `json:"ability_id"`
}

func starterEngineID(owner string) string {
	h := sha256.Sum256([]byte("CosmocRPG:bound-strizh-engine:" + owner))
	return "eng_" + hex.EncodeToString(h[:16])
}

func (p *Player) initStarterEngine() {
	p.starterEngine = nil
	p.boostRank = 0
	if p.ShipType != DefaultShip {
		return
	}
	p.starterEngine = &starterEngineItem{InstanceID: starterEngineID(p.ID), OwnerID: p.ID, PrototypeID: StrizhEnginePrototype, SlotID: StrizhEngineSlot, Equipped: !p.persistentEquipment}
	if s := p.starterLoadout; s != nil {
		p.starterEngine.Equipped = s.Equipped
	}
	p.boostRank = 1
}

func (p *Player) boostAccess() error {
	if p.ShipType != DefaultShip {
		return nil
	} // Other hulls are not migrated yet.
	item := p.starterEngine
	if item == nil || p.ID == "" || item.OwnerID != p.ID || item.InstanceID != starterEngineID(p.ID) || item.PrototypeID != StrizhEnginePrototype || item.SlotID != StrizhEngineSlot || !item.Equipped {
		return ErrModuleUnavailable
	}
	if p.boostRank < 1 || p.boostRank > 5 {
		return ErrAbilityNotLearned
	}
	return nil
}

func (p *Player) moduleList() []ModuleInfo {
	if p.ShipType != DefaultShip || p.starterEngine == nil {
		return nil
	}
	item := p.starterEngine
	return []ModuleInfo{{InstanceID: item.InstanceID, PrototypeID: item.PrototypeID, SlotID: item.SlotID, Name: "Штатный двигатель", Owned: item.OwnerID == p.ID && item.InstanceID == starterEngineID(p.ID), Equipped: item.Equipped, AbilityID: StrizhBoostAbility}}
}
