package ws

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/corp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Local memory-backed server: exercise real client UI and protocol without AI or production.
func TestCorporationGodotClient(t *testing.T) {
	binary := os.Getenv("COSMOC_TEST_GODOT")
	if binary == "" {
		t.Skip("COSMOC_TEST_GODOT not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	router := testRouter()
	store := world.NewMemStore()
	qs := quest.NewMemStore()
	q := quest.NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), qs, router.ObjectPos)
	for _, token := range []string{"corporation-owner-local-test", "corporation-member-local-test"} {
		pid := world.PublicID(world.TokenHash(token))
		if err := store.Save(ctx, world.Record{ID: pid, Name: token, Created: true, ShipType: "ftr001", PilotSpec: "fighter"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := qs.GrantCredits(ctx, pid, "seed", 20000); err != nil {
			t.Fatal(err)
		}
	}
	corporations := corp.NewService(corp.NewMemStore(), q.Spend)
	srv, h := newServer(t, Options{MaxMessage: 8192, RatePerSec: 200, World: router, Store: store, Quests: q, Corp: corporations})
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	clientRoot, err := filepath.Abs("../../../client")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "--headless", "--path", clientRoot, "--script", "res://tests/corporation_online_test.gd")
	cmd.Env = append(os.Environ(), "COSMOC_TEST_WS_URL=ws"+strings.TrimPrefix(srv.URL, "http"))
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "ERROR:") || !strings.Contains(string(output), "CORPORATION_ONLINE_PASS") {
		t.Fatalf("live corporation client: %v\n%s", err, output)
	}
	for _, item := range []struct {
		token string
		want  int64
	}{{"corporation-owner-local-test", 10000}, {"corporation-member-local-test", 20000}} {
		state, err := qs.Load(ctx, world.PublicID(world.TokenHash(item.token)))
		if err != nil || state.Credits != item.want {
			t.Fatalf("balance: want %d, got %d, err %v", item.want, state.Credits, err)
		}
	}
	t.Log("Godot UI -> real WS -> create/join/roster/profile/reconnect/leave; creation debited once")
}
