package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
)

const testPassword = "correct horse battery"

// --- пароли ---

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Fatalf("формат хеша: %s", h)
	}
	if ok, err := VerifyPassword(testPassword, h); err != nil || !ok {
		t.Fatalf("верный пароль не принят: ok=%v err=%v", ok, err)
	}
	if ok, _ := VerifyPassword("другой пароль", h); ok {
		t.Fatal("неверный пароль принят")
	}
	h2, _ := HashPassword(testPassword)
	if h == h2 {
		t.Fatal("соль должна различаться между хешами")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	for _, bad := range []string{
		"", "plain", "$argon2id$v=19$m=65536,t=3,p=2$abc",
		"$bcrypt$v=19$m=65536,t=3,p=2$YWJj$YWJj",
		"$argon2id$v=19$m=999999999,t=3,p=2$YWJj$YWJj", // защита от чрезмерной памяти
		"$argon2id$v=19$m=65536,t=3,p=0$YWJj$YWJj",
	} {
		if ok, err := VerifyPassword("x", bad); ok || err == nil {
			t.Errorf("хеш %q: ожидалась ошибка, ok=%v err=%v", bad, ok, err)
		}
	}
}

// --- ограничение попыток ---

func TestThrottle(t *testing.T) {
	th := newThrottle(3, time.Minute)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if th.blocked("a", now) {
			t.Fatalf("заблокировано раньше времени на попытке %d", i)
		}
		th.fail("a", now)
	}
	if !th.blocked("a", now) {
		t.Fatal("после 3 неудач ключ должен быть заблокирован")
	}
	if th.blocked("b", now) {
		t.Fatal("другой ключ не должен блокироваться")
	}
	if th.blocked("a", now.Add(61*time.Second)) {
		t.Fatal("после окна блокировка должна сняться")
	}
	th.fail("c", now)
	th.reset("c")
	if th.blocked("c", now) || len(th.failures["c"]) != 0 {
		t.Fatal("reset не очистил счётчик")
	}
}

func TestThrottleBoundsMemory(t *testing.T) {
	th := newThrottle(5, time.Hour)
	th.maxKeys = 50
	now := time.Now()
	for i := 0; i < 500; i++ {
		th.fail(string(rune('a'+i%26))+strings.Repeat("x", i), now)
	}
	if len(th.failures) > 51 {
		t.Fatalf("карта растёт без ограничения: %d", len(th.failures))
	}
}

// --- интеграция с базой ---

func testStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("COSMOC_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("COSMOC_TEST_DATABASE_URL не задана: интеграционный тест пропущен")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return NewStore(pool), pool
}

type env struct {
	t     *testing.T
	store *Store
	pool  *pgxpool.Pool
	h     *Handler
	srv   http.Handler
}

func newEnv(t *testing.T, opts Options) *env {
	store, pool := testStore(t)
	if _, err := store.CreateUser(context.Background(), "Admin", testPassword); err != nil {
		t.Fatal(err)
	}
	opts.Version, opts.StartedAt = "test", time.Now().Add(-90*time.Second)
	h := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)), opts)
	return &env{t: t, store: store, pool: pool, h: h, srv: h.Routes()}
}

func (e *env) do(method, path, body, cookie string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:5555"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	return rec
}

func loginBody(user, pass string) string {
	b, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	return string(b)
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("cookie сессии не установлена")
	return nil
}

func (e *env) login() string {
	rec := e.do("POST", "/api/login", loginBody("admin", testPassword), "", nil)
	if rec.Code != 200 {
		e.t.Fatalf("вход: %d %s", rec.Code, rec.Body)
	}
	return sessionCookie(e.t, rec).Value
}

func TestProtectedRoutesRequireLogin(t *testing.T) {
	e := newEnv(t, Options{})
	for _, p := range []string{"/api/me", "/api/status", "/api/audit"} {
		if rec := e.do("GET", p, "", "", nil); rec.Code != 401 {
			t.Errorf("GET %s без входа: %d", p, rec.Code)
		}
		if rec := e.do("GET", p, "", "подделка", nil); rec.Code != 401 {
			t.Errorf("GET %s с чужим токеном: %d", p, rec.Code)
		}
	}
	if rec := e.do("POST", "/api/logout", "", "", nil); rec.Code != 401 {
		t.Errorf("logout без входа: %d", rec.Code)
	}
}

