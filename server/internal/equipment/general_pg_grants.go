package equipment

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

func (s *GeneralPGStore) GrantSpecializationXP(ctx context.Context, owner, hull, spec, key string, amount int64) (bool, fitting.State, error) {
	if !s.valid(owner, hull) || len(key) < 1 || len(key) > 128 || strings.ContainsRune(key, 0) || amount <= 0 || amount > 1e9 {
		return false, fitting.State{}, fitting.ErrInvalidState
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fitting.State{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockGeneralOwner(ctx, tx, owner); err != nil {
		return false, fitting.State{}, err
	}
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2 FOR UPDATE`, owner, hull).Scan(&revision); err != nil {
		return false, fitting.State{}, err
	}
	out, err := s.read(ctx, tx, owner, hull)
	if err != nil {
		return false, fitting.State{}, err
	}
	current, exists := out.Profile.SpecializationXP[spec]
	if !exists {
		return false, fitting.State{}, fitting.ErrInvalidState
	}
	tag, err := tx.Exec(ctx, `INSERT INTO player_specialization_grants(owner_id,grant_key,ship_type,specialization_id,amount) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, owner, key, hull, spec, amount)
	if err != nil {
		return false, fitting.State{}, err
	}
	if tag.RowsAffected() == 0 {
		var priorHull, priorSpec string
		var priorAmount int64
		if err := tx.QueryRow(ctx, `SELECT ship_type,specialization_id,amount FROM player_specialization_grants WHERE owner_id=$1 AND grant_key=$2`, owner, key).Scan(&priorHull, &priorSpec, &priorAmount); err != nil {
			return false, fitting.State{}, err
		}
		if priorHull != hull || priorSpec != spec || priorAmount != amount {
			return false, fitting.State{}, fitting.ErrInvalidState
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fitting.State{}, err
		}
		return false, out, nil
	}
	capXP := fitting.DefaultSpecializationCurve.TotalXP(20)
	next := current + amount
	if next > capXP {
		next = capXP
	}
	if next > current {
		if out.Revision == math.MaxInt64 {
			return false, fitting.State{}, fitting.ErrConflict
		}
		out.Profile.SpecializationXP[spec] = next
		raw, err := json.Marshal(out.Profile)
		if err != nil {
			return false, fitting.State{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE player_rpg_profiles SET data=$3 WHERE owner_id=$1 AND ship_type=$2`, owner, hull, raw); err != nil {
			return false, fitting.State{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE player_ship_loadouts SET revision=revision+1,updated_at=now() WHERE owner_id=$1 AND ship_type=$2`, owner, hull); err != nil {
			return false, fitting.State{}, err
		}
	}
	out, err = s.read(ctx, tx, owner, hull)
	if err != nil {
		return false, fitting.State{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fitting.State{}, err
	}
	return true, out, nil
}

var _ GeneralStore = (*GeneralPGStore)(nil)
