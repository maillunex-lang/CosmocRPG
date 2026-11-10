package ws

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

type purchaseFault struct {
	*quest.PGStore
	afterCommit bool
	unreadable  atomic.Bool
	attempted   atomic.Bool
	entered     chan struct{}
	proceed     chan struct{}
}

func (f *purchaseFault) Purchase(ctx context.Context, r economy.Request) (economy.Result, error) {
	if f.entered != nil {
		close(f.entered)
		select {
		case <-f.proceed:
		case <-ctx.Done():
			return economy.Result{}, ctx.Err()
		}
	}
	result, err := f.PGStore.Purchase(ctx, r)
	f.attempted.Store(true)
	if err == nil && f.afterCommit {
		return economy.Result{}, errors.New("injected lost COMMIT reply")
	}
	return result, err
}
func (f *purchaseFault) LookupPurchase(ctx context.Context, pid, key, resource string, amount float64) (economy.Result, error) {
	if f.attempted.Load() && f.unreadable.Load() {
		return economy.Result{}, errors.New("injected read unavailable")
	}
	return f.PGStore.LookupPurchase(ctx, pid, key, resource, amount)
}
func purchaseServer(t *testing.T) (*pgxpool.Pool, *purchaseFault, *world.PGStore, *world.Router, *Handler, string, string) {
	t.Helper()
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	w := world.New()
	if err := w.ApplyLayout(lay, cfg); err != nil {
		t.Fatal(err)
	}
	router := world.NewRouter(w)
	router.Add(w)
	players := world.NewPGStore(pool)
	r, err := players.Resolve(ctx, "purchase-real-pg-player-token", "")
	if err != nil {
		t.Fatal(err)
	}
	r, err = world.CreatePilot(ctx, players, router, r.ID, "Покупатель", "m", "miner")
	if err != nil {
		t.Fatal(err)
	}
	r.Pos, _ = w.ObjectPos(world.StationID)
	if err = players.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	f := &purchaseFault{PGStore: quest.NewPGStore(pool)}
	if _, _, err = f.GrantCredits(ctx, r.ID, "fixture", 1000); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := quest.NewService(log, f, w.ObjectPos)
	if err = q.Init(ctx); err != nil {
		t.Fatal(err)
	}
	srv, h := newServer(t, Options{MaxMessage: 8192, RatePerSec: 200, World: router, Store: players, Quests: q, RequireCharacter: true})
	return pool, f, players, router, h, srv.URL, r.ID
}
func TestPurchaseLostCommitReplyReconcilesAndRetries(t *testing.T) {
	ctx := context.Background()
	_, f, players, router, _, url, pid := purchaseServer(t)
	f.afterCommit = true
	cl := connect(t, url)
	cl.join("purchase-real-pg-player-token")
	payload := map[string]any{"resource": "gas", "amount": 1, "operation_id": "lost-reply"}
	cl.send("buy", payload)
	cl.next("ok")
	cl.send("buy", payload)
	cl.next("ok")
	state, err := f.PGStore.LookupPurchase(ctx, pid, "lost-reply", "gas", 1)
	if err != nil || !state.Found || state.Cargo.Hold["gas"] != 1 || state.Credits != 1000-state.Spent {
		t.Fatal(state, err)
	}
	for _, r := range router.Records() {
		if r.ID == pid {
			if r.SkipCargo || r.Hold["gas"] != 1 {
				t.Fatal(r)
			}
		}
	}
	// Загрузка новым store после перезапуска читает уже купленный груз из БД.
	restored, err := players.Load(ctx, pid)
	if err != nil || restored.Hold["gas"] != 1 || restored.CargoRevision != 1 {
		t.Fatal(restored, err)
	}
}
func TestPurchaseUnknownReadFailureDoesNotAutosaveSpeculation(t *testing.T) {
	ctx := context.Background()
	_, f, players, router, _, url, pid := purchaseServer(t)
	f.afterCommit = true
	f.unreadable.Store(true)
	cl := connect(t, url)
	cl.join("purchase-real-pg-player-token")
	payload := map[string]any{"resource": "gas", "amount": 1, "operation_id": "unknown-reply"}
	cl.send("buy", payload)
	if code := cl.errCode(); code != "purchase_unavailable" {
		t.Fatal(code)
	}
	w := router.For(pid)
	if w.SetMove(pid, world.Vec{X: 1}) {
		t.Fatal("uncertain cargo not quarantined")
	}
	for _, r := range router.Records() {
		if r.ID == pid {
			if !r.SkipCargo {
				t.Fatal("uncertain cargo autosavable")
			}
			if err := players.Save(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
	}
	restored, err := players.Load(ctx, pid)
	if err != nil || restored.Hold["gas"] != 1 {
		t.Fatal("stale autosave lost committed item", restored, err)
	}
	f.unreadable.Store(false)
	cl.send("purchase_status", payload)
	cl.next("ok")
	if !w.SetMove(pid, world.Vec{X: 1}) {
		t.Fatal("receipt reconciliation did not release actor")
	}
	cl.send("buy", payload)
	cl.next("ok")
	state, err := f.PGStore.LookupPurchase(ctx, pid, "unknown-reply", "gas", 1)
	if err != nil || state.Cargo.Hold["gas"] != 1 || state.Credits != 1000-state.Spent {
		t.Fatal(state, err)
	}
}
func TestPurchaseRowLockDoesNotBlockOtherPlayerOrTick(t *testing.T) {
	ctx := context.Background()
	pool, f, players, router, h, url, pid := purchaseServer(t)
	f.entered = make(chan struct{})
	f.proceed = make(chan struct{})
	a := connect(t, url)
	a.join("purchase-real-pg-player-token")
	tokenB := "purchase-other-player-token"
	rec, err := players.Resolve(ctx, tokenB, "")
	if err != nil {
		t.Fatal(err)
	}
	rec, err = world.CreatePilot(ctx, players, router, rec.ID, "ВторойПилот", "m", "scout")
	if err != nil {
		t.Fatal(err)
	}
	rec.Pos = world.Vec{X: 100, Y: 100}
	if err = players.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	b := connect(t, url)
	b.join(tokenB)
	a.send("buy", map[string]any{"resource": "gas", "amount": 1, "operation_id": "blocked"})
	select {
	case <-f.entered:
	case <-time.After(time.Second):
		t.Fatal("purchase did not reserve actor")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT 1 FROM players WHERE public_id=$1 FOR UPDATE`, pid); err != nil {
		t.Fatal(err)
	}
	close(f.proceed)
	// Если Tick попробует проверить квест A через SQL, он тоже упрётся в row lock.
	done := make(chan struct{})
	go func() { h.Tick(2, time.Now()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		_ = tx.Rollback(ctx)
		<-done
		t.Fatal("buyer SQL blocks global tick")
	}
	b.send("move", map[string]any{"x": 1, "y": 0})
	b.send("ping", nil)
	b.next("pong")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	a.next("ok")
}
