package corp

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrWarehouseUnavailable = errors.New("warehouse_unavailable")
	ErrBadWarehouseResource = errors.New("bad_warehouse_resource")
	ErrWarehouseAmount      = errors.New("bad_warehouse_amount")
	ErrWarehouseAccess      = errors.New("warehouse_access_denied")
	ErrWarehouseEmpty       = errors.New("warehouse_empty")
)

var warehouseResources = map[string]bool{"iron_ore": true, "crystals": true, "gas": true, "scrap": true}

type WarehouseItem struct {
	Resource string  `json:"resource"`
	Amount   float64 `json:"amount"`
}
type WarehouseAudit struct {
	ID        string    `json:"id"`
	CorpID    string    `json:"corp_id"`
	PlayerID  string    `json:"player_id"`
	Kind      string    `json:"kind"`
	Resource  string    `json:"resource"`
	Amount    float64   `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

type WarehouseStore interface {
	Warehouse(ctx context.Context, pid string, limit int) (Corporation, []WarehouseItem, []WarehouseAudit, error)
	Deposit(ctx context.Context, pid, resource string, amount float64) (Corporation, error)
	Withdraw(ctx context.Context, pid, resource string, amount float64) (Corporation, error)
	ContributeResource(ctx context.Context, pid, resource string, amount float64) (Corporation, error)
	LogWarehouseAudit(ctx context.Context, corpID, pid, kind, resource string, amount float64) error
}

func validWarehouseAmount(resource string, amount float64) error {
	if !warehouseResources[resource] {
		return ErrBadWarehouseResource
	}
	if amount <= 0 || amount > 100000 {
		return ErrWarehouseAmount
	}
	return nil
}

func (s *Service) Warehouse(ctx context.Context, pid string, limit int) (Corporation, []WarehouseItem, []WarehouseAudit, error) {
	st, ok := s.store.(WarehouseStore)
	if !ok {
		return Corporation{}, nil, nil, ErrWarehouseUnavailable
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return st.Warehouse(ctx, pid, limit)
}

func (s *Service) Deposit(ctx context.Context, pid, resource string, amount float64) (Corporation, error) {
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	st, ok := s.store.(WarehouseStore)
	if !ok {
		return Corporation{}, ErrWarehouseUnavailable
	}
	return st.Deposit(ctx, pid, resource, amount)
}

func (s *Service) Withdraw(ctx context.Context, pid, resource string, amount float64) (Corporation, error) {
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	st, ok := s.store.(WarehouseStore)
	if !ok {
		return Corporation{}, ErrWarehouseUnavailable
	}
	return st.Withdraw(ctx, pid, resource, amount)
}

func (s *Service) ContributeTask(ctx context.Context, pid, resource string, amount float64) (Task, error) {
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Task{}, err
	}
	st, ok := s.store.(WarehouseStore)
	if !ok {
		return Task{}, ErrWarehouseUnavailable
	}
	if _, err := st.ContributeResource(ctx, pid, resource, amount); err != nil {
		return Task{}, err
	}
	task, err := s.NoteWork(ctx, pid, int(amount))
	if err != nil {
		_, _ = s.Deposit(ctx, pid, resource, amount)
		return Task{}, err
	}
	c, _, _ := s.store.CorporationByPlayer(ctx, pid)
	if st, ok := s.store.(WarehouseStore); ok {
		_ = st.LogWarehouseAudit(ctx, c.ID, pid, "task_contribution", resource, amount)
	}
	return task, nil
}

func warehouseAuditID(corpID, pid, kind string, now time.Time) string {
	return fmt.Sprintf("warehouse:%s:%s:%s:%d", corpID, pid, kind, now.UnixNano())
}
