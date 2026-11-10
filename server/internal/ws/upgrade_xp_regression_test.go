package ws

import (
	"context"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
	"testing"
)

func TestShipUpgradeUsesRemainingXP(t *testing.T) {
	ctx := context.Background()
	cfg := worldgen.DefaultConfig()
	lay := worldgen.Generate(cfg.Systems[0], cfg)
	token := "upgrade-partial-xp-repro-token"
	pid := world.PublicID(world.TokenHash(token))
	qs, _, url := livingServer(t, token, "TMP-MINE", world.Vec{X: lay.Hub[0], Y: lay.Hub[1]}, nil)
	if _, _, err := qs.GrantCredits(ctx, pid, "fund", 5000); err != nil {
		t.Fatal(err)
	}
	if _, err := livingShips.Grant(ctx, pid, "TMP-MINE", "seed-partial", "fixture", 100); err != nil {
		t.Fatal(err)
	}
	cl := connect(t, url)
	cl.join(token)
	if _, _, err := qs.Complete(ctx, pid, quest.Catalog()[0]); err != nil {
		t.Fatal(err)
	}
	before, err := qs.Load(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	cl.send("ship_upgrade", map[string]any{})
	cl.next("ok")
	st, err := livingShips.State(ctx, pid, "TMP-MINE")
	if err != nil {
		t.Fatal(err)
	}
	after, err := qs.Load(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if after.Credits != before.Credits-1000 {
		t.Fatal("wrong upgrade price", before.Credits, after.Credits)
	}
	if st.Level != 2 || st.XP != 150 || st.IntoLevel != 0 {
		t.Fatalf("paid upgrade did not grant level: level=%d into=%d remaining=%d", st.Level, st.IntoLevel, st.ToNext)
	}
}
