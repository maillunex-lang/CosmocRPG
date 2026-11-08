package rpg

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func testTree() TalentTree {
	return TalentTree{
		Version:  1,
		Branches: []string{"combat", "mining"},
		Nodes: []TalentNode{
			{ID: "c1", Name: "Огневая подготовка", Branch: "combat", Tier: 1, Cost: 1, MaxRank: 3, Effects: []TalentEffect{{StatDamage, OpMul, 0.05}}},
			{ID: "c2", Name: "Бронебойные", Branch: "combat", Tier: 2, Cost: 2, MaxRank: 2, Requires: []string{"c1"}, Effects: []TalentEffect{{StatDamage, OpAdd, 2}, {StatCritChance, OpAdd, 0.02}}},
			{ID: "m1", Name: "Буровые головки", Branch: "mining", Tier: 1, Cost: 1, MaxRank: 5, Effects: []TalentEffect{{StatMining, OpMul, 0.04}}},
		},
	}
}

func testCfg() Config {
	c := DefaultConfig()
	c.Talents = testTree()
	c.MasteryEffects = map[string][]MasteryEffect{"fighter": {{StatDamage, OpMul, 0.01}}}
	return c
}

func baseStats() map[StatID]float64 {
	return map[StatID]float64{StatHull: 100, StatDamage: 10, StatSpeed: 5, StatAccuracy: 0.8, StatCritChance: 0.1, StatMining: 1, StatResist: 0.1}
}

func fullInput() Input {
	return Input{
		Base: baseStats(), ShipLevel: 5,
		Attributes: map[Attribute]int{AttrTactics: 10, AttrPiloting: 4},
		Talents:    []LearnedTalent{{"c1", 2}, {"c2", 1}, {"m1", 3}},
		Mastery:    Mastery{Class: "fighter", Level: 3},
		Modules:    []Modifier{{Source: "module:slot1", Stat: StatDamage, Op: OpAdd, Value: 3}},
		Effects:    []Modifier{{Source: "effect:boost", Stat: StatSpeed, Op: OpMul, Value: 0.2}},
	}
}

func TestAllStatsCount(t *testing.T) {
	if len(AllStats) != 19 || len(AllAttributes) != 6 {
		t.Fatalf("ожидалось 19 характеристик корабля и 6 пилота, есть %d и %d", len(AllStats), len(AllAttributes))
	}
	seen := map[StatID]bool{}
	for _, s := range AllStats {
		if seen[s] {
			t.Fatalf("повтор характеристики %s", s)
		}
		seen[s] = true
	}
}

func TestComputeIdempotentAndPure(t *testing.T) {
	cfg, in := testCfg(), fullInput()
	before := baseStats()
	a := Compute(cfg, in)
	b := Compute(cfg, in)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("повторный расчёт дал другой итог")
	}
	if !reflect.DeepEqual(in.Base, before) {
		t.Fatal("расчёт изменил входные данные")
	}
}

func TestComputeOrderIndependent(t *testing.T) {
	cfg := testCfg()
	want := Compute(cfg, fullInput())
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		in := fullInput()
		in.Talents = append([]LearnedTalent(nil), in.Talents...)
		rng.Shuffle(len(in.Talents), func(a, b int) { in.Talents[a], in.Talents[b] = in.Talents[b], in.Talents[a] })
		in.Modules = append(in.Modules, Modifier{Source: "module:slot2", Stat: StatHull, Op: OpMul, Value: 0.1}, Modifier{Source: "module:slot3", Stat: StatHull, Op: OpAdd, Value: 5})
		rng.Shuffle(len(in.Modules), func(a, b int) { in.Modules[a], in.Modules[b] = in.Modules[b], in.Modules[a] })
		got := Compute(cfg, in)
		ref := fullInput()
		ref.Modules = append(ref.Modules, Modifier{Source: "module:slot2", Stat: StatHull, Op: OpMul, Value: 0.1}, Modifier{Source: "module:slot3", Stat: StatHull, Op: OpAdd, Value: 5})
		if !reflect.DeepEqual(got.Final, Compute(cfg, ref).Final) {
			t.Fatalf("итог зависит от порядка источников (итерация %d)", i)
		}
	}
	_ = want
}

