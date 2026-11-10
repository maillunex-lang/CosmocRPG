package ws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func (h *Handler) purchaseMsg(ctx context.Context, ss *session, m Message) {
	var p struct {
		Resource  string  `json:"resource"`
		Amount    float64 `json:"amount"`
		Operation string  `json:"operation_id"`
	}
	reply := func(msg Message) { _ = writeJSON(ctx, ss.conn, msg) }
	fail := func(err error) {
		code, text := "purchase_unavailable", "Не удалось подтвердить покупку. Проверьте её состояние."
		switch {
		case errors.Is(err, economy.ErrInvalid):
			code, text = "bad_message", "Некорректная покупка."
		case errors.Is(err, economy.ErrNoCredits):
			code, text = "no_credits", "Не хватает кредитов."
		case errors.Is(err, economy.ErrConflict):
			code, text = "purchase_conflict", "Запрос или груз изменился. Обновите состояние."
		case errors.Is(err, world.ErrHoldFull):
			code, text = "hold_full", "В трюме недостаточно места."
		case errors.Is(err, world.ErrNoStable):
			code, text = "not_at_station", "Для покупки подлетите к станции."
		case errors.Is(err, world.ErrNoResource):
			code, text = "no_item", "Этот товар не продаётся."
		case errors.Is(err, world.ErrDead):
			code, text = "dead", "Корабль уничтожен."
		case errors.Is(err, world.ErrEquipmentBusy):
			code, text = "equipment_busy", "Дождитесь завершения текущей операции."
		}
		reply(Message{Type: "error", ID: m.ID, Payload: mustJSON(map[string]any{"code": code, "message": text, "operation_id": p.Operation})})
	}
	if json.Unmarshal(m.Payload, &p) != nil {
		fail(economy.ErrInvalid)
		return
	}
	if _, err := economy.Intent(p.Resource, p.Amount); err != nil {
		fail(err)
		return
	}
	if p.Operation == "" {
		if m.Type == "purchase_status" {
			fail(economy.ErrInvalid)
			return
		}
		if m.ID != "" {
			hash := sha256.Sum256([]byte(ss.purchaseSession + ":" + m.ID))
			p.Operation = "legacy:" + hex.EncodeToString(hash[:])
		} else {
			p.Operation = "legacy:" + newSessionID()
		}
	}
	if !economy.ValidKey(p.Operation) {
		fail(economy.ErrInvalid)
		return
	}
	if h.opts.Quests == nil {
		fail(economy.ErrUnavailable)
		return
	}
	w := h.opts.World.For(ss.pid)
	if w == nil {
		fail(world.ErrNoPlayer)
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	previous, err := h.opts.Quests.LookupPurchase(readCtx, ss.pid, p.Operation, p.Resource, p.Amount)
	cancel()
	if err != nil {
		fail(err)
		return
	}
	if previous.Known {
		if err = w.RestorePurchaseCargo(ss.pid, previous.Cargo); err != nil {
			fail(err)
			return
		}
	}
	success := func(result economy.Result) {
		reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "buy", "operation_id": p.Operation, "credits": result.Credits, "spent": result.Spent})})
	}
	if previous.Found {
		success(previous)
		return
	}
	if m.Type == "purchase_status" {
		reply(Message{Type: "purchase_status", ID: m.ID, Payload: mustJSON(map[string]any{"found": false, "operation_id": p.Operation})})
		return
	}
	result, err := w.PurchaseResource(ss.pid, p.Operation, p.Resource, p.Amount, func(request economy.Request) (economy.Result, error) {
		writeCtx, writeCancel := context.WithTimeout(ctx, 2*time.Second)
		result, writeErr := h.opts.Quests.Purchase(writeCtx, request)
		writeCancel()
		if result.Known {
			return result, writeErr
		}
		// Свежая независимая deadline: отмена клиентского запроса не доказывает ROLLBACK.
		reconcile, reconcileCancel := context.WithTimeout(context.Background(), time.Second)
		defer reconcileCancel()
		state, readErr := h.opts.Quests.LookupPurchase(reconcile, ss.pid, p.Operation, p.Resource, p.Amount)
		if readErr != nil || !state.Known {
			return economy.Result{}, economy.ErrUnavailable
		}
		if state.Found {
			return state, nil
		}
		return state, writeErr
	})
	if err != nil {
		fail(err)
		return
	}
	success(result)
}
