package equipment

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

type fixture struct {
	store   StarterStore
	owners  [2]string
	pool    *pgxpool.Pool
	players *world.PGStore
}

func makeFixture(t *testing.T, backend string) fixture {
	t.Helper()
	tokens := [2]string{"equipment-owner-a-0001", "equipment-owner-b-0002"}
	f := fixture{}
	for i, token := range tokens {
		f.owners[i] = world.PublicID(world.TokenHash(token))
	}
	if backend == "memory" {
		f.store = NewMemStore(f.owners[:]...)
		return f
	}
	f.pool = testdb.Pool(t)
	if _, err := db.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	f.players = world.NewPGStore(f.pool)
	for _, token := range tokens {
		if _, err := f.players.Resolve(context.Background(), token, "Equipment Test"); err != nil {
			t.Fatal(err)
		}
	}
	f.store = NewPGStore(f.pool)
	return f
}

func requireUnchanged(t *testing.T, store StarterStore, want StarterState) {
	t.Helper()
	got, err := store.Load(context.Background(), want.OwnerID)
	if err != nil || got != want {
		t.Fatalf("state changed: got=%+v want=%+v err=%v", got, want, err)
	}
}

func TestStarterStoreFlowAndReinitialization(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := makeFixture(t, backend)
			s := f.store
			ctx := context.Background()
			owner := f.owners[0]
			if _, err := s.Load(ctx, owner); !errors.Is(err, ErrNotInitialized) {
				t.Fatalf("load implicitly granted equipment: %v", err)
			}
			if _, err := s.SetEngine(ctx, owner, 0, false); !errors.Is(err, ErrNotInitialized) {
				t.Fatalf("update implicitly initialized state: %v", err)
			}
			if _, err := s.Ensure(ctx, "unknown-owner"); !errors.Is(err, ErrNoOwner) {
				t.Fatalf("unknown owner granted: %v", err)
			}
			for _, invalid := range []string{"", " padded ", string(make([]byte, 65))} {
				if _, err := s.Ensure(ctx, invalid); !errors.Is(err, ErrInvalidState) {
					t.Fatalf("invalid owner: %v", err)
				}
				if _, err := s.Load(ctx, invalid); !errors.Is(err, ErrInvalidState) {
					t.Fatalf("invalid load owner: %v", err)
				}
			}
			initialState, err := s.Ensure(ctx, owner)
			if err != nil {
				t.Fatal(err)
			}
			w := world.New()
			w.Join(world.Record{ID: owner, Name: "Legacy", ShipType: ShipType})
			snapshot, _ := w.Snapshot(owner)
			if len(snapshot.Ship.Modules) != 1 || initialState.InstanceID != snapshot.Ship.Modules[0].InstanceID || initialState.Revision != 0 || !initialState.Equipped {
				t.Fatalf("starter identity changed: %+v", initialState)
			}
			if _, err := s.SetEngine(ctx, owner, -1, false); !errors.Is(err, ErrConflict) {
				t.Fatalf("negative revision: %v", err)
			}
			requireUnchanged(t, s, initialState)
			noop, err := s.SetEngine(ctx, owner, 0, true)
			if err != nil || noop != initialState {
				t.Fatalf("no-op changed state: %+v %v", noop, err)
			}
			removed, err := s.SetEngine(ctx, owner, 0, false)
			if err != nil || removed.Equipped || removed.Revision != 1 || removed.InstanceID != initialState.InstanceID {
				t.Fatalf("remove: %+v %v", removed, err)
			}
			again, err := s.Ensure(ctx, owner)
			if err != nil || again != removed {
				t.Fatalf("reconnect reset unequipped state: %+v %v", again, err)
			}
			// A new PG store/service sees the same committed state, not a bootstrap grant.
			if f.pool != nil {
				requireUnchanged(t, NewPGStore(f.pool), removed)
			}
			stale, err := s.SetEngine(ctx, owner, 0, true)
			if !errors.Is(err, ErrConflict) || !reflect.DeepEqual(stale, StarterState{}) {
				t.Fatalf("stale update accepted/returned partial state: %+v %v", stale, err)
			}
			requireUnchanged(t, s, removed)
			noop, err = s.SetEngine(ctx, owner, 1, false)
			if err != nil || noop != removed {
				t.Fatalf("repeat removed state: %+v %v", noop, err)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err := s.SetEngine(canceled, owner, 1, true); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled mutation: %v", err)
			}
			requireUnchanged(t, s, removed)
			if _, err := s.Ensure(canceled, f.owners[1]); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled grant: %v", err)
			}
			other, err := s.Ensure(ctx, f.owners[1])
			if err != nil || other.InstanceID == removed.InstanceID || other.OwnerID == owner {
				t.Fatalf("owners share item: %+v %v", other, err)
			}
			fitted, err := s.SetEngine(ctx, owner, 1, true)
			if err != nil || !fitted.Equipped || fitted.Revision != 2 || fitted.InstanceID != removed.InstanceID {
				t.Fatalf("refit: %+v %v", fitted, err)
			}
			if _, err := s.SetEngine(ctx, owner, 1, false); !errors.Is(err, ErrConflict) {
				t.Fatalf("second stale update: %v", err)
			}
			requireUnchanged(t, s, fitted)
			requireUnchanged(t, s, other)
		})
	}
}

