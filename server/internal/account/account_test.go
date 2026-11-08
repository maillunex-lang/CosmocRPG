package account

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

const tokA = "devicetokenaaaaaaaaaaaa"
const tokB = "devicetokenbbbbbbbbbbbb"

func TestLoginAndPasswordRules(t *testing.T) {
	for _, l := range []string{"ab", "a", "-abc", "имя_игрока", "with space", strings.Repeat("a", 33), "bad@mail", ""} {
		if ValidLogin(NormalizeLogin(l)) {
			t.Errorf("логин %q не должен проходить", l)
		}
	}
	for _, l := range []string{"abc", "Player_1", "a.b-c", "x" + strings.Repeat("y", 31)} {
		if !ValidLogin(NormalizeLogin(l)) {
			t.Errorf("логин %q должен проходить", l)
		}
	}
	if NormalizeLogin("  PlAyEr ") != "player" {
		t.Fatal("нормализация")
	}
	for _, p := range []string{"short", "1234567", "aaaaaaaaaa", "Player_1", strings.Repeat("x", 129)} {
		if CheckPassword("player_1", p) == nil && p != "Player_1" || (p == "Player_1" && CheckPassword("player_1", p) == nil) {
			t.Errorf("пароль %q не должен проходить", p)
		}
	}
	if CheckPassword("player", "хороший пароль 42") != nil || CheckPassword("player", "correct-horse") != nil {
		t.Fatal("нормальные пароли должны проходить (в том числе русские)")
	}
}

func svc() (*Service, *world.MemStore) {
	st := world.NewMemStore()
	return NewService(st), st
}

func TestRegisterAndLoginFlow(t *testing.T) {
	ctx := context.Background()
	s, st := svc()
	rec, login, err := s.Register(ctx, "1.1.1.1", tokA, "Пилот", "  Vasya_Pro ", "secret-pass-1")
	if err != nil || login != "vasya_pro" {
		t.Fatalf("регистрация: %v %s", err, login)
	}
	// прогресс игрока устройства A
	rec.Pos = world.Vec{X: 42, Y: -7}
	_ = st.Save(ctx, rec)
	// тот же логин занят, второй аккаунт у того же игрока невозможен
	if _, _, err := s.Register(ctx, "1.1.1.1", tokB, "", "VASYA_PRO", "another-pass-1"); err != world.ErrLoginTaken {
		t.Fatalf("занятый логин: %v", err)
	}
	if _, _, err := s.Register(ctx, "1.1.1.1", tokA, "", "other_name", "another-pass-1"); err != world.ErrHasAccount {
		t.Fatalf("второй аккаунт игрока: %v", err)
	}
	// новое устройство: неверный пароль и неизвестный логин дают одинаковый ответ
	if _, _, err := s.Login(ctx, "2.2.2.2", tokB, "vasya_pro", "wrong-password"); err != ErrBadCredentials {
		t.Fatalf("неверный пароль: %v", err)
	}
	if _, _, err := s.Login(ctx, "2.2.2.2", tokB, "nobody", "whatever-pass"); err != ErrBadCredentials {
		t.Fatalf("неизвестный логин должен давать тот же ответ: %v", err)
	}
	l, pid, err := s.Login(ctx, "2.2.2.2", tokB, "Vasya_Pro", "secret-pass-1")
	if err != nil || l != "vasya_pro" || pid != rec.ID {
		t.Fatalf("вход: %v %s %s", err, l, pid)
	}
	// токен нового устройства теперь ведёт к тому же игроку и его прогрессу
	again, err := st.Resolve(ctx, tokB, "")
	if err != nil || again.ID != rec.ID || again.Pos != (world.Vec{X: 42, Y: -7}) {
		t.Fatalf("после входа: %v %+v", err, again)
	}
	if s.LoginOf(ctx, rec.ID) != "vasya_pro" {
		t.Fatal("логин аккаунта игрока")
	}
	// старое устройство тоже продолжает работать
	old, _ := st.Resolve(ctx, tokA, "")
	if old.ID != rec.ID {
		t.Fatal("старое устройство потеряло игрока")
	}
}

func TestValidationErrors(t *testing.T) {
	ctx := context.Background()
	s, _ := svc()
	if _, _, err := s.Register(ctx, "ip", tokA, "", "a", "secret-pass-1"); err != ErrBadLogin {
		t.Fatalf("логин: %v", err)
	}
	if _, _, err := s.Register(ctx, "ip", tokA, "", "valid_login", "short"); err != ErrWeakPassword {
		t.Fatalf("пароль: %v", err)
	}
	if _, _, err := s.Register(ctx, "ip", "коротко", "", "valid_login", "secret-pass-1"); err == nil {
		t.Fatal("некорректный токен")
	}
}

