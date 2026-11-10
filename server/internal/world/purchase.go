package world

import (
	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
	"math"
)

type purchaseLease struct{ player *Player }

func (p *Player) actionBusy() bool {
	return p.equipmentLease != nil || p.purchaseLease != nil || p.cargoUncertain
}

// RestorePurchaseCargo получает только результат authoritative read/barrier.
// Более старый снимок не отменяет последующую добычу/продажу в живом мире.
func (w *World) RestorePurchaseCargo(id string, c *economy.Cargo) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return ErrNoPlayer
	}
	if p.equipmentLease != nil || p.purchaseLease != nil {
		return ErrEquipmentBusy
	}
	if c != nil {
		if economy.ValidateCargo(*c) != nil {
			return economy.ErrUnavailable
		}
		if c.Revision > p.cargoRevision {
			p.hold = economy.CopyHold(c.Hold)
			p.cargoRevision = c.Revision
		}
	}
	p.cargoUncertain = false
	return nil
}

// PurchaseResource фиксирует валидный intent в мире, затем отпускает World.mu
// до SQL. Урон продолжает применяться; только этот actor временно не меняет груз.
func (w *World) PurchaseResource(id, key, resource string, amount float64, commit func(economy.Request) (economy.Result, error)) (economy.Result, error) {
	w.mu.Lock()
	p := w.players[id]
	fail := func(err error) (economy.Result, error) { w.mu.Unlock(); return economy.Result{}, err }
	if p == nil {
		return fail(ErrNoPlayer)
	}
	if p.cargoUncertain {
		return fail(economy.ErrUnavailable)
	}
	if p.actionBusy() {
		return fail(ErrEquipmentBusy)
	}
	if p.deadUntil > w.t {
		return fail(ErrDead)
	}
	if !w.inSafe(p.Pos) {
		return fail(ErrNoStable)
	}
	resourceCfg, ok := w.wcfg.Resources[resource]
	if !ok || resourceCfg.Price <= 0 {
		return fail(ErrNoResource)
	}
	if !isFinitePurchase(amount) || amount <= 0 || amount > 1000 {
		return fail(economy.ErrInvalid)
	}
	if amount > p.spec.Cargo-holdUsed(p.hold)+1e-8 || amount > cargoRoom(p, resource)+1e-8 {
		return fail(ErrHoldFull)
	}
	request := economy.Request{Player: id, Key: key, Resource: resource, Amount: amount, UnitPrice: math.Max(resourceCfg.Price*1.5, 1), Capacity: p.spec.Cargo, Before: economy.Cargo{Hold: copyHold(p.hold), Revision: p.cargoRevision}}
	if _, _, _, err := economy.Prepare(request); err != nil {
		return fail(err)
	}
	if commit == nil {
		return fail(economy.ErrUnavailable)
	}
	lease := &purchaseLease{p}
	p.purchaseLease = lease
	w.mu.Unlock()
	defer w.abandonPurchase(lease)
	result, err := commit(request)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.players[id] != p || p.purchaseLease != lease {
		return result, economy.ErrUnavailable
	}
	p.purchaseLease = nil
	if !result.Known || (result.Found && result.Cargo == nil) || (result.Cargo != nil && economy.ValidateCargo(*result.Cargo) != nil) {
		p.cargoUncertain = true
		p.Move = Vec{}
		p.Autopilot = false
		p.mining = ""
		return result, economy.ErrUnavailable
	}
	if result.Cargo != nil && result.Cargo.Revision > p.cargoRevision {
		p.hold = copyHold(result.Cargo.Hold)
		p.cargoRevision = result.Cargo.Revision
	}
	if err == nil && !result.Found {
		return result, economy.ErrUnavailable
	}
	return result, err
}
func (w *World) abandonPurchase(l *purchaseLease) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := l.player
	if w.players[p.ID] == p && p.purchaseLease == l {
		p.purchaseLease = nil
		p.cargoUncertain = true
		p.Move = Vec{}
		p.Autopilot = false
		p.mining = ""
	}
}
func isFinitePurchase(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// PersistenceBusy позволяет циклу рассылки отложить SQL-проверки квестов этого
// actor; чужие клиенты и тики не должны ждать его записи.
func (w *World) PersistenceBusy(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	return p == nil || p.actionBusy()
}
