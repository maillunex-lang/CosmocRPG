package ws

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitPlayerWaiter(t *testing.T, h *Handler, id string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h.operationsMu.Lock()
		op := h.operations[id]
		waiting := op != nil && op.refs >= 2
		h.operationsMu.Unlock()
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("player operation waiter did not register")
}

func TestPlayerOperationCancellationIsolationAndCleanup(t *testing.T) {
	h := NewHandler(nil, Options{})
	first, err := h.lockPlayer(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		unlock, err := h.lockPlayer(ctx, "owner")
		if unlock != nil {
			unlock()
		}
		done <- err
	}()
	waitPlayerWaiter(t, h, "owner")
	other, err := h.lockPlayer(context.Background(), "other")
	if err != nil {
		t.Fatal(err)
	}
	other()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	first()
	first() // idempotent release; no duplicate token
	h.operationsMu.Lock()
	remaining := len(h.operations)
	h.operationsMu.Unlock()
	if remaining != 0 {
		t.Fatalf("idle locks leaked: %d", remaining)
	}
	retry, err := h.lockPlayer(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	retry()
}
