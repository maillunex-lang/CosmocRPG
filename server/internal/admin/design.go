package admin

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/maillunex-lang/CosmocRPG/server/internal/design"
)

func (h *Handler) designRoutes(mux *http.ServeMux) {
	a := h.auth
	mux.HandleFunc("GET /api/design/status", a(h.designStatus))
	mux.HandleFunc("GET /api/design/elements", a(h.designElements))
	mux.HandleFunc("GET /api/design/elements/{id}", a(h.designElement))
	mux.HandleFunc("POST /api/design/elements/{id}/upload", a(h.designUpload))
	mux.HandleFunc("POST /api/design/elements/{id}/unpublish", a(h.designUnpublish))
	mux.HandleFunc("POST /api/design/assets/{aid}/publish", a(h.designPublish))
	mux.HandleFunc("DELETE /api/design/assets/{aid}", a(h.designDelete))
	mux.HandleFunc("GET /api/design/assets/{aid}/file", a(h.designFile))
	mux.HandleFunc("GET /api/design/prompt", a(h.designPrompt))
}

func (h *Handler) designFail(w http.ResponseWriter, err error) {
	var inv *design.InvalidError
	switch {
	case errors.As(err, &inv):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": inv.Error(), "report": inv.Report})
	case errors.Is(err, design.ErrNoElement), errors.Is(err, design.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, design.ErrDuplicate), errors.Is(err, design.ErrPublished), errors.Is(err, design.ErrState):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, design.ErrNoSigner):
		writeErr(w, http.StatusPreconditionFailed, err.Error())
	case errors.Is(err, design.ErrBadRequest):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("admin: дизайн", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
	}
}

func (h *Handler) designStatus(w http.ResponseWriter, r *http.Request, _ User) {
	st, err := h.opts.Design.Status(r.Context())
	if err != nil {
		h.designFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) designElements(w http.ResponseWriter, r *http.Request, _ User) {
	q := r.URL.Query()
	items, facets, err := h.opts.Design.Elements(r.Context(), design.Filter{Category: q.Get("category"), Group: q.Get("group"), Tag: q.Get("tag"), State: q.Get("state"), Q: q.Get("q")})
	if err != nil {
		h.designFail(w, err)
		return
	}
	if items == nil {
		items = []design.ElementView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "facets": facets})
}

func (h *Handler) designElement(w http.ResponseWriter, r *http.Request, _ User) {
	el, hist, err := h.opts.Design.Detail(r.Context(), r.PathValue("id"))
	if err != nil {
		h.designFail(w, err)
		return
	}
	if hist == nil {
		hist = []design.Asset{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"element": el, "assets": hist})
}

// designUpload принимает файл замены в теле запроса (имя — параметром name), проверяет и сохраняет черновиком.
func (h *Handler) designUpload(w http.ResponseWriter, r *http.Request, u User) {
	id := r.PathValue("id")
	el, _, err := h.opts.Design.Detail(r.Context(), id)
	if err != nil {
		h.designFail(w, err)
		return
	}
	max := design.MaxUploadBytes(el.Category)
	r.Body = http.MaxBytesReader(w, r.Body, max+1)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "файл слишком большой для этой категории")
		return
	}
	a, err := h.opts.Design.Upload(r.Context(), id, r.URL.Query().Get("name"), data, r.URL.Query().Get("notes"), u.Username)
	if err != nil {
		h.designFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "design_upload", h.clientIP(r), map[string]any{"element": id, "version": a.Version, "sha256": a.SHA256, "size": a.Size})
	writeJSON(w, http.StatusCreated, a)
}

func (h *Handler) designPublish(w http.ResponseWriter, r *http.Request, u User) {
	aid, err := strconv.ParseInt(r.PathValue("aid"), 10, 64)
	if err != nil || aid < 1 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	a, err := h.opts.Design.File(r.Context(), aid)
	if err != nil {
		h.designFail(w, err)
		return
	}
	v, err := h.opts.Design.Publish(r.Context(), aid)
	if err != nil {
		h.designFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "design_publish", h.clientIP(r), map[string]any{"element": a.ElementID, "version": a.Version, "manifest": v})
	h.changed("оформление")
	writeJSON(w, http.StatusOK, map[string]any{"manifest_version": v})
}

func (h *Handler) designUnpublish(w http.ResponseWriter, r *http.Request, u User) {
	id := r.PathValue("id")
	if _, _, err := h.opts.Design.Detail(r.Context(), id); err != nil {
		h.designFail(w, err)
		return
	}
	v, err := h.opts.Design.Unpublish(r.Context(), id)
	if err != nil {
		h.designFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "design_unpublish", h.clientIP(r), map[string]any{"element": id, "manifest": v})
	h.changed("оформление")
	writeJSON(w, http.StatusOK, map[string]any{"manifest_version": v})
}

func (h *Handler) designDelete(w http.ResponseWriter, r *http.Request, u User) {
	aid, err := strconv.ParseInt(r.PathValue("aid"), 10, 64)
	if err != nil || aid < 1 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	a, err := h.opts.Design.File(r.Context(), aid)
	if err != nil {
		h.designFail(w, err)
		return
	}
	if err := h.opts.Design.Delete(r.Context(), aid); err != nil {
		h.designFail(w, err)
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "design_delete", h.clientIP(r), map[string]any{"element": a.ElementID, "version": a.Version})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// designFile отдаёт файл замены для предпросмотра и скачивания в админке. Содержимое загружено администратором и проверено,
// но всё равно отдаётся в «песочнице» и без определения типа браузером.
func (h *Handler) designFile(w http.ResponseWriter, r *http.Request, _ User) {
	aid, err := strconv.ParseInt(r.PathValue("aid"), 10, 64)
	if err != nil || aid < 1 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	a, err := h.opts.Design.File(r.Context(), aid)
	if err != nil {
		h.designFail(w, err)
		return
	}
	ct := a.Mime
	if ct == "model/gltf-binary" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("Content-Length", strconv.Itoa(len(a.Data)))
	_, _ = w.Write(a.Data)
}

func (h *Handler) designPrompt(w http.ResponseWriter, r *http.Request, _ User) {
	p, err := h.opts.Design.Prompt(r.URL.Query().Get("element"))
	if err != nil {
		h.designFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"prompt": p})
}
