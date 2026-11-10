package world

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

func aiWorld(t *testing.T) *World {
	t.Helper()
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	w := New()
	w.eventRNG = rand.New(rand.NewSource(1))
	if err := w.ApplyLayout(lay, cfg); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestSpawnDespawnAndCounts(t *testing.T) {
	w := aiWorld(t)
	w.Join(Record{ID: "p1", Name: "Игрок"})
	if !w.SpawnAI(AISpawn{ID: "cpt-0001", Name: "Валун", Behavior: AIMiner, ShipType: "TMP-MINE", Seed: 1}) {
		t.Fatal("ИИ не появился")
	}
	if w.SpawnAI(AISpawn{ID: "cpt-0001", Name: "Дубль"}) {
		t.Fatal("повторный идентификатор должен отклоняться")
	}
	if w.SpawnAI(AISpawn{ID: "p1", Name: "Клон игрока"}) {
		t.Fatal("идентификатор игрока занят")
	}
	if w.SpawnAI(AISpawn{ID: ""}) {
		t.Fatal("пустой идентификатор")
	}
	if w.AICount() != 1 || w.PlayerCount() != 1 {
		t.Fatalf("ИИ не должны считаться игроками: ИИ %d, игроков %d", w.AICount(), w.PlayerCount())
	}
	if ids := w.AIIDs(); len(ids) != 1 || ids[0] != "cpt-0001" {
		t.Fatalf("идентификаторы: %v", ids)
	}
	if !w.DespawnAI("cpt-0001") || w.DespawnAI("cpt-0001") || w.AICount() != 0 {
		t.Fatal("удаление ИИ")
	}
	if !w.SpawnAI(AISpawn{ID: "cpt-0002", ShipType: "НЕТ-ТАКОГО"}) {
		t.Fatal("неизвестный корабль заменяется кораблём по умолчанию")
	}
	if typ := w.ais[0].ShipType; typ != DefaultShip {
		t.Fatalf("корабль по умолчанию: %s", typ)
	}
}

func TestAIMovesOnRoutesAndStaysInside(t *testing.T) {
	w := aiWorld(t)
	for i, b := range []string{AIMiner, AITrader, AIScout, AIPatrol, AISupport} {
		w.SpawnAI(AISpawn{ID: fmt.Sprintf("ai-%d", i), Name: b, Behavior: b, ShipType: "FTR-001", Seed: uint32(i + 1)})
	}
	start := map[string]Vec{}
	for _, a := range w.ais {
		start[a.ID] = a.Pos
	}
	moved := map[string]float64{}
	for i := 0; i < 4000; i++ { // 200 с игрового времени
		w.Step(0.05, time.Now())
		for _, a := range w.ais {
			if a.Pos.length() > w.radius+1e-6 || math.IsNaN(a.Pos.X) {
				t.Fatalf("%s вышел за пределы системы: %+v", a.ID, a.Pos)
			}
			if d := dist(a.Pos, start[a.ID]); d > moved[a.ID] {
				moved[a.ID] = d
			}
		}
	}
	for id, d := range moved {
		if d < 5 {
			t.Errorf("%s почти не двигался (макс. смещение %.1f)", id, d)
		}
	}
}

func TestMinerVisitsDepositsAndStation(t *testing.T) {
	w := aiWorld(t)
	w.SpawnAI(AISpawn{ID: "m1", Behavior: AIMiner, ShipType: "TMP-MINE", Seed: 7})
	a := w.aiByID["m1"]
	nearDep, nearStation := false, false
	for i := 0; i < 20000 && !(nearDep && nearStation); i++ { // до 1000 с
		w.Step(0.05, time.Now())
		for _, d := range w.deposits {
			if dist(a.Pos, d.Pos) < 8 {
				nearDep = true
			}
		}
		for _, s := range w.aiStations() {
			if dist(a.Pos, s) < 20 && a.state == aiWork && a.phase > 0 {
				nearStation = true
			}
		}
	}
	if !nearDep || !nearStation {
		t.Fatalf("шахтёр должен бывать у месторождений (%v) и у станции (%v)", nearDep, nearStation)
	}
}

func TestAIVisibilityRadiusAndTargeting(t *testing.T) {
	w := aiWorld(t)
	w.Join(Record{ID: "p1", Name: "Игрок", Pos: Vec{0, 0}})
	w.SpawnAI(AISpawn{ID: "near", Name: "Рядом", Seed: 1})
	w.SpawnAI(AISpawn{ID: "far", Name: "Далеко", Seed: 2})
	w.mu.Lock()
	w.aiByID["near"].Pos = Vec{100, 0}
	w.aiByID["far"].Pos = Vec{AIViewRadius + 50, 0}
	w.mu.Unlock()
	st, _ := w.Snapshot("p1")
	var near, far *Object
	for i := range st.Objects {
		switch st.Objects[i].ID {
		case "near":
			near = &st.Objects[i]
		case "far":
			far = &st.Objects[i]
		}
	}
	if near == nil || !near.AI || near.Kind != "ai" || near.Name != "Рядом" || near.Ship == "" {
		t.Fatalf("ближний ИИ в состоянии: %+v", near)
	}
	if far == nil || !far.AI {
		t.Fatal("пилот той же системы должен оставаться на карте, даже далеко от игрока")
	}
	// ИИ можно выбрать целью и лететь к нему, но атаковать нельзя
	if !w.SetTarget("p1", "near") {
		t.Fatal("ИИ должен выбираться целью")
	}
	if err := w.SetAttack("p1", true); err == nil {
		t.Fatal("атаковать ИИ-капитана нельзя (PvP выключен)")
	}
	if !w.SetAutopilot("p1", true) {
		t.Fatal("автопилот к ИИ")
	}
}

func TestAIDeterministic(t *testing.T) {
	run := func() []Vec {
		w := aiWorld(t)
		for i := 0; i < 5; i++ {
			w.SpawnAI(AISpawn{ID: fmt.Sprintf("d%d", i), Behavior: []string{AIMiner, AIScout, AIPatrol, AITrader, AISupport}[i], ShipType: "SCT-001", Seed: uint32(100 + i)})
		}
		for i := 0; i < 1500; i++ {
			w.Step(0.05, time.Now())
		}
		var out []Vec
		for _, a := range w.ais {
			out = append(out, a.Pos)
		}
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("одинаковое зерно дало разные маршруты: %v и %v", a[i], b[i])
		}
	}
}

