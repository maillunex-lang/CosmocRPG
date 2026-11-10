package media

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// PublicHandler раздаёт каталог и файлы только на чтение: GET /media/catalog.json и GET /media/files/<sha>.<ext>.
func (s *Service) PublicHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		items, err := s.PublicList(r.Context())
		if err != nil {
			http.Error(w, "каталог недоступен", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(map[string]any{"tracks": items})
	})
	mux.HandleFunc("GET /media/files/{name}", func(w http.ResponseWriter, r *http.Request) {
		b, mime, err := s.Read(r.PathValue("name"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		_, _ = w.Write(b)
	})
	return mux
}
