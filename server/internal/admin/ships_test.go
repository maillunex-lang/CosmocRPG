package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func newShipEnv(t *testing.T) (*env, string, *world.World) {
	store, pool := testStore(t)
	if _, err := store.CreateUser(context.Background(), "Admin", testPassword); err != nil {
		t.Fatal(err)
	}
	w := world.New()
	ships := shipcatalog.NewService(shipcatalog.NewPGStore(pool), w.SetCatalog)
	if err := ships.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{Version: "test", StartedAt: time.Now(), Ships: ships})
	e := &env{t: t, store: store, pool: pool, h: h, srv: h.Routes()}
	return e, e.login(), w
}

func TestShipRoutesRequireLogin(t *testing.T) {
	e, _, _ := newShipEnv(t)
	for _, c := range [][2]string{{"GET", "/api/ships"}, {"PUT", "/api/ships/FTR-001"}, {"POST", "/api/ships/patch/export"}, {"POST", "/api/ships/patch/import"},
		{"POST", "/api/ships/patch/x/publish"}, {"GET", "/api/ships/versions"}, {"POST", "/api/ships/versions/1/rollback"}, {"GET", "/api/ships/prompt"}} {
		if rec := e.raw(c[0], c[1], []byte("{}"), "application/json", ""); rec.Code != 401 {
			t.Errorf("%s %s без входа: %d", c[0], c[1], rec.Code)
		}
	}
}

