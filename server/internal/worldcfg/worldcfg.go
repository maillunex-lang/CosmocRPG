// Package worldcfg — версии конфигурации мира (системы, враги, ресурсы, визуальные параметры живых локаций):
// предпросмотр, публикация, откат. Активная конфигурация применяется к миру на лету через onChange.
package worldcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
)

var (
	// ErrConflict — активная версия изменилась после предпросмотра.
	ErrConflict = errors.New("конфигурация изменилась, обновите страницу")
	// ErrNotFound — версия не найдена.
	ErrNotFound = errors.New("не найдено")
	// MaxConfigBytes — предельный размер конфигурации.
	MaxConfigBytes = 256 << 10
)

// VersionInfo — версия без содержимого.
type VersionInfo struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Admin     string    `json:"admin"`
	Note      string    `json:"note"`
	Source    string    `json:"source"` // seed | edit | rollback
	Active    bool      `json:"active"`
}

// Version — версия с содержимым.
type Version struct {
	VersionInfo
	Config json.RawMessage `json:"-"`
}

// NewVersion — запрос на создание и активацию версии.
type NewVersion struct {
	Admin, Note, Source string
	Config              json.RawMessage
	ExpectActive        *int64
}

// Store — хранилище версий.
type Store interface {
	Active(ctx context.Context) (Version, bool, error)
	Save(ctx context.Context, v NewVersion) (int64, error)
	List(ctx context.Context, limit int) ([]VersionInfo, error)
	Get(ctx context.Context, id int64) (Version, error)
}

// SystemSummary — краткие числа раскладки системы для предпросмотра.
type SystemSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Bodies   int    `json:"bodies"`
	Stations int    `json:"stations"`
	Zones    int    `json:"zones"`
	Deposits int    `json:"deposits"`
	Spawns   int    `json:"spawns"`
	Points   int    `json:"points"`
}

// Preview — результат проверки конфигурации без публикации.
type Preview struct {
	OK      bool            `json:"ok"`
	Errors  []string        `json:"errors"`
	Systems []SystemSummary `json:"systems"`
}

// Service — конфигурация мира с версиями. onChange вызывается после смены активной версии.
type Service struct {
	store    Store
	onChange func(worldgen.Config)
}

// NewService создаёт службу.
func NewService(store Store, onChange func(worldgen.Config)) *Service {
	if onChange == nil {
		onChange = func(worldgen.Config) {}
	}
	return &Service{store: store, onChange: onChange}
}

// Init создаёт начальную версию из встроенной конфигурации (если версий нет) и применяет активную.
func (s *Service) Init(ctx context.Context) error {
	v, ok, err := s.store.Active(ctx)
	if err != nil {
		return err
	}
	if !ok {
		raw, _ := json.Marshal(worldgen.DefaultConfig())
		zero := int64(0)
		if _, err := s.store.Save(ctx, NewVersion{Admin: "система", Note: "Начальная конфигурация из встроенных данных", Source: "seed", Config: raw, ExpectActive: &zero}); err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
		if v, ok, err = s.store.Active(ctx); err != nil || !ok {
			return fmt.Errorf("не удалось создать начальную конфигурацию мира: %v", err)
		}
	}
	cfg, err := worldgen.Parse(v.Config)
	if err != nil {
		return fmt.Errorf("активная конфигурация мира повреждена: %w", err)
	}
	s.onChange(cfg)
	return nil
}

// Active возвращает активную конфигурацию (JSON) и номер версии.
func (s *Service) Active(ctx context.Context) (json.RawMessage, int64, error) {
	v, ok, err := s.store.Active(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		raw, _ := json.Marshal(worldgen.DefaultConfig())
		return raw, 0, nil
	}
	return v.Config, v.ID, nil
}

// Versions возвращает последние версии.
func (s *Service) Versions(ctx context.Context) ([]VersionInfo, error) { return s.store.List(ctx, 50) }

// Check проверяет конфигурацию и возвращает сводку по системам; ничего не публикует.
func Check(raw []byte) Preview {
	p := Preview{Errors: []string{}, Systems: []SystemSummary{}}
	if len(raw) == 0 || len(raw) > MaxConfigBytes {
		p.Errors = append(p.Errors, fmt.Sprintf("размер конфигурации: от 1 байта до %d КиБ", MaxConfigBytes>>10))
		return p
	}
	var c worldgen.Config
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		p.Errors = append(p.Errors, "JSON: "+err.Error())
		return p
	}
	if errs := c.Validate(); len(errs) > 0 {
		p.Errors = errs
		return p
	}
	for _, sp := range c.Systems {
		l := worldgen.Generate(sp, c)
		p.Systems = append(p.Systems, SystemSummary{ID: sp.ID, Name: sp.Name, Bodies: len(l.Bodies), Stations: len(l.Stations), Zones: len(l.Zones),
			Deposits: len(l.Deposits), Spawns: len(l.Spawns), Points: len(l.Points)})
	}
	p.OK = true
	return p
}