func TestConcurrentStarterGrantAndCompareAndSwap(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := makeFixture(t, backend)
			s := f.store
			owner := f.owners[0]
			ctx := context.Background()
			const workers = 16
			var wg sync.WaitGroup
			results := make(chan StarterState, workers)
			errs := make(chan error, workers)
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); v, err := s.Ensure(ctx, owner); results <- v; errs <- err }()
			}
			wg.Wait()
			for i := 0; i < workers; i++ {
				if err := <-errs; err != nil {
					t.Fatal(err)
				}
				if got := <-results; got != initial(owner) {
					t.Fatalf("duplicate/reset grant: %+v", got)
				}
			}
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); _, err := s.SetEngine(ctx, owner, 0, false); errs <- err }()
			}
			wg.Wait()
			successes := 0
			for i := 0; i < workers; i++ {
				err := <-errs
				if err == nil {
					successes++
				} else if !errors.Is(err, ErrConflict) {
					t.Fatal(err)
				}
			}
			if successes != 1 {
				t.Fatalf("CAS winners: %d", successes)
			}
			removed, err := s.Load(ctx, owner)
			if err != nil || removed.Equipped || removed.Revision != 1 {
				t.Fatalf("CAS result: %+v %v", removed, err)
			}
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); v, err := s.Ensure(ctx, owner); results <- v; errs <- err }()
			}
			wg.Wait()
			for i := 0; i < workers; i++ {
				if err := <-errs; err != nil {
					t.Fatal(err)
				}
				if got := <-results; got != removed {
					t.Fatalf("concurrent bootstrap reset: %+v", got)
				}
			}
			if f.pool != nil {
				var items, loadouts, slots int
				for query, target := range map[string]*int{
					`SELECT count(*) FROM player_module_items WHERE owner_id=$1`:       &items,
					`SELECT count(*) FROM player_ship_loadouts WHERE owner_id=$1`:      &loadouts,
					`SELECT count(*) FROM player_ship_loadout_slots WHERE owner_id=$1`: &slots,
				} {
					if err := f.pool.QueryRow(ctx, query, owner).Scan(target); err != nil {
						t.Fatal(err)
					}
				}
				if items != 1 || loadouts != 1 || slots != 0 {
					t.Fatalf("duplicate/orphan rows: %d/%d/%d", items, loadouts, slots)
				}
			}
		})
	}
}

func TestReadersSeeConsistentSlotAndRevision(t *testing.T) {
	for _, backend := range []string{"memory", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			f := makeFixture(t, backend)
			ctx := context.Background()
			owner := f.owners[0]
			if _, err := f.store.Ensure(ctx, owner); err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			errs := make(chan error, 2)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				for revision := int64(0); revision < 20; revision++ {
					if _, err := f.store.SetEngine(ctx, owner, revision, revision%2 == 1); err != nil {
						errs <- err
						return
					}
				}
			}()
			go func() {
				defer wg.Done()
				<-start
				for i := 0; i < 100; i++ {
					s, err := f.store.Load(ctx, owner)
					if err != nil {
						errs <- err
						return
					}
					if s.Equipped != (s.Revision%2 == 0) {
						errs <- fmt.Errorf("torn snapshot: %+v", s)
						return
					}
				}
			}()
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			final, err := f.store.Load(ctx, owner)
			if err != nil || final.Revision != 20 || !final.Equipped {
				t.Fatalf("final state: %+v %v", final, err)
			}
		})
	}
}
