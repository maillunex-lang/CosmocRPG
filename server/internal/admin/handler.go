package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
	"github.com/maillunex-lang/CosmocRPG/server/internal/design"
	"github.com/maillunex-lang/CosmocRPG/server/internal/media"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldcfg"
)

const (
	cookieName    = "cosmoc_admin"
	maxBodyBytes  = 4096
	maxLoginFails = 5
	failWindow    = 15 * time.Minute
)

// Options — настройки обработчика админ-API.
type Options struct {
	SessionTTL    time.Duration
	CookieSecure  bool // true за HTTPS (Caddy); false только для локальной разработки и тестов
	TrustProxy    bool // доверять заголовку X-Forwarded-For от обратного прокси (Caddy)
	Version       string
	StartedAt     time.Time
	WSSessions    func() int64         // число активных игровых сессий
	AI            *aistudio.Service    // AI Civilization Training Studio (nil: раздел выключен)
	Agent         *aiagent.Manager     // ИИ-игроки в мире: статус, настройки, капитаны (nil: раздел выключен)
	Ships         *shipcatalog.Service // каталог кораблей: параметры и ZIP-патчи (nil: раздел выключен)
	World         *worldcfg.Service    // конфигурация мира: локации, враги, ресурсы, графика (nil: раздел выключен)
	Chat          *chat.Service        // чат: журнал, мьюты, настройки (nil: раздел выключен)
	Design        *design.Service      // библиотека дизайна: каталог элементов, замены, публикация (nil: раздел выключен)
	Media         *media.Service       // библиотека музыки и радио (nil: раздел выключен)
	LivingEnabled bool                 // включены ли живые локации на сервере (флаг COSMOC_FEATURE_LIVING)
}

// Handler — HTTP-обработчик админ-API (/api/...).
type Handler struct {
	store *Store
	log   *slog.Logger
	opts  Options
	byIP  *throttle
	byUsr *throttle
	now   func() time.Time
}

// NewHandler создаёт обработчик.
func NewHandler(store *Store, log *slog.Logger, opts Options) *Handler {
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 12 * time.Hour
	}
	if opts.WSSessions == nil {
		opts.WSSessions = func() int64 { return 0 }
	}
	return &Handler{
		store: store, log: log, opts: opts, now: time.Now,
		byIP:  newThrottle(maxLoginFails*2, failWindow),
		byUsr: newThrottle(maxLoginFails, failWindow),
	}
}

// Routes возвращает маршруты админ-API.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", h.login)
	mux.HandleFunc("POST /api/logout", h.auth(h.logout))
	mux.HandleFunc("POST /api/password", h.auth(h.changePassword))
	mux.HandleFunc("GET /api/me", h.auth(h.me))
	mux.HandleFunc("GET /api/status", h.auth(h.status))
	mux.HandleFunc("GET /api/audit", h.auth(h.audit))
	if h.opts.AI != nil {
		h.aiRoutes(mux)
		if h.opts.Agent != nil {
			h.aiPlayersRoutes(mux)
		}
	}
	if h.opts.Ships != nil {
		h.shipRoutes(mux)
	}
	if h.opts.World != nil {
		h.worldRoutes(mux)
	}
	if h.opts.Chat != nil {
		h.chatRoutes(mux)
	}
	if h.opts.Design != nil {
		h.designRoutes(mux)
	}
	if h.opts.Media != nil {
		h.mediaRoutes(mux)
	}
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (h *Handler) clientIP(r *http.Request) string {
	if h.opts.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			// правый элемент добавлен нашим доверенным прокси; левые подконтрольны клиенту
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type ctxKey struct{}

// auth проверяет cookie сессии и кладёт администратора в контекст.
func (h *Handler) auth(next func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "требуется вход")
			return
		}
		u, err := h.store.SessionUser(r.Context(), c.Value)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				h.log.Error("admin: проверка сессии", "err", err)
				writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
				return
			}
			writeErr(w, http.StatusUnauthorized, "требуется вход")
			return
		}
		next(w, r, u)
	}
}