func (s *Service) commit(ctx context.Context, admin, note, source string, raw []byte, base int64) (int64, error) {
	cfg, err := worldgen.Parse(raw)
	if err != nil {
		return 0, err
	}
	norm, _ := json.Marshal(cfg) // сохраняется нормализованный вид
	id, err := s.store.Save(ctx, NewVersion{Admin: admin, Note: note, Source: source, Config: norm, ExpectActive: &base})
	if err != nil {
		return 0, err
	}
	s.onChange(cfg)
	return id, nil
}

// Publish публикует конфигурацию: base — версия, на основе которой правил администратор (защита от перезаписи чужих правок).
func (s *Service) Publish(ctx context.Context, admin, note string, raw []byte, base int64) (int64, error) {
	if len(note) > 200 {
		note = note[:200]
	}
	if note == "" {
		note = "Правка конфигурации мира"
	}
	return s.commit(ctx, admin, note, "edit", raw, base)
}

// Rollback возвращает конфигурацию к прежней версии (создаётся новая версия, история сохраняется).
func (s *Service) Rollback(ctx context.Context, admin string, id int64) (int64, error) {
	v, err := s.store.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	_, cur, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.commit(ctx, admin, fmt.Sprintf("Откат к версии %d", id), "rollback", v.Config, cur)
}

// ---- PostgreSQL ----

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const cols = `id, created_at, admin, note, source, is_active, snapshot`

func scan(row pgx.Row) (Version, error) {
	var v Version
	var snap []byte
	if err := row.Scan(&v.ID, &v.CreatedAt, &v.Admin, &v.Note, &v.Source, &v.Active, &snap); err != nil {
		return v, err
	}
	v.Config = snap
	return v, nil
}

func (s *PGStore) Active(ctx context.Context) (Version, bool, error) {
	v, err := scan(s.pool.QueryRow(ctx, `SELECT `+cols+` FROM world_config_versions WHERE is_active`))
	if err == pgx.ErrNoRows {
		return Version{}, false, nil
	}
	return v, err == nil, err
}

func (s *PGStore) Save(ctx context.Context, nv NewVersion) (int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(724011)`); err != nil {
		return 0, err
	}
	var cur int64
	if err := tx.QueryRow(ctx, `SELECT id FROM world_config_versions WHERE is_active`).Scan(&cur); err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	if nv.ExpectActive != nil && *nv.ExpectActive != cur {
		return 0, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE world_config_versions SET is_active = false WHERE is_active`); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO world_config_versions (admin, note, source, is_active, snapshot) VALUES ($1,$2,$3,true,$4) RETURNING id`,
		nv.Admin, nv.Note, nv.Source, []byte(nv.Config)).Scan(&id); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (s *PGStore) List(ctx context.Context, limit int) ([]VersionInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, created_at, admin, note, source, is_active FROM world_config_versions ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.ID, &v.CreatedAt, &v.Admin, &v.Note, &v.Source, &v.Active); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *PGStore) Get(ctx context.Context, id int64) (Version, error) {
	v, err := scan(s.pool.QueryRow(ctx, `SELECT `+cols+` FROM world_config_versions WHERE id = $1`, id))
	if err == pgx.ErrNoRows {
		return v, ErrNotFound
	}
	return v, err
}

// ---- память (тесты) ----

// MemStore — хранилище в памяти.
type MemStore struct {
	mu       sync.Mutex
	versions []Version
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore { return &MemStore{} }

func (m *MemStore) Active(_ context.Context) (Version, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.versions {
		if v.Active {
			return v, true, nil
		}
	}
	return Version{}, false, nil
}

func (m *MemStore) Save(_ context.Context, nv NewVersion) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := int64(0)
	for _, v := range m.versions {
		if v.Active {
			cur = v.ID
		}
	}
	if nv.ExpectActive != nil && *nv.ExpectActive != cur {
		return 0, ErrConflict
	}
	for i := range m.versions {
		m.versions[i].Active = false
	}
	id := int64(len(m.versions) + 1)
	m.versions = append(m.versions, Version{VersionInfo: VersionInfo{ID: id, CreatedAt: time.Now(), Admin: nv.Admin, Note: nv.Note, Source: nv.Source, Active: true}, Config: append(json.RawMessage{}, nv.Config...)})
	return id, nil
}

func (m *MemStore) List(_ context.Context, limit int) ([]VersionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []VersionInfo{}
	for i := len(m.versions) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.versions[i].VersionInfo)
	}
	return out, nil
}

func (m *MemStore) Get(_ context.Context, id int64) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.versions {
		if v.ID == id {
			return v, nil
		}
	}
	return Version{}, ErrNotFound
}