func TestLoginSuccessSetsSecureCookie(t *testing.T) {
	e := newEnv(t, Options{CookieSecure: true})
	rec := e.do("POST", "/api/login", loginBody("  ADMIN ", testPassword), "", nil) // имя без учёта регистра
	if rec.Code != 200 {
		t.Fatalf("код %d: %s", rec.Code, rec.Body)
	}
	c := sessionCookie(t, rec)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
		t.Fatalf("атрибуты cookie: %+v", c)
	}
	if len(c.Value) < 40 {
		t.Fatalf("токен слишком короткий: %d", len(c.Value))
	}
	// в базе лежит только хеш токена
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM admin_sessions WHERE token_hash = $1", []byte(c.Value)).Scan(&n)
	if n != 0 {
		t.Fatal("сырой токен не должен храниться в базе")
	}
	if rec := e.do("GET", "/api/me", "", c.Value, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"admin"`) {
		t.Fatalf("/api/me: %d %s", rec.Code, rec.Body)
	}
}

func TestLoginFailures(t *testing.T) {
	e := newEnv(t, Options{})
	for _, tc := range []struct{ name, user, pass string }{
		{"неверный пароль", "admin", "неверный пароль!!"},
		{"неизвестный пользователь", "nobody", testPassword},
		{"пустые поля", "", ""},
	} {
		rec := e.do("POST", "/api/login", loginBody(tc.user, tc.pass), "", nil)
		if rec.Code != 401 {
			t.Errorf("%s: код %d", tc.name, rec.Code)
		}
		if rec.Result().Header.Get("Set-Cookie") != "" {
			t.Errorf("%s: cookie не должна устанавливаться", tc.name)
		}
		if !strings.Contains(rec.Body.String(), "неверное имя пользователя или пароль") {
			t.Errorf("%s: сообщение не должно различать причины: %s", tc.name, rec.Body)
		}
	}
}

func TestLoginRequestValidation(t *testing.T) {
	e := newEnv(t, Options{})
	// не JSON
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader("username=admin&password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Errorf("форма: %d", rec.Code)
	}
	// лишние поля, битый JSON, слишком большое тело
	for name, body := range map[string]string{
		"лишнее поле": `{"username":"admin","password":"x","role":"root"}`,
		"битый JSON":  `{"username":`,
		"большое":     `{"username":"admin","password":"` + strings.Repeat("a", 10000) + `"}`,
	} {
		if rec := e.do("POST", "/api/login", body, "", nil); rec.Code != 400 {
			t.Errorf("%s: код %d", name, rec.Code)
		}
	}
	// GET на login не разрешён
	if rec := e.do("GET", "/api/login", "", "", nil); rec.Code != 405 {
		t.Errorf("GET /api/login: %d", rec.Code)
	}
}

func TestLoginThrottled(t *testing.T) {
	e := newEnv(t, Options{})
	for i := 0; i < maxLoginFails; i++ {
		if rec := e.do("POST", "/api/login", loginBody("admin", "неверно-"+strings.Repeat("x", i+1)), "", nil); rec.Code != 401 {
			t.Fatalf("попытка %d: %d", i, rec.Code)
		}
	}
	// даже верный пароль после лимита не принимается
	rec := e.do("POST", "/api/login", loginBody("admin", testPassword), "", nil)
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("ожидался 429 с Retry-After, получен %d", rec.Code)
	}
	var blocked int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM admin_audit_log WHERE action='login_blocked'").Scan(&blocked)
	if blocked != 1 {
		t.Fatalf("блокировка не записана в журнал: %d", blocked)
	}
}

func TestSuccessfulLoginResetsCounter(t *testing.T) {
	e := newEnv(t, Options{})
	for i := 0; i < maxLoginFails-1; i++ {
		e.do("POST", "/api/login", loginBody("admin", "неверно-"+strings.Repeat("x", i+1)), "", nil)
	}
	e.login() // успех сбрасывает счётчик
	for i := 0; i < maxLoginFails-1; i++ {
		if rec := e.do("POST", "/api/login", loginBody("admin", "опять-неверно-"+strings.Repeat("x", i+1)), "", nil); rec.Code != 401 {
			t.Fatalf("после сброса попытка %d дала %d", i, rec.Code)
		}
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	e := newEnv(t, Options{})
	tok := e.login()
	rec := e.do("POST", "/api/logout", "", tok, nil)
	if rec.Code != 200 {
		t.Fatalf("logout: %d", rec.Code)
	}
	if c := sessionCookie(t, rec); c.Value != "" || c.MaxAge >= 0 && !c.Expires.Before(time.Now()) {
		t.Fatalf("cookie должна быть сброшена: %+v", c)
	}
	if rec := e.do("GET", "/api/me", "", tok, nil); rec.Code != 401 {
		t.Fatalf("после выхода токен ещё действует: %d", rec.Code)
	}
}

func TestExpiredAndDisabledSessions(t *testing.T) {
	e := newEnv(t, Options{})
	ctx := context.Background()

	tok := e.login()
	if _, err := e.pool.Exec(ctx, "UPDATE admin_sessions SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if rec := e.do("GET", "/api/me", "", tok, nil); rec.Code != 401 {
		t.Fatalf("просроченная сессия принята: %d", rec.Code)
	}
	if n, err := e.store.CleanupSessions(ctx); err != nil || n != 1 {
		t.Fatalf("очистка: n=%d err=%v", n, err)
	}

	tok = e.login()
	if _, err := e.pool.Exec(ctx, "UPDATE admin_users SET disabled = true"); err != nil {
		t.Fatal(err)
	}
	if rec := e.do("GET", "/api/me", "", tok, nil); rec.Code != 401 {
		t.Fatalf("сессия отключённого администратора принята: %d", rec.Code)
	}
	if rec := e.do("POST", "/api/login", loginBody("admin", testPassword), "", nil); rec.Code != 401 {
		t.Fatalf("вход отключённого администратора: %d", rec.Code)
	}
}

func TestStatus(t *testing.T) {
	e := newEnv(t, Options{WSSessions: func() int64 { return 3 }})
	tok := e.login()
	rec := e.do("GET", "/api/status", "", tok, nil)
	if rec.Code != 200 {
		t.Fatalf("status: %d", rec.Code)
	}
	var s map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s["version"] != "test" || s["database"] != "ok" || s["ws_sessions"] != float64(3) || s["uptime_seconds"].(float64) < 89 {
		t.Fatalf("статус: %v", s)
	}
}

func TestAuditLogRecordsActions(t *testing.T) {
	e := newEnv(t, Options{})
	e.do("POST", "/api/login", loginBody("admin", "неверно-неверно"), "", nil)
	tok := e.login()
	e.do("POST", "/api/logout", "", tok, nil)
	tok = e.login()

	rec := e.do("GET", "/api/audit?limit=10", "", tok, nil)
	var out struct{ Entries []AuditEntry }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 {
		t.Fatalf("audit: %d %v", rec.Code, err)
	}
	var actions []string
	for _, en := range out.Entries {
		actions = append(actions, en.Action)
		if en.IP != "203.0.113.7" {
			t.Errorf("IP в журнале: %q", en.IP)
		}
	}
	got := strings.Join(actions, ",")
	if got != "login,logout,login,login_failed" { // новые первыми
		t.Fatalf("журнал: %s", got)
	}
	for _, en := range out.Entries {
		if strings.Contains(en.Action+en.Username, testPassword) {
			t.Fatal("пароль попал в журнал")
		}
	}
}

func TestClientIPAndProxyTrust(t *testing.T) {
	spoof := map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.9"}

	e := newEnv(t, Options{TrustProxy: false})
	e.do("POST", "/api/login", loginBody("admin", "неверно-неверно"), "", spoof)
	var ip string
	_ = e.pool.QueryRow(context.Background(), "SELECT ip FROM admin_audit_log WHERE action='login_failed'").Scan(&ip)
	if ip != "203.0.113.7" {
		t.Fatalf("без доверия к прокси X-Forwarded-For игнорируется, получен %q", ip)
	}

	e2 := newEnv(t, Options{TrustProxy: true})
	e2.do("POST", "/api/login", loginBody("admin", "неверно-неверно"), "", spoof)
	_ = e2.pool.QueryRow(context.Background(), "SELECT ip FROM admin_audit_log WHERE action='login_failed'").Scan(&ip)
	if ip != "198.51.100.9" {
		t.Fatalf("с доверием берётся правый адрес (добавлен прокси), получен %q", ip)
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newEnv(t, Options{})
	rec := e.do("GET", "/api/me", "", "", nil)
	for k, v := range map[string]string{
		"Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer",
	} {
		if rec.Header().Get(k) != v {
			t.Errorf("заголовок %s = %q", k, rec.Header().Get(k))
		}
	}
}

func TestCreateUserValidation(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	if _, err := store.CreateUser(ctx, "ab", testPassword); err == nil {
		t.Error("слишком короткое имя принято")
	}
	if _, err := store.CreateUser(ctx, "someone", "short"); err == nil {
		t.Error("слишком короткий пароль принят")
	}
	if _, err := store.CreateUser(ctx, "someone", testPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(ctx, "SOMEONE", testPassword); err == nil {
		t.Error("дубликат имени (другой регистр) принят")
	}
	u, err := store.UserByName(ctx, "Someone")
	if err != nil || !strings.HasPrefix(u.Hash, "$argon2id$") || bytes.Contains([]byte(u.Hash), []byte(testPassword)) {
		t.Fatalf("пользователь/хеш: %+v err=%v", u, err)
	}
}
