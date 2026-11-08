package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
)

func newAIEnv(t *testing.T) (*env, string) {
	store, pool := testStore(t)
	if _, err := store.CreateUser(context.Background(), "Admin", testPassword); err != nil {
		t.Fatal(err)
	}
	opts := Options{Version: "test", StartedAt: time.Now(), AI: aistudio.NewService(aistudio.NewPGStore(pool))}
	h := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)), opts)
	e := &env{t: t, store: store, pool: pool, h: h, srv: h.Routes()}
	return e, e.login()
}

func (e *env) raw(method, path string, body []byte, ct, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.7:5555"
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	return rec
}

func TestAIRoutesRequireLogin(t *testing.T) {
	e, _ := newAIEnv(t)
	for _, c := range [][2]string{{"GET", "/api/ai/dashboard"}, {"POST", "/api/ai/generate"}, {"GET", "/api/ai/captains"},
		{"POST", "/api/ai/zip/export"}, {"POST", "/api/ai/zip/import"}, {"GET", "/api/ai/prompt"}, {"POST", "/api/ai/academy/tests"},
		{"POST", "/api/ai/versions/1/rollback"}, {"GET", "/api/ai/dialogue/facts"}} {
		if rec := e.raw(c[0], c[1], []byte("{}"), "application/json", ""); rec.Code != 401 {
			t.Errorf("%s %s без входа: %d", c[0], c[1], rec.Code)
		}
	}
}

func jsonBody(v any) []byte { b, _ := json.Marshal(v); return b }

