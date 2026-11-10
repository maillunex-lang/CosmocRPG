package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldcfg"
)

func (h *Handler) worldRoutes(mux *http.ServeMux) {
	a := h.auth
	mux.HandleFunc("GET /api/world", a(h.worldGet))
	mux.HandleFunc("POST /api/world/preview", a(h.worldPreview))
	mux.HandleFunc("PUT /api/world", a(h.worldPublish))
	mux.HandleFunc("POST /api/world/versions/{id}/rollback", a(h.worldRollback))
}

func (h *Handler) worldFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, worldcfg.ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, worldcfg.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	}
}

func (h *Handler) worldGet(w http.ResponseWriter, r *http.Request, _ User) {
	raw, ver, err := h.opts.World.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	vs, _ := h.opts.World.Versions(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"version": ver, "config": json.RawMessage(raw), "versions": vs, "living_enabled": h.opts.LivingEnabled,
		"preview": worldcfg.Check(raw)})
}

func (h *Handler) worldPreview(w http.ResponseWriter, r *http.Request, _ User) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(worldcfg.MaxConfigBytes)+1)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "конфигурация слишком большая")
		return
	}
	writeJSON(w, http.StatusOK, worldcfg.Check(raw))
}

func (h *Handler) worldPublish(w http.ResponseWriter, r *http.Request, u User) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(worldcfg.MaxConfigBytes)+4096)
	var req struct {
		Config json.RawMessage `json:"config"`
		Note   string          `json:"note"`
		Base   int64           `json:"base_version"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ver, err := h.opts.World.Publish(r.Context(), u.Username, req.Note, req.Config, req.Base)
	if err != nil {
		h.worldFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "world_publish", h.clientIP(r), map[string]any{"version": ver, "base": req.Base})
	h.changed("мир")
	writeJSON(w, http.StatusOK, map[string]any{"version": ver, "living_enabled": h.opts.LivingEnabled})
}

func (h *Handler) worldRollback(w http.ResponseWriter, r *http.Request, u User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "некорректный номер")
		return
	}
	ver, err := h.opts.World.Rollback(r.Context(), u.Username, id)
	if err != nil {
		h.worldFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "world_rollback", h.clientIP(r), map[string]any{"to": id, "version": ver})
	h.changed("мир")
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}
