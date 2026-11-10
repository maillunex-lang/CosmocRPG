package media

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type memStore struct {
	items  []Track
	nextID int64
}

func (m *memStore) Insert(_ context.Context, t Track) (Track, error) {
	for _, x := range m.items {
		if x.SHA256 == t.SHA256 {
			return Track{}, ErrDuplicate
		}
	}
	m.nextID++
	t.ID = m.nextID
	m.items = append(m.items, t)
	return t, nil
}

func (m *memStore) UpdateMeta(_ context.Context, t Track) (Track, error) {
	for i := range m.items {
		if m.items[i].ID == t.ID {
			prev := m.items[i]
			t.SHA256, t.Size, t.Ext, t.MIME, t.DurationMS, t.UploadedBy, t.CreatedAt = prev.SHA256, prev.Size, prev.Ext, prev.MIME, prev.DurationMS, prev.UploadedBy, prev.CreatedAt
			m.items[i] = t
			return t, nil
		}
	}
	return Track{}, ErrNotFound
}

func (m *memStore) Delete(_ context.Context, id int64) (Track, error) {
	for i, t := range m.items {
		if t.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return t, nil
		}
	}
	return Track{}, ErrNotFound
}

func (m *memStore) ByID(_ context.Context, id int64) (Track, error) {
	for _, t := range m.items {
		if t.ID == id {
			return t, nil
		}
	}
	return Track{}, ErrNotFound
}

func (m *memStore) BySHA(_ context.Context, sha string) (Track, error) {
	for _, t := range m.items {
		if t.SHA256 == sha {
			return t, nil
		}
	}
	return Track{}, ErrNotFound
}

func (m *memStore) List(context.Context) ([]Track, error) {
	out := append([]Track(nil), m.items...)
	return out, nil
}

func TestUploadListDisableAndPublic(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(&memStore{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	data := mp3FrameBytes()
	tr, err := svc.Upload(ctx, "across.mp3", data, Meta{Title: "Тихая вода", Artist: "Рубеж", Kind: "music", AIGenerated: true, Enabled: true}, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Source != "Google Gemini" || !tr.AIGenerated || !tr.Enabled || tr.URL == "" {
		t.Fatalf("%+v", tr)
	}
	if _, err := os.Stat(filepath.Join(dir, tr.SHA256+".mp3")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Upload(ctx, "across.mp3", data, Meta{Title: "ещё"}, "owner"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("повтор: %v", err)
	}
	off, err := svc.Update(ctx, tr.ID, Meta{Title: tr.Title, Kind: "music", Enabled: false})
	if err != nil || off.Enabled {
		t.Fatal(err, off.Enabled)
	}
	pub, err := svc.PublicList(ctx)
	if err != nil || len(pub) != 0 {
		t.Fatalf("выключенный трек попал в каталог: %+v %v", pub, err)
	}
	body, mime, err := svc.Read(tr.SHA256 + ".mp3")
	if err != nil || mime != "audio/mpeg" || len(body) != len(data) {
		t.Fatalf("файл: %v %s %d", err, mime, len(body))
	}
	if _, _, err := svc.Read("../" + tr.SHA256 + ".mp3"); err == nil {
		t.Fatal("обход каталога")
	}
	h := svc.PublicHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/catalog.json", nil))
	if rec.Code != 200 || !bytesContains(rec.Body.Bytes(), []byte(`"tracks"`)) {
		t.Fatalf("каталог %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tr.URL, nil))
	if rec.Code != 200 || rec.Body.Len() != len(data) {
		t.Fatalf("раздача %d %d", rec.Code, rec.Body.Len())
	}
	if err := svc.Delete(ctx, tr.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, tr.SHA256+".mp3")); !os.IsNotExist(err) {
		t.Fatalf("файл остался: %v", err)
	}
}

func TestUploadRejectsBadTitle(t *testing.T) {
	svc, err := NewService(&memStore{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Upload(context.Background(), "a.mp3", mp3FrameBytes(), Meta{}, "owner")
	var inv *InvalidError
	if !errors.As(err, &inv) {
		t.Fatal(err)
	}
}

func bytesContains(b, sub []byte) bool {
	return len(b) >= len(sub) && (string(b) == string(sub) || len(sub) == 0 || indexOf(b, sub) >= 0)
}

func indexOf(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		ok := true
		for j := range sub {
			if b[i+j] != sub[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
