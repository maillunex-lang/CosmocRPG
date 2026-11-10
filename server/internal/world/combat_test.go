package world

import (
	"encoding/json"
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
	if st.Ship.HullMax != 1200 || st.Ship.ShieldMax != 600 || st.Ship.Type != "FTR-001" || st.Ship.Abilities[0].ID != "boost" || !hasRole(st.Ship.Abilities, "strike") || !hasRole(st.Ship.Abilities, "ult") {
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

func plantMark(w *World) *npc {
	n := &npc{ID: "mark", Name: "Мишень", Kind: "hostile", Stance: "hostile", Pos: Vec{40, -30}, Hull: 1500, HullMax: 1500, Shield: 300, ShieldMax: 300, respawn: 10}
	w.npcs = append([]*npc{n}, w.npcs...)
	return n
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
	p.Pos = Vec{40, -30 + 20}
	plantMark(w)
	w.SetTarget("a", "mark")
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
	if !hasEvent(evs, "hit_hull") || !hasEvent(evs, "death") {
		t.Fatalf("мишень не уничтожена: hull=%.0f", dr.Hull)
	}
	if p.Target != "" || p.Attacking {
		t.Fatal("после уничтожения цель и атака сбрасываются")
	}
	stepFor(w, 10.5)
	if dr.Hull != 1500 || dr.deadUntil != 0 {
		t.Fatal("мишень должна возродиться")
	}
}

func TestOutOfRangeNoDamage(t *testing.T) {
	w := New()
	p := newPilot(t, w, "a")
	plantMark(w)
	p.Pos = Vec{300, 300}
	p.Target = "mark"
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
	plantMark(w)
	if err := w.UseAbility("a", "intercept"); err != ErrNoTarget {
		t.Fatalf("без цели: %v", err)
	}
	p.Pos = Vec{300, 300}
	w.SetTarget("a", "mark")
	if err := w.UseAbility("a", "intercept"); err != ErrRange {
		t.Fatalf("далеко: %v", err)
	}
	p.Pos = Vec{40, 15}
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

func TestNickRules(t *testing.T) {
	ok := []string{"Vasya", "Вася_Пупкин", "Ace-99", "ЗлойКот", "abc", "Ёжик2024"}
	for _, n := range ok {
		if got, err := CleanNick("  " + n + " "); err != nil || got != n {
			t.Errorf("ник %q должен проходить: %v %q", n, err, got)
		}
	}
	bad := []string{"", "ab", "x", "Очень_длинный_ник_17!", "has space", "_start", "end_", "123456", "ник@", "Привет!", "a🙂b", "кот кот"}
	for _, n := range bad {
		if _, err := CleanNick(n); err != ErrBadNick {
			t.Errorf("ник %q должен быть некорректным: %v", n, err)
		}
	}
	for _, n := range []string{"Admin", "АдминИгры", "xXModerXx", "хуйло", "Fuck_You", "Сервер1", "Бот-Х"} {
		if _, err := CleanNick(n); err != ErrNickReserved && err != ErrBadNick {
			t.Errorf("ник %q должен быть недоступен: %v", n, err)
		}
	}
	if !ValidGender("m") || !ValidGender("f") || ValidGender("x") || ValidGender("") {
		t.Fatal("пол")
	}
}

func TestStartersAndTemporaryShips(t *testing.T) {
	w := New()
	cat := w.Catalog()
	if len(cat.Ships) < 3 {
		t.Fatalf("кораблей в каталоге %d", len(cat.Ships))
	}
	for _, ps := range PilotSpecs {
		st, ok := w.Starter(ps)
		if !ok {
			t.Fatalf("нет стартового корабля для %s", ps)
		}
		if _, ok := cat.Spec(st.Ship); !ok {
			t.Fatalf("стартовый корабль %s не найден", st.Ship)
		}
	}
	// классы кораблей сохраняются отдельно от направления пилота: у «Стрижа» остаются свои специализации
	st1, _ := w.Starter("interceptor")
	st2, _ := w.Starter("scout")
	if st1.Ship != "FTR-001" || st2.Ship != "FTR-001" || st1.Spec == st2.Spec {
		t.Fatal("перехватчик и разведчик летают на «Стриже» с разными специализациями корабля")
	}
	for _, id := range []string{"TMP-HAUL", "TMP-MINE"} {
		sp, _ := cat.Spec(id)
		if !sp.Temporary {
			t.Errorf("%s должен быть помечен как временная модель", id)
		}
	}
	if sp, _ := cat.Spec("FTR-001"); sp.Temporary {
		t.Error("«Стриж» не временный")
	}
	// игрок на временном корабле: паспорт корабля применяется (груз), способности специализации «Стрижа» недоступны
	p, _ := w.Join(Record{ID: "h", Name: "h", ShipType: "TMP-HAUL", Skin: "a", Spec: "hauler"})
	if p.ShipType != "TMP-HAUL" || p.Hull != 1800 || p.spec.Cargo != 60 {
		t.Fatalf("грузовик: %+v", p.spec)
	}
	if err := w.UseAbility("h", "scan"); err != ErrUnknown && err != ErrWrongSpec {
		t.Fatalf("сканер у грузовика: %v", err)
	}
	st, _ := w.Snapshot("h")
	if st.Ship.Type != "TMP-HAUL" || st.Ship.Cargo != 60 || st.Ship.Abilities[0].Role != "" || !hasRole(st.Ship.Abilities, "ult") {
		t.Fatalf("снимок: %+v", st.Ship)
	}
	// неизвестный тип корабля заменяется стандартным
	q, _ := w.Join(Record{ID: "q", Name: "q", ShipType: "НЕТ"})
	if q.ShipType != "FTR-001" {
		t.Fatalf("неизвестный корабль: %s", q.ShipType)
	}
}

func TestSetCatalogLive(t *testing.T) {
	w := New()
	p, _ := w.Join(Record{ID: "a", Name: "a"})
	cat := w.Catalog()
	sp := cat.Ships["FTR-001"]
	sp.Hull, sp.Shield, sp.Speed = 500, 100, 20
	sp.Abilities["boost"] = AbilityCfg{Name: "Форсаж", SpeedMul: 3, Duration: 3, Cooldown: 12, Energy: 20}
	cat.Ships["FTR-001"] = sp
	w.SetCatalog(cat)
	if p.Hull != 500 || p.Shield != 100 || p.spec.Speed != 20 {
		t.Fatalf("паспорт игрока не обновился: hull=%.0f shield=%.0f speed=%.0f", p.Hull, p.Shield, p.spec.Speed)
	}
	w.SetMove("a", Vec{1, 0})
	stepFor(w, 1)
	if math.Abs(p.Speed-20) > 0.1 {
		t.Fatalf("новая скорость %.2f", p.Speed)
	}
	if err := w.UseAbility("a", "boost"); err != nil {
		t.Fatal(err)
	}
	stepFor(w, 1)
	if math.Abs(p.Speed-60) > 0.5 {
		t.Fatalf("новый множитель форсажа: %.2f", p.Speed)
	}
	// корабль удалён из каталога: игрок получает корабль по умолчанию
	cat2 := w.Catalog()
	delete(cat2.Ships, "TMP-HAUL")
	cat2.Starters["trader"] = Starter{Ship: "FTR-001", Spec: "interceptor", Skin: "a"}
	w.SetCatalog(cat2)
}

func TestCatalogStartersValidation(t *testing.T) {
	var d Doc
	_ = jsonUnmarshal(shipsJSON, &d)
	d.Starters["miner"] = Starter{Ship: "НЕТ", Spec: "x", Skin: "a"}
	b, _ := jsonMarshal(d)
	if _, err := LoadCatalog(b); err == nil {
		t.Fatal("стартовый корабль должен существовать")
	}
	delete(d.Starters, "miner")
	b, _ = jsonMarshal(d)
	if _, err := LoadCatalog(b); err == nil {
		t.Fatal("для каждого направления пилота нужен стартовый корабль")
	}
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }

// Catalogs can contain future or externally imported ability keys. Until a
// handler exists they must fail explicitly without committing any combat state.
func TestUnsupportedCatalogAbilityFailsAtomically(t *testing.T) {
	w := New()
	p := newPilot(t, w, "unsupported")
	c := w.Catalog()
	sp := c.Ships[p.ShipType]
	sp.Abilities["custom"] = AbilityCfg{Name: "Unsupported", Energy: 10, Cooldown: 5}
	c.Ships[p.ShipType] = sp
	w.SetCatalog(c)
	energy := p.Energy
	cooldowns := len(p.cooldowns)
	position := p.Pos
	for _, ability := range []string{"custom", "missing", "pulse"} {
		if err := w.UseAbility(p.ID, ability); err != ErrUnknown {
			t.Errorf("%s: got %v, want ErrUnknown", ability, err)
		}
	}
	if p.Energy != energy || len(p.cooldowns) != cooldowns || p.Pos != position {
		t.Fatal("rejected ability changed player state")
	}
	if events := w.DrainEvents(); len(events) != 0 {
		t.Fatalf("rejected ability emitted events: %+v", events)
	}
}
