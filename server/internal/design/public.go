package design

import (
	"net/http"
	"regexp"
	"strconv"
)

var fileRe = regexp.MustCompile(`^([0-9a-f]{64})\.(glb|png|jpg|webp|svg|json)$`)

// PublicHandler раздаёт клиентам подписанный манифест ресурсов и файлы по сумме. Маршруты: /assets/manifest.json,
// /assets/manifest.json.sig, /assets/files/<sha256>.<расширение>. Без аутентификации: содержимое публичное и подписано.
func (s *Service) PublicHandler() http.Handler {
	mux := http.NewServeMux()
	serve := func(w http.ResponseWriter, r *http.Request, sigOnly bool) {
		body, sig, v, err := s.Manifest(r.Context())
		if err != nil {
			http.Error(w, "манифест недоступен", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Assets-Version", strconv.FormatInt(v, 10))
		if sigOnly {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(sig))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
	mux.HandleFunc("GET /assets/manifest.json", func(w http.ResponseWriter, r *http.Request) { serve(w, r, false) })
	mux.HandleFunc("GET /assets/manifest.json.sig", func(w http.ResponseWriter, r *http.Request) { serve(w, r, true) })
	mux.HandleFunc("GET /assets/files/{name}", func(w http.ResponseWriter, r *http.Request) {
		m := fileRe.FindStringSubmatch(r.PathValue("name"))
		if m == nil {
			http.NotFound(w, r)
			return
		}
		a, err := s.store.BySHA(r.Context(), m[1])
		if err != nil || a.Ext() != m[2] {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Content-Length", strconv.Itoa(len(a.Data)))
		_, _ = w.Write(a.Data)
	})
	return mux
}
