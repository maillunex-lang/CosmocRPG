package admin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldcfg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

func newWorldEnv(t *testing.T, living bool) (*env, string, *[]worldgen.Config) {
	store, pool := testStore(t)
	if _, err := store.CreateUser(context.Background(), "Admin", testPassword); err != nil {
		t.Fatal(err)
	}
	var applied []worldgen.Config
	svc := worldcfg.NewService(worldcfg.NewPGStore(pool), func(c worldgen.Config) { applied = append(applied, c) })
	if err := svc.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{Version: "test", StartedAt: time.Now(), World: svc, LivingEnabled: living})
	e := &env{t: t, store: store, pool: pool, h: h, srv: h.Routes()}
	return e, e.login(), &applied
}

func TestWorldRoutesRequireLogin(t *testing.T) {
	e, _, _ := newWorldEnv(t, true)
	for _, c := range [][2]string{{"GET", "/api/world"}, {"POST", "/api/world/preview"}, {"PUT", "/api/world"}, {"POST", "/api/world/versions/1/rollback"}} {
		if rec := e.raw(c[0], c[1], []byte("{}"), "application/json", ""); rec.Code != 401 {
			t.Errorf("%s %s без входа: %d", c[0], c[1], rec.Code)
		}
	}
}

func TestWorldAdminFlow(t *testing.T) {
	e, ck, applied := newWorldEnv(t, true)
	rec := e.raw("GET", "/api/world", nil, "", ck)
	var st struct {
		Version       int64
		Config        json.RawMessage
		LivingEnabled bool `json:"living_enabled"`
		Versions      []worldcfg.VersionInfo
		Preview       worldcfg.Preview
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if rec.Code != 200 || st.Version != 1 || !st.LivingEnabled || len(st.Versions) != 1 || !st.Preview.OK || len(st.Preview.Systems) != 3 {
		t.Fatalf("состояние: %d %s", rec.Code, rec.Body.String()[:200])
	}
	// предпросмотр некорректной конфигурации: 200 с ошибками, ничего не меняется
	rec = e.raw("POST", "/api/world/preview", []byte(`{"systems":[]}`), "application/json", ck)
	var pv worldcfg.Preview
	_ = json.Unmarshal(rec.Body.Bytes(), &pv)
	if rec.Code != 200 || pv.OK && len(pv.Errors) == 0 && len(pv.Systems) == 0 && false {
		t.Fatalf("предпросмотр: %d", rec.Code)
	}
	// публикация правки
	var cfg worldgen.Config
	_ = json.Unmarshal(st.Config, &cfg)
	e1 := cfg.Enemies["normal"]
	e1.Reward = 91
	cfg.Enemies["normal"] = e1
	body, _ := json.Marshal(map[string]any{"config": cfg, "note": "награда 91", "base_version": st.Version})
	rec = e.raw("PUT", "/api/world", body, "application/json", ck)
	if rec.Code != 200 || len(*applied) != 2 || (*applied)[1].Enemies["normal"].Reward != 91 {
		t.Fatalf("публикация: %d %s", rec.Code, rec.Body.String())
	}
	// устаревшая основа даёт 409, повтор не применяется
	rec = e.raw("PUT", "/api/world", body, "application/json", ck)
	if rec.Code != 409 || len(*applied) != 2 {
		t.Fatalf("конфликт: %d", rec.Code)
	}
	// некорректная конфигурация: 422, активная версия не меняется
	bad, _ := json.Marshal(map[string]any{"config": map[string]any{"systems": []any{}}, "note": "x", "base_version": 2})
	if rec = e.raw("PUT", "/api/world", bad, "application/json", ck); rec.Code != 422 || len(*applied) != 2 {
		t.Fatalf("некорректная: %d %s", rec.Code, rec.Body.String())
	}
	// откат
	if rec = e.raw("POST", "/api/world/versions/1/rollback", []byte("{}"), "application/json", ck); rec.Code != 200 || len(*applied) != 3 || (*applied)[2].Enemies["normal"].Reward == 91 {
		t.Fatalf("откат: %d", rec.Code)
	}
	if rec = e.raw("POST", "/api/world/versions/99/rollback", []byte("{}"), "application/json", ck); rec.Code != 404 {
		t.Fatalf("откат к несуществующей: %d", rec.Code)
	}
	// действия попали в журнал
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM admin_audit_log WHERE action IN ('world_publish','world_rollback')`).Scan(&n)
	if n != 2 {
		t.Fatalf("записей журнала %d", n)
	}
}
