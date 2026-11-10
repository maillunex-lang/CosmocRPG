package admin

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"github.com/maillunex-lang/CosmocRPG/server/internal/aiciv"
)

// ИИ-3 — админ-API «ИИ-игроки»: состояние и настройки агента мира, живой статус капитанов,
// индивидуальное отключение капитанов. Всё меняется на лету, записи идут в журнал аудита.

// aiPlayersRoutes маршруты раздела (вызывается из Routes при наличии агента).
func (h *Handler) aiPlayersRoutes(mux *http.ServeMux) {
	a := h.auth
	mux.HandleFunc("GET /api/ai-players", a(h.aiPlayersGet))
	mux.HandleFunc("PUT /api/ai-players/settings", a(h.aiPlayersSettingsPut))
	mux.HandleFunc("POST /api/ai-players/captains/{id}/disable", a(h.aiCaptainDisable(true)))
	mux.HandleFunc("POST /api/ai-players/captains/{id}/enable", a(h.aiCaptainDisable(false)))
}

// GET /api/ai-players — состояние, настройки и список капитанов с онлайн-статусом.
func (h *Handler) aiPlayersGet(w http.ResponseWriter, r *http.Request, _ User) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   h.opts.Agent.Status(),
		"settings": h.opts.Agent.Settings(),
		"captains": h.opts.Agent.Captains(),
	})
}

// PUT /api/ai-players/settings — проверить, сохранить и применить настройки.
func (h *Handler) aiPlayersSettingsPut(w http.ResponseWriter, r *http.Request, u User) {
	var s aiagent.Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON настроек")
		return
	}
	if err := h.opts.Agent.SetSettings(r.Context(), s, u.Username); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s = h.opts.Agent.Settings()
	if h.opts.Civ != nil {
		h.opts.Civ.Set(aiciv.Knobs{Enabled: s.CivEnabled, Cast: s.CivCast, Chat: s.CivChat, TalkMin: s.CivTalkMin, TalkMax: s.CivTalkMax, IntervalSec: s.CivIntervalSec, RetreatPct: s.CivRetreatPct, Help: s.CivHelp})
	}
	h.audit_(r.Context(), 0, u.Username, "ai_players_settings", h.clientIP(r), map[string]any{"enabled": s.Enabled, "min": s.MinOnline, "max": s.MaxOnline, "civ": s.CivEnabled, "cast": s.CivCast})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": s})
}

// aiCaptainDisable отключает/включает капитана индивидуально (ИИ-3).
func (h *Handler) aiCaptainDisable(disable bool) func(http.ResponseWriter, *http.Request, User) {
	return func(w http.ResponseWriter, r *http.Request, u User) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" || len(id) > 48 {
			writeErr(w, http.StatusBadRequest, "некорректный идентификатор")
			return
		}
		s := h.opts.Agent.Settings()
		set := s.DisabledSet()
		if disable {
			set[id] = true
		} else {
			delete(set, id)
		}
		s.DisabledCaptains = make([]string, 0, len(set))
		for k := range set {
			s.DisabledCaptains = append(s.DisabledCaptains, k)
		}
		sort.Strings(s.DisabledCaptains)
		if err := h.opts.Agent.SetSettings(r.Context(), s, u.Username); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		action := "ai_captain_enable"
		if disable {
			action = "ai_captain_disable"
		}
		h.audit_(r.Context(), 0, u.Username, action, h.clientIP(r), map[string]any{"captain": id})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "disabled": disable})
	}
}