func TestShipAdminFlow(t *testing.T) {
	e, ck, w := newShipEnv(t)
	rec := e.raw("GET", "/api/ships", nil, "", ck)
	var list struct {
		Version  int64
		Ships    []world.ShipSpec
		Starters map[string]world.Starter
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || list.Version != 1 || len(list.Ships) != 12 || len(list.Starters) != 4 {
		t.Fatalf("список: %d %s", rec.Code, rec.Body.String()[:120])
	}
	var strizh world.ShipSpec
	for _, s := range list.Ships {
		if s.ID == "FTR-001" {
			strizh = s
		}
	}
	// правка параметров применяется к миру на лету
	strizh.Speed, strizh.Cargo = 17, 12
	if rec := e.raw("PUT", "/api/ships/FTR-001", jsonBody(strizh), "application/json", ck); rec.Code != 200 {
		t.Fatalf("правка: %d %s", rec.Code, rec.Body)
	}
	if got := w.Catalog().Ships["FTR-001"]; got.Speed != 17 || got.Cargo != 12 {
		t.Fatalf("мир не получил правку: %+v", got.Speed)
	}
	strizh.Hull = 1
	if rec := e.raw("PUT", "/api/ships/FTR-001", jsonBody(strizh), "application/json", ck); rec.Code != 422 {
		t.Errorf("значение вне диапазона: %d", rec.Code)
	}
	if rec := e.raw("PUT", "/api/ships/FTR-002", jsonBody(strizh), "application/json", ck); rec.Code != 400 {
		t.Errorf("несовпадение id: %d", rec.Code)
	}
	if rec := e.raw("PUT", "/api/ships/FTR-001", []byte(`{"id":"FTR-001","god_mode":true}`), "application/json", ck); rec.Code != 400 {
		t.Errorf("лишнее поле: %d", rec.Code)
	}
	// ZIP-патч: экспорт, правка, импорт, публикация
	rec = e.raw("POST", "/api/ships/patch/export", jsonBody(map[string]any{"ids": []string{"TMP-MINE"}}), "application/json", ck)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("экспорт патча: %d", rec.Code)
	}
	patch := rebuildShipPatch(t, rec.Body.Bytes(), func(m map[string]any) { m["cargo"] = 99; m["speed"] = 12.5 })
	if rec := e.raw("POST", "/api/ships/patch/import", patch, "application/json", ck); rec.Code != 415 {
		t.Errorf("тип содержимого: %d", rec.Code)
	}
	rec = e.raw("POST", "/api/ships/patch/import?filename=p.zip", patch, "application/zip", ck)
	var pv shipcatalog.Preview
	_ = json.Unmarshal(rec.Body.Bytes(), &pv)
	if rec.Code != 200 || pv.Status != "ready" || len(pv.Errors) != 0 || pv.Integrity != "verified" {
		t.Fatalf("предпросмотр: %d %+v", rec.Code, pv.Errors)
	}
	if got := w.Catalog().Ships["TMP-MINE"]; got.Cargo != 40 {
		t.Fatal("предпросмотр изменил мир")
	}
	rec = e.raw("POST", "/api/ships/patch/"+pv.ID+"/publish", []byte("{}"), "application/json", ck)
	if rec.Code != 200 {
		t.Fatalf("публикация: %d %s", rec.Code, rec.Body)
	}
	if got := w.Catalog().Ships["TMP-MINE"]; got.Cargo != 99 || got.Speed != 12.5 {
		t.Fatalf("патч не применён: %+v", got)
	}
	// плохой патч отклоняется и ничего не меняет
	bad := rebuildShipPatch(t, rec.Body.Bytes(), nil)
	_ = bad
	badZip := e.raw("POST", "/api/ships/patch/import", []byte("мусор"), "application/zip", ck)
	var bp shipcatalog.Preview
	_ = json.Unmarshal(badZip.Body.Bytes(), &bp)
	if bp.Status != "rejected" || len(bp.Errors) == 0 {
		t.Fatalf("мусор: %+v", bp)
	}
	if rec := e.raw("POST", "/api/ships/patch/"+bp.ID+"/publish", []byte("{}"), "application/json", ck); rec.Code != 422 {
		t.Errorf("публикация отклонённого: %d", rec.Code)
	}
	// версии и откат
	rec = e.raw("GET", "/api/ships/versions", nil, "", ck)
	var vs struct{ Items []shipcatalog.VersionInfo }
	_ = json.Unmarshal(rec.Body.Bytes(), &vs)
	if len(vs.Items) != 3 {
		t.Fatalf("версий %d", len(vs.Items))
	}
	if rec := e.raw("POST", "/api/ships/versions/1/rollback", []byte("{}"), "application/json", ck); rec.Code != 200 {
		t.Fatalf("откат: %d %s", rec.Code, rec.Body)
	}
	if got := w.Catalog().Ships["FTR-001"]; got.Speed != 14 {
		t.Fatalf("после отката скорость %.1f", got.Speed)
	}
	if rec := e.raw("POST", "/api/ships/versions/999/rollback", []byte("{}"), "application/json", ck); rec.Code != 404 {
		t.Errorf("откат несуществующей: %d", rec.Code)
	}
	if rec := e.raw("GET", "/api/ships/prompt", nil, "", ck); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Ship_Patch_Pack_v1") {
		t.Errorf("промпт: %d", rec.Code)
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM admin_audit_log WHERE action LIKE 'ship_%'`).Scan(&n)
	if n < 5 {
		t.Errorf("в журнал записано %d действий над кораблями", n)
	}
}

// rebuildShipPatch меняет паспорт TMP-MINE в экспортированном патче и пересобирает архив с верными суммами.
func rebuildShipPatch(t *testing.T, data []byte, f func(m map[string]any)) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	files := map[string][]byte{}
	for _, x := range zr.File {
		rc, _ := x.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[x.Name] = b
	}
	if f != nil {
		var m map[string]any
		_ = json.Unmarshal(files["ships/TMP-MINE.json"], &m)
		f(m)
		files["ships/TMP-MINE.json"], _ = json.Marshal(m)
	}
	var man map[string]any
	_ = json.Unmarshal(files["manifest.json"], &man)
	var list []map[string]string
	for n, b := range files {
		if n != "manifest.json" {
			list = append(list, map[string]string{"path": n, "sha256": shaHex(b)})
		}
	}
	man["files"], man["pack_id"] = list, "chatgpt-adm01"
	files["manifest.json"], _ = json.Marshal(man)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, b := range files {
		w, _ := zw.Create(n)
		_, _ = w.Write(b)
	}
	_ = zw.Close()
	return buf.Bytes()
}
