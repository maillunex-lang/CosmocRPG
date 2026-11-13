package corp

import (
	"context"
	"sort"
	"time"
)

func (m *MemStore) warehouseCorp(pid string) (*memCorp, error) {
	id, ok := m.player[pid]
	if !ok {
		return nil, ErrNotMember
	}
	c := m.byID[id]
	if c == nil {
		return nil, ErrNoCorp
	}
	return c, nil
}

func (m *MemStore) Warehouse(_ context.Context, pid string, limit int) (Corporation, []WarehouseItem, []WarehouseAudit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.warehouseCorp(pid)
	if err != nil {
		return Corporation{}, nil, nil, err
	}
	items := []WarehouseItem{}
	for r, a := range m.warehouse[c.ID] {
		if a > 0 {
			items = append(items, WarehouseItem{Resource: r, Amount: a})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Resource < items[j].Resource })
	logs := append([]WarehouseAudit(nil), m.warehouseLog[c.ID]...)
	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })
	if len(logs) > limit {
		logs = logs[:limit]
	}
	return c.Corporation, items, logs, nil
}

func (m *MemStore) Deposit(_ context.Context, pid, resource string, amount float64) (Corporation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	c, err := m.warehouseCorp(pid)
	if err != nil {
		return Corporation{}, err
	}
	if m.warehouse[c.ID] == nil {
		m.warehouse[c.ID] = map[string]float64{}
	}
	m.warehouse[c.ID][resource] += amount
	now := time.Now().UTC()
	m.warehouseLog[c.ID] = append(m.warehouseLog[c.ID], WarehouseAudit{ID: warehouseAuditID(c.ID, pid, "deposit", now), CorpID: c.ID, PlayerID: pid, Kind: "deposit", Resource: resource, Amount: amount, CreatedAt: now})
	return c.Corporation, nil
}

func (m *MemStore) Withdraw(_ context.Context, pid, resource string, amount float64) (Corporation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	c, err := m.warehouseCorp(pid)
	if err != nil {
		return Corporation{}, err
	}
	if c.members[pid].Role != "owner" {
		return Corporation{}, ErrWarehouseAccess
	}
	if m.warehouse[c.ID][resource]+1e-6 < amount {
		return Corporation{}, ErrWarehouseEmpty
	}
	m.warehouse[c.ID][resource] -= amount
	now := time.Now().UTC()
	m.warehouseLog[c.ID] = append(m.warehouseLog[c.ID], WarehouseAudit{ID: warehouseAuditID(c.ID, pid, "withdraw", now), CorpID: c.ID, PlayerID: pid, Kind: "withdraw", Resource: resource, Amount: amount, CreatedAt: now})
	return c.Corporation, nil
}

func (m *MemStore) ContributeResource(_ context.Context, pid, resource string, amount float64) (Corporation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validWarehouseAmount(resource, amount); err != nil {
		return Corporation{}, err
	}
	c, err := m.warehouseCorp(pid)
	if err != nil {
		return Corporation{}, err
	}
	if m.warehouse[c.ID][resource]+1e-6 < amount {
		return Corporation{}, ErrWarehouseEmpty
	}
	m.warehouse[c.ID][resource] -= amount
	now := time.Now().UTC()
	m.warehouseLog[c.ID] = append(m.warehouseLog[c.ID], WarehouseAudit{ID: warehouseAuditID(c.ID, pid, "task_contribution", now), CorpID: c.ID, PlayerID: pid, Kind: "task_contribution", Resource: resource, Amount: amount, CreatedAt: now})
	return c.Corporation, nil
}

func (m *MemStore) LogWarehouseAudit(_ context.Context, corpID, pid, kind, resource string, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.warehouseLog[corpID] = append(m.warehouseLog[corpID], WarehouseAudit{ID: warehouseAuditID(corpID, pid, kind, now), CorpID: corpID, PlayerID: pid, Kind: kind, Resource: resource, Amount: amount, CreatedAt: now})
	return nil
}