func (h *Handler) setCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: value, Path: "/", Expires: expires,
		HttpOnly: true, Secure: h.opts.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) audit_(ctx context.Context, id int64, user, action, ip string, details map[string]any) {
	if err := h.store.Audit(ctx, id, user, action, ip, details); err != nil {
		h.log.Error("admin: запись в журнал", "action", action, "err", err)
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	// Только JSON: это вместе с SameSite=Strict закрывает CSRF-вход через формы.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeErr(w, http.StatusUnsupportedMediaType, "ожидается application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req loginRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный запрос")
		return
	}
	username := NormalizeUsername(req.Username)
	ip := h.clientIP(r)
	now := h.now()
	if h.byIP.blocked(ip, now) || h.byUsr.blocked(username, now) {
		h.audit_(r.Context(), 0, username, "login_blocked", ip, nil)
		w.Header().Set("Retry-After", strconv.Itoa(int(failWindow.Seconds())))
		writeErr(w, http.StatusTooManyRequests, "слишком много попыток, попробуйте позже")
		return
	}

	u, err := h.store.UserByName(r.Context(), username)
	hash := dummyHash // для несуществующего пользователя тратим то же время
	if err == nil {
		hash = u.Hash
	} else if !errors.Is(err, ErrNotFound) {
		h.log.Error("admin: поиск пользователя", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	ok, verr := VerifyPassword(req.Password, hash)
	if verr != nil {
		h.log.Error("admin: проверка пароля", "err", verr)
	}
	if err != nil || verr != nil || !ok || u.Disabled {
		h.byIP.fail(ip, now)
		h.byUsr.fail(username, now)
		h.audit_(r.Context(), 0, username, "login_failed", ip, nil)
		writeErr(w, http.StatusUnauthorized, "неверное имя пользователя или пароль")
		return
	}

	token, expires, err := h.store.CreateSession(r.Context(), u.ID, h.opts.SessionTTL)
	if err != nil {
		h.log.Error("admin: создание сессии", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	h.byIP.reset(ip)
	h.byUsr.reset(username)
	h.setCookie(w, token, expires)
	h.audit_(r.Context(), u.ID, u.Username, "login", ip, nil)
	writeJSON(w, http.StatusOK, map[string]string{"username": u.Username})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, u User) {
	if c, err := r.Cookie(cookieName); err == nil {
		if err := h.store.DeleteSession(r.Context(), c.Value); err != nil {
			h.log.Error("admin: удаление сессии", "err", err)
		}
	}
	h.setCookie(w, "", time.Unix(0, 0))
	h.audit_(r.Context(), u.ID, u.Username, "logout", h.clientIP(r), nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type passwordRequest struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

// changePassword меняет пароль текущего администратора. Требует текущий пароль (защита от
// чужого открытого окна), ограничивает число неверных попыток и завершает остальные сессии.
// Неверный текущий пароль отдаёт 403, а не 401: 401 интерфейс трактует как истёкшую сессию.
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request, u User) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeErr(w, http.StatusUnsupportedMediaType, "ожидается application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req passwordRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный запрос")
		return
	}
	ip, now, key := h.clientIP(r), h.now(), "pw:"+u.Username
	if h.byUsr.blocked(key, now) {
		h.audit_(r.Context(), u.ID, u.Username, "password_change_blocked", ip, nil)
		w.Header().Set("Retry-After", strconv.Itoa(int(failWindow.Seconds())))
		writeErr(w, http.StatusTooManyRequests, "слишком много попыток, попробуйте позже")
		return
	}
	ok, err := VerifyPassword(req.Current, u.Hash)
	if err != nil {
		h.log.Error("admin: проверка пароля при смене", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if !ok {
		h.byUsr.fail(key, now)
		h.audit_(r.Context(), u.ID, u.Username, "password_change_failed", ip, nil)
		writeErr(w, http.StatusForbidden, "неверный текущий пароль")
		return
	}
	switch {
	case len([]rune(req.New)) < MinPasswordLen:
		writeErr(w, http.StatusBadRequest, "новый пароль короче "+strconv.Itoa(MinPasswordLen)+" символов")
		return
	case req.New == req.Current:
		writeErr(w, http.StatusBadRequest, "новый пароль совпадает с текущим")
		return
	case NormalizeUsername(req.New) == u.Username:
		writeErr(w, http.StatusBadRequest, "пароль не должен совпадать с именем пользователя")
		return
	}
	if err := h.store.SetPassword(r.Context(), u.ID, req.New); err != nil {
		h.log.Error("admin: смена пароля", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	// остальные сессии (в том числе возможно украденные) завершаются, текущая остаётся
	revoked := int64(0)
	if c, cerr := r.Cookie(cookieName); cerr == nil {
		if revoked, err = h.store.DeleteOtherSessions(r.Context(), u.ID, c.Value); err != nil {
			h.log.Error("admin: завершение остальных сессий", "err", err)
		}
	}
	h.byUsr.reset(key)
	h.audit_(r.Context(), u.ID, u.Username, "password_changed", ip, map[string]any{"other_sessions_revoked": revoked})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "other_sessions_revoked": revoked})
}

func (h *Handler) me(w http.ResponseWriter, _ *http.Request, u User) {
	writeJSON(w, http.StatusOK, map[string]string{"username": u.Username})
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request, _ User) {
	dbState := "ok"
	if err := h.store.Ping(r.Context()); err != nil {
		dbState = "unavailable"
	}
	boots, _ := h.store.BootCount(r.Context())
	now := h.now()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        h.opts.Version,
		"database":       dbState,
		"started_at":     h.opts.StartedAt.UTC().Format(time.RFC3339),
		"uptime_seconds": int64(now.Sub(h.opts.StartedAt).Seconds()),
		"server_time":    now.UTC().Format(time.RFC3339),
		"ws_sessions":    h.opts.WSSessions(),
		"boots_total":    boots,
	})
}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request, _ User) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := h.store.RecentAudit(r.Context(), limit)
	if err != nil {
		h.log.Error("admin: чтение журнала", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
