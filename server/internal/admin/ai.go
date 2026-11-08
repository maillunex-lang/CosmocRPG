package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
)

const (
	aiMaxJSON = 4 << 20
	aiMaxZip  = aistudio.MaxZipBytes
)

func (h *Handler) aiRoutes(mux *http.ServeMux) {
	a := h.auth
	mux.HandleFunc("GET /api/ai/dashboard", a(h.aiDashboard))
	mux.HandleFunc("POST /api/ai/generate", a(h.aiGenerate))
	mux.HandleFunc("POST /api/ai/nicknames", a(h.aiNicknames))
	mux.HandleFunc("GET /api/ai/captains", a(h.aiCaptains))
	mux.HandleFunc("GET /api/ai/captains/{id}", a(h.aiCaptain))
	mux.HandleFunc("PUT /api/ai/captains/{id}", a(h.aiCaptainPut))
	mux.HandleFunc("GET /api/ai/captains/{id}/export", a(h.aiCaptainExport))
	mux.HandleFunc("POST /api/ai/captains/import", a(h.aiCaptainImport))
	mux.HandleFunc("GET /api/ai/uniqueness", a(h.aiUniqueness))
	mux.HandleFunc("GET /api/ai/dialogue/{section}", a(h.aiSectionGet))
	mux.HandleFunc("PUT /api/ai/dialogue/{section}", a(h.aiSectionPut))
	mux.HandleFunc("POST /api/ai/zip/export", a(h.aiZipExport))
	mux.HandleFunc("POST /api/ai/zip/import", a(h.aiZipImport))
	mux.HandleFunc("GET /api/ai/imports", a(h.aiImports))
	mux.HandleFunc("GET /api/ai/imports/{id}", a(h.aiImport))
	mux.HandleFunc("POST /api/ai/imports/{id}/publish", a(h.aiPublish))
	mux.HandleFunc("POST /api/ai/imports/{id}/discard", a(h.aiDiscard))
	mux.HandleFunc("GET /api/ai/versions", a(h.aiVersions))
	mux.HandleFunc("POST /api/ai/versions/{id}/rollback", a(h.aiRollback))
	mux.HandleFunc("GET /api/ai/sessions", a(h.aiSessions))
	mux.HandleFunc("POST /api/ai/academy/simulate", a(h.aiSimulate))
	mux.HandleFunc("POST /api/ai/academy/group", a(h.aiGroup))
	mux.HandleFunc("POST /api/ai/academy/tests", a(h.aiTests))
	mux.HandleFunc("GET /api/ai/academy/reports", a(h.aiReports))
	mux.HandleFunc("GET /api/ai/prompt", a(h.aiPrompt))
}

func (h *Handler) aiFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, aistudio.ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, aistudio.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, aiMaxJSON)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный запрос")
		return false
	}
	return true
}

func pathID(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return n, err == nil && n > 0
}

func (h *Handler) aiAudit(r *http.Request, u User, action string, d map[string]any) {
	h.audit_(r.Context(), u.ID, u.Username, action, h.clientIP(r), d)
}

