package main

import (
	"context"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestSyncShipsAddsMissingKeepsEdits(t *testing.T) {
	ctx := context.Background()
	store := shipcatalog.NewMemStore()
	// старый каталог production: 3 корабля, у TMP-MINE нет способности добычи, у «Стрижа» правка из админки
	old := world.DefaultDoc()
	var ships []world.ShipSpec
	for _, s := range old.Ships {
		switch s.ID {
		case "FTR-001":
			s.Hull = 1999
			ships = append(ships, s)
		case "TMP-MINE":
			delete(s.Abilities, "mine")
			ships = append(ships, s)
		case "TMP-HAUL":
			ships = append(ships, s)
		}
	}
	old.Ships = ships
	zero := int64(0)
	if _, err := store.Save(ctx, shipcatalog.NewVersion{Admin: "test", Source: "seed", Doc: old, ExpectActive: &zero}); err != nil {
		t.Fatal(err)
	}
	svc := shipcatalog.NewService(store, nil)
	n, err := syncShips(ctx, svc, func(string) {})
	if err != nil || n != 10 { // 9 новых кораблей и TMP-MINE
		t.Fatalf("синхронизация: %d %v", n, err)
	}
	doc, _, _ := svc.Active(ctx)
	got := map[string]world.ShipSpec{}
	for _, s := range doc.Ships {
		got[s.ID] = s
	}
	if len(got) != 12 || got["FTR-001"].Hull != 1999 {
		t.Fatalf("кораблей %d, корпус «Стрижа» %v (правка админа должна сохраниться)", len(got), got["FTR-001"].Hull)
	}
	if _, ok := got["TMP-MINE"].Abilities["mine"]; !ok {
		t.Fatal("способность добычи не добавлена")
	}
	// повторный запуск ничего не меняет
	if n, err := syncShips(ctx, svc, func(string) {}); err != nil || n != 0 {
		t.Fatalf("повтор: %d %v", n, err)
	}
}
