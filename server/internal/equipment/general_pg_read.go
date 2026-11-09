package equipment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

type GeneralPGStore struct {
	pool    *pgxpool.Pool
	catalog *fitting.Catalog
}

func NewGeneralPGStore(pool *pgxpool.Pool, c *fitting.Catalog) *GeneralPGStore {
	return &GeneralPGStore{pool: pool, catalog: c}
}
func (s *GeneralPGStore) valid(owner, hull string) bool {
	return s.catalog.HasHull(hull) && fitting.ValidID(owner) && len(owner) <= 64
}
func strictDecode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fitting.ErrInvalidState
	}
	if d.Decode(new(any)) != io.EOF {
		return fitting.ErrInvalidState
	}
	return nil
}
func copySlots(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func lockGeneralOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	// NO KEY UPDATE serializes credits/learning without blocking native FK
	// KEY SHARE checks in an old bootstrap transaction.
	var id string
	err := tx.QueryRow(ctx, `SELECT public_id FROM players WHERE public_id=$1 FOR NO KEY UPDATE`, owner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fitting.ErrNoOwner
	}
	return err
}
func readSlots(ctx context.Context, tx pgx.Tx, owner, hull string) (map[string]string, error) {
	rows, err := tx.Query(ctx, `SELECT slot_id,instance_id FROM player_ship_loadout_slots WHERE owner_id=$1 AND ship_type=$2`, owner, hull)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var slot, id string
		if err := rows.Scan(&slot, &id); err != nil {
			return nil, err
		}
		out[slot] = id
	}
	return out, rows.Err()
}
func (s *GeneralPGStore) read(ctx context.Context, tx pgx.Tx, owner, hull string) (fitting.State, error) {
	out := fitting.State{Owner: owner, Hull: hull, Items: map[string]fitting.Item{}}
	var attrs []byte
	err := tx.QueryRow(ctx, `SELECT pilot_xp,credits,pilot_attrs FROM players WHERE public_id=$1`, owner).Scan(&out.Progress.PilotXP, &out.Progress.Credits, &attrs)
	if errors.Is(err, pgx.ErrNoRows) {
		return fitting.State{}, fitting.ErrNoOwner
	}
	if err != nil {
		return fitting.State{}, err
	}
	if err := json.Unmarshal(attrs, &out.Progress.Attributes); err != nil {
		return fitting.State{}, fitting.ErrInvalidState
	}
	err = tx.QueryRow(ctx, `SELECT ship_xp FROM player_ships WHERE player_id=$1 AND ship_type=$2`, owner, hull).Scan(&out.Progress.ShipXP)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fitting.State{}, err
	}
	err = tx.QueryRow(ctx, `SELECT revision FROM player_ship_loadouts WHERE owner_id=$1 AND ship_type=$2`, owner, hull).Scan(&out.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return fitting.State{}, fitting.ErrNotInitialized
	}
	if err != nil {
		return fitting.State{}, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT data FROM player_rpg_profiles WHERE owner_id=$1 AND ship_type=$2`, owner, hull).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return fitting.State{}, fitting.ErrNotInitialized
	}
	if err != nil {
		return fitting.State{}, err
	}
	if err := strictDecode(raw, &out.Profile); err != nil {
		return fitting.State{}, err
	}
	var pilotRevision int64
	err = tx.QueryRow(ctx, `SELECT revision,data FROM player_rpg_pilots WHERE owner_id=$1`, owner).Scan(&pilotRevision, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	if err != nil {
		return fitting.State{}, err
	}
	if err := strictDecode(raw, &out.Pilot); err != nil {
		return fitting.State{}, err
	}
	out.Pilot.Revision = pilotRevision
	rows, err := tx.Query(ctx, `SELECT i.instance_id,i.prototype_id,i.ship_type,i.grant_key,i.engineering_rank,COALESCE(s.ship_type,''),COALESCE(s.slot_id,'') FROM player_module_items i LEFT JOIN player_ship_loadout_slots s ON s.instance_id=i.instance_id AND s.owner_id=i.owner_id WHERE i.owner_id=$1`, owner)
	if err != nil {
		return fitting.State{}, err
	}
	for rows.Next() {
		item := fitting.Item{Owner: owner}
		var origin, grant string
		if err := rows.Scan(&item.ID, &item.Prototype, &origin, &grant, &item.Engineering, &item.EquippedHull, &item.EquippedSlot); err != nil {
			rows.Close()
			return fitting.State{}, err
		}
		if fitting.BoundGrant(grant) {
			item.Hull = origin
		}
		out.Items[item.ID] = item
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fitting.State{}, err
	}
	out.Slots, err = readSlots(ctx, tx, owner, hull)
	if err != nil {
		return fitting.State{}, err
	}
	if out.Profile.ActiveBuild < 0 || out.Profile.ActiveBuild >= len(out.Profile.Builds) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	// SQL slots are canonical, including a valid legacy engine toggle written
	// before this profile existed. Saved builds are references, never item copies.
	out.Profile.Builds[out.Profile.ActiveBuild].Slots = copySlots(out.Slots)
	if _, err := s.catalog.ValidateState(out); err != nil {
		return fitting.State{}, fmt.Errorf("invalid fitting state: %w", err)
	}
	return out, nil
}
func (s *GeneralPGStore) Load(ctx context.Context, owner, hull string) (fitting.State, error) {
	if !s.valid(owner, hull) {
		return fitting.State{}, fitting.ErrInvalidState
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fitting.State{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.read(ctx, tx, owner, hull)
	if err != nil {
		return fitting.State{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return fitting.State{}, err
	}
	return out, nil
}
