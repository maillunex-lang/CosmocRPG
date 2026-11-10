package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func (h *Handler) shipRoutes(mux *http.ServeMux) {
	a := h.auth
	mux.HandleFunc("GET /api/ships", a(h.shipsList))
	mux.HandleFunc("PUT /api/ships/{id}", a(h.shipPut))
	mux.HandleFunc("POST /api/ships/patch/export", a(h.shipPatchExport))
	mux.HandleFunc("POST /api/ships/patch/import", a(h.shipPatchImport))
	mux.HandleFunc("POST /api/ships/patch/{pid}/publish", a(h.shipPatchPublish))
	mux.HandleFunc("POST /api/ships/patch/{pid}/discard", a(h.shipPatchDiscard))
	mux.HandleFunc("GET /api/ships/versions", a(h.shipVersions))
	mux.HandleFunc("POST /api/ships/versions/{id}/rollback", a(h.shipRollback))
	mux.HandleFunc("GET /api/ships/prompt", a(h.shipPrompt))
}

func (h *Handler) shipFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, shipcatalog.ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, shipcatalog.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	}
}

func (h *Handler) shipsList(w http.ResponseWriter, r *http.Request, _ User) {
	doc, ver, err := h.opts.Ships.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	vs, _ := h.opts.Ships.Versions(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"version": ver, "ships": doc.Ships, "starters": doc.Starters, "versions": vs, "pilot_specs": world.PilotSpecs})
}

func (h *Handler) shipPut(w http.ResponseWriter, r *http.Request, u User) {
	r.Body = http.MaxBytesReader(w, r.Body, aiMaxJSON)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var sp world.ShipSpec
	if err := dec.Decode(&sp); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный паспорт корабля: "+err.Error())
		return
	}
	if sp.ID != r.PathValue("id") {
		writeErr(w, http.StatusBadRequest, "идентификатор в теле не совпадает с адресом")
		return
	}
	ver, err := h.opts.Ships.UpdateShip(r.Context(), u.Username, sp)
	if err != nil {
		h.shipFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "ship_edit", h.clientIP(r), map[string]any{"ship": sp.ID, "version": ver})
	h.changed("корабли")
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) shipPatchExport(w http.ResponseWriter, r *http.Request, u User) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	data, name, err := h.opts.Ships.ExportPatch(r.Context(), req.IDs)
	if err != nil {
		h.shipFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "ship_patch_export", h.clientIP(r), map[string]any{"file": name})
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(data)
}

func (h *Handler) shipPatchImport(w http.ResponseWriter, r *http.Request, u User) {
	if ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); ct != "application/zip" {
		writeErr(w, http.StatusUnsupportedMediaType, "ожидается Content-Type: application/zip")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, aistudio.MaxZipBytes+1)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "файл слишком большой")
		return
	}
	name := r.URL.Query().Get("filename")
	if len(name) > 120 {
		name = name[:120]
	}
	pv, err := h.opts.Ships.PreviewPatch(r.Context(), u.Username, name, bytes.Clone(data))
	if err != nil {
		h.log.Error("ships: предпросмотр патча", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "ship_patch_import", h.clientIP(r), map[string]any{"preview": pv.ID, "status": pv.Status, "errors": len(pv.Errors), "sha256": pv.SHA256})
	writeJSON(w, http.StatusOK, pv)
}

func (h *Handler) shipPatchPublish(w http.ResponseWriter, r *http.Request, u User) {
	pid := r.PathValue("pid")
	ver, err := h.opts.Ships.PublishPreview(r.Context(), u.Username, pid)
	if err != nil {
		h.shipFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "ship_patch_publish", h.clientIP(r), map[string]any{"preview": pid, "version": ver})
	h.changed("корабли")
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) shipPatchDiscard(w http.ResponseWriter, r *http.Request, u User) {
	if err := h.opts.Ships.DiscardPreview(r.PathValue("pid")); err != nil {
		h.shipFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "ship_patch_discard", h.clientIP(r), nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) shipVersions(w http.ResponseWriter, r *http.Request, _ User) {
	vs, err := h.opts.Ships.Versions(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": vs})
}

func (h *Handler) shipRollback(w http.ResponseWriter, r *http.Request, u User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "некорректный номер")
		return
	}
	ver, err := h.opts.Ships.Rollback(r.Context(), u.Username, id)
	if err != nil {
		h.shipFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "ship_rollback", h.clientIP(r), map[string]any{"to": id, "version": ver})
	h.changed("корабли")
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) shipPrompt(w http.ResponseWriter, _ *http.Request, _ User) {
	writeJSON(w, http.StatusOK, map[string]string{"prompt": shipcatalog.PatchPrompt()})
}
