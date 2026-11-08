package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
)

func (h *Handler) chatRoutes(mux *http.ServeMux) {
	a := h.auth
	mux.HandleFunc("GET /api/chat/messages", a(h.chatMessages))
	mux.HandleFunc("GET /api/chat/settings", a(h.chatSettingsGet))
	mux.HandleFunc("PUT /api/chat/settings", a(h.chatSettingsPut))
	mux.HandleFunc("GET /api/chat/mutes", a(h.chatMutes))
	mux.HandleFunc("POST /api/chat/mutes", a(h.chatMute))
	mux.HandleFunc("DELETE /api/chat/mutes/{id}", a(h.chatUnmute))
}

func (h *Handler) chatMessages(w http.ResponseWriter, r *http.Request, _ User) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ch := r.URL.Query().Get("channel")
	if ch != "" && ch != chat.General && ch != chat.DM && ch != chat.System {
		writeErr(w, http.StatusBadRequest, "неизвестный канал")
		return
	}
	items, err := h.opts.Chat.Search(r.Context(), ch, r.URL.Query().Get("q"), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) chatSettingsGet(w http.ResponseWriter, _ *http.Request, _ User) {
	writeJSON(w, http.StatusOK, h.opts.Chat.Settings())
}

func (h *Handler) chatSettingsPut(w http.ResponseWriter, r *http.Request, u User) {
	var st chat.Settings
	if !readJSON(w, r, &st) {
		return
	}
	if err := h.opts.Chat.UpdateSettings(r.Context(), st); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "chat_settings", h.clientIP(r), map[string]any{"enabled": st.Enabled, "retention_days": st.RetentionDays, "min_interval_ms": st.MinIntervalMs})
	writeJSON(w, http.StatusOK, h.opts.Chat.Settings())
}

func (h *Handler) chatMutes(w http.ResponseWriter, r *http.Request, _ User) {
	items, err := h.opts.Chat.Mutes(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if items == nil {
		items = []chat.Mute{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) chatMute(w http.ResponseWriter, r *http.Request, u User) {
	var req struct {
		Player  string `json:"player"`
		Minutes int    `json:"minutes"` // 0: бессрочно (бан в чате)
		Reason  string `json:"reason"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Player) == "" || req.Minutes < 0 || req.Minutes > 60*24*365 {
		writeErr(w, http.StatusBadRequest, "укажите игрока и срок (0 — бессрочно, не более года)")
		return
	}
	m, err := h.opts.Chat.MuteBy(r.Context(), u.Username, req.Player, req.Minutes, req.Reason)
	if errors.Is(err, chat.ErrNoRecipient) {
		writeErr(w, http.StatusNotFound, "игрок не найден")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "chat_mute", h.clientIP(r), map[string]any{"player": m.PlayerID, "minutes": req.Minutes, "reason": m.Reason})
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) chatUnmute(w http.ResponseWriter, r *http.Request, u User) {
	id := r.PathValue("id")
	if id == "" || len(id) > 64 {
		writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
		return
	}
	if err := h.opts.Chat.Unmute(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	h.audit_(r.Context(), u.ID, u.Username, "chat_unmute", h.clientIP(r), map[string]any{"player": id})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
