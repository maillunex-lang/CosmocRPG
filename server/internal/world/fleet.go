package world

import "sort"

// ShipSummary — краткие данные корабля каталога для верфи.
type ShipSummary struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Class     string   `json:"class"`
	Role      string   `json:"role"`
	Temporary bool     `json:"temporary"`
	Hull      float64  `json:"hull"`
	Shield    float64  `json:"shield"`
	Armor     float64  `json:"armor"`
	Energy    float64  `json:"energy"`
	Cargo     float64  `json:"cargo"`
	Speed     float64  `json:"speed"`
	Specs     []string `json:"specs"`
	Skins     []string `json:"skins"`
}

// Fleet возвращает корабли каталога, доступные для смены (на время альфы доступны все), упорядоченные по идентификатору.
func (w *World) Fleet() []ShipSummary {
	w.mu.Lock()
	defer w.mu.Unlock()
	ids := make([]string, 0, len(w.catalog.Ships))
	for id := range w.catalog.Ships {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ShipSummary, 0, len(ids))
	for _, id := range ids {
		s := w.catalog.Ships[id]
		out = append(out, ShipSummary{ID: s.ID, Name: s.Name, Class: s.Class, Role: s.Role, Temporary: s.Temporary, Hull: s.Hull, Shield: s.Shield,
			Armor: s.Armor, Energy: s.Energy, Cargo: s.Cargo, Speed: s.Speed, Specs: append([]string{}, s.Specs...), Skins: append([]string{}, s.Skins...)})
	}
	return out
}

// SetShip меняет корабль игрока. Только у станции (в безопасной зоне), живым игроком, и только если груз поместится в новый корабль.
// На время альфы смена бесплатна; покупка и производство кораблей появятся на этапе экономики.
func (w *World) SetShip(id, shipID string) (Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if !ok {
		return Record{}, ErrNoPlayer
	}
	if p.actionBusy() {
		return Record{}, ErrEquipmentBusy
	}
	if p.deadUntil > w.t {
		return Record{}, ErrDead
	}
	sp, ok := w.catalog.Spec(shipID)
	if !ok {
		return Record{}, ErrNoShip
	}
	if !w.inSafe(p.Pos) {
		return Record{}, ErrNoStable
	}
	if holdUsed(p.hold) > sp.Cargo {
		return Record{}, ErrHoldBig
	}
	hold := p.hold
	w.initShip(p, shipID, "", "")
	p.hold, p.mining, p.Attacking = hold, "", false
	w.emit(Event{Type: "ship_changed", From: p.ID})
	return p.record(), nil
}
