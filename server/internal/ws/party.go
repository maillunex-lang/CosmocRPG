package ws

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func (h *Handler) partyMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	var body struct {
		ID string `json:"id"`
	}
	if m.Type == "party_invite" && (json.Unmarshal(m.Payload, &body) != nil || body.ID == "" || len(body.ID) > 64) {
		reply(errMsg(m.ID, "bad_message", "некорректное приглашение"))
		return
	}
	var err error
	switch m.Type {
	case "party_invite":
		err = h.opts.World.PartyInvite(ss.pid, body.ID)
	case "party_join":
		err = h.opts.World.PartyJoin(ss.pid)
	case "party_leave":
		err = h.opts.World.PartyLeave(ss.pid)
	case "party_start":
		err = h.opts.World.PartyStart(ss.pid)
	default:
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	if err != nil {
		reply(errMsg(m.ID, partyCode(err), partyText(err)))
		return
	}
	if st, ok := h.opts.World.Snapshot(ss.pid); ok {
		h.pushState(ss, st)
	}
}

func partyCode(err error) string {
	switch {
	case errors.Is(err, world.ErrPartyFull):
		return "party_full"
	case errors.Is(err, world.ErrPartyFar):
		return "party_too_far"
	case errors.Is(err, world.ErrPartyNeed):
		return "party_need_mate"
	case errors.Is(err, world.ErrPartyLeader):
		return "party_not_leader"
	case errors.Is(err, world.ErrPartyNone):
		return "party_none"
	case errors.Is(err, world.ErrPartyBusy):
		return "party_busy"
	case errors.Is(err, world.ErrPartyNobody):
		return "party_no_one"
	case errors.Is(err, world.ErrPartyAlready):
		return "party_already"
	case errors.Is(err, world.ErrDead):
		return "dead"
	case errors.Is(err, world.ErrEquipmentBusy):
		return "equipment_busy"
	default:
		return "party_busy"
	}
}

func partyText(err error) string {
	switch {
	case errors.Is(err, world.ErrPartyFull):
		return "в пати уже четыре пилота"
	case errors.Is(err, world.ErrPartyFar):
		return "слишком далеко"
	case errors.Is(err, world.ErrPartyNeed):
		return "нужен ещё хотя бы один пилот"
	case errors.Is(err, world.ErrPartyLeader):
		return "это может сделать только ведущий"
	case errors.Is(err, world.ErrPartyNone):
		return "приглашения нет"
	case errors.Is(err, world.ErrPartyBusy):
		return "пати уже в бою"
	case errors.Is(err, world.ErrPartyNobody):
		return "такого пилота рядом нет"
	case errors.Is(err, world.ErrPartyAlready):
		return "уже в пати"
	case errors.Is(err, world.ErrDead):
		return "корабль уничтожен"
	case errors.Is(err, world.ErrEquipmentBusy):
		return "дождитесь завершения смены оснащения"
	default:
		return "сейчас нельзя войти в стычку"
	}
}
