// Package equipment stores ownership and versioned loadouts. This first slice
// manages only the existing bound Strizh engine. It exposes no player RPC and
// does not decide whether a ship is docked or in combat: a future world
// coordinator must validate those conditions before enabling equipment changes.
package equipment

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

const ShipType = "FTR-001"
const starterGrant = "native-engine:v1"

var (
	ErrNoOwner        = errors.New("equipment_no_owner")
	ErrNotInitialized = errors.New("equipment_not_initialized")
	ErrConflict       = errors.New("equipment_revision_conflict")
	ErrInvalidState   = errors.New("equipment_invalid_state")
)

// StarterState is a server-owned storage snapshot, never authorization supplied
// by a client. Equipped=false retains ownership; Ensure must never re-equip it.
type StarterState = world.StarterLoadout

// StarterStore is deliberately narrow: there is no arbitrary item grant,
// transfer, sale or caller-selected prototype/instance/slot operation.
type StarterStore interface {
	Ensure(context.Context, string) (StarterState, error)
	Load(context.Context, string) (StarterState, error)
	SetEngine(context.Context, string, int64, bool) (StarterState, error)
}

func validOwner(owner string) bool {
	return owner != "" && len(owner) <= 64 && strings.TrimSpace(owner) == owner
}

func initial(owner string) StarterState {
	return StarterState{OwnerID: owner, ShipType: ShipType, InstanceID: world.StarterEngineInstanceID(owner), PrototypeID: world.StrizhEnginePrototype, SlotID: world.StrizhEngineSlot, Equipped: true}
}

func valid(s StarterState, owner string) bool {
	want := initial(owner)
	return s.OwnerID == want.OwnerID && s.ShipType == want.ShipType && s.InstanceID == want.InstanceID && s.PrototypeID == want.PrototypeID && s.SlotID == want.SlotID && s.Revision >= 0
}

func next(s StarterState, revision int64, equipped bool) (StarterState, error) {
	if revision < 0 || s.Revision != revision {
		return StarterState{}, ErrConflict
	}
	if s.Equipped == equipped {
		return s, nil
	}
	if s.Revision == math.MaxInt64 {
		return StarterState{}, ErrConflict
	}
	s.Equipped = equipped
	s.Revision++
	return s, nil
}
