package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const newPassword = "новый надёжный пароль 2026"

func pwBody(current, next string) string {
	b, _ := json.Marshal(map[string]string{"current": current, "new": next})
	return string(b)
}

func (e *env) changePassword(cookie, current, next string) *httptest.ResponseRecorder {
	return e.do("POST", "/api/password", pwBody(current, next), cookie, nil)
}

func TestChangePasswordRequiresLoginAndJSON(t *testing.T) {
	e := newEnv(t, Options{})
	if rec := e.changePassword("", testPassword, newPassword); rec.Code != 401 {
		t.Errorf("без входа: %d", rec.Code)
	}
	tok := e.login()
	req := httptest.NewRequest("POST", "/api/password", strings.NewReader("current=a&new=b"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Errorf("форма вместо JSON: %d", rec.Code)
	}
	if rec := e.do("POST", "/api/password", `{"current":"x","new":"y","role":"root"}`, tok, nil); rec.Code != 400 {
		t.Errorf("лишнее поле: %d", rec.Code)
	}
}

func TestChangePasswordRejectsBadInput(t *testing.T) {
	e := newEnv(t, Options{})
	tok := e.login()
	for _, tc := range []struct {
		name, current, next string
		code                int
	}{
		{"неверный текущий пароль", "не тот пароль вовсе", newPassword, 403},
		{"пустой текущий пароль", "", newPassword, 403},
		{"короткий новый пароль", testPassword, "short", 400},
		{"новый совпадает с текущим", testPassword, testPassword, 400},
		{"новый совпадает с именем", testPassword, "ADMIN", 400},
	} {
		if rec := e.changePassword(tok, tc.current, tc.next); rec.Code != tc.code {
			t.Errorf("%s: код %d, ожидался %d (%s)", tc.name, rec.Code, tc.code, rec.Body)
		}
	}
	// после всех отказов старый пароль продолжает работать
	if rec := e.do("POST", "/api/login", loginBody("admin", testPassword), "", nil); rec.Code != 200 {
		t.Fatalf("старый пароль перестал работать после отказов: %d", rec.Code)
	}
}

func TestChangePasswordSuccess(t *testing.T) {
	e := newEnv(t, Options{})
	tok := e.login()
	other := e.login() // вторая сессия того же администратора

	rec := e.changePassword(tok, testPassword, newPassword)
	if rec.Code != 200 {
		t.Fatalf("смена пароля: %d %s", rec.Code, rec.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["other_sessions_revoked"] != float64(1) {
		t.Errorf("должна быть завершена 1 другая сессия: %v", out)
	}

	if r := e.do("GET", "/api/me", "", tok, nil); r.Code != 200 {
		t.Errorf("текущая сессия должна сохраниться: %d", r.Code)
	}
	if r := e.do("GET", "/api/me", "", other, nil); r.Code != 401 {
		t.Errorf("другая сессия должна быть завершена: %d", r.Code)
	}
	if r := e.do("POST", "/api/login", loginBody("admin", testPassword), "", nil); r.Code != 401 {
		t.Errorf("старый пароль ещё принимается: %d", r.Code)
	}
	if r := e.do("POST", "/api/login", loginBody("admin", newPassword), "", nil); r.Code != 200 {
		t.Errorf("новый пароль не принимается: %d", r.Code)
	}

	// в журнале есть запись, самих паролей в нём нет
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM admin_audit_log WHERE action='password_changed'").Scan(&n)
	if n != 1 {
		t.Errorf("записей password_changed: %d", n)
	}
	var leaks int
	_ = e.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM admin_audit_log WHERE details::text LIKE '%' || $1 || '%' OR details::text LIKE '%' || $2 || '%'",
		testPassword, newPassword).Scan(&leaks)
	if leaks != 0 {
		t.Error("пароль попал в журнал")
	}
	// в базе лежит хеш argon2id, не пароль
	u, err := e.store.UserByName(context.Background(), "admin")
	if err != nil || !strings.HasPrefix(u.Hash, "$argon2id$") || strings.Contains(u.Hash, newPassword) {
		t.Errorf("хеш пароля: %q err=%v", u.Hash, err)
	}
}

func TestChangePasswordThrottled(t *testing.T) {
	e := newEnv(t, Options{})
	tok := e.login()
	for i := 0; i < maxLoginFails; i++ {
		if rec := e.changePassword(tok, "неверный-"+strings.Repeat("x", i+1), newPassword); rec.Code != 403 {
			t.Fatalf("попытка %d: %d", i, rec.Code)
		}
	}
	// после лимита даже верный текущий пароль не принимается
	rec := e.changePassword(tok, testPassword, newPassword)
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("ожидался 429 с Retry-After, получен %d", rec.Code)
	}
	if r := e.do("POST", "/api/login", loginBody("admin", testPassword), "", nil); r.Code != 200 {
		t.Errorf("пароль не должен был измениться: %d", r.Code)
	}
}

func TestSetPasswordValidation(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	u, err := store.CreateUser(ctx, "someone", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword(ctx, u.ID, "short"); err == nil {
		t.Error("короткий пароль принят")
	}
	if err := store.SetPassword(ctx, 999999, newPassword); err != ErrNotFound {
		t.Errorf("несуществующий администратор: %v", err)
	}
	if err := store.SetPassword(ctx, u.ID, newPassword); err != nil {
		t.Fatal(err)
	}
	got, _ := store.UserByName(ctx, "someone")
	if ok, _ := VerifyPassword(newPassword, got.Hash); !ok {
		t.Error("новый пароль не подходит к сохранённому хешу")
	}
	if ok, _ := VerifyPassword(testPassword, got.Hash); ok {
		t.Error("старый пароль всё ещё подходит")
	}
}