func TestAIFullCycleOverHTTP(t *testing.T) {
	e, ck := newAIEnv(t)
	// генерация 50 капитанов
	rec := e.raw("POST", "/api/ai/generate", jsonBody(map[string]any{"count": 50}), "application/json", ck)
	if rec.Code != 200 {
		t.Fatalf("генерация: %d %s", rec.Code, rec.Body)
	}
	if rec := e.raw("POST", "/api/ai/generate", jsonBody(map[string]any{"count": 0}), "application/json", ck); rec.Code != 400 {
		t.Errorf("count=0: %d", rec.Code)
	}
	// реестр, фильтры, карточка
	rec = e.raw("GET", "/api/ai/captains?limit=10&formality=formal", nil, "", ck)
	var list struct {
		Total int
		Items []aistudio.CaptainSummary
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || list.Total == 0 || len(list.Items) == 0 || list.Items[0].Formality != "formal" {
		t.Fatalf("реестр: %d %s", rec.Code, rec.Body.String()[:200])
	}
	rec = e.raw("GET", "/api/ai/captains?q=cpt-0007", nil, "", ck)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Fatalf("поиск по id: %d", list.Total)
	}
	if rec := e.raw("GET", "/api/ai/captains/cpt-9999", nil, "", ck); rec.Code != 404 {
		t.Errorf("несуществующий капитан: %d", rec.Code)
	}
	// экспорт одного капитана и повторный импорт
	rec = e.raw("GET", "/api/ai/captains/cpt-0003/export", nil, "", ck)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "cpt-0003.json") {
		t.Fatalf("экспорт капитана: %d", rec.Code)
	}
	one := rec.Body.Bytes()
	if rec := e.raw("POST", "/api/ai/captains/import", one, "application/json", ck); rec.Code != 422 {
		t.Errorf("импорт существующего без подтверждения: %d", rec.Code)
	}
	if rec := e.raw("POST", "/api/ai/captains/import?overwrite=1", one, "application/json", ck); rec.Code != 200 {
		t.Errorf("импорт с заменой: %d %s", rec.Code, rec.Body)
	}
	// правка капитана
	var cp aistudio.Captain
	_ = json.Unmarshal(one, &cp)
	cp.Bio += " Дописано владельцем вручную, чтобы проверить сохранение правки и защиту от перезаписи."
	if rec := e.raw("PUT", "/api/ai/captains/cpt-0003", jsonBody(cp), "application/json", ck); rec.Code != 200 {
		t.Fatalf("правка: %d %s", rec.Code, rec.Body)
	}
	cp.Gender = "x"
	if rec := e.raw("PUT", "/api/ai/captains/cpt-0003", jsonBody(cp), "application/json", ck); rec.Code != 422 {
		t.Errorf("некорректная правка: %d", rec.Code)
	}
	// шаблон и партия
	rec = e.raw("POST", "/api/ai/zip/export", jsonBody(map[string]any{"kind": "template"}), "application/json", ck)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("шаблон: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if _, rep := aistudio.ParsePack(rec.Body.Bytes()); !rep.OK {
		t.Fatalf("шаблон не проходит проверку: %+v", rep.Errors)
	}
	rec = e.raw("POST", "/api/ai/zip/export", jsonBody(map[string]any{"kind": "batch", "size": 25, "index": 1}), "application/json", ck)
	if rec.Code != 200 {
		t.Fatalf("партия: %d %s", rec.Code, rec.Body)
	}
	batch := rec.Body.Bytes()
	if rec := e.raw("POST", "/api/ai/zip/export", jsonBody(map[string]any{"kind": "batch", "size": 25, "index": 9}), "application/json", ck); rec.Code != 422 {
		t.Errorf("несуществующая партия: %d", rec.Code)
	}
	// импорт: неверный тип содержимого и не-ZIP
	if rec := e.raw("POST", "/api/ai/zip/import", batch, "application/json", ck); rec.Code != 415 {
		t.Errorf("импорт без application/zip: %d", rec.Code)
	}
	rec = e.raw("POST", "/api/ai/zip/import?filename=bad.zip", []byte("мусор"), "application/zip", ck)
	var bad aistudio.ImportRec
	_ = json.Unmarshal(rec.Body.Bytes(), &bad)
	if rec.Code != 200 || bad.Status != "rejected" || bad.Report.OK || len(bad.Report.Errors) == 0 {
		t.Fatalf("мусор должен давать отклонённый отчёт: %d %+v", rec.Code, bad.Status)
	}
	// «обученный» пакет: меняем биографию одного капитана и пересобираем с верными суммами
	trained := retrain(t, batch)
	rec = e.raw("POST", "/api/ai/zip/import?filename=trained.zip", trained, "application/zip", ck)
	var imp aistudio.ImportRec
	_ = json.Unmarshal(rec.Body.Bytes(), &imp)
	if rec.Code != 200 || imp.Status != "validated" || imp.Plan.Changed != 1 || imp.Report.Integrity != "verified" {
		t.Fatalf("предпросмотр: %d статус=%s план=%s ошибки=%+v", rec.Code, imp.Status, imp.Plan.Summary(), imp.Report.Errors)
	}
	// публикация и откат
	rec = e.raw("POST", "/api/ai/imports/"+itoa(imp.ID)+"/publish", jsonBody(map[string]any{"accept_warnings": true}), "application/json", ck)
	if rec.Code != 200 {
		t.Fatalf("публикация: %d %s", rec.Code, rec.Body)
	}
	if rec := e.raw("POST", "/api/ai/imports/"+itoa(imp.ID)+"/publish", jsonBody(map[string]any{"accept_warnings": true}), "application/json", ck); rec.Code != 422 {
		t.Errorf("повторная публикация: %d", rec.Code)
	}
	rec = e.raw("GET", "/api/ai/versions", nil, "", ck)
	var vs struct{ Items []aistudio.VersionInfo }
	_ = json.Unmarshal(rec.Body.Bytes(), &vs)
	if len(vs.Items) < 3 {
		t.Fatalf("версий %d", len(vs.Items))
	}
	if rec := e.raw("POST", "/api/ai/versions/"+itoa(vs.Items[len(vs.Items)-1].ID)+"/rollback", []byte("{}"), "application/json", ck); rec.Code != 200 {
		t.Errorf("откат: %d %s", rec.Code, rec.Body)
	}
	if rec := e.raw("POST", "/api/ai/versions/999/rollback", []byte("{}"), "application/json", ck); rec.Code != 404 {
		t.Errorf("откат несуществующей: %d", rec.Code)
	}
	// промпт
	rec = e.raw("GET", "/api/ai/prompt?batch=25", nil, "", ck)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "AI_Civilization_Pack_v1") || !strings.Contains(rec.Body.String(), "25 капитанов") {
		t.Errorf("промпт: %d", rec.Code)
	}
	// симулятор, группа, тесты
	rec = e.raw("POST", "/api/ai/academy/simulate", jsonBody(map[string]any{"captain_id": "cpt-0002", "say": []string{"привет", "как включить автопилот"}}), "application/json", ck)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "transcript") {
		t.Errorf("симулятор: %d", rec.Code)
	}
	rec = e.raw("POST", "/api/ai/academy/group", jsonBody(map[string]any{"ids": []string{"cpt-0001", "cpt-0002"}, "opening": "привет", "turns": 6}), "application/json", ck)
	if rec.Code != 200 {
		t.Errorf("группа: %d %s", rec.Code, rec.Body)
	}
	rec = e.raw("POST", "/api/ai/academy/tests", jsonBody(map[string]any{"kinds": []string{"structure", "facts"}, "sample": 10}), "application/json", ck)
	if rec.Code != 200 {
		t.Fatalf("тесты: %d %s", rec.Code, rec.Body)
	}
	if rec := e.raw("POST", "/api/ai/academy/tests", jsonBody(map[string]any{"kinds": []string{"нет"}}), "application/json", ck); rec.Code != 400 {
		t.Errorf("неизвестная проверка: %d", rec.Code)
	}
	// сводка, отчёты, редактор раздела
	rec = e.raw("GET", "/api/ai/dashboard", nil, "", ck)
	var d aistudio.Dashboard
	_ = json.Unmarshal(rec.Body.Bytes(), &d)
	if rec.Code != 200 || d.Captains != 50 || d.LastTests == nil || len(d.Imports) < 2 || len(d.Sessions) < 2 {
		t.Fatalf("сводка: %d %+v", rec.Code, d.Captains)
	}
	rec = e.raw("GET", "/api/ai/dialogue/facts", nil, "", ck)
	var sec struct {
		Doc json.RawMessage `json:"doc"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sec)
	if rec.Code != 200 || !strings.Contains(string(sec.Doc), "station_rubezh") {
		t.Fatalf("раздел: %d", rec.Code)
	}
	if rec := e.raw("PUT", "/api/ai/dialogue/facts", []byte(`{"facts":[{"id":"BAD","text":"x"}]}`), "application/json", ck); rec.Code != 422 {
		t.Errorf("некорректный раздел: %d", rec.Code)
	}
	if rec := e.raw("GET", "/api/ai/dialogue/нет", nil, "", ck); rec.Code != 404 {
		t.Errorf("неизвестный раздел: %d", rec.Code)
	}
	rec = e.raw("GET", "/api/ai/uniqueness", nil, "", ck)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "duplicates") {
		t.Errorf("уникальность: %d", rec.Code)
	}
	// журнал действий
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM admin_audit_log WHERE action LIKE 'ai_%'`).Scan(&n)
	if n < 8 {
		t.Errorf("в журнал записано %d действий AI", n)
	}
}

func itoa(n int64) string    { return strings.TrimSpace(strings.Replace(jsonNum(n), "\n", "", -1)) }
func jsonNum(n int64) string { b, _ := json.Marshal(n); return string(b) }

// retrain имитирует работу ChatGPT: меняет биографию одного капитана и пересобирает пакет с верными суммами.
func retrain(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = b
	}
	var cp aistudio.Captain
	_ = json.Unmarshal(files["captains/cpt-0001.json"], &cp)
	cp.Bio += " Дополнено при обучении: капитан помнит всех, с кем летал, и всегда рассказывает об этом с теплом."
	cp.Revision++
	cp.Meta.Source = "chatgpt"
	files["captains/cpt-0001.json"], _ = json.MarshalIndent(cp, "", "  ")
	var man aistudio.Manifest
	_ = json.Unmarshal(files["manifest.json"], &man)
	man.PackID, man.CreatedBy, man.Files = "chatgpt-http01", "chatgpt", nil
	for n, b := range files {
		if n != "manifest.json" {
			man.Files = append(man.Files, aistudio.ManifestFile{Path: n, SHA256: shaHex(b)})
		}
	}
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

func shaHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
