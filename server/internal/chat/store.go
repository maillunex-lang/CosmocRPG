package chat

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- PostgreSQL ----

// PGStore — хранилище чата в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const msgCols = `id, channel, from_id, from_name, from_ai, COALESCE(to_id,''), COALESCE(to_name,''), body, created_at`

func scanMsgs(rows pgx.Rows) ([]Message, error) {
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var t time.Time
		if err := rows.Scan(&m.ID, &m.Channel, &m.FromID, &m.FromName, &m.FromAI, &m.ToID, &m.ToName, &m.Text, &t); err != nil {
			return nil, err
		}
		m.At = t.UnixMilli()
		out = append(out, m)
	}
	return out, rows.Err()
}

func reverse(m []Message) []Message {
	for i, j := 0, len(m)-1; i < j; i, j = i+1, j-1 {
		m[i], m[j] = m[j], m[i]
	}
	return m
}

func (s *PGStore) Save(ctx context.Context, m Message) (Message, error) {
	var t time.Time
	var to, toName any
	if m.ToID != "" {
		to, toName = m.ToID, m.ToName
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO chat_messages (channel, from_id, from_name, from_ai, to_id, to_name, body) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`,
		m.Channel, m.FromID, m.FromName, m.FromAI, to, toName, m.Text).Scan(&m.ID, &t)
	m.At = t.UnixMilli()
	return m, err
}

func (s *PGStore) General(ctx context.Context, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM chat_messages WHERE channel IN ('general','system') ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	m, err := scanMsgs(rows)
	return reverse(m), err
}

func (s *PGStore) DMs(ctx context.Context, playerID string, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM chat_messages WHERE channel = 'dm' AND (to_id = $1 OR from_id = $1) ORDER BY id DESC LIMIT $2`, playerID, limit)
	if err != nil {
		return nil, err
	}
	m, err := scanMsgs(rows)
	return reverse(m), err
}

func (s *PGStore) Search(ctx context.Context, channel, q string, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM chat_messages
		WHERE ($1 = '' OR channel = $1) AND ($2 = '' OR body ILIKE '%'||$2||'%' OR from_name ILIKE '%'||$2||'%' OR to_name ILIKE '%'||$2||'%')
		ORDER BY id DESC LIMIT $3`, channel, q, limit)
	if err != nil {
		return nil, err
	}
	return scanMsgs(rows)
}

func (s *PGStore) Purge(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM chat_messages WHERE created_at < $1`, before)
	return tag.RowsAffected(), err
}

func (s *PGStore) FindPlayer(ctx context.Context, q string) (Person, bool, error) {
	var p Person
	err := s.pool.QueryRow(ctx, `SELECT public_id, name FROM players WHERE character_created AND (public_id = $1 OR lower(name) = lower($1)) LIMIT 1`, q).Scan(&p.ID, &p.Name)
	if err == pgx.ErrNoRows {
		return Person{}, false, nil
	}
	return p, err == nil, err
}

func (s *PGStore) MuteOf(ctx context.Context, playerID string) (Mute, bool, error) {
	var m Mute
	err := s.pool.QueryRow(ctx, `SELECT m.player_id, p.name, m.until, m.reason, m.admin, m.created_at FROM chat_mutes m JOIN players p ON p.public_id = m.player_id WHERE m.player_id = $1`, playerID).
		Scan(&m.PlayerID, &m.PlayerName, &m.Until, &m.Reason, &m.Admin, &m.CreatedAt)
	if err == pgx.ErrNoRows {
		return Mute{}, false, nil
	}
	return m, err == nil, err
}

func (s *PGStore) SetMute(ctx context.Context, m Mute) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_mutes (player_id, until, reason, admin) VALUES ($1,$2,$3,$4)
		ON CONFLICT (player_id) DO UPDATE SET until = EXCLUDED.until, reason = EXCLUDED.reason, admin = EXCLUDED.admin, created_at = now()`, m.PlayerID, m.Until, m.Reason, m.Admin)
	return err
}

func (s *PGStore) Unmute(ctx context.Context, playerID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM chat_mutes WHERE player_id = $1`, playerID)
	return err
}