func TestLoginThrottle(t *testing.T) {
	ctx := context.Background()
	s, _ := svc()
	now := time.Now()
	s.now = func() time.Time { return now }
	if _, _, err := s.Register(ctx, "9.9.9.9", tokA, "", "victim", "secret-pass-1"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, _, err := s.Login(ctx, "3.3.3.3", tokB, "victim", "bad-guess-"+string(rune('a'+i))); err != ErrBadCredentials {
			t.Fatalf("попытка %d: %v", i, err)
		}
	}
	// после пяти неудач блокируются и верный пароль, и другие адреса для этого логина
	if _, _, err := s.Login(ctx, "3.3.3.3", tokB, "victim", "secret-pass-1"); err != ErrThrottled {
		t.Fatalf("блокировка по адресу: %v", err)
	}
	if _, _, err := s.Login(ctx, "4.4.4.4", tokB, "victim", "secret-pass-1"); err != ErrThrottled {
		t.Fatalf("блокировка по логину: %v", err)
	}
	now = now.Add(16 * time.Minute)
	if _, _, err := s.Login(ctx, "4.4.4.4", tokB, "victim", "secret-pass-1"); err != nil {
		t.Fatalf("после окна вход должен работать: %v", err)
	}
}

func TestRegistrationRateLimitPerIP(t *testing.T) {
	ctx := context.Background()
	s, _ := svc()
	for i := 0; i < 5; i++ {
		tok := "tokentokentokentoken" + string(rune('a'+i)) + "xx"
		if _, _, err := s.Register(ctx, "7.7.7.7", tok, "", "user_"+string(rune('a'+i)), "secret-pass-1"); err != nil {
			t.Fatalf("регистрация %d: %v", i, err)
		}
	}
	if _, _, err := s.Register(ctx, "7.7.7.7", "tokentokentokentokenzzz", "", "user_z", "secret-pass-1"); err != ErrThrottled {
		t.Fatalf("шестая регистрация с адреса: %v", err)
	}
	if _, _, err := s.Register(ctx, "8.8.8.8", "tokentokentokentokenzzz", "", "user_z", "secret-pass-1"); err != nil {
		t.Fatalf("другой адрес: %v", err)
	}
}

func TestPGAccounts(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := world.NewPGStore(pool)
	s := NewService(st)
	rec, _, err := s.Register(ctx, "1.1.1.1", tokA, "Пилот", "pg_user", "secret-pass-1")
	if err != nil {
		t.Fatal(err)
	}
	rec.Pos, rec.Skin = world.Vec{X: 5, Y: 6}, "b"
	if err := st.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Register(ctx, "1.1.1.1", tokB, "", "PG_USER", "secret-pass-1"); err != world.ErrLoginTaken {
		t.Fatalf("уникальность логина без учёта регистра: %v", err)
	}
	if _, _, err := s.Register(ctx, "1.1.1.1", tokA, "", "pg_other", "secret-pass-1"); err != world.ErrHasAccount {
		t.Fatalf("один аккаунт на игрока: %v", err)
	}
	if _, _, err := s.Login(ctx, "2.2.2.2", tokB, "pg_user", "nope-nope-nope"); err != ErrBadCredentials {
		t.Fatalf("неверный пароль: %v", err)
	}
	if _, pid, err := s.Login(ctx, "2.2.2.2", tokB, "pg_user", "secret-pass-1"); err != nil || pid != rec.ID {
		t.Fatalf("вход: %v", err)
	}
	b, err := st.Resolve(ctx, tokB, "")
	if err != nil || b.ID != rec.ID || b.Pos != (world.Vec{X: 5, Y: 6}) || b.Skin != "b" {
		t.Fatalf("новое устройство получило игрока: %v %+v", err, b)
	}
	// в базе нет пароля в открытом виде и токена
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE password_hash NOT LIKE '$argon2id$%' OR password_hash LIKE '%secret-pass%'`).Scan(&n)
	if n != 0 {
		t.Fatal("пароль хранится небезопасно")
	}
	if s.LoginOf(ctx, rec.ID) != "pg_user" {
		t.Fatal("логин игрока")
	}
}
