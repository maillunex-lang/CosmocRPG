package admin

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/design"
)

const designSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor"/></svg>`

func newDesignEnv(t *testing.T, withKey bool) (*env, string) {
	store, pool := testStore(t)
	if _, err := store.CreateUser(context.Background(), "Admin", testPassword); err != nil {
		t.Fatal(err)
	}
	var sg *design.Signer
	if withKey {
		k, _ := rsa.GenerateKey(rand.Reader, 2048)
		der, _ := x509.MarshalPKCS8PrivateKey(k)
		var err error
		sg, err = design.ParseSigner(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
		if err != nil {
			t.Fatal(err)
		}
	}
	svc, err := design.NewService(design.NewPGStore(pool), sg)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{Version: "test", StartedAt: time.Now(), Design: svc})
	e := &env{t: t, store: store, pool: pool, h: h, srv: h.Routes()}
	return e, e.login()
}

func TestDesignRoutesRequireLogin(t *testing.T) {
	e, _ := newDesignEnv(t, true)
	for _, c := range [][2]string{{"GET", "/api/design/status"}, {"GET", "/api/design/elements"}, {"GET", "/api/design/elements/icon:gauge"},
		{"POST", "/api/design/elements/icon:gauge/upload"}, {"POST", "/api/design/elements/icon:gauge/unpublish"}, {"POST", "/api/design/assets/1/publish"},
		{"DELETE", "/api/design/assets/1"}, {"GET", "/api/design/assets/1/file"}, {"GET", "/api/design/prompt?element=icon:gauge"}} {
		if rec := e.raw(c[0], c[1], []byte("x"), "", ""); rec.Code != 401 {
			t.Errorf("%s %s без входа: %d", c[0], c[1], rec.Code)
		}
	}
}

func TestDesignFlow(t *testing.T) {
	e, ck := newDesignEnv(t, true)
	// каталог и фильтры
	rec := e.raw("GET", "/api/design/elements?category=icon&q=gauge", nil, "", ck)
	var list struct {
		Items  []design.ElementView `json:"items"`
		Facets design.Facets        `json:"facets"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list.Items) != 1 || list.Items[0].ID != "icon:gauge" || list.Facets.Categories["model"] != 0 {
		t.Fatalf("список: %d %s", rec.Code, rec.Body.String())
	}
	// плохой файл: 422 и отчёт с причиной
	rec = e.raw("POST", "/api/design/elements/icon:gauge/upload?name=g.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>x</script></svg>`), "application/octet-stream", ck)
	var bad struct {
		Error  string
		Report design.Report
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bad)
	if rec.Code != 422 || bad.Report.OK || len(bad.Report.Errors) == 0 {
		t.Fatalf("плохой файл: %d %s", rec.Code, rec.Body.String())
	}
	// хороший файл
	rec = e.raw("POST", "/api/design/elements/icon:gauge/upload?name=g.svg&notes="+url.QueryEscape("новая версия"), []byte(designSVG), "application/octet-stream", ck)
	var a design.Asset
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	if rec.Code != 201 || a.Version != 1 || a.Status != "draft" || a.Notes != "новая версия" || !a.Report.OK {
		t.Fatalf("загрузка: %d %s", rec.Code, rec.Body.String())
	}
	if rec = e.raw("POST", "/api/design/elements/icon:gauge/upload?name=g2.svg", []byte(designSVG), "", ck); rec.Code != 409 {
		t.Fatalf("тот же файл повторно: %d", rec.Code)
	}
	if rec = e.raw("POST", "/api/design/elements/icon:нет/upload?name=g.svg", []byte(designSVG), "", ck); rec.Code != 404 {
		t.Fatalf("неизвестный элемент: %d", rec.Code)
	}
	if rec = e.raw("POST", "/api/design/elements/icon:gauge/upload?name=g.svg", make([]byte, design.MaxIconBytes+10), "", ck); rec.Code != 413 {
		t.Fatalf("слишком большой файл: %d", rec.Code)
	}
	// предпросмотр файла в песочнице
	rec = e.raw("GET", "/api/design/assets/"+itoa(a.ID)+"/file", nil, "", ck)
	if rec.Code != 200 || rec.Body.String() != designSVG || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("файл: %d %v", rec.Code, rec.Header())
	}
	// публикация и манифест
	if rec = e.raw("POST", "/api/design/assets/"+itoa(a.ID)+"/publish", nil, "", ck); rec.Code != 200 {
		t.Fatalf("публикация: %d %s", rec.Code, rec.Body.String())
	}
	if rec = e.raw("POST", "/api/design/assets/"+itoa(a.ID)+"/publish", nil, "", ck); rec.Code != 409 {
		t.Fatalf("повторная публикация: %d", rec.Code)
	}
	if rec = e.raw("DELETE", "/api/design/assets/"+itoa(a.ID), nil, "", ck); rec.Code != 409 {
		t.Fatalf("удаление опубликованного: %d", rec.Code)
	}
	rec = e.raw("GET", "/api/design/elements/icon:gauge", nil, "", ck)
	var det struct {
		Element design.ElementView `json:"element"`
		Assets  []design.Asset     `json:"assets"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &det)
	if det.Element.State != "replaced" || len(det.Assets) != 1 || det.Assets[0].Status != "published" {
		t.Fatalf("карточка после публикации: %s", rec.Body.String())
	}
	rec = e.raw("GET", "/api/design/status", nil, "", ck)
	var st design.Status
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if !st.CanPublish || st.ManifestVersion != 1 || st.Published != 1 || !strings.Contains(st.PublicKey, "PUBLIC KEY") {
		t.Fatalf("статус: %s", rec.Body.String())
	}
	// снятие и удаление
	if rec = e.raw("POST", "/api/design/elements/icon:gauge/unpublish", nil, "", ck); rec.Code != 200 {
		t.Fatalf("снятие: %d", rec.Code)
	}
	if rec = e.raw("DELETE", "/api/design/assets/"+itoa(a.ID), nil, "", ck); rec.Code != 200 {
		t.Fatalf("удаление: %d", rec.Code)
	}
	// журнал действий
	rec = e.raw("GET", "/api/audit", nil, "", ck)
	for _, act := range []string{"design_upload", "design_publish", "design_unpublish", "design_delete"} {
		if !strings.Contains(rec.Body.String(), act) {
			t.Errorf("в журнале нет действия %s", act)
		}
	}
}

func TestDesignPromptAndNoKey(t *testing.T) {
	e, ck := newDesignEnv(t, false)
	rec := e.raw("GET", "/api/design/prompt?element="+url.QueryEscape("model:COR-001:a"), nil, "", ck)
	var p struct{ Prompt string }
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if rec.Code != 200 || !strings.Contains(p.Prompt, ".glb") || !strings.Contains(p.Prompt, "Барс") {
		t.Fatalf("промпт: %d %s", rec.Code, rec.Body.String())
	}
	if rec = e.raw("GET", "/api/design/prompt?element=нет", nil, "", ck); rec.Code != 404 {
		t.Fatalf("промпт неизвестного элемента: %d", rec.Code)
	}
	rec = e.raw("POST", "/api/design/elements/icon:gauge/upload?name=g.svg", []byte(designSVG), "", ck)
	var a design.Asset
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	if rec.Code != 201 {
		t.Fatalf("черновик без ключа: %d", rec.Code)
	}
	if rec = e.raw("POST", "/api/design/assets/"+itoa(a.ID)+"/publish", nil, "", ck); rec.Code != 412 {
		t.Fatalf("публикация без ключа должна давать 412: %d %s", rec.Code, rec.Body.String())
	}
}
