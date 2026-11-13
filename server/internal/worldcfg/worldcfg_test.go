package worldcfg

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

func newSvc(t *testing.T) (*Service, *[]worldgen.Config) {
	t.Helper()
	var applied []worldgen.Config
	s := NewService(NewMemStore(), func(c worldgen.Config) { applied = append(applied, c) })
	if err := s.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, &applied
}

func TestInitSeedsAndApplies(t *testing.T) {
	s, applied := newSvc(t)
	raw, ver, err := s.Active(context.Background())
	if err != nil || ver != 1 || len(*applied) != 1 || len(raw) == 0 {
		t.Fatalf("init: ver=%d applied=%d err=%v", ver, len(*applied), err)
	}
	// повторный Init не создаёт вторую версию
	if err := s.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if vs, _ := s.Versions(context.Background()); len(vs) != 1 {
		t.Fatalf("версий %d", len(vs))
	}
}

func TestCheckRejectsBadConfig(t *testing.T) {
	good, _ := json.Marshal(worldgen.DefaultConfig())
	if p := Check(good); !p.OK || len(p.Systems) < 3 || p.Systems[0].Deposits == 0 {
		t.Fatalf("корректная конфигурация: %+v", p)
	}
	for name, mut := range map[string]func(*worldgen.Config){
		"радиус": func(c *worldgen.Config) { c.Systems[0].Radius = 10 },
		"враг":   func(c *worldgen.Config) { e := c.Enemies["normal"]; e.Hull = -1; c.Enemies["normal"] = e },
		"цена":   func(c *worldgen.Config) { r := c.Resources["gas"]; r.Price = -5; c.Resources["gas"] = r },
	} {
		c := worldgen.DefaultConfig()
		mut(&c)
		raw, _ := json.Marshal(c)
		if p := Check(raw); p.OK || len(p.Errors) == 0 {
			t.Fatalf("%s: некорректная конфигурация принята", name)
		}
	}
	if p := Check([]byte(`{"systems":[],"unknown":1}`)); p.OK {
		t.Fatal("неизвестные поля должны отклоняться")
	}
	if p := Check([]byte(strings.Repeat("x", MaxConfigBytes+1))); p.OK {
		t.Fatal("слишком большой файл принят")
	}
}

func TestPublishConflictAndRollback(t *testing.T) {
	s, applied := newSvc(t)
	ctx := context.Background()
	c := worldgen.DefaultConfig()
	e := c.Enemies["normal"]
	e.Reward = 77
	c.Enemies["normal"] = e
	raw, _ := json.Marshal(c)
	v2, err := s.Publish(ctx, "админ", "награда", raw, 1)
	if err != nil || v2 != 2 || len(*applied) != 2 || (*applied)[1].Enemies["normal"].Reward != 77 {
		t.Fatalf("публикация: %d %v", v2, err)
	}
	// устаревшая основа
	if _, err := s.Publish(ctx, "другой", "", raw, 1); err != ErrConflict {
		t.Fatalf("конфликт: %v", err)
	}
	// некорректную конфигурацию нельзя опубликовать, активная остаётся
	if _, err := s.Publish(ctx, "админ", "", []byte(`{}`), 2); err == nil {
		t.Fatal("пустая конфигурация принята")
	}
	if _, ver, _ := s.Active(ctx); ver != 2 || len(*applied) != 2 {
		t.Fatal("после отказа активная версия изменилась")
	}
	v3, err := s.Rollback(ctx, "админ", 1)
	if err != nil || v3 != 3 || (*applied)[2].Enemies["normal"].Reward == 77 {
		t.Fatalf("откат: %d %v", v3, err)
	}
	if _, err := s.Rollback(ctx, "админ", 99); err != ErrNotFound {
		t.Fatalf("откат к несуществующей: %v", err)
	}
	if vs, _ := s.Versions(ctx); len(vs) != 3 || !vs[0].Active || vs[0].Source != "rollback" {
		t.Fatalf("история: %+v", vs)
	}
}

func TestPGStore(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var applied int
	s := NewService(NewPGStore(pool), func(worldgen.Config) { applied++ })
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	raw, ver, err := s.Active(ctx)
	if err != nil || ver != 1 || applied != 1 {
		t.Fatalf("init: %d %d %v", ver, applied, err)
	}
	v2, err := s.Publish(ctx, "админ", "тест", raw, ver)
	if err != nil || v2 != 2 {
		t.Fatalf("publish: %d %v", v2, err)
	}
	if _, err := s.Publish(ctx, "админ", "тест", raw, ver); err != ErrConflict {
		t.Fatalf("конфликт: %v", err)
	}
	if v3, err := s.Rollback(ctx, "админ", 1); err != nil || v3 != 3 {
		t.Fatalf("откат: %d %v", v3, err)
	}
	var active int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM world_config_versions WHERE is_active`).Scan(&active)
	if active != 1 {
		t.Fatalf("активных версий %d", active)
	}
}
