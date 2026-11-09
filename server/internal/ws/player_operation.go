package ws

import (
	"context"
	"sync"
)

type playerOperation struct {
	token chan struct{}
	refs  int // includes holders and waiters, guarded by Handler.operationsMu
}

// lockPlayer serializes only one authenticated public ID. No global mutex is
// held while waiting, doing IO, or calling World. Idle entries are removed so
// disconnected accounts cannot accumulate permanent locks in the handler.
func (h *Handler) lockPlayer(ctx context.Context, id string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.operationsMu.Lock()
	if h.operations == nil {
		h.operations = map[string]*playerOperation{}
	}
	op := h.operations[id]
	if op == nil {
		op = &playerOperation{token: make(chan struct{}, 1)}
		op.token <- struct{}{}
		h.operations[id] = op
	}
	op.refs++
	h.operationsMu.Unlock()
	drop := func() {
		h.operationsMu.Lock()
		op.refs--
		if op.refs == 0 {
			delete(h.operations, id)
		}
		h.operationsMu.Unlock()
	}
	select {
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	case <-op.token:
	}
	var once sync.Once
	unlock := func() { once.Do(func() { op.token <- struct{}{}; drop() }) }
	if err := ctx.Err(); err != nil {
		unlock()
		return nil, err
	}
	return unlock, nil
}
