package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound — запись не найдена (в том числе просроченная сессия).
var ErrNotFound = errors.New("не найдено")

// User — администратор.
type User struct {
	ID       int64
	Username string
	Hash     string
	Disabled bool
}

// AuditEntry — запись журнала административных действий.
type AuditEntry struct {
	ID       int64          `json:"id"`
	At       time.Time      `json:"at"`
	Username string         `json:"username"`
	Action   string         `json:"action"`
	IP       string         `json:"ip"`
	Details  map[string]any `json:"details"`
}

// Store — хранилище администраторов, сессий и журнала.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore создаёт хранилище поверх пула соединений.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// NormalizeUsername приводит имя к нижнему регистру и обрезает пробелы.
func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// CreateUser создаёт администратора. Пароль проверяется на минимальную длину.
func (s *Store) CreateUser(ctx context.Context, username, password string) (User, error) {
	username = NormalizeUsername(username)
	if len(username) < 3 || len(username) > 64 {
		return User{}, errors.New("имя пользователя: от 3 до 64 символов")
	}
	if len([]rune(password)) < MinPasswordLen {
		return User{}, fmt.Errorf("пароль короче %d символов", MinPasswordLen)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	u := User{Username: username, Hash: hash}
	err = s.pool.QueryRow(ctx,
		"INSERT INTO admin_users(username, password_hash) VALUES($1,$2) RETURNING id", username, hash).Scan(&u.ID)
	if err != nil {
		return User{}, fmt.Errorf("создание администратора: %w", err)
	}
	return u, nil
}

// UserByName ищет администратора по имени.
func (s *Store) UserByName(ctx context.Context, username string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		"SELECT id, username, password_hash, disabled FROM admin_users WHERE username=$1",
		NormalizeUsername(username)).Scan(&u.ID, &u.Username, &u.Hash, &u.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func hashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

// CreateSession создаёт сессию и возвращает сырой токен (в базе хранится только его хеш).
func (s *Store) CreateSession(ctx context.Context, adminID int64, ttl time.Duration) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	expires := time.Now().Add(ttl)
	_, err := s.pool.Exec(ctx,
		"INSERT INTO admin_sessions(token_hash, admin_id, expires_at) VALUES($1,$2,$3)",
		hashToken(raw), adminID, expires)
	return raw, expires, err
}

// SessionUser возвращает администратора по токену сессии. Просроченные и отключённые не проходят.
func (s *Store) SessionUser(ctx context.Context, raw string) (User, error) {
	if raw == "" {
		return User{}, ErrNotFound
	}
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.password_hash, u.disabled
		FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND NOT u.disabled`,
		hashToken(raw)).Scan(&u.ID, &u.Username, &u.Hash, &u.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// DeleteSession завершает сессию.
func (s *Store) DeleteSession(ctx context.Context, raw string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM admin_sessions WHERE token_hash=$1", hashToken(raw))
	return err
}

// CleanupSessions удаляет просроченные сессии.
func (s *Store) CleanupSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, "DELETE FROM admin_sessions WHERE expires_at <= now()")
	return tag.RowsAffected(), err
}

// Audit пишет запись в журнал. adminID может быть 0 (неизвестный пользователь).
func (s *Store) Audit(ctx context.Context, adminID int64, username, action, ip string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	js, err := json.Marshal(details)
	if err != nil {
		return err
	}
	var id any
	if adminID != 0 {
		id = adminID
	}
	_, err = s.pool.Exec(ctx,
		"INSERT INTO admin_audit_log(admin_id, username, action, ip, details) VALUES($1,$2,$3,$4,$5)",
		id, username, action, ip, js)
	return err
}

// RecentAudit возвращает последние записи журнала, новые первыми.
func (s *Store) RecentAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		"SELECT id, at, username, action, ip, details FROM admin_audit_log ORDER BY id DESC LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var raw []byte
		if err := rows.Scan(&e.ID, &e.At, &e.Username, &e.Action, &e.IP, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &e.Details)
		out = append(out, e)
	}
	return out, rows.Err()
}

// BootCount возвращает число запусков сервера.
func (s *Store) BootCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM server_boots").Scan(&n)
	return n, err
}

// Ping проверяет доступность базы.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
