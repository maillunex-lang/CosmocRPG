package ws

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/coder/websocket"

	"github.com/maillunex-lang/CosmocRPG/server/internal/social"
)

func (h *Handler) friendMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	if h.opts.Social == nil {
		reply(errMsg(m.ID, "unknown_type", "неизвестный тип сообщения"))
		return
	}
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(m.Payload, &body)
	q := body.ID
	if q == "" {
		q = body.Name
	}
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var list []social.Friend
	var err error
	switch m.Type {
	case "friend_add":
		if q == "" || len(q) > 64 {
			reply(errMsg(m.ID, "bad_message", "укажите пилота"))
			return
		}
		list, err = h.opts.Social.Add(sctx, ss.pid, q)
	case "friend_remove":
		if body.ID == "" || len(body.ID) > 64 {
			reply(errMsg(m.ID, "bad_message", "некорректная команда"))
			return
		}
		list, err = h.opts.Social.Remove(sctx, ss.pid, body.ID)
	default:
		list, err = h.opts.Social.List(sctx, ss.pid)
	}
	if err != nil {
		code := "internal"
		text := "список друзей недоступен"
		switch {
		case errors.Is(err, social.ErrSelf):
			code, text = "friend_self", "нельзя добавить себя"
		case errors.Is(err, social.ErrNone):
			code, text = "friend_none", "такого пилота нет"
		case errors.Is(err, social.ErrFull):
			code, text = "friend_full", "в списке уже 50 друзей"
		default:
			h.log.Error("друзья", "player", ss.pid, "err", err)
		}
		reply(errMsg(m.ID, code, text))
		return
	}
	if list == nil {
		list = []social.Friend{}
	}
	reply(Message{Type: "friends", ID: m.ID, Payload: mustJSON(map[string]any{"friends": list})})
}
