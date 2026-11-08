package world

// Cosmetics — внешний вид корабля, ника и профиля: значок, цвета, аватарка, рамка, титул, VIP-уровень.
// Боевых параметров не меняет.
type Cosmetics struct {
	Vip       int    `json:"vip,omitempty"`
	Badge     string `json:"badge,omitempty"`
	NameColor string `json:"name_color,omitempty"`
	Engine    string `json:"engine,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	Frame     string `json:"frame,omitempty"`
	Title     string `json:"title,omitempty"`
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
