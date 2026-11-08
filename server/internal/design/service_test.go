package design

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testSigner(t *testing.T) (*Signer, *rsa.PublicKey) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	s, err := ParseSigner(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if err != nil {
		t.Fatal(err)
	}
	return s, &k.PublicKey
}

func newSvc(t *testing.T, withKey bool) (*Service, *rsa.PublicKey) {
	t.Helper()
	var sg *Signer
	var pub *rsa.PublicKey
	if withKey {
		sg, pub = testSigner(t)
	}
	s, err := NewService(NewMemStore(), sg)
	if err != nil {
		t.Fatal(err)
	}
	return s, pub
}

const goodSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor"/></svg>`

func TestUploadPublishManifestFlow(t *testing.T) {
	s, pub := newSvc(t, true)
	ctx := context.Background()
	a, err := s.Upload(ctx, "icon:gauge", "gauge.svg", []byte(goodSVG), "первая версия", "owner")
	if err != nil || a.Version != 1 || a.Status != StatusDraft || a.Mime != "image/svg+xml" {
		t.Fatalf("загрузка: %+v %v", a, err)
	}
	if _, err := s.Upload(ctx, "icon:gauge", "gauge2.svg", []byte(goodSVG), "", "owner"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("тот же файл должен отклоняться: %v", err)
	}
	if _, err := s.Upload(ctx, "icon:нет", "a.svg", []byte(goodSVG), "", "owner"); !errors.Is(err, ErrNoElement) {
		t.Fatalf("неизвестный элемент: %v", err)
	}
	var inv *InvalidError
	if _, err := s.Upload(ctx, "icon:gauge", "x.svg", []byte(`<svg><script/></svg>`), "", "owner"); !errors.As(err, &inv) || inv.Report.OK {
		t.Fatalf("плохой файл должен давать отчёт: %v", err)
	}
	for _, bad := range []string{"", "../x.svg", "a/b.svg", strings.Repeat("a", 130) + ".svg"} {
		if _, err := s.Upload(ctx, "icon:gauge", bad, []byte(goodSVG+" "), "", "owner"); err == nil {
			t.Errorf("имя файла %q должно отклоняться", bad)
		}
	}
	v, _, _ := s.Detail(ctx, "icon:gauge")
	if v.State != "draft" || v.Drafts != 1 || v.Published != nil {
		t.Fatalf("после загрузки: %+v", v)
	}
	if _, err := s.Publish(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	v, hist, _ := s.Detail(ctx, "icon:gauge")
	if v.State != "replaced" || v.Published == nil || len(hist) != 1 {
		t.Fatalf("после публикации: %+v", v)
	}
	body, sig, ver, err := s.Manifest(ctx)
	if err != nil || ver != 1 || !Verify(pub, body, sig) {
		t.Fatalf("манифест: ver=%d err=%v", ver, err)
	}
	var m struct {
		Schema  string
		Version int64
		Assets  []ManifestEntry
	}
	_ = json.Unmarshal(body, &m)
	if m.Schema != ManifestSchema || len(m.Assets) != 1 || m.Assets[0].Element != "icon:gauge" || m.Assets[0].File != a.SHA256+".svg" || m.Assets[0].Category != CatIcon {
		t.Fatalf("содержимое манифеста: %s", body)
	}
	if strings.Contains(string(body), "http") {
		t.Fatal("в манифесте не должно быть адресов: клиент строит их сам")
	}
	body[10] ^= 1
	if Verify(pub, body, sig) {
		t.Fatal("подпись должна ломаться при изменении манифеста")
	}
	// вторая версия вытесняет первую
	svg2 := strings.Replace(goodSVG, `r="9"`, `r="8"`, 1)
	b, _ := s.Upload(ctx, "icon:gauge", "gauge.svg", []byte(svg2), "", "owner")
	if b.Version != 2 {
		t.Fatalf("версия: %d", b.Version)
	}
	if _, err := s.Publish(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	body, _, ver, _ = s.Manifest(ctx)
	if ver != 2 || !strings.Contains(string(body), b.SHA256) || strings.Contains(string(body), a.SHA256) {
		t.Fatalf("после второй публикации: %s", body)
	}
	if err := s.Delete(ctx, b.ID); !errors.Is(err, ErrPublished) {
		t.Fatalf("опубликованное удалять нельзя: %v", err)
	}
	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatalf("архивную можно удалить: %v", err)
	}
	if _, err := s.Publish(ctx, b.ID); !errors.Is(err, ErrState) {
		t.Fatalf("повторная публикация: %v", err)
	}
	if _, err := s.Unpublish(ctx, "icon:gauge"); err != nil {
		t.Fatal(err)
	}
	body, _, ver, _ = s.Manifest(ctx)
	if ver != 3 || strings.Contains(string(body), b.SHA256) {
		t.Fatalf("после снятия: ver=%d %s", ver, body)
	}
	if _, err := s.Unpublish(ctx, "icon:gauge"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("снимать нечего: %v", err)
	}
}

func TestNoSignerBlocksPublishButNotUpload(t *testing.T) {
	s, _ := newSvc(t, false)
	ctx := context.Background()
	a, err := s.Upload(ctx, "icon:gauge", "g.svg", []byte(goodSVG), "", "owner")
	if err != nil {
		t.Fatalf("черновики без ключа работают: %v", err)
	}
	if _, err := s.Publish(ctx, a.ID); !errors.Is(err, ErrNoSigner) {
		t.Fatalf("публикация без ключа: %v", err)
	}
	if _, _, _, err := s.Manifest(ctx); !errors.Is(err, ErrNoSigner) {
		t.Fatalf("манифест без ключа: %v", err)
	}
	if st, _ := s.Status(ctx); st.CanPublish {
		t.Fatal("статус: публикация недоступна")
	}
}

func TestPublicHandler(t *testing.T) {
	s, pub := newSvc(t, true)
	ctx := context.Background()
	a, _ := s.Upload(ctx, "icon:gauge", "g.svg", []byte(goodSVG), "", "owner")
	if _, err := s.Publish(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	draft, _ := s.Upload(ctx, "icon:coins", "c.svg", []byte(strings.Replace(goodSVG, "r=\"9\"", "r=\"7\"", 1)), "", "owner")
	srv := httptest.NewServer(s.PublicHandler())
	defer srv.Close()
	get := func(path string) (*http.Response, []byte) {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		buf := make([]byte, 1<<20)
		n, _ := r.Body.Read(buf)
		return r, buf[:n]
	}
	r, body := get("/assets/manifest.json")
	_, sig := get("/assets/manifest.json.sig")
	if r.StatusCode != 200 || !Verify(pub, body, string(sig)) || r.Header.Get("X-Assets-Version") != "1" {
		t.Fatalf("манифест и подпись по HTTP: %d", r.StatusCode)
	}
	r, file := get("/assets/files/" + a.SHA256 + ".svg")
	if r.StatusCode != 200 || string(file) != goodSVG || !strings.Contains(r.Header.Get("Cache-Control"), "immutable") || r.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("файл: %d %q", r.StatusCode, r.Header)
	}
	for _, p := range []string{"/assets/files/" + draft.SHA256 + ".svg", "/assets/files/" + a.SHA256 + ".png", "/assets/files/нет.svg", "/assets/files/%2e%2e%2fmanifest.json", "/assets/files/" + strings.Repeat("a", 64) + ".svg"} {
		if r, _ := get(p); r.StatusCode != 404 {
			t.Errorf("%s: ожидался 404, получен %d (черновики и чужие расширения не раздаются)", p, r.StatusCode)
		}
	}
	noKey, _ := NewService(NewMemStore(), nil)
	srv2 := httptest.NewServer(noKey.PublicHandler())
	defer srv2.Close()
	if r, _ := http.Get(srv2.URL + "/assets/manifest.json"); r.StatusCode != 503 {
		t.Fatalf("без ключа манифест недоступен: %d", r.StatusCode)
	}
}

func TestElementsFilterAndFacets(t *testing.T) {
	s, _ := newSvc(t, true)
	ctx := context.Background()
	a, _ := s.Upload(ctx, "icon:gauge", "g.svg", []byte(goodSVG), "", "owner")
	_, _ = s.Publish(ctx, a.ID)
	all, fc, err := s.Elements(ctx, Filter{})
	if err != nil || len(all) != 57 || fc.Categories[CatIcon] != 28 || fc.States["replaced"] != 1 || fc.States["builtin"] != 56 {
		t.Fatalf("без фильтра: %d %+v %v", len(all), fc, err)
	}
	icons, _, _ := s.Elements(ctx, Filter{Category: CatIcon})
	if len(icons) != 28 {
		t.Fatalf("иконки: %d", len(icons))
	}
	models, fc, _ := s.Elements(ctx, Filter{Category: CatModel, Tag: "заглушка"})
	if len(models) != 12 || fc.Groups["3D-модели кораблей"] != 12 {
		t.Fatalf("модели-заглушки: %d %+v", len(models), fc.Groups)
	}
	if got, _, _ := s.Elements(ctx, Filter{Q: "стриж"}); len(got) < 2 {
		t.Fatalf("поиск по названию: %d", len(got))
	}
	if got, _, _ := s.Elements(ctx, Filter{Q: "ftr-001 модель"}); len(got) == 0 {
		t.Fatal("поиск по нескольким словам")
	}
	if got, _, _ := s.Elements(ctx, Filter{State: "replaced"}); len(got) != 1 || got[0].ID != "icon:gauge" {
		t.Fatalf("фильтр по состоянию: %+v", got)
	}
	if got, _, _ := s.Elements(ctx, Filter{Q: "нет-такого-слова"}); len(got) != 0 {
		t.Fatal("пустая выдача")
	}
}

func TestPromptsForEveryElement(t *testing.T) {
	s, _ := newSvc(t, false)
	cat, _ := LoadCatalog()
	for _, e := range cat.All() {
		p, err := s.Prompt(e.ID)
		if err != nil || len(p) < 600 || !strings.Contains(p, e.Name) || !strings.Contains(p, "ОБЩИЕ ПРАВИЛА") {
			t.Fatalf("промпт для %s: %v (%d симв.)", e.ID, err, len(p))
		}
		switch e.Category {
		case CatModel:
			if !strings.Contains(p, ".glb") || !strings.Contains(p, "Engine_L") || !strings.Contains(p, "120 000") {
				t.Fatalf("промпт модели %s неполный", e.ID)
			}
		case CatIcon:
			if !strings.Contains(p, "currentColor") || !strings.Contains(p, "<svg") {
				t.Fatalf("промпт иконки %s без требований и образца", e.ID)
			}
		case CatShipData:
			if !strings.Contains(p, e.Ship) || !strings.Contains(p, "\"skins\"") {
				t.Fatalf("промпт паспорта %s без текущего JSON", e.ID)
			}
		case CatBackground:
			if !strings.Contains(p, "WebP") {
				t.Fatalf("промпт фона %s", e.ID)
			}
		}
	}
	if _, err := s.Prompt("нет"); !errors.Is(err, ErrNoElement) {
		t.Fatal("неизвестный элемент")
	}
}

func TestSignerRejectsWeakAndBadKeys(t *testing.T) {
	if _, err := ParseSigner([]byte("не ключ")); err == nil {
		t.Fatal("мусор принят как ключ")
	}
	weak, _ := rsa.GenerateKey(rand.Reader, 1024)
	der, _ := x509.MarshalPKCS8PrivateKey(weak)
	if _, err := ParseSigner(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})); err == nil {
		t.Fatal("ключ короче 2048 бит принят")
	}
}
