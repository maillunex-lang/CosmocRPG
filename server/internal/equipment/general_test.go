package equipment

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type generalFixture struct {
	store   GeneralStore
	pool    *pgxpool.Pool
	owner   string
	legacy  StarterState
	catalog *fitting.Catalog
}

func generalTestFixture(t *testing.T, backend string) generalFixture {
	t.Helper()
	ctx := context.Background()
	c, err := fitting.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	owner := world.PublicID(world.TokenHash("general-equipment-owner-0001"))
	progress := fitting.Progress{PilotXP: rpg.DefaultPilotCurve.TotalXP(10), Credits: 50000, Attributes: map[string]int{}}
	xp := rpg.DefaultShipCurve.TotalXP(10)
	f := generalFixture{owner: owner, catalog: c}
	if backend == "memory" {
		s := NewGeneralMemStore(c, map[string]GeneralAccount{owner: {Progress: progress, ShipXP: map[string]int64{"FTR-001": xp, "COR-001": xp}}})
		f.legacy = initial(owner)
		f.legacy.Equipped = false
		f.legacy.Revision = 1
		if err := s.SeedLegacyStarter(f.legacy); err != nil {
			t.Fatal(err)
		}
		f.store = s
		return f
	}
	f.pool = testdb.Pool(t)
	if _, err := db.Migrate(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	if _, err := world.NewPGStore(f.pool).Resolve(ctx, "general-equipment-owner-0001", "General equipment"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE players SET pilot_xp=$2,credits=$3 WHERE public_id=$1`, owner, progress.PilotXP, progress.Credits); err != nil {
		t.Fatal(err)
	}
	for _, hull := range []string{"FTR-001", "COR-001"} {
		if _, err := f.pool.Exec(ctx, `INSERT INTO player_ships(player_id,ship_type,ship_xp) VALUES($1,$2,$3)`, owner, hull, xp); err != nil {
			t.Fatal(err)
		}
	}
	legacy := NewPGStore(f.pool)
	s, err := legacy.Ensure(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	f.legacy, err = legacy.SetEngine(ctx, owner, s.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	f.store = NewGeneralPGStore(f.pool, c)
	return f
}
func mustGeneralLoad(t *testing.T, f generalFixture, hull string) fitting.State {
	t.Helper()
	s, err := f.store.Load(context.Background(), f.owner, hull)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func requireGeneralSame(t *testing.T, f generalFixture, want fitting.State) {
	t.Helper()
	got := mustGeneralLoad(t, f, want.Hull)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state changed unexpectedly\ngot=%+v\nwant=%+v", got, want)
	}
}

func TestGeneralStoreMigrationOwnershipAndTransfer(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := generalTestFixture(t, backend)
			ctx := context.Background()
			if _, err := f.store.Load(ctx, f.owner, "FTR-001"); !errors.Is(err, fitting.ErrNotInitialized) {
				t.Fatal(err)
			}
			s, err := f.store.Ensure(ctx, f.owner, "FTR-001", "interceptor")
			if err != nil {
				t.Fatal(err)
			}
			if s.Revision != 2 || len(s.Items) != 5 || len(s.Slots) != 4 || s.Slots["tech-1"] != "" || s.Pilot.Learned["M01"] != 1 || s.Items[f.legacy.InstanceID].ID != f.legacy.InstanceID || s.Progress.Credits != 50000 || s.Progress.PilotXP != rpg.DefaultPilotCurve.TotalXP(10) {
				t.Fatalf("legacy bootstrap changed data: %+v", s)
			}
			again, err := f.store.Ensure(ctx, f.owner, "FTR-001", "scout")
			if err != nil || !reflect.DeepEqual(s, again) {
				t.Fatalf("bootstrap reset existing profile: %v", err)
			}
			if len(s.Profile.Builds) != 3 || len(s.Profile.Builds[0].Active) != 4 {
				t.Fatal("missing three builds/four slots")
			}
			copy := s.Clone()
			i := copy.Items[f.legacy.InstanceID]
			i.Owner = "thief"
			copy.Items[i.ID] = i
			requireGeneralSame(t, f, s)
			noop, err := f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(*fitting.State) error { return nil })
			if err != nil || !reflect.DeepEqual(noop, s) {
				t.Fatal("no-op changed revision")
			}
			before := s.Clone()
			if _, err := f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error { s.Progress.Credits -= 100; s.Slots["weapon-2"] = "stolen"; return nil }); err == nil {
				t.Fatal("missing/foreign item accepted")
			}
			requireGeneralSame(t, f, before)
			s, err = f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error { delete(s.Slots, "weapon-1"); return nil })
			if err != nil {
				t.Fatal(err)
			}
			old := before
			if _, err := f.store.Update(ctx, f.owner, old.Hull, old.Revision, old.Pilot.Revision, func(*fitting.State) error { t.Fatal("stale callback executed"); return nil }); !errors.Is(err, fitting.ErrConflict) {
				t.Fatal(err)
			}
			spec := s.Profile.Builds[0].Specialization
			granted, next, err := f.store.GrantSpecializationXP(ctx, f.owner, s.Hull, spec, "trusted:kill:1", 1000)
			if err != nil || !granted || next.Profile.SpecializationXP[spec] != 1000 {
				t.Fatalf("grant: %v %v", granted, err)
			}
			s = next
			granted, next, err = f.store.GrantSpecializationXP(ctx, f.owner, s.Hull, spec, "trusted:kill:1", 1000)
			if err != nil || granted || !reflect.DeepEqual(next, s) {
				t.Fatal("duplicate XP grant")
			}
			if _, _, err := f.store.GrantSpecializationXP(ctx, f.owner, s.Hull, spec, "trusted:kill:1", 1001); !errors.Is(err, fitting.ErrInvalidState) {
				t.Fatal("changed grant replay accepted")
			}
			const purchased = "purchased-weapon"
			s, err = f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error {
				s.Items[purchased] = fitting.Item{ID: purchased, Owner: s.Owner, Prototype: "LAS-S-WEAPON-00-COMMON"}
				s.Slots["weapon-2"] = purchased
				s.Progress.Credits -= 250
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if s.Items[purchased].Hull != "" || s.Items[purchased].EquippedHull != "FTR-001" || s.Progress.Credits != 49750 {
				t.Fatal("purchase/assignment/currency mismatch")
			}
			s, err = f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error {
				i := s.Items[purchased]
				i.Engineering = 1
				s.Items[purchased] = i
				s.Progress.Credits -= 100
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			cor, err := f.store.Ensure(ctx, f.owner, "COR-001", "hunter")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.Update(ctx, f.owner, cor.Hull, cor.Revision, cor.Pilot.Revision, func(s *fitting.State) error { s.Slots["weapon-2"] = purchased; s.Progress.Credits -= 1; return nil }); err == nil {
				t.Fatal("one item fitted on two hulls")
			}
			requireGeneralSame(t, f, cor)
			s = mustGeneralLoad(t, f, "FTR-001")
			s, err = f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error { delete(s.Slots, "weapon-2"); return nil })
			if err != nil {
				t.Fatal(err)
			}
			cor = mustGeneralLoad(t, f, "COR-001")
			cor, err = f.store.Update(ctx, f.owner, cor.Hull, cor.Revision, cor.Pilot.Revision, func(s *fitting.State) error { s.Slots["weapon-2"] = purchased; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if cor.Items[purchased].EquippedHull != "COR-001" || cor.Progress.Credits != 49650 {
				t.Fatal("transfer lost item/credits")
			}
			if cor.Items[f.legacy.InstanceID].Hull != "FTR-001" {
				t.Fatal("native engine lost binding")
			}
			if _, err := f.store.Update(ctx, f.owner, cor.Hull, cor.Revision, cor.Pilot.Revision, func(s *fitting.State) error { s.Slots["tech-1"] = f.legacy.InstanceID; return nil }); err == nil {
				t.Fatal("bound stock crossed hull")
			}
		})
	}
}

func TestGeneralStoreCASRollbackAndCancellation(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := generalTestFixture(t, backend)
			ctx := context.Background()
			s, err := f.store.Ensure(ctx, f.owner, "FTR-001", "interceptor")
			if err != nil {
				t.Fatal(err)
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err := f.store.Update(cancelled, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error { s.Progress.Credits--; return nil }); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			requireGeneralSame(t, f, s)
			injected := errors.New("injected validation error")
			if out, err := f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error {
				s.Progress.Credits--
				s.Profile.Builds[0].Name = "Discarded"
				return injected
			}); !errors.Is(err, injected) || !reflect.DeepEqual(out, fitting.State{}) {
				t.Fatal("failed operation exposed partial state")
			}
			requireGeneralSame(t, f, s)
			var winners atomic.Int32
			var wg sync.WaitGroup
			errs := make(chan error, 16)
			for i := 0; i < 16; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error { s.Progress.Credits--; s.Profile.Builds[0].Name = "Winner"; return nil })
					if err == nil {
						winners.Add(1)
					} else if !errors.Is(err, fitting.ErrConflict) {
						errs <- err
					}
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			if winners.Load() != 1 {
				t.Fatalf("CAS winners=%d", winners.Load())
			}
			got := mustGeneralLoad(t, f, s.Hull)
			if got.Progress.Credits != s.Progress.Credits-1 || got.Revision != s.Revision+1 {
				t.Fatal("CAS charged twice")
			}
		})
	}
}

func TestGeneralPGAtomicRollbackAndLegacyXP(t *testing.T) {
	f := generalTestFixture(t, "postgres")
	ctx := context.Background()
	s, err := f.store.Ensure(ctx, f.owner, "FTR-001", "interceptor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `CREATE FUNCTION fail_general_revision() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected after wallet/profile/slot updates'; END; $$; CREATE TRIGGER fail_general_revision BEFORE UPDATE ON player_ship_loadouts FOR EACH ROW EXECUTE FUNCTION fail_general_revision()`); err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error {
		s.Progress.Credits -= 1000
		delete(s.Slots, "weapon-1")
		s.Profile.Builds[0].Name = "Broken"
		s.Items["new-item"] = fitting.Item{ID: "new-item", Owner: s.Owner, Prototype: "LAS-S-WEAPON-00-COMMON"}
		return nil
	})
	if err == nil {
		t.Fatal("SQL failure succeeded")
	}
	requireGeneralSame(t, f, s)
	var pilotXP, shipXP int64
	if err := f.pool.QueryRow(ctx, `SELECT p.pilot_xp,s.ship_xp FROM players p JOIN player_ships s ON s.player_id=p.public_id WHERE p.public_id=$1 AND s.ship_type='FTR-001'`, f.owner).Scan(&pilotXP, &shipXP); err != nil {
		t.Fatal(err)
	}
	if pilotXP != s.Progress.PilotXP || shipXP != s.Progress.ShipXP {
		t.Fatal("old XP overwritten")
	}
}

