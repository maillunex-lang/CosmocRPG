package world

import (
	"reflect"
	"testing"
	"time"
)

func TestStrizhStarterInstanceIsBoundAndStable(t *testing.T) {
	w := New()
	p := newPilot(t, w, "owner")
	id := p.starterEngine.InstanceID
	if err := p.boostAccess(); err != nil {
		t.Fatal(err)
	}
	other := newPilot(t, w, "other")
	if id == other.starterEngine.InstanceID {
		t.Fatal("two owners share instance identity")
	}
	rec, ok := w.Remove(p.ID)
	if !ok {
		t.Fatal("remove")
	}
	p, _ = w.Join(rec)
	if p.starterEngine.InstanceID != id || len(p.moduleList()) != 1 {
		t.Fatal("rejoin created a different or duplicate starter")
	}
	p.ShipType = "TMP-MINE"
	p.initStarterEngine()
	if p.starterEngine != nil || len(p.moduleList()) != 0 {
		t.Fatal("starter leaked onto another hull")
	}
	p.ShipType = DefaultShip
	p.initStarterEngine()
	if p.starterEngine.InstanceID != id {
		t.Fatal("hull roundtrip created another starter identity")
	}
}

func TestM01PreservesActiveBoostAndSharesLegacyCooldown(t *testing.T) {
	makeWorld := func() (*World, *Player) {
		w := New()
		c := w.Catalog()
		sp := c.Ships[DefaultShip]
		ab := sp.Abilities["boost"]
		ab.Energy = 27
		ab.Cooldown = 17
		ab.Duration = 4
		ab.SpeedMul = 2.3
		sp.Abilities["boost"] = ab
		c.Ships[DefaultShip] = sp
		w.SetCatalog(c)
		return w, newPilot(t, w, "owner")
	}
	legacy, p := makeWorld()
	modern, q := makeWorld()
	if err := legacy.UseAbility(p.ID, "boost"); err != nil {
		t.Fatal(err)
	}
	if err := modern.UseAbility(q.ID, StrizhBoostAbility); err != nil {
		t.Fatal(err)
	}
	if p.Energy != 73 || q.Energy != p.Energy || p.boostUntil != 4 || q.boostUntil != p.boostUntil || !reflect.DeepEqual(p.cooldowns, q.cooldowns) {
		t.Fatal("alias changed legacy resource/timeout behavior")
	}
	if modern.speedFor(q) != 2.3 || legacy.speedFor(p) != modern.speedFor(q) {
		t.Fatal("speed multiplier drift")
	}
	if !reflect.DeepEqual(legacy.DrainEvents(), modern.DrainEvents()) {
		t.Fatal("alias changed legacy events")
	}
	for _, id := range []string{"boost", StrizhBoostAbility} {
		if err := modern.UseAbility(q.ID, id); err != ErrCooldown {
			t.Fatalf("%s bypassed shared cooldown: %v", id, err)
		}
	}
	if q.Energy != 73 || len(q.cooldowns) != 1 || len(modern.DrainEvents()) != 0 {
		t.Fatal("rejected alias spent resources or added another cooldown")
	}
	modern.Step(18, time.Now())
	if err := modern.UseAbility(q.ID, "boost"); err != nil {
		t.Fatalf("cooldown did not expire: %v", err)
	}
	if err := modern.UseAbility(q.ID, StrizhBoostAbility); err != ErrCooldown {
		t.Fatalf("reverse alias bypass: %v", err)
	}
}

func TestStarterEngineRejectsInvalidEntitlementAtomically(t *testing.T) {
	cases := []struct {
		name   string
		damage func(*Player)
		want   error
	}{
		{"missing", func(p *Player) { p.starterEngine = nil }, ErrModuleUnavailable},
		{"wrong owner", func(p *Player) { p.starterEngine.OwnerID = "other" }, ErrModuleUnavailable},
		{"wrong instance", func(p *Player) { p.starterEngine.InstanceID = "invented" }, ErrModuleUnavailable},
		{"wrong prototype", func(p *Player) { p.starterEngine.PrototypeID = "ELE-S-START" }, ErrModuleUnavailable},
		{"wrong slot", func(p *Player) { p.starterEngine.SlotID = "weapon-1" }, ErrModuleUnavailable},
		{"unequipped", func(p *Player) { p.starterEngine.Equipped = false }, ErrModuleUnavailable},
		{"unlearned", func(p *Player) { p.boostRank = 0 }, ErrAbilityNotLearned},
		{"invalid rank", func(p *Player) { p.boostRank = 6 }, ErrAbilityNotLearned},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := New()
			p := newPilot(t, w, "owner")
			tc.damage(p)
			energy := p.Energy
			position := p.Pos
			for _, id := range []string{StrizhBoostAbility, "boost"} {
				if err := w.UseAbility(p.ID, id); err != tc.want {
					t.Fatalf("%s: %v, want %v", id, err, tc.want)
				}
			}
			if p.Energy != energy || p.Pos != position || len(p.cooldowns) != 0 || p.boostUntil != 0 || len(w.DrainEvents()) != 0 {
				t.Fatal("rejection changed combat state")
			}
			snapshot, _ := w.Snapshot(p.ID)
			ab := snapshot.Ship.Abilities[0]
			if ab.Enabled || ab.UnavailableReason != tc.want.Error() || ab.CatalogID != StrizhBoostAbility {
				t.Fatalf("incorrect availability: %+v", ab)
			}
		})
	}
}

func TestM01StillChecksDeathEnergyAndOtherHulls(t *testing.T) {
	w := New()
	p := newPilot(t, w, "owner")
	p.deadUntil = 10
	if err := w.UseAbility(p.ID, StrizhBoostAbility); err != ErrDead {
		t.Fatalf("death: %v", err)
	}
	p.deadUntil = 0
	p.Energy = 0
	if err := w.UseAbility(p.ID, StrizhBoostAbility); err != ErrEnergy {
		t.Fatalf("energy: %v", err)
	}
	if len(p.cooldowns) != 0 || len(w.DrainEvents()) != 0 {
		t.Fatal("failed cast mutated state")
	}
	for _, ship := range []string{"TMP-MINE", "FTR-002"} {
		q, _ := w.Join(Record{ID: ship, Name: ship, ShipType: ship})
		if err := w.UseAbility(q.ID, StrizhBoostAbility); err != ErrUnknown {
			t.Fatalf("M01 enabled on %s: %v", ship, err)
		}
		if err := w.UseAbility(q.ID, "boost"); err != nil {
			t.Fatalf("legacy boost broken on %s: %v", ship, err)
		}
	}
}
