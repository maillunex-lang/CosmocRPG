package equipment

import (
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

func TestPublicViewSeparatesBagAndLocksEngine(t *testing.T) {
	c, err := fittingCatalog(t)
	if err != nil {
		t.Fatal(err)
	}
	h, ok := c.Hull("FTR-001")
	if !ok || len(h.Starter) < 2 {
		t.Fatal("нет стартового комплекта")
	}
	laser := h.Starter[0]
	engine := MountOf(h, "tech-1")
	state := newViewState(h.ID, laser, engine)
	state.Slots = map[string]string{engine.SlotID: "eng"}
	view := PublicView(c, state)
	if len(view.Bag) != 1 || view.Bag[0].ID != "laser" {
		t.Fatalf("снятый модуль должен лежать в сумке: %+v", view.Bag)
	}
	var locked bool
	for _, slot := range view.Slots {
		if slot.ID == "tech-1" {
			locked = slot.Locked && slot.Item != nil && slot.Item.ID == "eng"
		}
		if slot.ID == laser.SlotID && slot.Item != nil {
			t.Fatal("пустой слот показал модуль")
		}
	}
	if !locked {
		t.Fatal("двигатель Стрижа не закреплён за прежней кнопкой")
	}
}

func fittingCatalog(t *testing.T) (*fitting.Catalog, error) {
	t.Helper()
	return fitting.DefaultCatalog()
}

func newViewState(hull string, laser, engine fitting.Mount) fitting.State {
	return fitting.State{
		Owner: "pilot", Hull: hull, Revision: 3,
		Items: map[string]fitting.Item{
			"laser": {ID: "laser", Owner: "pilot", Prototype: laser.PrototypeID},
			"eng":   {ID: "eng", Owner: "pilot", Prototype: engine.PrototypeID},
		},
		Slots:    map[string]string{},
		Pilot:    fitting.NewPilotData(),
		Profile:  fitting.ShipData{Version: 1, SpecializationXP: map[string]int64{}, Builds: []fitting.Build{{Name: "Сборка 1", Slots: map[string]string{}, Active: []string{"", "", "", ""}, Talents: map[string]int{}, Choices: map[string]string{}}}},
		Progress: fitting.Progress{PilotXP: 0, ShipXP: 0, Credits: 1},
	}
}

func MountOf(h fitting.Hull, slot string) fitting.Mount {
	for _, m := range h.Starter {
		if m.SlotID == slot {
			return m
		}
	}
	return fitting.Mount{}
}
