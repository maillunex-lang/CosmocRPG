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

func TestRedStormNamesStayOnExistingObjects(t *testing.T) {
	cfg := DefaultConfig()
	var storm, aurora SystemParams
	for _, p := range cfg.Systems {
		switch p.ID {
		case "redstorm":
			storm = p
		case "aurora":
			aurora = p
		}
	}
	if storm.ID == "" || aurora.ID == "" {
		t.Fatal("нет Авроры или Красного Шторма")
	}
	l := Generate(storm, cfg)
	if errs := l.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
	if l.Stations[0].Name != "Форпост «Тихая пристань»" || l.Stations[0].ID != "st1" {
		t.Fatalf("хаб Шторма: %+v", l.Stations[0])
	}
	var derelict, contour int
	for _, p := range l.Points {
		switch p.Name {
		case "Контейнер чужого рейса":
			if p.Kind != "derelict" || p.ID != "dr-1" {
				t.Fatalf("контейнер: %+v", p)
			}
			derelict++
		case "Старый ретрансляционный контур":
			if p.Kind != "anomaly" || p.ID != "an-1" {
				t.Fatalf("контур: %+v", p)
			}
			contour++
		}
	}
	if derelict != 1 || contour != 1 {
		t.Fatalf("точки главы: контейнер %d, контур %d", derelict, contour)
	}
	want := map[string]string{
		"beacon-03":  "Повреждённый контрольный маяк",
		"shelter-05": "Защищённая точка диагностики",
		"clk-01":     "Эталонный контрольный маяк",
		"data-02":    "Локальный буферный модуль",
	}
	got := map[string]Point{}
	for _, p := range l.Points {
		if _, ok := want[p.ID]; ok {
			got[p.ID] = p
			if p.Hidden || p.Reward != 0 {
				t.Fatalf("сюжетная точка не должна быть скрытой или платной: %+v", p)
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("в Красном Шторме нет всех сюжетных точек: %v", got)
	}
	for id, name := range want {
		if got[id].Name != name {
			t.Fatalf("%s: %q", id, got[id].Name)
		}
	}
	home := Generate(aurora, cfg)
	if home.Stations[0].Name != "Станция «Рубеж»" {
		t.Fatalf("Аврора потеряла свою станцию: %s", home.Stations[0].Name)
	}
	auroraWant := map[string]string{
		"aur-beacon": "Контрольный учебный маяк",
		"aur-az01":   "Порог учебного коридора",
		"aur-az02":   "Сектор слабых помех",
		"aur-az03":   "Поисковый сектор",
		"aur-az04":   "Повреждённый буфер рейса",
		"aur-az05":   "Безопасный выход",
	}
	gotAurora := map[string]Point{}
	for _, p := range home.Points {
		if _, ok := auroraWant[p.ID]; ok {
			gotAurora[p.ID] = p
			if p.Hidden || p.Reward != 0 {
				t.Fatalf("учебная точка Авроры скрыта или платная: %+v", p)
			}
		}
	}
	if len(gotAurora) != len(auroraWant) {
		t.Fatalf("в Авроре нет всех точек главы 1: %v", gotAurora)
	}
	for id, name := range auroraWant {
		if gotAurora[id].Name != name {
			t.Fatalf("%s: %q", id, gotAurora[id].Name)
		}
	}
}

func TestMeridianStoryLocations(t *testing.T) {
	cfg := DefaultConfig()
	var meridian SystemParams
	for _, p := range cfg.Systems {
		if p.ID == "meridian" {
			meridian = p
		}
	}
	if meridian.ID == "" {
		t.Fatal("нет Меридиана")
	}
	l := Generate(meridian, cfg)
	if errs := l.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
	if l.Stations[0].Name != "Станция «Меридиан»" || l.Stations[0].ID != "st1" {
		t.Fatalf("хаб Меридиана: %+v", l.Stations[0])
	}
	want := map[string]string{
		"archive":        "Нейтральный архив",
		"relay-02":       "Периферийный ретранслятор",
		"protocol-index": "Указатель испытаний протокола",
	}
	got := map[string]bool{}
	for _, p := range l.Points {
		if name, ok := want[p.ID]; ok {
			if p.Name != name || p.Hidden {
				t.Fatalf("точка Меридиана: %+v", p)
			}
			got[p.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("не все точки Меридиана: %v", got)
	}
}

func TestAuroraDistricts(t *testing.T) {
	cfg := DefaultConfig()
	var aurora, meridian SystemParams
	for _, p := range cfg.Systems {
		switch p.ID {
		case "aurora":
			aurora = p
		case "meridian":
			meridian = p
		}
	}
	if aurora.ID == "" || meridian.ID == "" {
		t.Fatal("нет Авроры или Меридиана")
	}
	l := Generate(aurora, cfg)
	if !reflect.DeepEqual(l, Generate(aurora, cfg)) {
		t.Fatal("раскладка Авроры зависит от повторного вызова")
	}
	if errs := l.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
	if l.Radius != 1400 || l.Hub != ([2]float64{-12, 14}) || l.Stations[0].ID != "st1" {
		t.Fatalf("хаб или радиус сдвинуты: r=%v hub=%v st=%+v", l.Radius, l.Hub, l.Stations[0])
	}
	want := []string{"aur-beacon", "aur-az01", "aur-az02", "aur-az03", "aur-az04", "aur-az05"}
	byID := map[string]Point{}
	for _, p := range l.Points {
		byID[p.ID] = p
	}
	for _, id := range want {
		p, ok := byID[id]
		if !ok {
			t.Fatalf("нет сюжетной точки %s", id)
		}
		d := hypot(p.Pos[0]-l.Hub[0], p.Pos[1]-l.Hub[1])
		if d < HubFreeRadius || d > 480 {
			t.Fatalf("%s не в безопасном центре: %.0f", id, d)
		}
	}
	north, east, south, west := false, false, false, false
	for _, b := range l.Bodies {
		if b.Kind == "planet" && b.Pos[1] > l.Hub[1]+300 {
			north = true
		}
	}
	for _, z := range l.Zones {
		dx := z.Pos[0] - l.Hub[0]
		dy := z.Pos[1] - l.Hub[1]
		switch {
		case z.Type == "mining" && len(z.ID) >= 6 && z.ID[:6] == "z-belt" && dx > 120:
			east = true
		case z.Type == "mining" && len(z.ID) >= 8 && z.ID[:8] == "z-debris" && dx < -250:
			west = true
		case z.Type == "pve":
			if dy < -300 {
				south = true
			}
			if hypot(dx, dy) < 400 {
				t.Fatalf("опасная зона %s близко к хабу: %.0f", z.ID, hypot(dx, dy))
			}
		}
	}
	if !west {
		for _, p := range l.Points {
			if p.Kind == "derelict" && p.Pos[0] < l.Hub[0]-250 {
				west = true
			}
		}
	}
	for _, s := range l.Spawns {
		if d := hypot(s.Pos[0]-l.Hub[0], s.Pos[1]-l.Hub[1]); d < 280 {
			t.Fatalf("враг %s близко к хабу: %.0f", s.ID, d)
		}
	}
	if !north || !east || !south || !west {
		t.Fatalf("районы пусты: север %v восток %v юг %v запад %v", north, east, south, west)
	}
	if len(l.Regions) != 6 {
		t.Fatalf("ожидались 6 районов, получено %d", len(l.Regions))
	}
	roles := map[string]bool{}
	for _, r := range l.Regions {
		roles[r.Role] = true
		if hypot(r.Pos[0], r.Pos[1])+r.Radius > l.Radius*1.05 && r.Role != "outer" {
			t.Fatalf("район %s вылезает за систему", r.ID)
		}
	}
	for _, role := range []string{"safe", "explore", "industry", "danger", "ruins", "outer"} {
		if !roles[role] {
			t.Fatalf("нет роли %s", role)
		}
	}
	other := Generate(meridian, cfg)
	if len(other.Regions) != 0 {
		t.Fatal("районы Авроры не должны появляться в Меридиане")
	}
	if errs := other.Validate(); len(errs) > 0 {
		t.Fatal(errs)
	}
}
