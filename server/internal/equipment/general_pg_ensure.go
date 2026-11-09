package equipment

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

func (s *GeneralPGStore) Ensure(ctx context.Context, owner, hull, legacySpec string) (fitting.State, error) {
	if !s.valid(owner, hull) {
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
	created, err := tx.Exec(ctx, `INSERT INTO player_ship_loadouts(owner_id,ship_type) VALUES($1,$2) ON CONFLICT DO NOTHING`, owner, hull)
	if err != nil {
		return fitting.State{}, err
	}
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2 FOR UPDATE`, owner, hull).Scan(&revision); err != nil {
		return fitting.State{}, err
	}
	var profileExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_rpg_profiles WHERE owner_id=$1 AND ship_type=$2)`, owner, hull).Scan(&profileExists); err != nil {
		return fitting.State{}, err
	}
	if !profileExists {
		if created.RowsAffected() == 0 && hull == fitting.ShipStrizh {
			var id, prototype string
			err := tx.QueryRow(ctx, `SELECT instance_id,prototype_id FROM player_module_items WHERE owner_id=$1 AND ship_type=$2 AND grant_key='native-engine:v1'`, owner, hull).Scan(&id, &prototype)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && (id != fitting.NativeItemID(owner, hull, "tech-1", "ENG-S-START") || prototype != "ENG-S-START") {
				return fitting.State{}, fitting.ErrInvalidState
			}
			if err != nil {
				return fitting.State{}, err
			}
		}
		pilot := fitting.NewPilotData()
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT data FROM player_rpg_pilots WHERE owner_id=$1`, owner).Scan(&raw)
		if err == nil {
			if err := strictDecode(raw, &pilot); err != nil {
				return fitting.State{}, err
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
			var orphan bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_rpg_profiles WHERE owner_id=$1)`, owner).Scan(&orphan); err != nil {
				return fitting.State{}, err
			}
			if orphan {
				return fitting.State{}, fitting.ErrInvalidState
			}
			if _, err := tx.Exec(ctx, `INSERT INTO player_rpg_pilots(owner_id,data) VALUES($1,$2)`, owner, mustPilotJSON(pilot)); err != nil {
				return fitting.State{}, err
			}
		} else {
			return fitting.State{}, err
		}
		if pilot.Version != 1 || pilot.Learned == nil || pilot.Talents == nil || pilot.Research == nil {
			return fitting.State{}, fitting.ErrInvalidState
		}
		var pilotRevision int64
		if err := tx.QueryRow(ctx, `SELECT revision FROM player_rpg_pilots WHERE owner_id=$1`, owner).Scan(&pilotRevision); err != nil {
			return fitting.State{}, err
		}
		pilot.Revision = pilotRevision
		slots, err := readSlots(ctx, tx, owner, hull)
		if err != nil {
			return fitting.State{}, err
		}
		h, _ := s.catalog.Hull(hull)
		learned := false
		for _, m := range h.Starter {
			id, grant := fitting.NativeItemID(owner, hull, m.SlotID, m.PrototypeID), fitting.NativeGrant(hull, m.SlotID, m.PrototypeID)
			inserted, err := tx.Exec(ctx, `INSERT INTO player_module_items(instance_id,owner_id,ship_type,prototype_id,grant_key) VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_id,ship_type,grant_key) DO NOTHING`, id, owner, hull, m.PrototypeID, grant)
			if err != nil {
				return fitting.State{}, err
			}
			var actualID, actualPrototype string
			if err := tx.QueryRow(ctx, `SELECT instance_id,prototype_id FROM player_module_items WHERE owner_id=$1 AND ship_type=$2 AND grant_key=$3`, owner, hull, grant).Scan(&actualID, &actualPrototype); err != nil {
				return fitting.State{}, err
			}
			if actualID != id || actualPrototype != m.PrototypeID {
				return fitting.State{}, fitting.ErrInvalidState
			}
			if inserted.RowsAffected() == 1 && slots[m.SlotID] == "" {
				if _, err := tx.Exec(ctx, `INSERT INTO player_ship_loadout_slots(owner_id,ship_type,slot_id,instance_id) VALUES($1,$2,$3,$4)`, owner, hull, m.SlotID, id); err != nil {
					return fitting.State{}, err
				}
				slots[m.SlotID] = id
			}
			module, _ := s.catalog.Module(m.PrototypeID)
			for _, skill := range module.Abilities {
				if pilot.Learned[skill] == 0 {
					pilot.Learned[skill] = 1
					learned = true
				}
			}
		}
		profile := s.catalog.NewProfile(hull, legacySpec, slots)
		profileRaw, _ := json.Marshal(profile)
		if _, err := tx.Exec(ctx, `INSERT INTO player_rpg_profiles(owner_id,ship_type,data) VALUES($1,$2,$3)`, owner, hull, profileRaw); err != nil {
			return fitting.State{}, err
		}
		if created.RowsAffected() == 0 {
			if revision == math.MaxInt64 {
				return fitting.State{}, fitting.ErrConflict
			}
			if _, err := tx.Exec(ctx, `UPDATE player_ship_loadouts SET revision=revision+1,updated_at=now() WHERE owner_id=$1 AND ship_type=$2`, owner, hull); err != nil {
				return fitting.State{}, err
			}
		}
		if learned {
			if pilot.Revision == math.MaxInt64 {
				return fitting.State{}, fitting.ErrConflict
			}
			pilot.Revision++
			if _, err := tx.Exec(ctx, `UPDATE player_rpg_pilots SET revision=$2,data=$3 WHERE owner_id=$1`, owner, pilot.Revision, mustPilotJSON(pilot)); err != nil {
				return fitting.State{}, err
			}
		}
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
func mustPilotJSON(p fitting.PilotData) []byte { b, _ := json.Marshal(p); return b }
