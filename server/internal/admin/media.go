package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/maillunex-lang/CosmocRPG/server/internal/media"
)

func (h *Handler) mediaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/media/tracks", h.auth(h.mediaList))
	mux.HandleFunc("POST /api/media/tracks", h.auth(h.mediaUpload))
	mux.HandleFunc("POST /api/media/tracks/{id}", h.auth(h.mediaUpdate))
	mux.HandleFunc("DELETE /api/media/tracks/{id}", h.auth(h.mediaDelete))
	mux.HandleFunc("GET /api/media/tracks/{id}/file", h.auth(h.mediaFile))
}

func (h *Handler) mediaFail(w http.ResponseWriter, err error) {
	var inv *media.InvalidError
	switch {
	case errors.As(err, &inv):
		writeErr(w, http.StatusBadRequest, inv.Msg)
	case errors.Is(err, media.ErrNotFound):
		writeErr(w, http.StatusNotFound, "трек не найден")
	case errors.Is(err, media.ErrDuplicate):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		h.log.Error("admin: библиотека музыки", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
	}
}

func (h *Handler) mediaList(w http.ResponseWriter, r *http.Request, _ User) {
	items, err := h.opts.Media.List(r.Context())
	if err != nil {
		h.mediaFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) mediaUpload(w http.ResponseWriter, r *http.Request, u User) {
	q := r.URL.Query()
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxBytes+1)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "файл больше 12 МБ")
		return
	}
	ai := q.Get("ai") != "0" && q.Get("ai") != "false"
	enabled := q.Get("enabled") != "0" && q.Get("enabled") != "false"
	t, err := h.opts.Media.Upload(r.Context(), q.Get("name"), data, media.Meta{
		Kind: q.Get("kind"), Title: q.Get("title"), Artist: q.Get("artist"), License: q.Get("license"),
		Source: q.Get("source"), AIGenerated: ai, Tags: q.Get("tags"), Enabled: enabled,
	}, u.Username)
	if err != nil {
		h.mediaFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "media_upload", h.clientIP(r), map[string]any{"id": t.ID, "title": t.Title, "sha256": t.SHA256, "size": t.Size, "kind": t.Kind})
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) mediaUpdate(w http.ResponseWriter, r *http.Request, u User) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	var m media.Meta
	if err := json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&m); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректное описание")
		return
	}
	t, err := h.opts.Media.Update(r.Context(), id, m)
	if err != nil {
		h.mediaFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "media_update", h.clientIP(r), map[string]any{"id": t.ID, "enabled": t.Enabled, "kind": t.Kind})
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) mediaDelete(w http.ResponseWriter, r *http.Request, u User) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	if err := h.opts.Media.Delete(r.Context(), id); err != nil {
		h.mediaFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "media_delete", h.clientIP(r), map[string]any{"id": id})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) mediaFile(w http.ResponseWriter, r *http.Request, _ User) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	t, err := h.opts.Media.Get(r.Context(), id)
	if err != nil {
		h.mediaFail(w, err)
		return
	}
	b, mime, err := h.opts.Media.Read(t.SHA256 + "." + t.Ext)
	if err != nil {
		h.mediaFail(w, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(b)
}