func (h *Handler) aiDashboard(w http.ResponseWriter, r *http.Request, _ User) {
	d, err := h.opts.AI.Dashboard(r.Context())
	if err != nil {
		h.log.Error("ai: сводка", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) aiGenerate(w http.ResponseWriter, r *http.Request, u User) {
	var req struct {
		Seed  int64 `json:"seed"`
		Count int   `json:"count"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Seed == 0 {
		req.Seed = 20261108
	}
	if req.Count < 1 || req.Count > 400 {
		writeErr(w, http.StatusBadRequest, "количество капитанов: от 1 до 400")
		return
	}
	res, ver, err := h.opts.AI.Generate(r.Context(), u.Username, req.Seed, req.Count)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_generate", map[string]any{"added": len(res.Added), "version": ver})
	writeJSON(w, http.StatusOK, map[string]any{"added": len(res.Added), "skipped": len(res.Skipped), "version": ver})
}

func (h *Handler) aiCaptains(w http.ResponseWriter, r *http.Request, _ User) {
	cfg, ver, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	q := r.URL.Query()
	list := aistudio.ListCaptains(cfg, aistudio.CaptainFilter{Q: q.Get("q"), Faction: q.Get("faction"), Profession: q.Get("profession"),
		Corporation: q.Get("corporation"), Source: q.Get("source"), Formality: q.Get("formality")})
	total := len(list)
	off, _ := strconv.Atoi(q.Get("offset"))
	lim, _ := strconv.Atoi(q.Get("limit"))
	if lim <= 0 || lim > 400 {
		lim = 50
	}
	off = max(0, min(off, total))
	list = list[off:min(total, off+lim)]
	writeJSON(w, http.StatusOK, map[string]any{
		"total": total, "items": list, "version": ver,
		"factions": cfg.Factions, "corporations": cfg.Corporations, "professions": aistudio.Professions, "strategies": cfg.Strategies,
	})
}

func (h *Handler) aiCaptain(w http.ResponseWriter, r *http.Request, _ User) {
	cfg, _, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	c, ok := cfg.Captains[r.PathValue("id")]
	if !ok {
		writeErr(w, http.StatusNotFound, "капитана нет")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) aiCaptainPut(w http.ResponseWriter, r *http.Request, u User) {
	var c aistudio.Captain
	if !readJSON(w, r, &c) {
		return
	}
	if c.ID != r.PathValue("id") {
		writeErr(w, http.StatusBadRequest, "идентификатор в теле не совпадает с адресом")
		return
	}
	ver, err := h.opts.AI.EditCaptain(r.Context(), u.Username, c)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_captain_edit", map[string]any{"captain": c.ID, "version": ver})
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) aiCaptainExport(w http.ResponseWriter, r *http.Request, _ User) {
	cfg, _, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	id := r.PathValue("id")
	c, ok := cfg.Captains[id]
	if !ok {
		writeErr(w, http.StatusNotFound, "капитана нет")
		return
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.json"`)
	_, _ = w.Write(b)
}

func (h *Handler) aiCaptainImport(w http.ResponseWriter, r *http.Request, u User) {
	var c aistudio.Captain
	if !readJSON(w, r, &c) {
		return
	}
	ver, err := h.opts.AI.ImportCaptain(r.Context(), u.Username, c, r.URL.Query().Get("overwrite") == "1")
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_captain_import", map[string]any{"captain": c.ID, "version": ver})
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) aiUniqueness(w http.ResponseWriter, r *http.Request, _ User) {
	cfg, _, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, aistudio.Uniqueness(cfg))
}

func (h *Handler) aiSectionGet(w http.ResponseWriter, r *http.Request, _ User) {
	cfg, ver, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	doc, ok := aistudio.SectionDoc(cfg, r.PathValue("section"))
	if !ok {
		writeErr(w, http.StatusNotFound, "неизвестный раздел")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": ver, "doc": doc})
}

func (h *Handler) aiSectionPut(w http.ResponseWriter, r *http.Request, u User) {
	r.Body = http.MaxBytesReader(w, r.Body, aiMaxJSON)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный запрос")
		return
	}
	section := r.PathValue("section")
	ver, err := h.opts.AI.PutSection(r.Context(), u.Username, section, raw)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_section_edit", map[string]any{"section": section, "version": ver})
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) aiZipExport(w http.ResponseWriter, r *http.Request, u User) {
	var req aistudio.ExportRequest
	if !readJSON(w, r, &req) {
		return
	}
	data, name, err := h.opts.AI.Export(r.Context(), u.Username, req)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_zip_export", map[string]any{"kind": req.Kind, "file": name})
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("X-Filename", name)
	_, _ = w.Write(data)
}

func (h *Handler) aiZipImport(w http.ResponseWriter, r *http.Request, u User) {
	// только application/zip: не простой тип заставляет браузер делать preflight, чужие сайты не смогут отправить запрос
	if ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); ct != "application/zip" {
		writeErr(w, http.StatusUnsupportedMediaType, "ожидается Content-Type: application/zip")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, aiMaxZip+1)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "файл слишком большой")
		return
	}
	name := r.URL.Query().Get("filename")
	if len(name) > 120 {
		name = name[:120]
	}
	rec, err := h.opts.AI.Preview(r.Context(), u.Username, name, data, aistudio.MergeOptions{OverwriteManual: r.URL.Query().Get("overwrite_manual") == "1"})
	if err != nil {
		h.log.Error("ai: предпросмотр", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	h.aiAudit(r, u, "ai_zip_import", map[string]any{"import": rec.ID, "status": rec.Status, "errors": len(rec.Report.Errors), "sha256": rec.SHA256})
	writeJSON(w, http.StatusOK, rec)
}

func (h *Handler) aiImports(w http.ResponseWriter, r *http.Request, _ User) {
	list, err := h.opts.AI.Store().ListImports(r.Context(), 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *Handler) aiImport(w http.ResponseWriter, r *http.Request, _ User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "некорректный номер")
		return
	}
	rec, err := h.opts.AI.Store().GetImport(r.Context(), id)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *Handler) aiPublish(w http.ResponseWriter, r *http.Request, u User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "некорректный номер")
		return
	}
	var opt aistudio.PublishOptions
	if !readJSON(w, r, &opt) {
		return
	}
	ver, err := h.opts.AI.Publish(r.Context(), u.Username, id, opt)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_publish", map[string]any{"import": id, "version": ver})
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) aiDiscard(w http.ResponseWriter, r *http.Request, u User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "некорректный номер")
		return
	}
	if err := h.opts.AI.Discard(r.Context(), id); err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_discard", map[string]any{"import": id})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) aiVersions(w http.ResponseWriter, r *http.Request, _ User) {
	list, err := h.opts.AI.Store().ListVersions(r.Context(), 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *Handler) aiRollback(w http.ResponseWriter, r *http.Request, u User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "некорректный номер")
		return
	}
	ver, err := h.opts.AI.Rollback(r.Context(), u.Username, id)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_rollback", map[string]any{"to": id, "version": ver})
	writeJSON(w, http.StatusOK, map[string]any{"version": ver})
}

