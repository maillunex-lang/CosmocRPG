package worldgen

import (
	"reflect"
	"testing"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	if errs := cfg.Validate(); len(errs) > 0 {
		t.Fatalf("конфигурация по умолчанию некорректна: %v", errs)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Systems[0]
	a, b := Generate(p, cfg), Generate(p, cfg)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("одинаковый seed дал разные раскладки")
	}
	p2 := p
	p2.Seed++
	if reflect.DeepEqual(a, Generate(p2, cfg)) {
		t.Fatal("разные seed дали одинаковую раскладку")
	}
}

func TestGenerateRulesManySeeds(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Systems[0]
	for seed := int64(1); seed <= 300; seed++ {
		p.Seed = seed
		for _, spec := range []string{"mixed", "mining", "pirate", "trade", "explore"} {
			p.Specialization = spec
			l := Generate(p, cfg)
			if errs := l.Validate(); len(errs) > 0 {
				t.Fatalf("seed %d, %s: %v", seed, spec, errs)
			}
		}
	}
}

func TestDemoSystemContent(t *testing.T) {
	cfg := DefaultConfig()
	l := Generate(cfg.Systems[0], cfg)
	if len(l.Bodies) == 0 || len(l.Stations) == 0 || len(l.Deposits) == 0 || len(l.Spawns) == 0 || len(l.Points) == 0 {
		t.Fatalf("демо-система неполна: %d тел, %d станций, %d месторождений, %d врагов, %d точек",
			len(l.Bodies), len(l.Stations), len(l.Deposits), len(l.Spawns), len(l.Points))
	}
}

func TestFeatureCombinationsAndVariety(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Systems[0]
	all := []string{"asteroid_belt", "gas_giant", "pirate_territory", "anomalies", "busy_lanes", "debris_field", "derelict", "guarded_cache"}
	names := map[string]bool{}
	for seed := int64(1); seed <= 200; seed++ {
		p.Seed = seed
		p.Features = all
		l := Generate(p, cfg)
		if errs := l.Validate(); len(errs) > 0 {
			t.Fatalf("seed %d: %v", seed, errs)
		}
		for _, z := range l.Zones {
			names[z.Name] = true
		}
	}
	if len(names) < 10 {
		t.Fatalf("мало разнообразия названий зон: %d", len(names))
	}
}

func TestDemoHasDiverseKinds(t *testing.T) {
	cfg := DefaultConfig()
	l := Generate(cfg.Systems[0], cfg)
	kinds := map[string]bool{}
	for _, p := range l.Points {
		kinds[p.Kind] = true
	}
	res := map[string]bool{}
	for _, d := range l.Deposits {
		res[d.Resource] = true
	}
	if len(kinds) < 2 || len(res) < 2 {
		t.Fatalf("мало разнообразия в демо-системе: точки %v, ресурсы %v", kinds, res)
	}
}

func TestBigMapContentAndRules(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Systems[0]
	if p.Radius < 1200 {
		t.Fatalf("карта должна быть большой, радиус %v", p.Radius)
	}
	l := Generate(p, cfg)
	if errs := l.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
	zones := map[string]int{}
	for _, z := range l.Zones {
		zones[z.Type]++
	}
	tiers := map[string]int{}
	for _, s := range l.Spawns {
		tiers[s.Tier]++
	}
	if len(l.Stations) < 3 || zones["mining"] < 3 || zones["pve"] < 3 || zones["explore"] < 3 || len(l.Bodies) < 3 {
		t.Fatalf("мало локаций: станций %d, зоны %v, тел %d", len(l.Stations), zones, len(l.Bodies))
	}
	if tiers["worldboss"] != 1 || tiers["miniboss"] < 1 || tiers["elite"] < 3 || tiers["normal"] < 6 {
		t.Fatalf("состав врагов: %v", tiers)
	}
	res := map[string]bool{}
	for _, d := range l.Deposits {
		res[d.Resource] = true
	}
	if len(res) < 4 {
		t.Fatalf("ресурсов %v", res)
	}
	if len(l.Lanes) < 2 || len(l.Points) < 6 {
		t.Fatalf("маршрутов %d, точек исследования %d", len(l.Lanes), len(l.Points))
	}
	// новичок не должен лететь через полкарты: ближайший пояс и ближайшая станция недалеко от хаба
	nearBelt := 1e9
	for _, z := range l.Zones {
		if z.Type == "mining" {
			if d := hypot(z.Pos[0]-l.Hub[0], z.Pos[1]-l.Hub[1]); d < nearBelt {
				nearBelt = d
			}
		}
	}
	if nearBelt > 450 {
		t.Fatalf("ближайшая зона добычи слишком далеко: %.0f", nearBelt)
	}
}

func TestBigMapRulesManySeeds(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Systems[0]
	full, missing := 0, 0
	for seed := int64(1); seed <= 200; seed++ {
		p.Seed = seed
		l := Generate(p, cfg)
		if errs := l.Validate(); len(errs) > 0 {
			t.Fatalf("seed %d: %v", seed, errs)
		}
		if len(l.Stations) == 3 {
			full++
		} else {
			missing++
		}
	}
	if full < 150 {
		t.Fatalf("станции размещаются не всегда: полных раскладок %d из 200 (без станций %d)", full, missing)
	}
}
