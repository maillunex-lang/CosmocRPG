package quest

import (
	"context"
	"log/slog"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestChapter2ContentAndFlow(t *testing.T) {
	ids := map[string]bool{}
	for _, d := range Catalog() {
		if len(d.ID) >= 5 && d.ID[:5] == "ch02_" {
			ids[d.ID] = true
		}
	}
	if len(ids) != 9 {
		t.Fatalf("chapter 2 quest count: %d", len(ids))
	}
	for _, id := range []string{"ch02_q01", "ch02_q07", "ch02_q09"} {
		scene, ok := Dialogue(id)
		if !ok || len(scene.Nodes) == 0 {
			t.Fatalf("missing dialogue %s", id)
		}
	}
	s := NewService(slog.Default(), NewMemStore(), func(string) (world.Vec, bool) { return world.Vec{}, true })
	if err := s.Join(context.Background(), "chapter2"); err != nil {
		t.Fatal(err)
	}
}
