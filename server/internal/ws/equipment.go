package ws

import (
	"context"
	"errors"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func (h *Handler) loadStarter(ctx context.Context, rec *world.Record) error {
	if h.opts.Equipment == nil {
		return nil
	}
	rec.PersistentEquipment = true
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Unknown legacy hull IDs fall back to Strizh in world.initShip too.
	_, known := h.opts.World.Catalog().Spec(rec.ShipType)
	var s equipment.StarterState
	var err error
	if rec.ShipType == world.DefaultShip || !known {
		s, err = h.opts.Equipment.Ensure(ctx, rec.ID)
	} else {
		s, err = h.opts.Equipment.Load(ctx, rec.ID)
		if errors.Is(err, equipment.ErrNotInitialized) {
			return nil
		}
	}
	if err != nil {
		return err
	}
	if !s.Valid(rec.ID) {
		return equipment.ErrInvalidState
	}
	rec.StarterLoadout = &s
	return nil
}
