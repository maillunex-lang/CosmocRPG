package equipment

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
)

func TestGeneralServicePricingResearchAndEngineering(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := generalTestFixture(t, backend)
			ctx := context.Background()
			service := NewService(f.store, f.catalog)
			s, err := service.Ensure(ctx, f.owner, "FTR-001", "interceptor")
			if err != nil {
				t.Fatal(err)
			}
			run := func(cmd Command) error {
				next, err := service.Execute(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, cmd, nil)
				if err == nil {
					s = next
				}
				return err
			}
			for _, cmd := range []Command{{Op: "unknown"}, {Op: "buy", Prototype: "ENG-S-START"}, {Op: "buy", Prototype: "LAS-S-WEAPON-00-RARE"}, {Op: "buy", Prototype: "LAS-S-WEAPON-00-LEGENDARY"}, {Op: "buy", Prototype: "LAS-S-WEAPON-00-COMMON", Item: "spoof"}, {Op: "equip", Slot: "weapon-2", Item: "other-owner-item"}} {
				before := s.Clone()
				if err := run(cmd); err == nil {
					t.Fatalf("bad operation accepted: %+v", cmd)
				}
				requireGeneralSame(t, f, before)
			}
			if err := run(Command{Op: "research", Family: "LAS"}); err != nil {
				t.Fatal(err)
			}
			if s.Progress.Credits != 49500 || s.Pilot.Research["LAS"] != 1 {
				t.Fatal("research not charged atomically")
			}
			before := s.Clone()
			if err := run(Command{Op: "research", Family: "LAS"}); err == nil {
				t.Fatal("research tier bypassed pilot gate")
			}
			requireGeneralSame(t, f, before)
			if err := run(Command{Op: "buy", Prototype: "LAS-S-WEAPON-00-RARE"}); err != nil {
				t.Fatal(err)
			}
			if s.Progress.Credits != 48250 || len(s.Items) != 6 {
				t.Fatal("purchase price not server-owned")
			}
			var bought string
			for id, item := range s.Items {
				if item.Hull == "" {
					bought = id
				}
			}
			if bought == "" {
				t.Fatal("no purchased instance")
			}
			if err := run(Command{Op: "equip", Slot: "weapon-2", Item: bought}); err != nil {
				t.Fatal(err)
			}
			if s.Slots["weapon-2"] != bought || s.Items[bought].EquippedHull != "FTR-001" {
				t.Fatal("equip did not assign inventory instance")
			}
			if err := run(Command{Op: "engineer", Item: bought}); err != nil {
				t.Fatal(err)
			}
			if s.Items[bought].Engineering != 1 || s.Progress.Credits != 47625 {
				t.Fatalf("engineering price mismatch: %v", s.Progress.Credits)
			}
			if err := run(Command{Op: "engineer", Item: f.legacy.InstanceID}); err != nil {
				t.Fatal(err)
			}
			if err := run(Command{Op: "equip", Slot: "tech-1", Item: f.legacy.InstanceID}); err != nil {
				t.Fatal(err)
			}
			res, err := f.catalog.ValidateState(s)
			if err != nil {
				t.Fatal(err)
			}
			stats := rpg.Compute(rpg.DefaultConfig(), rpg.Input{Base: map[rpg.StatID]float64{rpg.StatSpeed: 20}, Modules: res.Modifiers})
			if math.Abs(stats.Final[rpg.StatSpeed]-20.1) > 1e-9 {
				t.Fatal("stock engineering has no actual improvement")
			}
			before = s.Clone()
			blocked := errors.New("server live cargo validation")
			out, err := service.Execute(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, Command{Op: "buy", Prototype: "LAS-S-WEAPON-00-COMMON"}, func(fitting.State) error { return blocked })
			if !errors.Is(err, blocked) || !reflect.DeepEqual(out, fitting.State{}) {
				t.Fatal("late authoritative validation did not roll back purchase")
			}
			requireGeneralSame(t, f, before)
			if err := run(Command{Op: "unequip", Slot: "tech-1"}); err != nil {
				t.Fatal(err)
			}
			if s.Pilot.Learned["M01"] != 1 || s.Slots["tech-1"] != "" {
				t.Fatal("removing module erased learned rank")
			}
			if err := run(Command{Op: "equip", Slot: "weapon-2", Item: f.legacy.InstanceID}); err == nil {
				t.Fatal("engine in weapon slot")
			}
		})
	}
}
