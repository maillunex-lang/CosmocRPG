package world

import "github.com/maillunex-lang/CosmocRPG/server/internal/rpg"

var shipLevelCfg = rpg.DefaultConfig()

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// applyShipLevelLocked пересчитывает характеристики корабля игрока из каталога и уровня (rpg.Compute, с нуля: повтор не накапливает рост).
// При росте максимума текущие корпус, щит и энергия растут на ту же величину (повышение уровня не лечит сверх максимума и не убирает урон).
func (w *World) applyShipLevelLocked(p *Player, level int) {
	base, ok := w.catalog.Spec(p.ShipType)
	if !ok {
		return
	}
	if level < 1 {
		level = 1
	}
	if level > rpg.DefaultShipCurve.MaxLevel {
		level = rpg.DefaultShipCurve.MaxLevel
	}
	res := rpg.Compute(shipLevelCfg, rpg.Input{
		Base:      map[rpg.StatID]float64{rpg.StatHull: base.Hull, rpg.StatShield: base.Shield, rpg.StatEnergy: base.Energy, rpg.StatCargo: base.Cargo},
		ShipLevel: level,
	})
	old := p.spec
	sp := base
	sp.Hull, sp.Shield, sp.Energy, sp.Cargo = res.Final[rpg.StatHull], res.Final[rpg.StatShield], res.Final[rpg.StatEnergy], res.Final[rpg.StatCargo]
	p.spec, p.shipLevel = sp, level
	grow := func(cur *float64, oldMax, newMax float64) {
		if p.deadUntil > w.t || *cur <= 0 {
			return
		}
		*cur += newMax - oldMax
		if *cur > newMax {
			*cur = newMax
		}
		if *cur < 0 {
			*cur = 0
		}
	}
	grow(&p.Hull, old.Hull, sp.Hull)
	grow(&p.Shield, old.Shield, sp.Shield)
	grow(&p.Energy, old.Energy, sp.Energy)
}

// SetShipLevel задаёт уровень текущего корабля игрока (вызывает хаб по данным хранилища).
func (w *World) SetShipLevel(id string, level int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[id]; ok {
		w.applyShipLevelLocked(p, level)
	}
}

// ShipTypeOf возвращает тип текущего корабля игрока.
func (w *World) ShipTypeOf(id string) (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[id]; ok {
		return p.ShipType, true
	}
	return "", false
}
