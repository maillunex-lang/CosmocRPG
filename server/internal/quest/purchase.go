package quest

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
)

func readPurchaseCargo(ctx context.Context, tx pgx.Tx, pid string) (economy.Result, error) {
	var result economy.Result
	var raw []byte
	var c economy.Cargo
	// FOR UPDATE — барьер неизвестного исхода COMMIT. Receipt читается отдельным
	// следующим statement, чтобы видеть завершившуюся транзакцию при READ COMMITTED.
	if err := tx.QueryRow(ctx, `SELECT credits,hold,cargo_revision FROM players WHERE public_id=$1 FOR UPDATE`, pid).Scan(&result.Credits, &raw, &c.Revision); err != nil {
		return result, err
	}
	if err := json.Unmarshal(raw, &c.Hold); err != nil {
		return result, err
	}
	if err := economy.ValidateCargo(c); err != nil {
		return result, err
	}
	result.Known = true
	result.Cargo = &c
	return result, nil
}
func lookupPurchase(ctx context.Context, tx pgx.Tx, pid, key string, hash [32]byte, result economy.Result) (economy.Result, error) {
	var old []byte
	err := tx.QueryRow(ctx, `SELECT intent_hash,spent FROM resource_purchase_receipts WHERE player_id=$1 AND operation_key=$2`, pid, key).Scan(&old, &result.Spent)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return economy.Result{}, err
	}
	if string(old) != string(hash[:]) {
		return result, economy.ErrConflict
	}
	result.Found = true
	return result, nil
}
func (s *PGStore) LookupPurchase(ctx context.Context, pid, key, resource string, amount float64) (economy.Result, error) {
	hash, err := economy.Intent(resource, amount)
	if err != nil || !economy.ValidKey(key) {
		return economy.Result{}, economy.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return economy.Result{}, err
	}
	defer tx.Rollback(ctx)
	state, err := readPurchaseCargo(ctx, tx, pid)
	if err != nil {
		return economy.Result{}, err
	}
	return lookupPurchase(ctx, tx, pid, key, hash, state)
}
func (s *PGStore) Purchase(ctx context.Context, r economy.Request) (economy.Result, error) {
	hash, cost, after, err := economy.Prepare(r)
	if err != nil {
		return economy.Result{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return economy.Result{}, err
	}
	defer tx.Rollback(ctx)
	state, err := readPurchaseCargo(ctx, tx, r.Player)
	if err != nil {
		return economy.Result{}, err
	}
	state, err = lookupPurchase(ctx, tx, r.Player, r.Key, hash, state)
	if err != nil || state.Found {
		return state, err
	}
	if state.Cargo.Revision > r.Before.Revision {
		return state, economy.ErrConflict
	}
	if state.Credits < cost {
		return state, economy.ErrNoCredits
	}
	raw, err := json.Marshal(after.Hold)
	if err != nil {
		return economy.Result{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE players SET credits=credits-$2,hold=$3::jsonb,cargo_revision=$4,updated_at=now() WHERE public_id=$1`, r.Player, cost, raw, after.Revision); err != nil {
		return economy.Result{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO resource_purchase_receipts(player_id,operation_key,intent_hash,spent,cargo_revision) VALUES($1,$2,$3,$4,$5)`, r.Player, r.Key, hash[:], cost, after.Revision); err != nil {
		return economy.Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return economy.Result{}, err
	}
	return economy.Result{Known: true, Found: true, Cargo: &after, Credits: state.Credits - cost, Spent: cost}, nil
}
func (s *PGStore) PurchaseCargo(ctx context.Context, pid string) (*economy.Cargo, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	state, err := readPurchaseCargo(ctx, tx, pid)
	return state.Cargo, err
}

type memoryPurchase struct {
	hash  [32]byte
	spent int64
}

func (m *MemStore) LookupPurchase(ctx context.Context, pid, key, resource string, amount float64) (economy.Result, error) {
	if err := ctx.Err(); err != nil {
		return economy.Result{}, err
	}
	hash, err := economy.Intent(resource, amount)
	if err != nil || !economy.ValidKey(key) {
		return economy.Result{}, economy.ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lookupPurchase(pid, key, hash)
}
func (m *MemStore) lookupPurchase(pid, key string, hash [32]byte) (economy.Result, error) {
	result := economy.Result{Known: true, Credits: m.credits[pid], Cargo: economy.Clone(m.purchaseCargo[pid])}
	if p, ok := m.purchases[pid+"|"+key]; ok {
		if p.hash != hash {
			return result, economy.ErrConflict
		}
		result.Found = true
		result.Spent = p.spent
	}
	return result, nil
}
func (m *MemStore) Purchase(ctx context.Context, r economy.Request) (economy.Result, error) {
	if err := ctx.Err(); err != nil {
		return economy.Result{}, err
	}
	hash, cost, after, err := economy.Prepare(r)
	if err != nil {
		return economy.Result{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result, err := m.lookupPurchase(r.Player, r.Key, hash)
	if err != nil || result.Found {
		return result, err
	}
	if result.Cargo != nil && result.Cargo.Revision > r.Before.Revision {
		return result, economy.ErrConflict
	}
	if result.Credits < cost {
		return result, economy.ErrNoCredits
	}
	if m.purchases == nil {
		m.purchases = map[string]memoryPurchase{}
		m.purchaseCargo = map[string]*economy.Cargo{}
	}
	m.credits[r.Player] -= cost
	m.purchases[r.Player+"|"+r.Key] = memoryPurchase{hash, cost}
	m.purchaseCargo[r.Player] = economy.Clone(&after)
	return economy.Result{Known: true, Found: true, Cargo: economy.Clone(&after), Credits: m.credits[r.Player], Spent: cost}, nil
}
func (m *MemStore) PurchaseCargo(ctx context.Context, pid string) (*economy.Cargo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return economy.Clone(m.purchaseCargo[pid]), nil
}

// Сервис использует тот же кошелёк, что награды/торговля существующих игроков.
func (s *Service) Purchase(ctx context.Context, r economy.Request) (economy.Result, error) {
	store, ok := s.store.(economy.Store)
	if !ok {
		return economy.Result{}, economy.ErrUnavailable
	}
	result, err := store.Purchase(ctx, r)
	s.purchaseCredits(r.Player, result)
	return result, err
}
func (s *Service) LookupPurchase(ctx context.Context, pid, key, resource string, amount float64) (economy.Result, error) {
	store, ok := s.store.(economy.Store)
	if !ok {
		return economy.Result{}, economy.ErrUnavailable
	}
	result, err := store.LookupPurchase(ctx, pid, key, resource, amount)
	s.purchaseCredits(pid, result)
	return result, err
}
func (s *Service) PurchaseCargo(ctx context.Context, pid string) (*economy.Cargo, error) {
	store, ok := s.store.(economy.Store)
	if !ok {
		return nil, nil
	}
	return store.PurchaseCargo(ctx, pid)
}
func (s *Service) purchaseCredits(pid string, r economy.Result) {
	if !r.Known {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ps := s.players[pid]; ps != nil {
		ps.data.Credits = r.Credits
	}
}