func TestGeneralStoreSharedPilotAndTrustedXP(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := generalTestFixture(t, backend)
			ctx := context.Background()
			s, err := f.store.Ensure(ctx, f.owner, "FTR-001", "")
			if err != nil {
				t.Fatal(err)
			}
			cor, err := f.store.Ensure(ctx, f.owner, "COR-001", "")
			if err != nil {
				t.Fatal(err)
			}
			s = mustGeneralLoad(t, f, s.Hull)
			_, err = f.store.Update(ctx, f.owner, s.Hull, s.Revision, s.Pilot.Revision, func(s *fitting.State) error { s.Pilot.Research["LAS"] = 1; s.Progress.Credits -= 500; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.Update(ctx, f.owner, cor.Hull, cor.Revision, cor.Pilot.Revision, func(*fitting.State) error { t.Fatal("stale global learning accepted"); return nil }); !errors.Is(err, fitting.ErrConflict) {
				t.Fatal(err)
			}
			cor = mustGeneralLoad(t, f, cor.Hull)
			if cor.Pilot.Research["LAS"] != 1 || cor.Progress.Credits != 49500 {
				t.Fatal("fresh hull duplicated global pilot progression/wallet")
			}
			if _, err := f.store.Update(ctx, f.owner, cor.Hull, cor.Revision, cor.Pilot.Revision, func(s *fitting.State) error {
				s.Profile.SpecializationXP[s.Profile.Builds[0].Specialization] = 100
				return nil
			}); !errors.Is(err, fitting.ErrInvalidState) {
				t.Fatal("generic update bypassed XP receipt ledger")
			}
			var winners atomic.Int32
			var wg sync.WaitGroup
			errs := make(chan error, 16)
			for i := 0; i < 16; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					granted, _, err := f.store.GrantSpecializationXP(ctx, f.owner, cor.Hull, cor.Profile.Builds[0].Specialization, "server-event-1", 1e9)
					if err != nil {
						errs <- err
					}
					if granted {
						winners.Add(1)
					}
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			if winners.Load() != 1 {
				t.Fatal("duplicate specialization grant")
			}
			got := mustGeneralLoad(t, f, cor.Hull)
			if got.Profile.SpecializationXP[cor.Profile.Builds[0].Specialization] != fitting.DefaultSpecializationCurve.TotalXP(20) {
				t.Fatal("specialization XP not capped at20")
			}
		})
	}
}
