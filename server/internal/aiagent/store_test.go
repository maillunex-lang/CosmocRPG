package aiagent

import (
	"context"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

func TestPGSettingsRoundTrip(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	if _, ok, err := st.Load(ctx); err != nil || ok {
		t.Fatalf("пустое хранилище: ok=%v err=%v", ok, err)
	}
	s := DefaultSettings()
	s.MinOnline, s.MaxOnline, s.Paused = 7, 33, true
	s.Shares = map[string]int{"miner": 50, "scout": 50}
	if err := st.Save(ctx, s, "owner"); err != nil {
		t.Fatal(err)
	}
	s.MaxOnline = 40
	if err := st.Save(ctx, s, "owner"); err != nil { // повторная запись заменяет, а не дублирует
		t.Fatal(err)
	}
	got, ok, err := st.Load(ctx)
	if err != nil || !ok || got.MinOnline != 7 || got.MaxOnline != 40 || !got.Paused || got.Shares["miner"] != 50 {
		t.Fatalf("настройки: %+v ok=%v err=%v", got, ok, err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_settings`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("должна быть одна строка: %d %v", n, err)
	}
}

func TestStudioSourceReadsProfiles(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	svc := aistudio.NewService(aistudio.NewPGStore(pool))
	if _, _, err := svc.Generate(ctx, "test", 1, 40); err != nil {
		t.Fatal(err)
	}
	ps, err := StudioSource{Svc: svc}.Profiles(ctx)
	if err != nil || len(ps) != 40 {
		t.Fatalf("профили: %d, %v", len(ps), err)
	}
	for _, p := range ps {
		if p.ID == "" || p.Name == "" || p.Profession == "" {
			t.Fatalf("неполный профиль: %+v", p)
		}
	}
}
