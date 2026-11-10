package ws

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
)

func (h *Handler) talkMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if h.opts.Quests == nil {
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	var p struct {
		Choice string `json:"choice"`
	}
	if json.Unmarshal(m.Payload, &p) != nil || len(p.Choice) > 32 {
		reply(errMsg(m.ID, "bad_message", "некорректный ответ"))
		return
	}
	pw := h.opts.World.For(ss.pid)
	if pw == nil || !pw.AtKind(ss.pid, "station") {
		reply(errMsg(m.ID, "not_at_station", "говорить можно только у станции"))
		return
	}
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	view, done, err := h.opts.Quests.Talk(tctx, ss.pid, pw.System().ID, p.Choice)
	if err != nil {
		switch {
		case errors.Is(err, quest.ErrBadChoice):
			reply(errMsg(m.ID, "bad_choice", "такого ответа нет"))
		case errors.Is(err, quest.ErrNoDialogue):
			reply(errMsg(m.ID, "no_dialogue", "сейчас не с кем говорить"))
		default:
			h.log.Error("диалог", "player", ss.pid, "err", err)
			reply(errMsg(m.ID, "internal", "разговор прервался"))
		}
		return
	}
	for _, c := range done {
		h.finishQuest(ss.pid, c)
	}
	reply(Message{Type: "dialogue", ID: m.ID, Payload: mustJSON(view)})
}
