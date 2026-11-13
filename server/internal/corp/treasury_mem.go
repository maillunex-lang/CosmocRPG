package corp

import (
	"context"
	"sort"
	"time"
)

func (m *MemStore) Donate(_ context.Context, pid string, amount int64) (Corporation, AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if amount <= 0 || amount > 1_000_000_000 {
		return Corporation{}, AuditEntry{}, ErrBadTreasuryAmount
	}
	id, ok := m.player[pid]
	if !ok {
		return Corporation{}, AuditEntry{}, ErrNotMember
	}
	c := m.byID[id]
	if c == nil {
		return Corporation{}, AuditEntry{}, ErrNoCorp
	}
	now := time.Now().UTC()
	c.Treasury += amount
	a := AuditEntry{ID: auditID(c.ID, pid, now), CorpID: c.ID, PlayerID: pid, Kind: "donation", Amount: amount, Balance: c.Treasury, CreatedAt: now}
	m.audit[c.ID] = append(m.audit[c.ID], a)
	return c.Corporation, a, nil
}

func (m *MemStore) Journal(_ context.Context, corpID string, limit int) ([]AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byID[corpID] == nil {
		return nil, ErrNoCorp
	}
	entries := append([]AuditEntry(nil), m.audit[corpID]...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}
