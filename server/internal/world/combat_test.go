package world

import (
	"math"
	"testing"
	"time"
)

func stepFor(w *World, sec float64) {
	for i := 0; i < int(sec*20+0.5); i++ {
		w.Step(0.05, time.Now())
	}
}

func hasEvent(evs []Event, typ string) bool {
	for _, e := range evs {
		if e.Type == typ {
			return true
		}
	}
	return false
}

func newPilot(t *testing.T, w *World, id string) *Player {
	p := join(t, w, id)
	return p
}

func TestStrizPassportMatchesSpec(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	sp, ok := w.catalog.Spec("FTR-001")
	if !ok || sp.Name != "Стриж" || sp.Hull != 1200 || sp.Shield != 600 || sp.Armor != 120 || sp.Energy != 100 || sp.Cargo != 8 || sp.Crew != 1 || sp.Engines != 2 {
		t.Fatalf("паспорт: %+v", sp)
	}
	if p.Hull != 1200 || p.Shield != 600 || p.Energy != 100 || p.ShipType != "FTR-001" || p.Skin != "a" || p.Spec != "interceptor" {
		t.Fatalf("стартовое состояние: %+v", p)
	}
	st, _ := w.Snapshot("a")
	if st.Ship.HullMax != 1200 || st.Ship.ShieldMax != 600 || st.Ship.Type != "FTR-001" || len(st.Ship.Abilities) != 2 {
		t.Fatalf("снимок: %+v", st.Ship)
	}
}

