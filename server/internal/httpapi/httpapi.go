// Package httpapi — HTTP-маршруты сервера.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Pinger проверяет доступность базы данных.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewMux собирает маршруты: /health и /ws.
func NewMux(db Pinger, version string, wsHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, code, dbState := "ok", http.StatusOK, "ok"
		if err := db.Ping(ctx); err != nil {
			status, code, dbState = "degraded", http.StatusServiceUnavailable, "unavailable"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": status, "database": dbState, "version": version,
		})
	})
	mux.Handle("GET /ws", wsHandler)
	return mux
}
