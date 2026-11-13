package corp

import (
	"context"
	"testing"
)

func TestWarehouseRolesAndTask(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	s := NewService(m, nil)
	if _, err := s.Create(ctx, "owner", "Warehouse", "WH"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Join(ctx, "member", "WH", "Pilot"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deposit(ctx, "member", "iron_ore", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Withdraw(ctx, "member", "iron_ore", 1); err != ErrWarehouseAccess {
		t.Fatalf("member withdrawal: %v", err)
	}
	if _, err := s.ContributeTask(ctx, "member", "iron_ore", 2); err != nil {
		t.Fatal(err)
	}
	_, items, logs, err := s.Warehouse(ctx, "member", 10)
	if err != nil || len(items) != 1 || items[0].Amount != 3 || len(logs) < 2 {
		t.Fatalf("warehouse: %+v %+v %v", items, logs, err)
	}
}
