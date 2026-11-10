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
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	if m.Type == "party_invite" && (json.Unmarshal(m.Payload, &body) != nil || body.ID == "" || len(body.ID) > 64) {
		reply(errMsg(m.ID, "bad_message", "некорректное приглашение"))
		return
	}
	if m.Type == "party_arm" && (json.Unmarshal(m.Payload, &body) != nil || (body.Kind != "credits" && body.Kind != "gear" && body.Kind != "loot")) {
		reply(errMsg(m.ID, "bad_message", "неизвестный коридор"))
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
	case "party_arm":
		err = h.opts.World.PartyArm(ss.pid, body.Kind)
	case "party_ready":
		err = h.opts.World.PartyReady(ss.pid)
	default:
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	if err != nil {
		reply(errMsg(m.ID, partyCode(err), partyText(err)))
		return
	}
	if m.Type == "party_start" || m.Type == "party_leave" || m.Type == "party_ready" {
		h.noteSystem(ss, ss.pid)
	}
	if st, ok := h.opts.World.Snapshot(ss.pid); ok {
		h.pushState(ss, st)
	}
}

// noteSystem сообщает клиенту о смене системы (вход в стычку и возврат), тем же сообщением, что и прыжок.
func (h *Handler) noteSystem(ss *session, pid string) {
	if h.opts.World == nil || ss == nil || pid == "" {
		return
	}
	hw := h.opts.World.For(pid)
	if hw == nil {
		return
	}
	sys := hw.System()
	h.mu.Lock()
	same := sys.ID == "" || sys.ID == ss.seenSystem
	if !same {
		ss.seenSystem = sys.ID
	}
	h.mu.Unlock()
	if same {
		return
	}
	payload := map[string]any{"system": sys, "spawn": map[string]float64{}}
	if lay, wcfg := hw.Layout(); lay.System != "" {
		payload["layout"] = lay
		payload["visual"] = wcfg.Visual
	}
	payload["systems"] = h.opts.World.Systems()
	payload["gates"] = h.opts.World.Gates(sys.ID)
	payload["links"] = h.opts.World.Links()
	msg := Message{Type: "system_changed", Payload: mustJSON(payload)}
	select {
	case ss.events <- msg:
	default:
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
	case errors.Is(err, world.ErrPartyPick):
		return "party_pick"
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
	case errors.Is(err, world.ErrPartyPick):
		return "сначала выберите коридор"
	case errors.Is(err, world.ErrDead):
		return "корабль уничтожен"
	case errors.Is(err, world.ErrEquipmentBusy):
		return "дождитесь завершения смены оснащения"
	default:
		return "сейчас нельзя войти в стычку"
	}
}