func TestLocalPilotsFillEveryZone(t *testing.T) {
	w := aiWorld(t)
	w.mu.Lock()
	zones := append([]worldgen.Zone(nil), w.layout.Zones...)
	pilots := append([]*aiShip(nil), w.ais...)
	w.mu.Unlock()
	if len(zones) == 0 {
		t.Fatal("нет зон")
	}
	for _, z := range zones {
		n := 0
		var level int
		for _, a := range pilots {
			if !strings.HasPrefix(a.ID, "loc-") {
				continue
			}
			if dist(a.home, Vec{z.Pos[0], z.Pos[1]}) < 1 {
				n++
				level = a.level
			}
		}
		if n < 2 {
			t.Fatalf("в зоне %s пилотов %d", z.Name, n)
		}
		want := 1 + z.Danger*3 + 4 // Аврора: сложность 2
		if level != want {
			t.Fatalf("зона %s опасность %d: уровень %d, ожидался %d", z.Name, z.Danger, level, want)
		}
	}
	for i := 0; i < 400; i++ {
		w.Step(0.05, time.Now())
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, a := range w.ais {
		if !strings.HasPrefix(a.ID, "loc-") {
			continue
		}
		if dist(a.Pos, a.home) > a.homeR+1 {
			t.Fatalf("%s ушёл из своей зоны: %.0f > %.0f", a.Name, dist(a.Pos, a.home), a.homeR)
		}
	}
}

func TestAIDoesNotAffectEconomy(t *testing.T) {
	w := aiWorld(t)
	before := map[string]float64{}
	for _, d := range w.deposits {
		before[d.ID] = d.Amount
	}
	w.SpawnAI(AISpawn{ID: "m1", Behavior: AIMiner, ShipType: "TMP-MINE", Seed: 3})
	for i := 0; i < 6000; i++ {
		w.Step(0.05, time.Now())
	}
	for _, d := range w.deposits {
		if d.Amount < before[d.ID]-1e-9 {
			t.Fatalf("ИИ не должен расходовать месторождения: %s %.1f → %.1f", d.ID, before[d.ID], d.Amount)
		}
	}
	if len(w.DrainRewards()) != 0 {
		t.Fatal("ИИ не получают наград")
	}
}
