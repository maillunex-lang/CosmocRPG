package world_test

import (
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

// В отличие от прежней проверки со вручную вписанными 5 единицами,
// добываем груз обычным стартовым кораблём из реальной раскладки Меридиана.
func TestStarterCanMineMandatoryDelivery(t *testing.T) {
	goal := 0
	for _, q := range quest.Catalog() {
		if q.ID == "meridian_delivery" {
			goal = q.Goal
		}
	}
	if goal <= 0 {
		t.Fatal("обязательная доставка отсутствует")
	}
	cfg := worldgen.DefaultConfig()
	var system worldgen.SystemParams
	for _, s := range cfg.Systems {
		if s.ID == "meridian" {
			system = s
		}
	}
	if system.ID == "" {
		t.Fatal("нет Меридиана")
	}
	layout := worldgen.Generate(system, cfg)
	var deposit worldgen.Deposit
	for _, d := range layout.Deposits {
		if d.Resource == "iron_ore" && d.Amount >= float64(goal) {
			deposit = d
			break
		}
	}
	if deposit.ID == "" {
		t.Fatal("в системе задания нет достаточного источника железной руды")
	}
	w := world.New()
	if err := w.ApplyLayout(layout, cfg); err != nil {
		t.Fatal(err)
	}
	_, ok := w.Join(world.Record{ID: "delivery-starter", Name: "Пилот", Created: true, ShipType: world.DefaultShip, PilotSpec: "interceptor", Pos: world.Vec{X: deposit.Pos[0], Y: deposit.Pos[1]}})
	if !ok {
		t.Fatal("стартовый корабль не вошёл")
	}
	before, _ := w.Snapshot("delivery-starter")
	oreCap := 0.0
	for _, b := range before.Ship.Bays {
		if b.ID == "ore" {
			oreCap = b.Cap
		}
	}
	if oreCap < float64(goal) || oreCap > before.Ship.Cargo {
		t.Fatalf("доставка %d не помещается: руда %.1f, общий %.1f", goal, oreCap, before.Ship.Cargo)
	}
	if err := w.StartMining("delivery-starter", deposit.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 1600; i++ {
		w.Step(0.05, now.Add(time.Duration(i)*50*time.Millisecond))
	}
	after, _ := w.Snapshot("delivery-starter")
	if after.Ship.Hold["iron_ore"] < float64(goal) || after.Ship.CargoUsed > after.Ship.Cargo {
		t.Fatalf("невыполнимая добыча: %+v", after.Ship)
	}
	if after.Ship.Hold["iron_ore"] > float64(goal)+0.01 {
		t.Fatalf("рудный лимит не соблюдён: %.1f", after.Ship.Hold["iron_ore"])
	}
}

func TestStarterDeliveryKeepsSharedCargoLimit(t *testing.T) {
	w := world.New()
	cfg := worldgen.DefaultConfig()
	layout := worldgen.Generate(cfg.Systems[0], cfg)
	if err := w.ApplyLayout(layout, cfg); err != nil {
		t.Fatal(err)
	}
	_, ok := w.Join(world.Record{ID: "mixed", Name: "Пилот", Created: true, ShipType: "FTR-001", Pos: world.Vec{X: layout.Hub[0], Y: layout.Hub[1]}})
	if !ok {
		t.Fatal("join")
	}
	if err := w.AddCargo("mixed", "iron_ore", 2); err != nil {
		t.Fatal(err)
	}
	if err := w.AddCargo("mixed", "scrap", 4); err != nil {
		t.Fatal(err)
	}
	if err := w.AddCargo("mixed", "scrap", 1); err != world.ErrHoldFull {
		t.Fatalf("общий трюм переполнен: %v", err)
	}
}
