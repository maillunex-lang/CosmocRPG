package world

// Cosmetics — внешний вид корабля и ника: значок, цвета, VIP-уровень. Боевых параметров не меняет.
type Cosmetics struct {
	Vip       int    `json:"vip,omitempty"`
	Badge     string `json:"badge,omitempty"`
	NameColor string `json:"name_color,omitempty"`
	Engine    string `json:"engine,omitempty"`
}

// SetCosmetics задаёт внешний вид игрока.
func (w *World) SetCosmetics(id string, c Cosmetics) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.players[id]
	if ok {
		p.cosmetics = c
	}
	return ok
}
