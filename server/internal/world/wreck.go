package world

import (
	"math"
	"sort"
)

// wreck — мелкий контейнер обломков. Состав решает сервер, подобрать можно один раз.
type wreck struct {
	ID       string
	Pos      Vec
	Resource string
	Amount   float64
	Until    float64
}

func (w *World) spawnWreck(n *npc) {
	if n == nil {
		return
	}
	w.wrecks = append(w.wrecks, &wreck{
		ID: "wreck-" + n.ID + "-" + itoa(n.kills), Pos: n.Pos, Resource: "scrap", Amount: 1.5, Until: w.t + 45,
	})
}

func (w *World) wreckObjects() []Object {
	out := make([]Object, 0, len(w.wrecks))
	for _, wr := range w.wrecks {
		if wr.Amount < 0.05 || wr.Until < w.t {
			continue
		}
		a := math.Round(wr.Amount*10) / 10
		out = append(out, Object{ID: wr.ID, Kind: "wreck", Stance: "neutral", Name: "Обломки", Pos: [2]float64{wr.Pos.X, wr.Pos.Y}, Resource: wr.Resource, Amount: &a})
	}
	return out
}

func (w *World) collectWrecks() {
	ids := make([]string, 0, len(w.players))
	for id := range w.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	left := make([]*wreck, 0, len(w.wrecks))
	for _, wr := range w.wrecks {
		if wr.Until < w.t || wr.Amount < 0.05 {
			continue
		}
		for _, id := range ids {
			p := w.players[id]
			if p.deadUntil > w.t || dist(p.Pos, wr.Pos) > 70 {
				continue
			}
			room := cargoRoom(p, wr.Resource)
			if room < 0.05 {
				continue
			}
			amt := math.Min(wr.Amount, room)
			if p.hold == nil {
				p.hold = map[string]float64{}
			}
			p.hold[wr.Resource] += amt
			p.cargoRevision++
			wr.Amount -= amt
			w.emit(Event{Type: "loot_pickup", From: wr.ID, To: p.ID, Amount: amt})
			if wr.Amount < 0.05 {
				break
			}
		}
		if wr.Amount >= 0.05 && wr.Until >= w.t {
			left = append(left, wr)
		}
	}
	w.wrecks = left
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
