package ws

import (
	"context"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
	"testing"
)

func TestAICivReproFullHoldBuy(t *testing.T) {
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	at := world.Vec{X: lay.Stations[0].Pos[0], Y: lay.Stations[0].Pos[1]}
	token := "repro-full-hold-token"
	qs, _, url := livingServer(t, token, "TMP-MINE", at, map[string]float64{"iron_ore": 100000})
	pid := world.PublicID(world.TokenHash(token))
	if _, _, err := qs.GrantCredits(context.Background(), pid, "fixture-credits", 1000); err != nil {
		t.Fatal(err)
	}
	cl := connect(t, url)
	cl.join(token)
	// Фиксируем стартовую награду до снимка баланса, чтобы тик квеста не мешал проверке.
	if _, _, err := qs.Complete(context.Background(), pid, quest.Catalog()[0]); err != nil {
		t.Fatal(err)
	}
	before, err := qs.Load(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	cl.send("buy", map[string]any{"resource": "gas", "amount": 1})
	code := cl.errCode()
	if code != "hold_full" {
		t.Fatalf("want hold_full, got %s", code)
	}
	after, err := qs.Load(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if after.Credits != before.Credits {
		t.Fatalf("buy rejected(%s) but debited: before=%d after=%d", code, before.Credits, after.Credits)
	}
}
