package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/maillunex-lang/CosmocRPG/server/internal/contract"
)

func deliveryTemplates(from string, now int64) []contract.Delivery {
	targets := map[string][]struct {
		to, res, risk string
		reward        int64
	}{
		"aurora":   {{to: "meridian", res: "iron_ore", risk: "низкий", reward: 350}, {to: "redstorm", res: "scrap", risk: "высокий", reward: 700}},
		"meridian": {{to: "aurora", res: "crystals", risk: "средний", reward: 550}, {to: "redstorm", res: "gas", risk: "высокий", reward: 800}},
		"redstorm": {{to: "meridian", res: "gas", risk: "средний", reward: 650}, {to: "aurora", res: "iron_ore", risk: "высокий", reward: 900}},
	}
	out := make([]contract.Delivery, 0, 2)
	for i, t := range targets[from] {
		d := contract.Delivery{ID: fmt.Sprintf("delivery_%s_%s_%d", from, t.to, i+1), Resource: t.res, Amount: 2, FromSystem: from, ToSystem: t.to, RewardCredits: t.reward, PenaltyCredits: t.reward / 4, Risk: t.risk, ExpiresAt: now + 30*60}
		out = append(out, d)
	}
	return out
}

func (h *Handler) contractMsg(ctx context.Context, c *websocket.Conn, ss *session, m Message) {
	reply := func(msg Message) { _ = writeJSON(ctx, c, msg) }
	w := h.opts.World.For(ss.pid)
	if w == nil {
		reply(errMsg(m.ID, "not_joined", "сначала войдите в игру"))
		return
	}
	now := time.Now().Unix()
	h.contractMu.Lock()
	active, has := h.contracts[ss.pid]
	h.contractMu.Unlock()
	switch m.Type {
	case "contracts":
		available := deliveryTemplates(w.System().ID, now)
		reply(Message{Type: "contracts", ID: m.ID, Payload: mustJSON(map[string]any{"active": active, "has_active": has, "available": available, "now": now})})
	case "contract_accept":
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(m.Payload, &p) != nil || p.ID == "" {
			reply(errMsg(m.ID, "bad_message", "некорректный контракт"))
			return
		}
		if has {
			reply(errMsg(m.ID, "contract_active", "сначала завершите текущий контракт"))
			return
		}
		var selected *contract.Delivery
		for _, d := range deliveryTemplates(w.System().ID, now) {
			if d.ID == p.ID {
				x := d
				selected = &x
				break
			}
		}
		if selected == nil {
			reply(errMsg(m.ID, "contract_unavailable", "контракт недоступен в этой системе"))
			return
		}
		h.contractMu.Lock()
		h.contracts[ss.pid] = *selected
		h.contractMu.Unlock()
		reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "contract_accept", "contract": *selected})})
	case "contract_cancel":
		if !has {
			reply(errMsg(m.ID, "contract_missing", "активного контракта нет"))
			return
		}
		h.contractMu.Lock()
		delete(h.contracts, ss.pid)
		h.contractMu.Unlock()
		reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "contract_cancel"})})
	case "contract_deliver":
		if !has {
			reply(errMsg(m.ID, "contract_missing", "активного контракта нет"))
			return
		}
		if now > active.ExpiresAt {
			h.contractMu.Lock()
			delete(h.contracts, ss.pid)
			h.contractMu.Unlock()
			reply(errMsg(m.ID, "contract_expired", "срок контракта истёк"))
			return
		}
		if w.System().ID != active.ToSystem {
			reply(errMsg(m.ID, "wrong_system", "перейдите в систему назначения"))
			return
		}
		state, ok := h.opts.World.Snapshot(ss.pid)
		if !ok || state.Ship.Hold[active.Resource]+1e-6 < active.Amount {
			reply(errMsg(m.ID, "cargo_required", fmt.Sprintf("нужно доставить %.0f ед. ресурса", active.Amount)))
			return
		}
		rec, removed := w.TakeCargo(ss.pid, active.Resource, active.Amount)
		if !removed {
			reply(errMsg(m.ID, "cargo_required", "груз не удалось списать"))
			return
		}
		h.save(rec)
		total := int64(0)
		if h.opts.Quests != nil {
			sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			var err error
			_, total, err = h.opts.Quests.Grant(sctx, ss.pid, fmt.Sprintf("delivery:%s", active.ID), active.RewardCredits)
			cancel()
			if err != nil {
				reply(errMsg(m.ID, "internal", "награда временно недоступна"))
				return
			}
		}
		h.contractMu.Lock()
		delete(h.contracts, ss.pid)
		h.contractMu.Unlock()
		reply(Message{Type: "ok", ID: m.ID, Payload: mustJSON(map[string]any{"action": "contract_deliver", "contract": active, "credits": total})})
	}
}
