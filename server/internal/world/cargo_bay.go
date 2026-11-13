package world

import "math"

// StarterOreCapacity покрывает обязательную стартовую доставку без покупки другого корпуса.
const StarterOreCapacity = 5.0

// CargoBay — отдельный отсек ресурсов. Модули сюда не кладутся.
type CargoBay struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Used float64 `json:"used"`
	Cap  float64 `json:"cap"`
}

func resourceBay(res string) string {
	switch res {
	case "iron_ore", "crystals":
		return "ore"
	case "gas":
		return "gas"
	default:
		return "parts"
	}
}

func bayFraction(shipType, spec, bay string) float64 {
	ore, gas, parts := 0.45, 0.35, 0.6
	switch {
	case shipType == "TMP-MINE":
		ore, gas, parts = 1, 0.55, 0.4
	case shipType == "TMP-HAUL":
		ore, gas, parts = 1, 1, 1
	case len(shipType) >= 3 && shipType[:3] == "FTR":
		ore, gas, parts = 0.25, 0.2, 0.5
	}
	switch spec {
	case "miner":
		ore = math.Min(1, ore+0.15)
	case "hauler", "trader":
		ore, gas, parts = math.Min(1, ore+0.2), math.Min(1, gas+0.2), math.Min(1, parts+0.2)
	}
	switch bay {
	case "ore":
		return ore
	case "gas":
		return gas
	default:
		return parts
	}
}

func bayCap(cargo float64, shipType, spec, bay string) float64 {
	if cargo <= 0 {
		return 0
	}
	cap := cargo * bayFraction(shipType, spec, bay)
	// Стартовая доставка требует 5 руды. Общий трюм остаётся прежним;
	// меняется только нижний предел рудного отсека «Стрижа».
	if shipType == DefaultShip && bay == "ore" {
		cap = math.Min(cargo, math.Max(cap, StarterOreCapacity))
	}
	return cap
}

func bayUsed(hold map[string]float64, bay string) float64 {
	sum := 0.0
	for res, amt := range hold {
		if amt > 0 && resourceBay(res) == bay {
			sum += amt
		}
	}
	return sum
}

func cargoRoom(p *Player, res string) float64 {
	if p == nil || p.spec.Cargo <= 0 {
		return 0
	}
	total := p.spec.Cargo - holdUsed(p.hold)
	bay := bayCap(p.spec.Cargo, p.ShipType, p.Spec, resourceBay(res)) - bayUsed(p.hold, resourceBay(res))
	return math.Max(0, math.Min(total, bay))
}

func (p *Player) cargoBays() []CargoBay {
	if p == nil {
		return nil
	}
	out := make([]CargoBay, 0, 3)
	for _, b := range []struct{ id, name string }{{"ore", "Руда"}, {"gas", "Газ"}, {"parts", "Лом"}} {
		cap := bayCap(p.spec.Cargo, p.ShipType, p.Spec, b.id)
		if cap <= 0 {
			continue
		}
		out = append(out, CargoBay{ID: b.id, Name: b.name, Used: bayUsed(p.hold, b.id), Cap: cap})
	}
	return out
}
