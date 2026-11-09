package world

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store хранит игроков между сессиями.
type Store interface {
	// CreateCharacter завершает создание персонажа один раз: ник (уникален), пол, направление пилота и стартовый корабль.
	CreateCharacter(ctx context.Context, id, nick, gender, pilot string, st Starter) (Record, error)
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
	var nm, skin, spec, shipType, gender, pilot string
	var created bool
	var holdRaw []byte
	var sysID string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO players (token_hash, public_id, name, pos_x, pos_y, dir_x, dir_y) VALUES ($1, $7, $2, $3, $4, $5, $6)
		ON CONFLICT (token_hash) DO UPDATE SET token_hash = players.token_hash
		RETURNING name, pos_x, pos_y, dir_x, dir_y, ship_skin, ship_spec, ship_type, gender, pilot_spec, character_created, hold, current_system`,
		h[:], rec.Name, rec.Pos.X, rec.Pos.Y, rec.Dir.X, rec.Dir.Y, rec.ID).Scan(&nm, &px, &py, &dx, &dy, &skin, &spec, &shipType, &gender, &pilot, &created, &holdRaw, &sysID)
	if err != nil {
		return Record{}, fmt.Errorf("игрок: %w", err)
	}
	return Record{ID: rec.ID, Name: nm, Pos: Vec{px, py}, Dir: Vec{dx, dy}, Skin: skin, Spec: spec, ShipType: shipType, Gender: gender, PilotSpec: pilot, Created: created, Hold: parseHold(holdRaw), System: sysID}, nil
}

func (s *PGStore) CreateCharacter(ctx context.Context, id, nick, gender, pilot string, st Starter) (Record, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE players SET name = $2, gender = $3, pilot_spec = $4, ship_type = $5, ship_skin = $6, ship_spec = $7, character_created = true, updated_at = now()
		WHERE public_id = $1 AND NOT character_created`, id, nick, gender, pilot, st.Ship, st.Skin, st.Spec)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return Record{}, ErrNickTaken
		}
		return Record{}, fmt.Errorf("персонаж: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Record{}, ErrCharacterSet
	}
	return s.byPublicID(ctx, id)
}

func (s *PGStore) byPublicID(ctx context.Context, pid string) (Record, error) {
	var px, py, dx, dy float64
	var nm, skin, spec, shipType, gender, pilot string
	var created bool
	var holdRaw []byte
	var sysID string
	err := s.pool.QueryRow(ctx, `SELECT name, pos_x, pos_y, dir_x, dir_y, ship_skin, ship_spec, ship_type, gender, pilot_spec, character_created, hold, current_system FROM players WHERE public_id = $1`, pid).
		Scan(&nm, &px, &py, &dx, &dy, &skin, &spec, &shipType, &gender, &pilot, &created, &holdRaw, &sysID)
	if err != nil {
		return Record{}, fmt.Errorf("игрок аккаунта: %w", err)
	}
	if sysID == "xi" {
		sysID = "aurora" // прежняя система «Кси» переименована в Аврору (Э-1)
	}
	return Record{ID: pid, Name: nm, Pos: Vec{px, py}, Dir: Vec{dx, dy}, Skin: skin, Spec: spec, ShipType: shipType, Gender: gender, PilotSpec: pilot, Created: created, Hold: parseHold(holdRaw), System: sysID}, nil
}

func parseHold(raw []byte) map[string]float64 {
	h := map[string]float64{}
	_ = json.Unmarshal(raw, &h)
	return h
}

func (s *PGStore) Save(ctx context.Context, r Record) error {
	hold, _ := json.Marshal(copyHold(r.Hold))
	_, err := s.pool.Exec(ctx, `
		UPDATE players SET pos_x = $2, pos_y = $3, dir_x = $4, dir_y = $5,
			ship_skin = CASE WHEN $6 <> '' THEN $6 ELSE ship_skin END,
			ship_spec = CASE WHEN $7 <> '' THEN $7 ELSE ship_spec END,
			ship_type = CASE WHEN $8 <> '' THEN $8 ELSE ship_type END, hold = $9::jsonb,
			current_system = CASE WHEN $10 <> '' THEN $10 ELSE current_system END, updated_at = now()
		WHERE public_id = $1`, r.ID, r.Pos.X, r.Pos.Y, r.Dir.X, r.Dir.Y, r.Skin, r.Spec, r.ShipType, string(hold), r.System)
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
	r.PersistentEquipment = false
	r.StarterLoadout = nil // equipment is stored independently, never by autosave
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

func (s *MemStore) CreateCharacter(_ context.Context, id, nick, gender, pilot string, st Starter) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return Record{}, errors.New("игрока нет")
	}
	if r.Created {
		return Record{}, ErrCharacterSet
	}
	for _, o := range s.recs {
		if o.Created && strings.EqualFold(o.Name, nick) {
			return Record{}, ErrNickTaken
		}
	}
	r.Name, r.Gender, r.PilotSpec, r.ShipType, r.Skin, r.Spec, r.Created = nick, gender, pilot, st.Ship, st.Skin, st.Spec, true
	s.recs[id] = r
	return r, nil
}
