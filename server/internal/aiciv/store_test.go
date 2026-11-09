package aiciv

import (
	"context"
	"errors"
	"fmt"
	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestIdentityMemoryRestartAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	a, err := s.Ensure(ctx, "captain-a")
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode != "DORMANT" || a.PlayerID != "" {
		t.Fatal("shadow fabricated player", a)
	}
	b, err := s.Ensure(ctx, "captain-b")
	if err != nil {
		t.Fatal(err)
	}
	player, err := world.NewPGStore(pool).Resolve(ctx, "civilization-test-player-token", "AI test")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Bind(ctx, a.CaptainID, player.ID, a.Revision); err != nil {
		t.Fatal(err)
	}
	if err = s.Bind(ctx, a.CaptainID, player.ID, a.Revision); !errors.Is(err, ErrConflict) {
		t.Fatal("stale bind", err)
	}
	if err = s.Bind(ctx, b.CaptainID, player.ID, b.Revision); err == nil {
		t.Fatal("shared player accepted")
	}
	now := time.Now().UTC().Truncate(time.Second)
	event := MemoryEvent{"event-1", "confirmed", "station", now, now.Add(time.Hour)}
	if err = s.Observe(ctx, a.CaptainID, event); err != nil {
		t.Fatal(err)
	}
	if err = s.Observe(ctx, a.CaptainID, event); err != nil {
		t.Fatal(err)
	}
	s = NewStore(pool)
	restored, err := s.Ensure(ctx, a.CaptainID)
	if err != nil || restored.PlayerID != player.ID || restored.Revision != 2 {
		t.Fatal(restored, err)
	}
	memory, err := s.Recall(ctx, a.CaptainID, now)
	if err != nil || len(memory) != 1 {
		t.Fatal(memory, err)
	}
	other, err := s.Recall(ctx, b.CaptainID, now)
	if err != nil || len(other) != 0 {
		t.Fatal("memory leak", other, err)
	}
	if err = s.Prune(ctx, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	memory, err = s.Recall(ctx, a.CaptainID, now.Add(time.Hour))
	if err != nil || len(memory) != 0 {
		t.Fatal(memory, err)
	}
}
func TestConcurrentMemoryBounded(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	if _, err := s.Ensure(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				e := MemoryEvent{fmt.Sprintf("%d-%d", w, i), "observed", "", now, now.Add(time.Hour)}
				if err := s.Observe(ctx, "a", e); err != nil {
					t.Error(err)
				}
			}
		}(worker)
	}
	wg.Wait()
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_memory_events WHERE agent_id='a'`).Scan(&count); err != nil || count != 128 {
		t.Fatal(count, err)
	}
}

type fixedSource struct{}

func (fixedSource) Profiles(context.Context) ([]aiagent.Profile, error) {
	return []aiagent.Profile{{ID: "studio-captain", Name: "Captain", Profession: "miner"}}, nil
}
func TestShadowDoesNotCreateGamePlayers(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	sh := Shadow{Source: fixedSource{}, Store: NewStore(pool), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	go func() { defer close(done); sh.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var agents int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM ai_agents`).Scan(&agents); err != nil {
			t.Fatal(err)
		}
		if agents == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shadow did not sync profile")
		}
		time.Sleep(10 * time.Millisecond)
	}
	var players int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM players`).Scan(&players); err != nil || players != 0 {
		t.Fatal("shadow created gameplay state", players, err)
	}
}
