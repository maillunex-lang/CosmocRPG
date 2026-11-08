package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func get(t *testing.T, db Pinger) (int, map[string]string) {
	t.Helper()
	mux := NewMux(db, "test", http.NotFoundHandler())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ответ не JSON: %v", err)
	}
	return rec.Code, body
}

func TestHealthOK(t *testing.T) {
	code, body := get(t, fakeDB{})
	if code != 200 || body["status"] != "ok" || body["database"] != "ok" || body["version"] != "test" {
		t.Fatalf("code=%d body=%v", code, body)
	}
}

func TestHealthDatabaseDown(t *testing.T) {
	code, body := get(t, fakeDB{err: errors.New("down")})
	if code != 503 || body["status"] != "degraded" || body["database"] != "unavailable" {
		t.Fatalf("code=%d body=%v", code, body)
	}
}

func TestHealthRejectsPost(t *testing.T) {
	mux := NewMux(fakeDB{}, "test", http.NotFoundHandler())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("ожидался 405, получен %d", rec.Code)
	}
}
