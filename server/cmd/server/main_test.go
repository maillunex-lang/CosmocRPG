package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthcheckURL(t *testing.T) {
	for addr, want := range map[string]string{
		"":               "http://127.0.0.1:8080/health",
		":8080":          "http://127.0.0.1:8080/health",
		"0.0.0.0:9000":   "http://127.0.0.1:9000/health",
		"127.0.0.1:7777": "http://127.0.0.1:7777/health",
	} {
		if got := healthcheckURL(addr); got != want {
			t.Errorf("addr %q: %s, ожидалось %s", addr, got, want)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	if err := healthcheck(addr); err != nil {
		t.Fatalf("здоровый сервер: %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := healthcheck(addr); err == nil {
		t.Fatal("статус 503 должен давать ошибку")
	}
	srv.Close()
	if err := healthcheck(addr); err == nil {
		t.Fatal("недоступный сервер должен давать ошибку")
	}
}
