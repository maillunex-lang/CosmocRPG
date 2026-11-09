package equipment

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

func immutableGeneral(before, after fitting.State) bool {
	if before.Owner != after.Owner || before.Hull != after.Hull || before.Revision != after.Revision || before.Pilot.Revision != after.Pilot.Revision || before.Progress.PilotXP != after.Progress.PilotXP || before.Progress.ShipXP != after.Progress.ShipXP || !reflect.DeepEqual(before.Progress.Attributes, after.Progress.Attributes) || after.Progress.Credits > before.Progress.Credits {
		return false
	}
	if !reflect.DeepEqual(before.Profile.SpecializationXP, after.Profile.SpecializationXP) {
		return false
	}
	for id, old := range before.Items {
		next, ok := after.Items[id]
		if !ok || old.ID != next.ID || old.Owner != next.Owner || old.Hull != next.Hull || old.Prototype != next.Prototype || old.EquippedHull != next.EquippedHull || old.EquippedSlot != next.EquippedSlot || next.Engineering < old.Engineering {
			return false
		}
		if next.Engineering != old.Engineering && old.EquippedHull != "" && old.EquippedHull != before.Hull {
			return false
		}
	}
	for id, item := range after.Items {
		if _, existed := before.Items[id]; !existed && (item.Owner != before.Owner || item.Hull != "" || item.Engineering != 0 || item.EquippedHull != "" || item.EquippedSlot != "") {
			return false
		}
	}
	return true
}

func (s *GeneralPGStore) Update(ctx context.Context, owner, hull string, revision, pilotRevision int64, fn func(*fitting.State) error) (fitting.State, error) {
	if !s.valid(owner, hull) || fn == nil {
		return fitting.State{}, fitting.ErrInvalidState
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fitting.State{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockGeneralOwner(ctx, tx, owner); err != nil {
		return fitting.State{}, err
	}
	var actual int64
	err = tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2 FOR UPDATE`, owner, hull).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return fitting.State{}, fitting.ErrNotInitialized
	}
	if err != nil {
		return fitting.State{}, err
	}
	before, err := s.read(ctx, tx, owner, hull)
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
	if _, err := s.catalog.ValidateState(after); err != nil {
		return fitting.State{}, err
	}
	if reflect.DeepEqual(before, after) {
		if err := tx.Commit(ctx); err != nil {
			return fitting.State{}, err
		}
		return before, nil
	}
	if before.Revision == math.MaxInt64 {
		return fitting.State{}, fitting.ErrConflict
	}
	pilotChanged := !reflect.DeepEqual(before.Pilot, after.Pilot)
	if pilotChanged && before.Pilot.Revision == math.MaxInt64 {
		return fitting.State{}, fitting.ErrConflict
	}
	// Slots, item upgrades/purchases, global learning and real credits commit
	// together. Any later FK/trigger/IO failure rolls back all of them.
	for id, item := range after.Items {
		old, existed := before.Items[id]
		if !existed {
			if _, err := tx.Exec(ctx, `INSERT INTO player_module_items(instance_id,owner_id,ship_type,prototype_id,grant_key,engineering_rank) VALUES($1,$2,$3,$4,$5,0)`, id, owner, hull, item.Prototype, "purchase:"+id); err != nil {
				return fitting.State{}, err
			}
		} else if item.Engineering != old.Engineering {
			if _, err := tx.Exec(ctx, `UPDATE player_module_items SET engineering_rank=$3 WHERE instance_id=$1 AND owner_id=$2`, id, owner, item.Engineering); err != nil {
				return fitting.State{}, err
			}
		}
	}
	if !reflect.DeepEqual(before.Slots, after.Slots) {
		if _, err := tx.Exec(ctx, `DELETE FROM player_ship_loadout_slots WHERE owner_id=$1 AND ship_type=$2`, owner, hull); err != nil {
			return fitting.State{}, err
		}
		for slot, id := range after.Slots {
			if id == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO player_ship_loadout_slots(owner_id,ship_type,slot_id,instance_id) VALUES($1,$2,$3,$4)`, owner, hull, slot, id); err != nil {
				return fitting.State{}, err
			}
		}
	}
	after.Profile.Builds[after.Profile.ActiveBuild].Slots = copySlots(after.Slots)
	profileRaw, err := json.Marshal(after.Profile)
	if err != nil {
		return fitting.State{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE player_rpg_profiles SET data=$3 WHERE owner_id=$1 AND ship_type=$2`, owner, hull, profileRaw); err != nil {
		return fitting.State{}, err
	}
	if pilotChanged {
		after.Pilot.Revision++
		if _, err := tx.Exec(ctx, `UPDATE player_rpg_pilots SET revision=$2,data=$3 WHERE owner_id=$1`, owner, after.Pilot.Revision, mustPilotJSON(after.Pilot)); err != nil {
			return fitting.State{}, err
		}
	}
	if after.Progress.Credits != before.Progress.Credits {
		if _, err := tx.Exec(ctx, `UPDATE players SET credits=$2,updated_at=now() WHERE public_id=$1`, owner, after.Progress.Credits); err != nil {
			return fitting.State{}, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE player_ship_loadouts SET revision=revision+1,updated_at=now() WHERE owner_id=$1 AND ship_type=$2`, owner, hull); err != nil {
		return fitting.State{}, err
	}
	out, err := s.read(ctx, tx, owner, hull)
	if err != nil {
		return fitting.State{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return fitting.State{}, err
	}
	return out, nil
}