func TestLayerOrderAndAddBeforeMul(t *testing.T) {
	cfg := Config{Caps: map[StatID]Cap{}}
	in := Input{Base: map[StatID]float64{StatDamage: 10}, Modules: []Modifier{
		{Source: "a", Stat: StatDamage, Op: OpAdd, Value: 5},
		{Source: "b", Stat: StatDamage, Op: OpMul, Value: 0.5},
		{Source: "c", Stat: StatDamage, Op: OpMul, Value: 0.5},
	}, Effects: []Modifier{{Source: "e", Stat: StatDamage, Op: OpMul, Value: 1}}}
	// слой модулей: (10+5)*(1+0.5+0.5)=30; слой эффектов: 30*2=60
	if got := Compute(cfg, in).Final[StatDamage]; !approx(got, 60) {
		t.Fatalf("ожидалось 60, получено %v", got)
	}
}

func TestEachSourceAppliedOnce(t *testing.T) {
	cfg := Config{Caps: map[StatID]Cap{}}
	in := Input{Base: map[StatID]float64{StatSpeed: 10}, Effects: []Modifier{
		{Source: "effect:boost", Stat: StatSpeed, Op: OpMul, Value: 0.5},
		{Source: "effect:boost", Stat: StatSpeed, Op: OpMul, Value: 0.5},
	}}
	r := Compute(cfg, in)
	if !approx(r.Final[StatSpeed], 15) || len(r.Warnings) != 1 {
		t.Fatalf("повтор источника должен применяться один раз: %v, предупреждения %v", r.Final[StatSpeed], r.Warnings)
	}
}

func TestTalentResetRemovesExactlyItsContribution(t *testing.T) {
	cfg := testCfg()
	with := fullInput()
	without := fullInput()
	without.Talents = []LearnedTalent{{"c1", 2}, {"m1", 3}} // сброшен c2
	a, b := Compute(cfg, with), Compute(cfg, without)
	if !approx(a.Final[StatHull], b.Final[StatHull]) || !approx(a.Final[StatMining], b.Final[StatMining]) {
		t.Fatal("сброс c2 изменил посторонние характеристики")
	}
	// c2 добавляет +2 к урону (слой талантов, до множителей мастерства): разница положительна
	if !(a.Final[StatDamage] > b.Final[StatDamage]) {
		t.Fatal("вклад c2 должен исчезнуть при сбросе")
	}
	for _, c := range b.Breakdown[StatDamage] {
		if c.Source == "talent:c2" {
			t.Fatal("в разбивке остался сброшенный талант")
		}
	}
}

func TestShipLevelAndAttributes(t *testing.T) {
	cfg := DefaultConfig()
	r := Compute(cfg, Input{Base: map[StatID]float64{StatHull: 100, StatSpeed: 10}, ShipLevel: 11, Attributes: map[Attribute]int{AttrPiloting: 10}})
	if !approx(r.Final[StatHull], 130) { // 100 * (1 + 0.03*10)
		t.Fatalf("корпус: %v", r.Final[StatHull])
	}
	if !approx(r.Final[StatSpeed], 11) { // 10 * (1 + 0.01*10)
		t.Fatalf("скорость: %v", r.Final[StatSpeed])
	}
	r1 := Compute(cfg, Input{Base: map[StatID]float64{StatHull: 100}, ShipLevel: 1})
	if !approx(r1.Final[StatHull], 100) {
		t.Fatal("уровень 1 не даёт прироста")
	}
}

func TestCapsAndUnknownInputs(t *testing.T) {
	cfg := testCfg()
	in := Input{Base: map[StatID]float64{StatResist: 0.7, StatSpeed: 1},
		Attributes: map[Attribute]int{AttrEngineering: 1000},
		Talents:    []LearnedTalent{{"нет-такого", 1}, {"c1", 99}},
		Modules:    []Modifier{{Source: "", Stat: StatSpeed, Op: OpAdd, Value: 1}, {Source: "x", Stat: "bogus", Op: OpAdd, Value: 1}, {Source: "y", Stat: StatSpeed, Op: "pow", Value: 1}}}
	r := Compute(cfg, in)
	if r.Final[StatResist] > 0.8+1e-12 {
		t.Fatalf("потолок сопротивления нарушен: %v", r.Final[StatResist])
	}
	if !approx(r.Final[StatSpeed], 1) {
		t.Fatalf("некорректные вклады не должны применяться: %v", r.Final[StatSpeed])
	}
	if len(r.Warnings) < 4 {
		t.Fatalf("ожидались предупреждения о неизвестных входах, есть %v", r.Warnings)
	}
	neg := Compute(Config{Caps: map[StatID]Cap{}}, Input{Base: map[StatID]float64{StatSpeed: 1}, Effects: []Modifier{{Source: "s", Stat: StatSpeed, Op: OpAdd, Value: -5}}})
	if neg.Final[StatSpeed] != 0 {
		t.Fatal("значение не должно уходить ниже пола")
	}
}

