package world

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store хранит игроков между сессиями.
type Store interface {
	// Resolve находит игрока по токену или создаёт нового. name (может быть пустым) применяется только к новому.
	Resolve(ctx context.Context, token, name string) (Record, error)
	Save(ctx context.Context, r Record) error
}

// TokenHash — SHA-256 токена: в базе токен не хранится.
func TokenHash(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// PublicID — публичный идентификатор игрока (часть хеша токена; токен по нему не восстановить).
func PublicID(h [32]byte) string { return hex.EncodeToString(h[:8]) }

// DefaultName — имя «Пилот-NNNN» из хеша токена.
func DefaultName(h [32]byte) string {
	n := (int(h[8])<<8 | int(h[9])) % 10000
	return fmt.Sprintf("Пилот-%04d", n)
}

// CleanName приводит имя к допустимому виду: без управляющих символов, 1-24 символа. "" — имя не подходит.
func CleanName(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	rs := []rune(strings.TrimSpace(b.String()))
	if len(rs) > 24 {
		rs = rs[:24]
	}
	return string(rs)
}

// ValidToken проверяет формат токена: 16-128 символов из безопасного набора.
func ValidToken(t string) bool {
	if len(t) < 16 || len(t) > 128 {
		return false
	}
	for _, r := range t {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func fresh(h [32]byte, name string) Record {
	if name == "" {
		name = DefaultName(h)
	}
	return Record{ID: PublicID(h), Name: name, Pos: Vec{}, Dir: Vec{1, 0}}
}

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Resolve(ctx context.Context, token, name string) (Record, error) {
	h := TokenHash(token)
	// токен, привязанный к аккаунту, ведёт к игроку аккаунта (вход с нового устройства)
	var linked string
	if err := s.pool.QueryRow(ctx, `SELECT player_public_id FROM player_tokens WHERE token_hash = $1`, h[:]).Scan(&linked); err == nil {
		return s.byPublicID(ctx, linked)
	}
	rec := fresh(h, CleanName(name))
	var px, py, dx, dy float64
	var nm, skin, spec string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO players (token_hash, public_id, name, pos_x, pos_y, dir_x, dir_y) VALUES ($1, $7, $2, $3, $4, $5, $6)
		ON CONFLICT (token_hash) DO UPDATE SET token_hash = players.token_hash
		RETURNING name, pos_x, pos_y, dir_x, dir_y, ship_skin, ship_spec`,
		h[:], rec.Name, rec.Pos.X, rec.Pos.Y, rec.Dir.X, rec.Dir.Y, rec.ID).Scan(&nm, &px, &py, &dx, &dy, &skin, &spec)
	if err != nil {
		return Record{}, fmt.Errorf("игрок: %w", err)
	}
	return Record{ID: rec.ID, Name: nm, Pos: Vec{px, py}, Dir: Vec{dx, dy}, Skin: skin, Spec: spec}, nil
}

func (s *PGStore) byPublicID(ctx context.Context, pid string) (Record, error) {
	var px, py, dx, dy float64
	var nm, skin, spec string
	err := s.pool.QueryRow(ctx, `SELECT name, pos_x, pos_y, dir_x, dir_y, ship_skin, ship_spec FROM players WHERE public_id = $1`, pid).
		Scan(&nm, &px, &py, &dx, &dy, &skin, &spec)
	if err != nil {
		return Record{}, fmt.Errorf("игрок аккаунта: %w", err)
	}
	return Record{ID: pid, Name: nm, Pos: Vec{px, py}, Dir: Vec{dx, dy}, Skin: skin, Spec: spec}, nil
}

func (s *PGStore) Save(ctx context.Context, r Record) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE players SET pos_x = $2, pos_y = $3, dir_x = $4, dir_y = $5,
			ship_skin = CASE WHEN $6 <> '' THEN $6 ELSE ship_skin END,
			ship_spec = CASE WHEN $7 <> '' THEN $7 ELSE ship_spec END, updated_at = now()
		WHERE public_id = $1`, r.ID, r.Pos.X, r.Pos.Y, r.Dir.X, r.Dir.Y, r.Skin, r.Spec)
	return err
}

// MemStore — хранилище в памяти (тесты).
type MemStore struct {
	mu   sync.Mutex
	recs map[string]Record
	accs *memAccounts
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore { return &MemStore{recs: map[string]Record{}} }

func (s *MemStore) Resolve(_ context.Context, token, name string) (Record, error) {
	h := TokenHash(token)
	a := s.acc()
	a.mu.Lock()
	linked, isLinked := a.tokens[PublicID(h)]
	a.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	id := PublicID(h)
	if isLinked { // токен привязан к аккаунту: ведёт к его игроку
		if r, ok := s.recs[linked]; ok {
			return r, nil
		}
	}
	if r, ok := s.recs[id]; ok {
		return r, nil
	}
	r := fresh(h, CleanName(name))
	s.recs[id] = r
	return r, nil
}

func (s *MemStore) Save(_ context.Context, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs[r.ID] = r
	return nil
}

// Get возвращает сохранённую запись (тесты).
func (s *MemStore) Get(id string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	return r, ok
}