func (s *PGStore) Mutes(ctx context.Context) ([]Mute, error) {
	rows, err := s.pool.Query(ctx, `SELECT m.player_id, p.name, m.until, m.reason, m.admin, m.created_at FROM chat_mutes m JOIN players p ON p.public_id = m.player_id ORDER BY m.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Mute{}
	for rows.Next() {
		var m Mute
		if err := rows.Scan(&m.PlayerID, &m.PlayerName, &m.Until, &m.Reason, &m.Admin, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *PGStore) LoadSettings(ctx context.Context) (Settings, bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT data FROM chat_settings WHERE id = 1`).Scan(&raw)
	if err == pgx.ErrNoRows {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, false, err
	}
	st := DefaultSettings()
	if err := json.Unmarshal(raw, &st); err != nil {
		return Settings{}, false, err
	}
	return st, true, nil
}

func (s *PGStore) SaveSettings(ctx context.Context, st Settings) error {
	raw, _ := json.Marshal(st)
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_settings (id, data) VALUES (1, $1) ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data`, raw)
	return err
}

// ---- память (тесты) ----

// MemStore — хранилище в памяти.
type MemStore struct {
	mu       sync.Mutex
	msgs     []Message
	players  map[string]Person
	mutes    map[string]Mute
	settings *Settings
	seq      int64
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore {
	return &MemStore{players: map[string]Person{}, mutes: map[string]Mute{}}
}

// AddPlayer регистрирует игрока для поиска адресатов (тесты).
func (m *MemStore) AddPlayer(id, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.players[id] = Person{ID: id, Name: name}
}

func (m *MemStore) Save(_ context.Context, msg Message) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	msg.ID = m.seq
	m.msgs = append(m.msgs, msg)
	return msg, nil
}

func tail(in []Message, limit int) []Message {
	if len(in) > limit {
		in = in[len(in)-limit:]
	}
	return append([]Message{}, in...)
}

func (m *MemStore) General(_ context.Context, limit int) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Message
	for _, x := range m.msgs {
		if x.Channel == General || x.Channel == System {
			out = append(out, x)
		}
	}
	return tail(out, limit), nil
}

func (m *MemStore) DMs(_ context.Context, pid string, limit int) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Message
	for _, x := range m.msgs {
		if x.Channel == DM && (x.ToID == pid || x.FromID == pid) {
			out = append(out, x)
		}
	}
	return tail(out, limit), nil
}

func (m *MemStore) Search(_ context.Context, channel, q string, limit int) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Message
	for i := len(m.msgs) - 1; i >= 0 && len(out) < limit; i-- {
		x := m.msgs[i]
		hay := strings.ToLower(x.Text + " " + x.FromName + " " + x.ToName)
		if (channel == "" || x.Channel == channel) && (q == "" || strings.Contains(hay, strings.ToLower(q))) {
			out = append(out, x)
		}
	}
	return out, nil
}

func (m *MemStore) Purge(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keep []Message
	var n int64
	for _, x := range m.msgs {
		if x.At < before.UnixMilli() {
			n++
		} else {
			keep = append(keep, x)
		}
	}
	m.msgs = keep
	return n, nil
}

func (m *MemStore) FindPlayer(_ context.Context, q string) (Person, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.players {
		if p.ID == q || strings.EqualFold(p.Name, q) {
			return p, true, nil
		}
	}
	return Person{}, false, nil
}

func (m *MemStore) MuteOf(_ context.Context, pid string) (Mute, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.mutes[pid]
	return x, ok, nil
}

func (m *MemStore) SetMute(_ context.Context, x Mute) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mutes[x.PlayerID] = x
	return nil
}

func (m *MemStore) Unmute(_ context.Context, pid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.mutes, pid)
	return nil
}

func (m *MemStore) Mutes(_ context.Context) ([]Mute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Mute{}
	for _, x := range m.mutes {
		out = append(out, x)
	}
	return out, nil
}

func (m *MemStore) LoadSettings(_ context.Context) (Settings, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settings == nil {
		return Settings{}, false, nil
	}
	return *m.settings, true, nil
}

func (m *MemStore) SaveSettings(_ context.Context, s Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = &s
	return nil
}