func TestCurves(t *testing.T) {
	for name, c := range map[string]Curve{"пилот": DefaultPilotCurve, "корабль": DefaultShipCurve} {
		if !c.Valid() {
			t.Fatalf("%s: кривая некорректна", name)
		}
		var prev int64 = -1
		for l := 1; l <= c.MaxLevel; l++ {
			tot := c.TotalXP(l)
			if tot <= prev {
				t.Fatalf("%s: суммарный опыт не растёт на уровне %d", name, l)
			}
			prev = tot
			lvl, into, next := c.LevelFor(tot)
			if lvl != l || into != 0 {
				t.Fatalf("%s: LevelFor(TotalXP(%d)) = %d/%d", name, l, lvl, into)
			}
			if l < c.MaxLevel {
				if _, _, n := c.LevelFor(tot + c.XPForNext(l) - 1); n != 1 {
					t.Fatalf("%s: до следующего уровня должно оставаться 1, есть %d", name, n)
				}
				if next != c.XPForNext(l) {
					t.Fatalf("%s: toNext на уровне %d", name, l)
				}
			}
		}
		lvl, _, next := c.LevelFor(1 << 50)
		if lvl != c.MaxLevel || next != 0 {
			t.Fatalf("%s: потолок уровня не держится", name)
		}
	}
	if lvl, into, _ := DefaultPilotCurve.LevelFor(-5); lvl != 1 || into != 0 {
		t.Fatal("отрицательный опыт считается нулём")
	}
	if TalentPoints(60) < 40 || AttributePoints(1) != 0 || TalentPoints(1) != 0 {
		t.Fatal("очки по уровням заданы неверно")
	}
}

func TestTalentTreeValidate(t *testing.T) {
	if errs := testTree().Validate(); len(errs) != 0 {
		t.Fatalf("корректное дерево отклонено: %v", errs)
	}
	bad := map[string]func(*TalentTree){
		"неизвестная ветвь": func(tr *TalentTree) { tr.Nodes[0].Branch = "нет" },
		"повтор id":         func(tr *TalentTree) { tr.Nodes[1].ID = "c1" },
		"нет требования":    func(tr *TalentTree) { tr.Nodes[1].Requires = []string{"zzz"} },
		"ярус не выше":      func(tr *TalentTree) { tr.Nodes[1].Tier = 1 },
		"нулевая стоимость": func(tr *TalentTree) { tr.Nodes[0].Cost = 0 },
		"неизвестная стат.": func(tr *TalentTree) { tr.Nodes[0].Effects[0].Stat = "x" },
		"нет эффектов":      func(tr *TalentTree) { tr.Nodes[2].Effects = nil },
		"самотребование":    func(tr *TalentTree) { tr.Nodes[0].Requires = []string{"c1"} },
		"повторная ветвь":   func(tr *TalentTree) { tr.Branches = append(tr.Branches, "combat") },
		"неизвестная опер.": func(tr *TalentTree) { tr.Nodes[0].Effects[0].Op = "pow" },
	}
	for name, mut := range bad {
		tr := testTree()
		tr.Nodes = append([]TalentNode(nil), tr.Nodes...)
		for i := range tr.Nodes {
			tr.Nodes[i].Effects = append([]TalentEffect(nil), tr.Nodes[i].Effects...)
		}
		mut(&tr)
		if len(tr.Validate()) == 0 {
			t.Errorf("не замечено: %s", name)
		}
	}
	// цикл при правильных ярусах невозможен, но защита должна срабатывать при любых данных
	cyc := TalentTree{Branches: []string{"b"}, Nodes: []TalentNode{
		{ID: "a", Branch: "b", Tier: 2, Cost: 1, MaxRank: 1, Requires: []string{"b"}, Effects: []TalentEffect{{StatHull, OpAdd, 1}}},
		{ID: "b", Branch: "b", Tier: 1, Cost: 1, MaxRank: 1, Requires: []string{"a"}, Effects: []TalentEffect{{StatHull, OpAdd, 1}}},
	}}
	if len(cyc.Validate()) == 0 {
		t.Error("цикл не замечен")
	}
}
