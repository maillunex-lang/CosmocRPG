package ws

import (
	"context"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPurchaseGodotClientPostgres(t *testing.T) {
	binary := os.Getenv("COSMOC_TEST_GODOT")
	if binary == "" {
		t.Skip("COSMOC_TEST_GODOT not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	pool, f, _, _, h, url, pid := purchaseServer(t)
	// Стартовый квест оплачен до измерения, чтобы world ticks не меняли baseline.
	if _, err := f.Assign(ctx, pid, quest.Catalog()[0]); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Complete(ctx, pid, quest.Catalog()[0]); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := pool.QueryRow(ctx, `SELECT credits FROM players WHERE public_id=$1`, pid).Scan(&before); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); h.Run(ctx) }()
	defer func() { cancel(); <-done }()
	clientRoot, err := filepath.Abs("../../../client")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "--headless", "--path", clientRoot, "--script", "res://tests/purchase_online_test.gd")
	cmd.Env = append(os.Environ(), "COSMOC_TEST_WS_URL=ws"+strings.TrimPrefix(url, "http"), "COSMOC_TEST_PURCHASE_TOKEN=purchase-real-pg-player-token")
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "ERROR:") || !strings.Contains(string(output), "PURCHASE_ONLINE_PASS") {
		t.Fatalf("live purchase: %v\n%s", err, output)
	}
	var count int
	var spent, credits int64
	var gas float64
	if err = pool.QueryRow(ctx, `SELECT count(*),COALESCE(sum(spent),0) FROM resource_purchase_receipts WHERE player_id=$1`, pid).Scan(&count, &spent); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT credits,(hold->>'gas')::double precision FROM players WHERE public_id=$1`, pid).Scan(&credits, &gas); err != nil {
		t.Fatal(err)
	}
	if count != 1 || gas != 1 || credits != before-spent {
		t.Fatalf("reconnect duplicated purchase: %d receipts,gas=%v,credits=%d expected=%d", count, gas, credits, before-spent)
	}
	t.Log("Godot -> WS -> PG -> lost application ACK -> restart -> status PASS")
}
