package equipment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// One statement gives Load a consistent item/slot/revision snapshot even when
// another transaction is fitting an item concurrently.
func readStarter(ctx context.Context, q rowQuerier, owner string) (StarterState, error) {
	if !validOwner(owner) {
		return StarterState{}, ErrInvalidState
	}
	var revision, slots int64
	var instance, prototype, fitted *string
	err := q.QueryRow(ctx, `
 SELECT l.revision,i.instance_id,i.prototype_id,s.instance_id,
   (SELECT count(*) FROM player_ship_loadout_slots a WHERE a.owner_id=l.owner_id AND a.ship_type=l.ship_type)
 FROM player_ship_loadouts l
 LEFT JOIN player_module_items i ON i.owner_id=l.owner_id AND i.ship_type=l.ship_type AND i.grant_key=$3
 LEFT JOIN player_ship_loadout_slots s ON s.owner_id=l.owner_id AND s.ship_type=l.ship_type AND s.slot_id=$4
 WHERE l.owner_id=$1 AND l.ship_type=$2`, owner, ShipType, starterGrant, world.StrizhEngineSlot).Scan(&revision, &instance, &prototype, &fitted, &slots)
	if errors.Is(err, pgx.ErrNoRows) {
		return StarterState{}, ErrNotInitialized
	}
	if err != nil {
		return StarterState{}, fmt.Errorf("load starter equipment: %w", err)
	}
	if instance == nil || prototype == nil {
		return StarterState{}, ErrInvalidState
	}
	s := initial(owner)
	s.InstanceID = *instance
	s.PrototypeID = *prototype
	s.Revision = revision
	s.Equipped = fitted != nil
	if !valid(s, owner) || (fitted != nil && *fitted != s.InstanceID) || (s.Equipped && slots != 1) || (!s.Equipped && slots != 0) {
		return StarterState{}, ErrInvalidState
	}
	return s, nil
}

func (s *PGStore) Load(ctx context.Context, owner string) (StarterState, error) {
	return readStarter(ctx, s.pool, owner)
}

// Ensure materializes the existing bound entitlement atomically. Only a newly
// created loadout gets its initial fitted slot. Existing unequipped state is
// never reset by reconnect/bootstrap, and missing/corrupt existing items fail.
func (s *PGStore) Ensure(ctx context.Context, owner string) (StarterState, error) {
	if !validOwner(owner) {
		return StarterState{}, ErrInvalidState
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return StarterState{}, err
	}
	defer tx.Rollback(ctx)
	created, err := tx.Exec(ctx, `INSERT INTO player_ship_loadouts(owner_id,ship_type) VALUES($1,$2) ON CONFLICT(owner_id,ship_type) DO NOTHING`, owner, ShipType)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23503" {
			return StarterState{}, ErrNoOwner
		}
		return StarterState{}, fmt.Errorf("initialize loadout: %w", err)
	}
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2 FOR UPDATE`, owner, ShipType).Scan(&revision); err != nil {
		return StarterState{}, err
	}
	if created.RowsAffected() == 1 {
		item := initial(owner)
		if _, err := tx.Exec(ctx, `INSERT INTO player_module_items(instance_id,owner_id,ship_type,prototype_id,grant_key) VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_id,ship_type,grant_key) DO NOTHING`, item.InstanceID, owner, ShipType, item.PrototypeID, starterGrant); err != nil {
			return StarterState{}, fmt.Errorf("materialize starter: %w", err)
		}
		// Validate a reused grant before fitting; never adopt an arbitrary instance.
		var instance, prototype string
		if err := tx.QueryRow(ctx, `SELECT instance_id,prototype_id FROM player_module_items WHERE owner_id=$1 AND ship_type=$2 AND grant_key=$3`, owner, ShipType, starterGrant).Scan(&instance, &prototype); err != nil {
			return StarterState{}, err
		}
		if instance != item.InstanceID || prototype != item.PrototypeID {
			return StarterState{}, ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `INSERT INTO player_ship_loadout_slots(owner_id,ship_type,slot_id,instance_id) VALUES($1,$2,$3,$4)`, owner, ShipType, item.SlotID, item.InstanceID); err != nil {
			return StarterState{}, fmt.Errorf("fit initial starter: %w", err)
		}
	}
	state, err := readStarter(ctx, tx, owner)
	if err != nil {
		return StarterState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StarterState{}, err
	}
	return state, nil
}

// SetEngine is a storage primitive, not the player equipment command. A future
// coordinator must first authorize owner/dock/combat and reconcile the world
// with committed state. Old autosave code never writes these separate tables.
func (s *PGStore) SetEngine(ctx context.Context, owner string, revision int64, equipped bool) (StarterState, error) {
	if !validOwner(owner) {
		return StarterState{}, ErrInvalidState
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return StarterState{}, err
	}
	defer tx.Rollback(ctx)
	var currentRevision int64
	err = tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2 FOR UPDATE`, owner, ShipType).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return StarterState{}, ErrNotInitialized
	}
	if err != nil {
		return StarterState{}, err
	}
	current, err := readStarter(ctx, tx, owner)
	if err != nil {
		return StarterState{}, err
	}
	updated, err := next(current, revision, equipped)
	if err != nil {
		return StarterState{}, err
	}
	if updated.Revision != current.Revision {
		if equipped {
			_, err = tx.Exec(ctx, `INSERT INTO player_ship_loadout_slots(owner_id,ship_type,slot_id,instance_id) VALUES($1,$2,$3,$4)`, owner, ShipType, world.StrizhEngineSlot, current.InstanceID)
		} else {
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `DELETE FROM player_ship_loadout_slots WHERE owner_id=$1 AND ship_type=$2 AND slot_id=$3 AND instance_id=$4`, owner, ShipType, world.StrizhEngineSlot, current.InstanceID)
			if err == nil && tag.RowsAffected() != 1 {
				return StarterState{}, ErrConflict
			}
		}
		if err != nil {
			return StarterState{}, fmt.Errorf("change starter slot: %w", err)
		}
		tag, err := tx.Exec(ctx, `UPDATE player_ship_loadouts SET revision=$3,updated_at=now() WHERE owner_id=$1 AND ship_type=$2 AND revision=$4`, owner, ShipType, updated.Revision, current.Revision)
		if err != nil {
			return StarterState{}, fmt.Errorf("commit loadout revision: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return StarterState{}, ErrConflict
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return StarterState{}, err
	}
	return updated, nil
}

var _ StarterStore = (*PGStore)(nil)
