package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func (h *Handler) loadStarter(ctx context.Context, rec *world.Record) error {
	if h.opts.Equipment == nil {
		return nil
	}
	rec.PersistentEquipment = true
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Unknown legacy hull IDs fall back to Strizh in world.initShip too.
	_, known := h.opts.World.Catalog().Spec(rec.ShipType)
	var s equipment.StarterState
	var err error
	if rec.ShipType == world.DefaultShip || !known {
		s, err = h.opts.Equipment.Ensure(ctx, rec.ID)
	} else {
		s, err = h.opts.Equipment.Load(ctx, rec.ID)
		if errors.Is(err, equipment.ErrNotInitialized) {
			return nil
		}
	}
	if err != nil {
		return err
	}
	if !s.Valid(rec.ID) {
		return equipment.ErrInvalidState
	}
	rec.StarterLoadout = &s
	return nil
}

func (h *Handler) equipmentMsg(ctx context.Context, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, ss.conn, msg) }
	if h.opts.Equipment == nil {
		reply(errMsg(m.ID, "equipment_unavailable", "оснащение недоступно"))
		return
	}
	// Serialize session replacement with the equipment command; a replaced
	// connection must not mutate equipment belonging to its successor.
	h.mu.Lock()
	if h.active[ss.pid] != ss {
		h.mu.Unlock()
		reply(errMsg(m.ID, "replaced", "сессия заменена"))
		return
	}
	w := h.opts.World.For(ss.pid)
	if w == nil {
		h.mu.Unlock()
		reply(errMsg(m.ID, "not_joined", "сначала войдите в игру"))
		return
	}
	var err error
	if m.Type == "equipment_get" {
		readCtx, cancel := context.WithTimeout(ctx, time.Second)
		state, readErr := h.opts.Equipment.Load(readCtx, ss.pid)
		cancel()
		if readErr != nil || !w.RestoreStarter(ss.pid, state) {
			err = world.ErrEquipmentUnavailable
		}
	} else {
		var p struct {
			Revision *int64 `json:"revision"`
			Equipped *bool  `json:"equipped"`
		}
		dec := json.NewDecoder(bytes.NewReader(m.Payload))
		dec.DisallowUnknownFields()
		if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF || p.Revision == nil || p.Equipped == nil || *p.Revision < 0 {
			h.mu.Unlock()
			reply(errMsg(m.ID, "bad_message", "укажите ревизию и состояние двигателя"))
			return
		}
		err = w.ChangeStarter(ss.pid, *p.Revision, *p.Equipped, func() (world.StarterLoadout, error) {
			writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			state, writeErr := h.opts.Equipment.SetEngine(writeCtx, ss.pid, *p.Revision, *p.Equipped)
			cancel()
			if writeErr == nil {
				return state, nil
			}
			// A failed reply after COMMIT does not prove rollback. Reconcile with
			// a fresh deadline even if the requesting connection was cancelled.
			readCtx, readCancel := context.WithTimeout(context.Background(), time.Second)
			state, readErr := h.opts.Equipment.Load(readCtx, ss.pid)
			readCancel()
			if readErr != nil {
				return world.StarterLoadout{}, world.ErrEquipmentUnavailable
			}
			if errors.Is(writeErr, equipment.ErrConflict) {
				return state, world.ErrEquipmentConflict
			}
			return state, world.ErrEquipmentUnavailable
		})
	}
	h.mu.Unlock()
	if err != nil {
		code := err.Error()
		if errors.Is(err, world.ErrNoPlayer) {
			code = "not_joined"
		}
		reply(errMsg(m.ID, code, equipmentText(code)))
	} else {
		reply(Message{Type: "equipment", ID: m.ID})
	}
	if st, ok := h.opts.World.Snapshot(ss.pid); ok {
		h.pushState(ss, st)
	}
}

func equipmentText(code string) string {
	switch code {
	case "not_at_station":
		return "для смены оснащения подлетите к станции"
	case "equipment_in_combat":
		return "смена оснащения недоступна в бою и 10 секунд после него"
	case "equipment_busy":
		return "остановитесь, отключите автопилот и дождитесь окончания форсажа"
	case "equipment_revision_conflict":
		return "оснащение уже изменилось; проверьте актуальное состояние"
	case "equipment_unsupported_ship":
		return "смена оснащения пока доступна только Стрижу"
	case "dead":
		return "корабль уничтожен"
	default:
		return "не удалось подтвердить оснащение; обновите состояние или войдите заново"
	}
}