func TestCatalogValidation(t *testing.T) {
	for name, js := range map[string]string{
		"пусто":            `{"ships":[]}`,
		"нет корпуса":      `{"ships":[{"id":"FTR-001","hull":0,"energy":1,"speed":1,"skins":["a"],"specs":["x"],"default_spec":"x"}]}`,
		"нет способностей": `{"ships":[{"id":"FTR-001","hull":1,"energy":1,"speed":1,"skins":["a"],"specs":["x"],"default_spec":"x","abilities":{}}]}`,
		"не json":          `{{`,
	} {
		if _, err := LoadCatalog([]byte(js)); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
	if _, err := LoadCatalog(shipsJSON); err != nil {
		t.Fatalf("встроенная конфигурация: %v", err)
	}
}

func TestAttackRulesAndDamage(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	if err := w.SetAttack("a", true); err != ErrNoTarget {
		t.Fatalf("атака без цели: %v", err)
	}
	newPilot(t, w, "b")
	w.SetTarget("a", "b")
	if err := w.SetAttack("a", true); err != ErrBadTarget {
		t.Fatalf("игроков атаковать нельзя: %v", err)
	}
	p.Pos = Vec{40, -30 + 20} // 20 ед. от дрона dr1 (дальность 30)
	w.SetTarget("a", "dr1")
	p.Energy = 50
	if err := w.SetAttack("a", true); err != nil {
		t.Fatal(err)
	}
	stepFor(w, 0.5) // один выстрел (период 1 с)
	evs := w.DrainEvents()
	if !hasEvent(evs, "fire") || !hasEvent(evs, "hit_shield") {
		t.Fatalf("события: %+v", evs)
	}
	dr := w.npcs[0]
	if math.Abs(dr.Shield-(300-44)) > 0.01 || dr.Hull != 1500 {
		t.Fatalf("после выстрела щит=%.1f корпус=%.1f", dr.Shield, dr.Hull)
	}
	if p.Energy >= 50+8*0.5-0.01 {
		t.Fatalf("выстрел должен тратить энергию: %.2f", p.Energy)
	}
	// дрон уничтожается, цель сбрасывается, затем возрождается
	for i := 0; i < 1400 && dr.deadUntil == 0; i++ {
		w.Step(0.05, time.Now())
	}
	evs = append(evs, w.DrainEvents()...)
	if dr.deadUntil == 0 || !hasEvent(evs, "hit_hull") || !hasEvent(evs, "death") {
		t.Fatalf("дрон не уничтожен: hull=%.0f", dr.Hull)
	}
	if p.Target != "" || p.Attacking {
		t.Fatal("после уничтожения цель и атака сбрасываются")
	}
	stepFor(w, 10.5)
	if dr.Hull != 1500 || dr.deadUntil != 0 {
		t.Fatal("дрон должен возродиться")
	}
}

func TestOutOfRangeNoDamage(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	p.Pos = Vec{300, 300}
	p.Target = "dr1"
	w.mu.Lock()
	p.Attacking = true
	w.mu.Unlock()
	stepFor(w, 3)
	if w.npcs[0].Shield != 300 {
		t.Fatal("вне дальности урона быть не должно")
	}
}

func TestBoostSpeedCooldownEnergy(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	w.SetMove("a", Vec{1, 0})
	stepFor(w, 1)
	base := p.Speed
	if err := w.UseAbility("a", "boost"); err != nil {
		t.Fatal(err)
	}
	stepFor(w, 1)
	if math.Abs(p.Speed-base*1.8) > 0.05 {
		t.Fatalf("форсаж: скорость %.2f, ожидалось %.2f", p.Speed, base*1.8)
	}
	if err := w.UseAbility("a", "boost"); err != ErrCooldown {
		t.Fatalf("повтор до перезарядки: %v", err)
	}
	stepFor(w, 3)
	if math.Abs(p.Speed-base) > 0.05 {
		t.Fatalf("после форсажа скорость %.2f", p.Speed)
	}
	if left := w.cooldownLeft(p, "boost"); left <= 0 || left > 12 {
		t.Fatalf("перезарядка %.2f", left)
	}
	// не хватает энергии
	w2 := New()
	q := newPilot(t, w2, "q")
	q.Energy = 5
	if err := w2.UseAbility("q", "boost"); err != ErrEnergy {
		t.Fatalf("энергия: %v", err)
	}
	if err := w2.UseAbility("q", "pulse"); err != ErrUnknown {
		t.Fatalf("автоатака не вызывается как способность: %v", err)
	}
	if err := w2.UseAbility("q", "fireball"); err != ErrUnknown {
		t.Fatalf("неизвестная способность: %v", err)
	}
	// энергия восстанавливается
	stepFor(w2, 5)
	if q.Energy <= 5 {
		t.Fatal("энергия не восстанавливается")
	}
}

func TestInterceptOnlyInterceptor(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	if err := w.UseAbility("a", "intercept"); err != ErrNoTarget {
		t.Fatalf("без цели: %v", err)
	}
	p.Pos = Vec{300, 300}
	w.SetTarget("a", "dr1")
	if err := w.UseAbility("a", "intercept"); err != ErrRange {
		t.Fatalf("далеко: %v", err)
	}
	p.Pos = Vec{40, 15} // 45 ед. от dr1
	if err := w.UseAbility("a", "intercept"); err != nil {
		t.Fatal(err)
	}
	d := dist(p.Pos, w.npcs[0].Pos)
	if math.Abs(d-10) > 0.01 {
		t.Fatalf("после рывка дистанция %.2f, ожидалось 10", d)
	}
	dr := w.npcs[0]
	if math.Abs((300-dr.Shield)-110) > 0.01 {
		t.Fatalf("урон перехвата по щиту %.1f, ожидалось 110", 300-dr.Shield)
	}
	if !hasEvent(w.DrainEvents(), "intercept") {
		t.Fatal("нет события перехвата")
	}
	if err := w.UseAbility("a", "intercept"); err != ErrCooldown {
		t.Fatalf("перезарядка: %v", err)
	}
	// разведчик перехват не использует
	if _, err := w.SetSpec("a", "scout"); err != nil {
		t.Fatal(err)
	}
	p.cooldowns = map[string]float64{}
	if err := w.UseAbility("a", "intercept"); err != ErrWrongSpec {
		t.Fatalf("специализация: %v", err)
	}
}

func TestScannerRevealsHidden(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	count := func() int {
		st, _ := w.Snapshot("a")
		n := 0
		for _, o := range st.Objects {
			if o.Kind == "signal" {
				n++
			}
		}
		return n
	}
	if count() != 0 {
		t.Fatal("скрытые сигналы видны без сканирования")
	}
	if err := w.UseAbility("a", "scan"); err != ErrWrongSpec {
		t.Fatalf("перехватчик не сканирует: %v", err)
	}
	if _, err := w.SetSpec("a", "scout"); err != nil {
		t.Fatal(err)
	}
	p.Pos = Vec{80, 55} // рядом с sig1 (90,60), далеко от sig2 (-70,85)
	if err := w.UseAbility("a", "scan"); err != nil {
		t.Fatal(err)
	}
	if count() != 1 {
		t.Fatalf("раскрыто сигналов %d, ожидался 1 (в радиусе)", count())
	}
	if !hasEvent(w.DrainEvents(), "scan") {
		t.Fatal("нет события сканирования")
	}
	stepFor(w, 16)
	if count() != 0 {
		t.Fatal("сигнал должен снова скрыться")
	}
	if err := w.UseAbility("a", "scan"); err == nil {
		t.Fatal("перезарядка сканера должна ещё идти (20 с)")
	}
}

func TestSkinAndSpecChoice(t *testing.T) {
	w := New()
	newPilot(t, w, "a")
	if rec, err := w.SetSkin("a", "b"); err != nil || rec.Skin != "b" {
		t.Fatalf("облик: %v %+v", err, rec)
	}
	if _, err := w.SetSkin("a", "zzz"); err != ErrBadSkin {
		t.Fatal("несуществующий облик")
	}
	if _, err := w.SetSpec("a", "tank"); err != ErrBadSpec {
		t.Fatal("несуществующая специализация")
	}
	if _, err := w.SetSkin("нет", "a"); err != ErrNoPlayer {
		t.Fatal("игрока нет")
	}
	// характеристики не зависят от облика
	pa, _ := w.SpecInfo("a")
	if pa.Hull != 1200 {
		t.Fatal("облик не должен менять характеристики")
	}
	// выбор переживает повторный вход
	w.Disconnect("a", time.Now())
	rec := w.players["a"].record()
	w2 := New()
	p2, _ := w2.Join(rec)
	if p2.Skin != "b" {
		t.Fatalf("облик после входа: %s", p2.Skin)
	}
	// неверные значения в записи заменяются значениями по умолчанию
	p3, _ := w2.Join(Record{ID: "c", Skin: "bad", Spec: "bad"})
	if p3.Skin != "a" || p3.Spec != "interceptor" {
		t.Fatalf("значения по умолчанию: %s %s", p3.Skin, p3.Spec)
	}
}

func TestPlayerDamageDeathRespawn(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	w.DamagePlayer("a", 700) // щит 600 + 100 по корпусу с бронёй
	if p.Shield != 0 || p.Hull >= 1200 || p.Hull < 1100 {
		t.Fatalf("щит=%.0f корпус=%.1f", p.Shield, p.Hull)
	}
	evs := w.DrainEvents()
	if !hasEvent(evs, "hit_shield") || !hasEvent(evs, "hit_hull") {
		t.Fatal("нет событий попаданий")
	}
	w.DamagePlayer("a", 100000)
	if !hasEvent(w.DrainEvents(), "death") || p.deadUntil <= 0 {
		t.Fatal("гибель не зафиксирована")
	}
	w.SetMove("a", Vec{1, 0})
	before := p.Pos
	stepFor(w, 1)
	if p.Pos != before {
		t.Fatal("уничтоженный корабль не двигается")
	}
	stepFor(w, 3)
	if p.Hull != 1200 || p.Shield != 600 || !hasEvent(w.DrainEvents(), "respawn") {
		t.Fatalf("возрождение: корпус %.0f щит %.0f", p.Hull, p.Shield)
	}
}

func TestShieldRegeneratesAfterDelay(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	w.DamagePlayer("a", 200)
	s0 := p.Shield
	stepFor(w, 3)
	if p.Shield != s0 {
		t.Fatal("щит не должен восстанавливаться во время задержки")
	}
	stepFor(w, 3)
	if p.Shield <= s0 {
		t.Fatal("щит должен восстанавливаться после задержки")
	}
}