func (h *Handler) aiSessions(w http.ResponseWriter, r *http.Request, _ User) {
	list, err := h.opts.AI.Store().ListSessions(r.Context(), 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *Handler) aiSimulate(w http.ResponseWriter, r *http.Request, _ User) {
	var req struct {
		CaptainID string   `json:"captain_id"`
		Say       []string `json:"say"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Say) == 0 || len(req.Say) > 30 {
		writeErr(w, http.StatusBadRequest, "реплик: от 1 до 30")
		return
	}
	for _, s := range req.Say {
		if len([]rune(s)) > 300 {
			writeErr(w, http.StatusBadRequest, "реплика длиннее 300 знаков")
			return
		}
	}
	cfg, _, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	tr, err := aistudio.Simulate(&cfg, req.CaptainID, req.Say)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transcript": tr})
}

func (h *Handler) aiGroup(w http.ResponseWriter, r *http.Request, _ User) {
	var req struct {
		IDs     []string `json:"ids"`
		Opening string   `json:"opening"`
		Turns   int      `json:"turns"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Turns < 2 || req.Turns > 30 || req.Opening == "" || len([]rune(req.Opening)) > 300 {
		writeErr(w, http.StatusBadRequest, "укажите вступительную реплику и число реплик от 2 до 30")
		return
	}
	cfg, _, err := h.opts.AI.Active(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	rep, err := aistudio.SimulateGroup(&cfg, req.IDs, req.Opening, req.Turns)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *Handler) aiTests(w http.ResponseWriter, r *http.Request, u User) {
	var req struct {
		Kinds  []string `json:"kinds"`
		Sample int      `json:"sample"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Sample < 0 || req.Sample > 300 {
		writeErr(w, http.StatusBadRequest, "выборка: от 0 до 300 (0 — все капитаны)")
		return
	}
	known := map[string]bool{}
	for _, k := range aistudio.AllKinds {
		known[k] = true
	}
	for _, k := range req.Kinds {
		if !known[k] {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("неизвестная проверка «%s»", k))
			return
		}
	}
	run, err := h.opts.AI.RunTests(r.Context(), u.Username, req.Kinds, req.Sample)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_tests", map[string]any{"run": run.ID, "passed": run.Passed, "total": run.Total})
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) aiReports(w http.ResponseWriter, r *http.Request, _ User) {
	runs, err := h.opts.AI.Store().ListTestRuns(r.Context(), 20)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	cfg, _, _ := h.opts.AI.Active(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs, "quality": aistudio.Quality(&cfg), "kinds": aistudio.AllKinds})
}

func (h *Handler) aiPrompt(w http.ResponseWriter, r *http.Request, _ User) {
	size, _ := strconv.Atoi(r.URL.Query().Get("batch"))
	note := ""
	switch size {
	case 10, 25, 50:
		note = fmt.Sprintf("Обучай партию из %d капитанов, перечисленных в manifest.json. Работай только с ними.", size)
	case 300:
		note = "Полный пакет на 300 капитанов. Работай по очереди с группами по 25 капитанов, не теряя единый стиль и не повторяясь между группами."
	}
	writeJSON(w, http.StatusOK, map[string]string{"prompt": aistudio.MasterPrompt(note)})
}

func (h *Handler) aiNicknames(w http.ResponseWriter, r *http.Request, u User) {
	res, err := h.opts.AI.ApplyNicknames(r.Context(), u.Username, 20261108)
	if err != nil {
		h.aiFail(w, err)
		return
	}
	h.aiAudit(r, u, "ai_nicknames", map[string]any{"renamed": res.Renamed, "version": res.Version})
	writeJSON(w, http.StatusOK, res)
}
