package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func (h *Handler) fittingMsg(ctx context.Context, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, ss.conn, msg) }
	if h.opts.Fitting == nil {
		reply(errMsg(m.ID, "fitting_unavailable", "инвентарь модулей недоступен"))
		return
	}
	st, ok := h.opts.World.Snapshot(ss.pid)
	if !ok {
		reply(errMsg(m.ID, "not_joined", "вы не в мире"))
		return
	}
	if m.Type == "fitting_get" {
		view, err := h.loadFitting(ctx, ss.pid, st.Ship.Type, st.Ship.Spec)
		if err != nil {
			reply(errMsg(m.ID, "fitting_unavailable", "не удалось прочитать модули"))
			return
		}
		h.decorateFitting(&view, ss.pid)
		reply(Message{Type: "fitting", ID: m.ID, Payload: mustJSON(view)})
		return
	}
	var p struct {
		Op            string `json:"op"`
		Slot          string `json:"slot"`
		Item          string `json:"item"`
		Revision      *int64 `json:"revision"`
		PilotRevision *int64 `json:"pilot_revision"`
	}
	dec := json.NewDecoder(bytes.NewReader(m.Payload))
	dec.DisallowUnknownFields()
	if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF || p.Revision == nil || p.PilotRevision == nil || *p.Revision < 0 || *p.PilotRevision < 0 {
		reply(errMsg(m.ID, "bad_message", "укажите слот, модуль и ревизию"))
		return
	}
	if p.Op != "equip" && p.Op != "unequip" {
		reply(errMsg(m.ID, "bad_message", "можно только вставить или снять модуль"))
		return
	}
	if st.Ship.Type == fitting.ShipStrizh && p.Slot == "tech-1" {
		reply(errMsg(m.ID, "engine_slot_locked", "штатный двигатель снимается своей кнопкой"))
		return
	}
	shipWorld := h.opts.World.For(ss.pid)
	if shipWorld == nil {
		reply(errMsg(m.ID, "not_joined", "вы не в мире"))
		return
	}
	if err := shipWorld.CanRefit(ss.pid); err != nil {
		code := err.Error()
		if errors.Is(err, world.ErrNoPlayer) {
			code = "not_joined"
		}
		reply(errMsg(m.ID, code, equipmentText(code)))
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	next, err := h.opts.Fitting.Execute(writeCtx, ss.pid, st.Ship.Type, *p.Revision, *p.PilotRevision, equipment.Command{Op: p.Op, Slot: p.Slot, Item: p.Item}, nil)
	cancel()
	if err != nil {
		reply(errMsg(m.ID, fittingCode(err), fittingText(err)))
		return
	}
	h.pushCategories(ss.pid, next)
	view := equipment.PublicView(h.opts.Fitting.Catalog(), next)
	h.decorateFitting(&view, ss.pid)
	reply(Message{Type: "fitting", ID: m.ID, Payload: mustJSON(view)})
}

func (h *Handler) loadFitting(ctx context.Context, owner, hull, spec string) (equipment.View, error) {
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	state, err := h.opts.Fitting.Ensure(readCtx, owner, hull, spec)
	if err != nil {
		return equipment.View{}, err
	}
	h.pushCategories(owner, state)
	return equipment.PublicView(h.opts.Fitting.Catalog(), state), nil
}

func (h *Handler) decorateFitting(view *equipment.View, id string) {
	shipWorld := h.opts.World.For(id)
	if shipWorld == nil {
		view.Reason = "not_joined"
		return
	}
	if err := shipWorld.CanRefit(id); err != nil {
		view.Reason = err.Error()
		return
	}
	view.CanChange = true
}

func (h *Handler) pushCategories(id string, state fitting.State) {
	shipWorld := h.opts.World.For(id)
	if shipWorld == nil || h.opts.Fitting == nil {
		return
	}
	shipWorld.SetModuleCategories(id, equipment.ModuleCategories(h.opts.Fitting.Catalog(), state))
}

func (h *Handler) syncFitting(ctx context.Context, id, hull, spec string) {
	if h.opts.Fitting == nil || hull == "" {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	state, err := h.opts.Fitting.Ensure(readCtx, id, hull, spec)
	if err != nil {
		return
	}
	h.pushCategories(id, state)
}

func fittingCode(err error) string {
	var v *fitting.Violation
	if errors.As(err, &v) {
		return v.Code
	}
	if errors.Is(err, fitting.ErrConflict) {
		return "equipment_revision_conflict"
	}
	return "fitting_unavailable"
}

func fittingText(err error) string {
	switch fittingCode(err) {
	case "unknown_slot":
		return "такого слота у корабля нет"
	case "module_category", "module_size", "module_class":
		return "этот модуль сюда не встаёт"
	case "not_owned":
		return "этого модуля нет в инвентаре"
	case "equipment_revision_conflict":
		return "оснащение уже изменилось; откройте ангар ещё раз"
	default:
		return "не удалось изменить оснащение"
	}
}
