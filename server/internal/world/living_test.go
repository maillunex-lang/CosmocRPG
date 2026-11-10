package world

import (
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

func livingWorld(t *testing.T) (*World, worldgen.Layout) {
	t.Helper()
	cfg := worldgen.DefaultConfig()
	l := worldgen.Generate(cfg.Systems[0], cfg)
	w := New()
	if err := w.ApplyLayout(l, cfg); err != nil {
		t.Fatal(err)
	}
	return w, l
}

func step(w *World, sec float64) {
	for i := 0; i < int(sec*20); i++ {
		w.Step(0.05, time.Now())
	}
}

func firstEnemy(w *World) *npc {
	for _, n := range w.npcs {
		if n.ai != nil {
			return n
		}
	}
	return nil
}

func TestLayoutKeepsTrainingObjects(t *testing.T) {
	w, _ := livingWorld(t)
	for _, id := range []string{"dr1", "dr2", "sig1", "sig2"} {
		if w.npcByID(id, nil) == nil && id[0] == 'd' {
			t.Fatalf("пропал учебный объект %s", id)
		}
	}
	if firstEnemy(w) == nil || len(w.deposits) == 0 {
		t.Fatal("нет врагов или месторождений")
	}
	// повторное применение не дублирует наполнение
	n1, d1 := len(w.npcs), len(w.deposits)
	cfg := worldgen.DefaultConfig()
	if err := w.ApplyLayout(worldgen.Generate(cfg.Systems[0], cfg), cfg); err != nil {
		t.Fatal(err)
	}
	if len(w.npcs) != n1 || len(w.deposits) != d1 {
		t.Fatal("повторное применение раскладки дублирует объекты")
	}
}

func joinAt(w *World, id, ship string, at Vec) *Player {
	p, _ := w.Join(Record{ID: id, Name: id, Pos: at, Created: true, ShipType: ship, PilotSpec: "miner"})
	return p
}

func TestEnemyChasesAttacksAndReturns(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	p := joinAt(w, "p1", "FTR-001", Vec{e.Pos.X + 20, e.Pos.Y})
	step(w, 6)
	if p.Hull+p.Shield >= p.spec.Hull+p.spec.Shield {
		t.Fatal("враг не атаковал игрока в зоне агро")
	}
	// игрок убегает за пределы поводка (телепортом) — враг возвращается домой и лечится
	p.Pos = Vec{0, 0}
	w.inSafeForce(p)
	step(w, 60)
	if dist(e.Pos, e.ai.home) > e.ai.patrolR+3 {
		t.Fatalf("враг не вернулся домой: %.1f от дома", dist(e.Pos, e.ai.home))
	}
}

func TestNoAttackInsideSafeZone(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	p := joinAt(w, "p1", "FTR-001", Vec{-12 + 10, 14})
	e.Pos = p.Pos.add(Vec{5, 0}) // враг вплотную к игроку у станции
	hp := p.Hull + p.Shield
	step(w, 10)
	if p.Hull+p.Shield < hp {
		t.Fatal("враг атаковал игрока в безопасной зоне станции")
	}
}

func TestKillRewardIdempotentKeys(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	p := joinAt(w, "p1", "FTR-001", e.Pos)
	for k := 0; k < 2; k++ {
		e.Hull, e.Shield = 1, 0
		w.mu.Lock()
		w.hitNPC(p, e, 100)
		e.deadUntil, e.Hull = 0, e.HullMax
		w.mu.Unlock()
	}
	rs := w.DrainRewards()
	if len(rs) != 2 || rs[0].Key == rs[1].Key || rs[0].Credits != int64(e.ai.cfg.Reward) {
		t.Fatalf("награды за убийство: %+v", rs)
	}
}

func TestMiningRequiresModuleRangeAndCargo(t *testing.T) {
	w, _ := livingWorld(t)
	d := w.deposits[0]
	f := joinAt(w, "f", "FTR-001", d.Pos)
	if err := w.StartMining("f", d.ID); err != nil {
		t.Fatalf("истребитель должен уметь собирать вручную: %v", err)
	}
	m := joinAt(w, "m", "TMP-MINE", d.Pos)
	w.StopMining("f")
	mFar := d.Pos.add(Vec{200, 0})
	m.Pos = mFar
	if err := w.StartMining("m", d.ID); err != ErrRange {
		t.Fatalf("вне дальности: %v", err)
	}
	m.Pos = d.Pos.add(Vec{5, 0})
	f.Pos = d.Pos.add(Vec{4, 0})
	before := d.Amount
	if err := w.StartMining("m", d.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.StartMining("f", d.ID); err != nil {
		t.Fatal(err)
	}
	step(w, 6)
	if m.hold[d.Resource] <= f.hold[d.Resource] || f.hold[d.Resource] <= 0 || d.Amount >= before {
		t.Fatalf("шахтёр должен собирать быстрее: шахтёр %.2f, истребитель %.2f, жила %.1f", m.hold[d.Resource], f.hold[d.Resource], d.Amount)
	}
	// груз не превышает вместимость, добыча останавливается при заполнении
	step(w, 60)
	if holdUsed(m.hold) > m.spec.Cargo+1e-9 || m.mining != "" {
		t.Fatalf("переполнение груза или добыча не остановилась: %.1f/%.1f, %q", holdUsed(m.hold), m.spec.Cargo, m.mining)
	}
	if err := w.StartMining("m", d.ID); err != ErrFull {
		t.Fatalf("полный груз: %v", err)
	}
}

func TestMiningInterruptedByDamage(t *testing.T) {
	w, _ := livingWorld(t)
	d := w.deposits[0]
	m := joinAt(w, "m", "TMP-MINE", d.Pos.add(Vec{3, 0}))
	if err := w.StartMining("m", d.ID); err != nil {
		t.Fatal(err)
	}
	step(w, 2)
	w.DamagePlayer("m", 10)
	step(w, 1)
	if m.mining != "" {
		t.Fatal("урон не прервал добычу")
	}
}

func TestDepositRegenerates(t *testing.T) {
	w, _ := livingWorld(t)
	d := w.deposits[0]
	d.Amount = d.Max / 2
	step(w, 120)
	if d.Amount <= d.Max/2 {
		t.Fatal("месторождение не восстанавливается")
	}
	if d.Amount > d.Max {
		t.Fatal("превышен максимум месторождения")
	}
}

func TestSellOnlyAtStation(t *testing.T) {
	w, _ := livingWorld(t)
	m := joinAt(w, "m", "TMP-MINE", Vec{200, 200})
	m.hold = map[string]float64{"iron_ore": 10}
	if _, err := w.SellCargo("m"); err != ErrNoStable {
		t.Fatalf("продажа вдали от станции: %v", err)
	}
	m.Pos = Vec{-12 + 10, 14}
	c, err := w.SellCargo("m")
	if err != nil || c != int64(10*w.wcfg.Resources["iron_ore"].Price) || holdUsed(m.hold) != 0 {
		t.Fatalf("продажа: %d %v", c, err)
	}
	if rs := w.DrainRewards(); len(rs) != 1 || rs[0].Credits != c {
		t.Fatalf("награда за продажу: %+v", rs)
	}
}

func TestDiscoveryOncePerPlayer(t *testing.T) {
	w, l := livingWorld(t)
	var an *worldgen.Point
	for i := range l.Points {
		if !l.Points[i].Hidden {
			an = &l.Points[i]
			break
		}
	}
	if an == nil {
		t.Skip("нет видимой точки")
	}
	joinAt(w, "p", "FTR-001", Vec{an.Pos[0], an.Pos[1]})
	step(w, 2)
	step(w, 2)
	rs := w.DrainRewards()
	if len(rs) != 1 || rs[0].Key != "discover:"+an.ID {
		t.Fatalf("награды исследования: %+v", rs)
	}
}

func TestHoldSanitizedOnJoin(t *testing.T) {
	w, _ := livingWorld(t)
	p, _ := w.Join(Record{ID: "x", Name: "x", Created: true, ShipType: "TMP-MINE", Hold: map[string]float64{"iron_ore": 1e9, "scrap": -5}})
	if holdUsed(p.hold) > p.spec.Cargo || p.hold["scrap"] != 0 {
		t.Fatalf("груз не очищен: %v", p.hold)
	}
}

// inSafeForce сбрасывает запомненную цель врагов для игрока.
func (w *World) inSafeForce(p *Player) {
	for _, n := range w.npcs {
		if n.ai != nil && n.ai.target == p.ID {
			n.ai.target = ""
		}
	}
}

func TestSetShipRules(t *testing.T) {
	w, _ := livingWorld(t)
	p := joinAt(w, "s", "FTR-001", Vec{-12 + 8, 14})
	if _, err := w.SetShip("s", "NOPE-1"); err != ErrNoShip {
		t.Fatalf("неизвестный корабль: %v", err)
	}
	rec, err := w.SetShip("s", "COR-001")
	if err != nil || rec.ShipType != "COR-001" || p.ShipType != "COR-001" || p.Hull != p.spec.Hull {
		t.Fatalf("смена у станции: %v %+v", err, rec)
	}
	p.Pos = Vec{200, 200}
	if _, err := w.SetShip("s", "FTR-001"); err != ErrNoStable {
		t.Fatalf("смена вдали от станции: %v", err)
	}
	p.Pos = Vec{-12 + 8, 14}
	p.hold = map[string]float64{"iron_ore": 30}
	if _, err := w.SetShip("s", "FTR-001"); err != ErrHoldBig {
		t.Fatalf("груз не помещается: %v", err)
	}
	if len(w.Fleet()) < 12 {
		t.Fatalf("в каталоге %d кораблей", len(w.Fleet()))
	}
}

func TestBigMapRadiusAppliesToMovement(t *testing.T) {
	w, l := livingWorld(t)
	if l.Radius < 1200 || w.radius != l.Radius {
		t.Fatalf("радиус мира %v, раскладки %v", w.radius, l.Radius)
	}
	// игрок далеко от центра (на большой карте) остаётся там, а не телепортируется в центр
	p, _ := w.Join(Record{ID: "far", Name: "far", Pos: Vec{X: 900, Y: -300}, Created: true})
	if p.Pos.X != 900 {
		t.Fatalf("позиция на большой карте сброшена: %v", p.Pos)
	}
	p.Move = Vec{X: 1}
	step(w, 2)
	if p.Pos.X <= 900 {
		t.Fatalf("игрок не движется на большой карте: %v", p.Pos)
	}
	// за границей карты — упор в край
	p.Pos = Vec{X: l.Radius - 1}
	p.Move = Vec{X: 1}
	step(w, 3)
	if p.Pos.length() > l.Radius+1e-6 {
		t.Fatalf("игрок вышел за границу карты: %v", p.Pos.length())
	}
	// без раскладки радиус прежний
	if New().radius != Radius {
		t.Fatal("радиус по умолчанию изменился")
	}
}

func TestAutoRetaliateOnAttack(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	p := joinAt(w, "r", "FTR-001", Vec{e.Pos.X + 15, e.Pos.Y})
	step(w, 4) // враг замечает игрока и атакует
	if p.Target != e.ID || !p.Attacking {
		t.Fatalf("автоответ по умолчанию включён: цель %q, атака %v", p.Target, p.Attacking)
	}
	// выключенный автоответ: игрок не отвечает
	w2, _ := livingWorld(t)
	e2 := firstEnemy(w2)
	p2 := joinAt(w2, "r2", "FTR-001", Vec{e2.Pos.X + 15, e2.Pos.Y})
	if err := w2.SetOption("r2", "auto_retaliate", false); err != nil {
		t.Fatal(err)
	}
	step(w2, 4)
	if p2.Hull+p2.Shield >= p2.spec.Hull+p2.spec.Shield {
		t.Fatal("враг не атаковал: тест не проверяет автоответ")
	}
	if p2.Attacking || p2.Target != "" {
		t.Fatalf("автоответ выключен, но игрок ответил: цель %q", p2.Target)
	}
	// включили обратно: при следующей атаке отвечает
	_ = w2.SetOption("r2", "auto_retaliate", true)
	step(w2, 3)
	if !p2.Attacking || !strings.HasPrefix(p2.Target, "en-") {
		t.Fatalf("после включения игрок не ответил: цель %q", p2.Target)
	}
	if err := w2.SetOption("r2", "nope", true); err != ErrBadOption {
		t.Fatalf("неизвестная настройка: %v", err)
	}
	if err := w2.SetOption("ghost", "auto_retaliate", true); err != ErrNoPlayer {
		t.Fatalf("нет игрока: %v", err)
	}
}

func TestAutoRetaliateKeepsCurrentLiveTarget(t *testing.T) {
	w, _ := livingWorld(t)
	e := firstEnemy(w)
	p := joinAt(w, "k", "FTR-001", Vec{e.Pos.X + 15, e.Pos.Y})
	// игрок уже атакует учебного дрона: нападение врага не переключает цель
	p.Target, p.Attacking = "dr1", true
	w.mu.Lock()
	w.retaliate(p, e.ID)
	w.mu.Unlock()
	if p.Target != "dr1" {
		t.Fatalf("цель переключена с живой на нападающего: %q", p.Target)
	}
	// мёртвого нападающего не атакуют
	p.Target, p.Attacking = "", false
	e.deadUntil = w.t + 100
	w.mu.Lock()
	w.retaliate(p, e.ID)
	w.mu.Unlock()
	if p.Attacking {
		t.Fatal("автоответ на мёртвого нападающего")
	}
}

func TestStationServicesRepairAndCargo(t *testing.T) {
	w, lay := livingWorld(t)
	p, _ := w.Join(Record{ID: "svc", Name: "S", Created: true, ShipType: "TMP-HAUL", Pos: Vec{X: lay.Hub[0], Y: lay.Hub[1]}})
	if err := w.AddCargo(p.ID, "iron_ore", 5); err != nil {
		t.Fatalf("покупка: %v", err)
	}
	if got := p.hold["iron_ore"]; got != 5 {
		t.Fatalf("груз: %v", got)
	}
	// вне станции — отказ
	p.Pos = Vec{X: 500, Y: 500}
	if err := w.AddCargo(p.ID, "iron_ore", 1); err != ErrNoStable {
		t.Fatalf("вне станции: %v", err)
	}
	if _, err := w.RepairCost(p.ID); err != ErrNoStable {
		t.Fatalf("ремонт вне станции: %v", err)
	}
	// ремонт у станции
	p.Pos = Vec{}
	p.Hull, p.Shield = p.spec.Hull/2, 0
	cost, err := w.RepairCost(p.ID)
	if err != nil || cost <= 0 {
		t.Fatalf("цена ремонта: %d %v", cost, err)
	}
	if err := w.Repair(p.ID); err != nil {
		t.Fatal(err)
	}
	if p.Hull != p.spec.Hull || p.Shield != p.spec.Shield {
		t.Fatalf("не восстановлено: %v/%v", p.Hull, p.Shield)
	}
	if _, ok := w.ResourcePrice("iron_ore"); !ok {
		t.Fatal("цена iron_ore")
	}
	if _, ok := w.ResourcePrice("unobtanium"); ok {
		t.Fatal("неизвестный ресурс")
	}
}
