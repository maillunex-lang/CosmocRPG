package corp

import (
	"context"
	"testing"
)

func TestTreasuryDonationAndJournal(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	s := NewService(m, nil)
	if _, err := s.Create(ctx, "owner", "Treasury", "TRS"); err != nil {
		t.Fatal(err)
	}
	c, a, err := s.Donate(ctx, "owner", 1200)
	if err != nil || c.Treasury != 1200 || a.Amount != 1200 {
		t.Fatalf("donate: %+v %+v %v", c, a, err)
	}
	journal, err := s.Journal(ctx, "owner", 10)
	if err != nil || len(journal) != 1 || journal[0].Balance != 1200 {
		t.Fatalf("journal: %+v %v", journal, err)
	}
	if _, _, err := s.Donate(ctx, "owner", 0); err != ErrBadTreasuryAmount {
		t.Fatalf("expected amount validation, got %v", err)
	}
}
