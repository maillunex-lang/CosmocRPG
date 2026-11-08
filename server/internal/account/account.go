// Package account — аккаунты игроков: вход по логину и паролю, чтобы прогресс переживал удаление приложения.
// Пароль хранится только как хеш argon2id; вход защищён ограничением попыток. Токен устройства остаётся
// основным способом входа, аккаунт нужен для переноса на новое устройство.
package account

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/maillunex-lang/CosmocRPG/server/internal/passhash"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Коды ошибок (совпадают с кодами протокола).
var (
	ErrBadLogin       = errors.New("bad_login")
	ErrWeakPassword   = errors.New("weak_password")
	ErrBadCredentials = errors.New("bad_credentials")
	ErrThrottled      = errors.New("too_many_attempts")
)

const (
	MinPassword = 8
	MaxPassword = 128
	maxFails    = 5
	failWindow  = 15 * time.Minute
	maxKeys     = 10000
)

var loginRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.\-]{2,31}$`)

// NormalizeLogin приводит логин к нижнему регистру без пробелов по краям.
func NormalizeLogin(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidLogin: 3-32 символа, латиница, цифры, «_», «.», «-», начинается с буквы или цифры.
func ValidLogin(s string) bool { return loginRe.MatchString(s) }

// CheckPassword проверяет требования к паролю: 8-128 символов, не совпадает с логином, не из одного повторяющегося символа.
func CheckPassword(login, pass string) error {
	n := utf8.RuneCountInString(pass)
	if n < MinPassword || n > MaxPassword {
		return ErrWeakPassword
	}
	if strings.EqualFold(pass, login) {
		return ErrWeakPassword
	}
	first, same := []rune(pass)[0], true
	for _, r := range pass {
		if r != first {
			same = false
			break
		}
	}
	if same {
		return ErrWeakPassword
	}
	return nil
}

type throttle struct {
	mu     sync.Mutex
	fails  map[string][]time.Time
	window time.Duration // окно подсчёта (0 — failWindow)
	max    int           // допустимое число событий (0 — maxFails)
}

func (t *throttle) blocked(key string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.max
	if m == 0 {
		m = maxFails
	}
	return t.count(key, now) >= m
}

func (t *throttle) count(key string, now time.Time) int {
	var keep []time.Time
	for _, x := range t.fails[key] {
		w := t.window
		if w == 0 {
			w = failWindow
		}
		if now.Sub(x) < w {
			keep = append(keep, x)
		}
	}
	if len(keep) == 0 {
		delete(t.fails, key)
	} else {
		t.fails[key] = keep
	}
	return len(keep)
}

func (t *throttle) fail(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.fails) >= maxKeys { // память ограничена: при переполнении старые записи сбрасываются
		t.fails = map[string][]time.Time{}
	}
	t.fails[key] = append(t.fails[key], now)
}

func (t *throttle) reset(key string) {
	t.mu.Lock()
	delete(t.fails, key)
	t.mu.Unlock()
}

// Service — регистрация и вход.
type Service struct {
	store world.AccountStore
	now   func() time.Time
	byIP  *throttle
	byLog *throttle
	regIP *throttle // регистрации с одного адреса: каждая стоит дорогого хеширования
}

// NewService создаёт сервис.
func NewService(store world.AccountStore) *Service {
	return &Service{store: store, now: time.Now, byIP: &throttle{fails: map[string][]time.Time{}}, byLog: &throttle{fails: map[string][]time.Time{}},
		regIP: &throttle{fails: map[string][]time.Time{}, window: time.Hour, max: 5}}
}

// Register привязывает аккаунт к игроку этого устройства (создаёт игрока, если токен новый).
func (s *Service) Register(ctx context.Context, ip, token, name, login, password string) (world.Record, string, error) {
	login = NormalizeLogin(login)
	if !ValidLogin(login) {
		return world.Record{}, "", ErrBadLogin
	}
	if err := CheckPassword(login, password); err != nil {
		return world.Record{}, "", err
	}
	if !world.ValidToken(token) {
		return world.Record{}, "", errors.New("bad_token")
	}
	now := s.now()
	if s.regIP.blocked(ip, now) {
		return world.Record{}, "", ErrThrottled
	}
	s.regIP.fail(ip, now)
	hash, err := passhash.HashPassword(password)
	if err != nil {
		return world.Record{}, "", err
	}
	rec, err := s.store.CreateAccount(ctx, token, name, login, hash)
	return rec, login, err
}

// Login проверяет пароль и привязывает токен этого устройства к игроку аккаунта.
// Для неизвестного логина и неверного пароля ответ одинаковый и занимает одинаковое время.
func (s *Service) Login(ctx context.Context, ip, token, login, password string) (string, string, error) {
	login = NormalizeLogin(login)
	now := s.now()
	if s.byIP.blocked(ip, now) || s.byLog.blocked(login, now) {
		return "", "", ErrThrottled
	}
	if !world.ValidToken(token) {
		return "", "", errors.New("bad_token")
	}
	if len(password) > MaxPassword*4 {
		return "", "", ErrBadCredentials
	}
	hash, pid, ok, err := s.store.AccountHash(ctx, login)
	if err != nil {
		return "", "", err
	}
	if !ok {
		hash = passhash.DummyHash // то же время, что и при неверном пароле
	}
	good, verr := passhash.VerifyPassword(password, hash)
	if !ok || verr != nil || !good {
		s.byIP.fail(ip, now)
		s.byLog.fail(login, now)
		return "", "", ErrBadCredentials
	}
	s.byIP.reset(ip)
	s.byLog.reset(login)
	if err := s.store.LinkToken(ctx, token, pid); err != nil {
		return "", "", err
	}
	return login, pid, nil
}

// LoginOf возвращает логин аккаунта игрока ("" — аккаунта нет).
func (s *Service) LoginOf(ctx context.Context, playerID string) string {
	l, _ := s.store.AccountLogin(ctx, playerID)
	return l
}
