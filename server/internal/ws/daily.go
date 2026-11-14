package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/maillunex-lang/CosmocRPG/server/internal/daily"
)

func (h *Handler) dailyMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	now := time.Now().UTC()
	day := daily.Day(now)

	h.dailyMu.Lock()
	st := h.daily[ss.pid]
	if st == nil {
		st = &dailyState{day: day, streak: 1, claimed: map[string]bool{}}
		h.daily[ss.pid] = st
	} else if st.day != day {
		if st.day != "" {
			previous := parseDailyDay(st.day)
			if !previous.IsZero() && now.Sub(previous) <= 48*time.Hour {
				st.streak++
			} else {
				st.streak = 1
			}
		}
		st.day = day
		st.claimed = map[string]bool{}
		st.returnClaimed = false
	}
	streak := st.streak
	claimed := make(map[string]bool, len(st.claimed))
	for k, v := range st.claimed { claimed[k] = v }
	returnClaimed := st.returnClaimed
	h.dailyMu.Unlock()

	var p struct { Action, ID string }
	if len(m.Payload) > 0 && json.Unmarshal(m.Payload, &p) != nil {
		reply(errMsg(m.ID, "bad_message", "некорректная ежедневная команда"))
		return
	}
	if p.Action == "claim" {
		if h.opts.Quests == nil {
			reply(errMsg(m.ID, "daily_unavailable", "ежедневные награды временно недоступны"))
			return
		}
		reward := int64(0)
		key := ""
		if p.ID == "return" {
			if returnClaimed {
				reply(errMsg(m.ID, "daily_claimed", "награда за возвращение уже получена"))
				return
			}
			reward = daily.ReturnReward(streak)
			key = "daily:return:" + day
		} else {
			found := false
			for _, a := range daily.Activities(now) {
				if a.ID == p.ID {
					reward, found = a.Reward, true
					break
				}
			}
			if !found {
				reply(errMsg(m.ID, "daily_unknown", "такой активности сегодня нет"))
				return
			}
			if claimed[p.ID] {
				reply(errMsg(m.ID, "daily_claimed", "активность уже отмечена"))
				return
			}
			key = "daily:" + day + ":" + p.ID
		}
		grantCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		granted, total, err := h.opts.Quests.Grant(grantCtx, ss.pid, key, reward)
		cancel()
		if err != nil {
			reply(errMsg(m.ID, "internal", "награда временно недоступна"))
			return
		}
		if granted {
			h.dailyMu.Lock()
			if p.ID == "return" { st.returnClaimed = true } else { st.claimed[p.ID] = true }
			h.dailyMu.Unlock()
		}
		reply(Message{Type: "daily_reward", ID: m.ID, Payload: mustJSON(map[string]any{"id": p.ID, "granted": granted, "reward": reward, "credits": total})})
		return
	}
	reply(Message{Type: "daily", ID: m.ID, Payload: mustJSON(map[string]any{
		"day": day, "activities": daily.Activities(now), "streak": streak,
		"can_claim_return": !returnClaimed, "return_reward": daily.ReturnReward(streak),
		"claimed": claimed,
	})})
}

func parseDailyDay(day string) time.Time {
	t, err := time.Parse("2006-01-02", day)
	if err != nil { return time.Time{} }
	return t
}
