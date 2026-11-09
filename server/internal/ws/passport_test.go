package ws

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestShipsPassportUsesActiveScaledBaseAndPreservesLegacyFleet(t *testing.T) {
	passports, err := shipcatalog.DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter()
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: router, ShipPassports: &passports})
	cl := connect(t, srv.URL)
	welcome := cl.join(strings.Repeat("p", 24))
	pid := welcome["player_id"].(string)
	w := router.For(pid)
	// Simulate an active admin version. Draft metadata must never reset it.
	catalog := w.Catalog()
	sp := catalog.Ships[world.DefaultShip]
	sp.Hull = 1750
	sp.Energy = 130
	sp.Armor = 175
	boost := sp.Abilities["boost"]
	boost.Energy = 23
	sp.Abilities["boost"] = boost
	catalog.Ships[world.DefaultShip] = sp
	w.SetCatalog(catalog)
	w.SetShipLevel(pid, 11)
	active, _ := w.SpecInfo(pid)
	cl.send("ships", map[string]any{})
	msg := cl.next("ships")
	var got struct {
		Ships    []world.ShipSummary      `json:"ships"`
		Passport shipcatalog.ShipPassport `json:"passport"`
		MaxLevel int                      `json:"ship_max_level"`
	}
	if err := json.Unmarshal(msg.Payload, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Ships) != 12 || got.MaxLevel != 50 {
		t.Fatalf("fleet/cap changed: %d/%d", len(got.Ships), got.MaxLevel)
	}
	if got.Passport.ID != world.DefaultShip || !reflect.DeepEqual(got.Passport.Base, active) {
		t.Fatal("passport did not use authoritative current base")
	}
	after, _ := w.SpecInfo(pid)
	if !reflect.DeepEqual(active, after) || len(w.Catalog().Ships) != 12 {
		t.Fatal("metadata request changed world")
	}
	// Old decoders ignore the extension and retain the same fleet payload.
	var old struct {
		Ships []world.ShipSummary `json:"ships"`
	}
	if err := json.Unmarshal(msg.Payload, &old); err != nil || !reflect.DeepEqual(old.Ships, w.Fleet()) {
		t.Fatalf("old response compatibility: %v", err)
	}
	// Scope is only Strizh. Other hulls continue on their existing path.
	if _, err := w.SetShip(pid, "TMP-MINE"); err != nil {
		t.Fatal(err)
	}
	cl.send("ships", map[string]any{})
	var other map[string]json.RawMessage
	if err := json.Unmarshal(cl.next("ships").Payload, &other); err != nil {
		t.Fatal(err)
	}
	if _, ok := other["passport"]; ok {
		t.Fatal("passport enabled on another hull")
	}
}

func TestShipsWithoutPassportProvider(t *testing.T) {
	srv, _ := newServer(t, Options{MaxMessage: 4096, RatePerSec: 100, World: testRouter()})
	cl := connect(t, srv.URL)
	cl.join(strings.Repeat("q", 24))
	cl.send("ships", map[string]any{})
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(cl.next("ships").Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["passport"]; ok {
		t.Fatal("metadata sent with feature disabled")
	}
}

// Run locally or from the Godot CI job with COSMOC_TEST_GODOT=godot. This uses
// an ephemeral memory-backed localhost server, never a configured database/VPS.
func TestPassportGodotClient(t *testing.T) {
	binary := os.Getenv("COSMOC_TEST_GODOT")
	if binary == "" {
		t.Skip("COSMOC_TEST_GODOT not set: live Godot scenario belongs to client job")
	}
	passports, err := shipcatalog.DefaultPassportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter()
	srv, h := newServer(t, Options{MaxMessage: 8192, RatePerSec: 100, World: router, ShipPassports: &passports})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { h.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	clientRoot, err := filepath.Abs("../../../client")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "--headless", "--path", clientRoot, "--script", "res://tests/passport_online_test.gd")
	cmd.Env = append(os.Environ(), "COSMOC_TEST_WS_URL=ws"+strings.TrimPrefix(srv.URL, "http"))
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "ERROR:") || !strings.Contains(string(output), "PASSPORT_ONLINE_PASS") {
		t.Fatalf("live Godot scenario: %v\n%s", err, output)
	}
	t.Log("Godot -> real local WebSocket -> hangar: PASS")
}

func TestM01OverWebSocketUsesBoundEngineAndLegacyCooldown(t *testing.T) {
	_, _, url := gameServer(t)
	cl := connect(t, url)
	cl.join(strings.Repeat("m", 24))
	var initial struct {
		Ship world.Ship `json:"ship"`
	}
	if err := json.Unmarshal(cl.next("state").Payload, &initial); err != nil {
		t.Fatal(err)
	}
	if len(initial.Ship.Modules) != 1 || !initial.Ship.Modules[0].Owned || !initial.Ship.Modules[0].Equipped {
		t.Fatal("bound starter missing in state")
	}
	ab := initial.Ship.Abilities[0]
	if ab.ID != "boost" || ab.CatalogID != "M01" || !ab.Enabled || ab.ModuleID != initial.Ship.Modules[0].InstanceID {
		t.Fatalf("incorrect compatibility binding: %+v", ab)
	}
	cl.send("ability", map[string]any{"id": "M01"})
	boosted := false
	for i := 0; i < 20; i++ {
		var state struct {
			Ship world.Ship `json:"ship"`
		}
		if err := json.Unmarshal(cl.next("state").Payload, &state); err != nil {
			t.Fatal(err)
		}
		if state.Ship.Boosting {
			boosted = true
			break
		}
	}
	if !boosted {
		t.Fatal("M01 did not execute through the server")
	}
	cl.send("ability", map[string]any{"id": "boost"})
	if code := errCode(t, cl.next("error")); code != "cooldown" {
		t.Fatalf("legacy cooldown bypass: %s", code)
	}
}
