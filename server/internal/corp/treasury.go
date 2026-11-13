package corp

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrBadTreasuryAmount   = errors.New("bad_treasury_amount")
	ErrTreasuryUnavailable = errors.New("treasury_unavailable")
	ErrNotEnoughCredits    = errors.New("no_credits")
)

// AuditEntry — неизменяемая запись операции казны.
type AuditEntry struct {
	ID        string    `json:"id"`
	CorpID    string    `json:"corp_id"`
	PlayerID  string    `json:"player_id"`
	Kind      string    `json:"kind"`
	Amount    int64     `json:"amount"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

type TreasuryStore interface {
	Donate(ctx context.Context, pid string, amount int64) (Corporation, AuditEntry, error)
	Journal(ctx context.Context, corpID string, limit int) ([]AuditEntry, error)
}

func (s *Service) Donate(ctx context.Context, pid string, amount int64) (Corporation, AuditEntry, error) {
	if amount <= 0 || amount > 1_000_000_000 {
		return Corporation{}, AuditEntry{}, ErrBadTreasuryAmount
	}
	st, ok := s.store.(TreasuryStore)
	if !ok {
		return Corporation{}, AuditEntry{}, ErrTreasuryUnavailable
	}
	return st.Donate(ctx, pid, amount)
}

func (s *Service) Journal(ctx context.Context, pid string, limit int) ([]AuditEntry, error) {
	st, ok := s.store.(TreasuryStore)
	if !ok {
		return nil, ErrTreasuryUnavailable
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	c, member, err := s.store.CorporationByPlayer(ctx, pid)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotMember
	}
	return st.Journal(ctx, c.ID, limit)
}

func auditID(corpID, pid string, now time.Time) string {
	return fmt.Sprintf("treasury:%s:%s:%d", corpID, pid, now.UnixNano())
}
