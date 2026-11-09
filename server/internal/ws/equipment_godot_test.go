package ws

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/equipment"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestEquipmentGodotClientPostgres(t *testing.T) {
	binary := os.Getenv("COSMOC_TEST_GODOT")
	if binary == "" {
		t.Skip("COSMOC_TEST_GODOT not set")
	}
	pool := testdb.Pool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	players := world.NewPGStore(pool)
	token := "equipment-godot-isolated-test-token"
	rec, err := players.Resolve(ctx, token, "Godot equipment")
	if err != nil {
		t.Fatal(err)
	}
	r := testRouter()
	rec.Pos, _ = r.Primary().ObjectPos(world.StationID)
	if err := players.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	gear := equipment.NewPGStore(pool)
	srv, h := newServer(t, Options{MaxMessage: 8192, RatePerSec: 100, World: r, Store: players, Equipment: gear})
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	clientRoot, err := filepath.Abs("../../../client")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "--headless", "--path", clientRoot, "--script", "res://tests/equipment_online_test.gd")
	cmd.Env = append(os.Environ(), "COSMOC_TEST_WS_URL=ws"+strings.TrimPrefix(srv.URL, "http"), "COSMOC_TEST_EQUIPMENT_TOKEN="+token)
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "ERROR:") || !strings.Contains(string(output), "EQUIPMENT_ONLINE_PASS") {
		t.Fatalf("live equipment Godot: %v\n%s", err, output)
	}
	s, err := gear.Load(ctx, rec.ID)
	if err != nil || !s.Equipped || s.Revision != 2 {
		t.Fatalf("SQL final state: %+v %v", s, err)
	}
	t.Log("Godot -> WS -> PostgreSQL -> unequip/reconnect/refit/M01 PASS")
}
