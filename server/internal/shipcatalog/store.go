package shipcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// ErrConflict — активная версия каталога изменилась.
var ErrConflict = errors.New("каталог изменился, повторите предпросмотр")

// ErrNotFound — версия не найдена.
var ErrNotFound = errors.New("не найдено")

// VersionInfo — версия каталога без содержимого.
type VersionInfo struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Admin     string    `json:"admin"`
	Note      string    `json:"note"`
	Source    string    `json:"source"` // seed | edit | patch | rollback
	Active    bool      `json:"active"`
	Ships     int       `json:"ships"`
}

// Version — версия с содержимым.
type Version struct {
	VersionInfo
	Doc world.Doc `json:"-"`
}

// NewVersion — запрос на создание и активацию версии.
type NewVersion struct {
	Admin, Note, Source string
	Doc                 world.Doc
	ExpectActive        *int64 // версия создаётся, только если активна именно эта (0 — активной нет)
}

// Store — хранилище версий каталога.
type Store interface {
	Active(ctx context.Context) (Version, bool, error)
	Save(ctx context.Context, v NewVersion) (int64, error)
	List(ctx context.Context, limit int) ([]VersionInfo, error)
	Get(ctx context.Context, id int64) (Version, error)
}

// ---- PostgreSQL ----

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func scanVersion(row pgx.Row) (Version, error) {
	var v Version
	var snap []byte
	if err := row.Scan(&v.ID, &v.CreatedAt, &v.Admin, &v.Note, &v.Source, &v.Active, &snap); err != nil {
		return v, err
	}
	if err := json.Unmarshal(snap, &v.Doc); err != nil {
		return v, err
	}
	v.Ships = len(v.Doc.Ships)
	return v, nil
}

const cols = `id, created_at, admin, note, source, is_active, snapshot`

func (s *PGStore) Active(ctx context.Context) (Version, bool, error) {
	v, err := scanVersion(s.pool.QueryRow(ctx, `SELECT `+cols+` FROM ship_catalog_versions WHERE is_active`))
	if err == pgx.ErrNoRows {
		return Version{}, false, nil
	}
	return v, err == nil, err
}

func (s *PGStore) Save(ctx context.Context, nv NewVersion) (int64, error) {
	snap, err := json.Marshal(nv.Doc)
	if err != nil {
		return 0, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(724002)`); err != nil {
		return 0, err
	}
	var cur int64
	if err := tx.QueryRow(ctx, `SELECT id FROM ship_catalog_versions WHERE is_active`).Scan(&cur); err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	if nv.ExpectActive != nil && *nv.ExpectActive != cur {
		return 0, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE ship_catalog_versions SET is_active = false WHERE is_active`); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO ship_catalog_versions (admin, note, source, is_active, snapshot) VALUES ($1,$2,$3,true,$4) RETURNING id`,
		nv.Admin, nv.Note, nv.Source, snap).Scan(&id); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (s *PGStore) List(ctx context.Context, limit int) ([]VersionInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, created_at, admin, note, source, is_active, jsonb_array_length(snapshot->'ships') FROM ship_catalog_versions ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VersionInfo
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.ID, &v.CreatedAt, &v.Admin, &v.Note, &v.Source, &v.Active, &v.Ships); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *PGStore) Get(ctx context.Context, id int64) (Version, error) {
	v, err := scanVersion(s.pool.QueryRow(ctx, `SELECT `+cols+` FROM ship_catalog_versions WHERE id = $1`, id))
	if err == pgx.ErrNoRows {
		return v, ErrNotFound
	}
	return v, err
}

// ---- память (тесты) ----

type MemStore struct {
	mu       sync.Mutex
	versions []Version
}

func NewMemStore() *MemStore { return &MemStore{} }

func clone(d world.Doc) world.Doc {
	b, _ := json.Marshal(d)
	var out world.Doc
	_ = json.Unmarshal(b, &out)
	return out
}

func (m *MemStore) Active(_ context.Context) (Version, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.versions {
		if v.Active {
			v.Doc = clone(v.Doc)
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
	m.versions = append(m.versions, Version{VersionInfo: VersionInfo{ID: id, CreatedAt: time.Now(), Admin: nv.Admin, Note: nv.Note, Source: nv.Source, Active: true, Ships: len(nv.Doc.Ships)}, Doc: clone(nv.Doc)})
	return id, nil
}

func (m *MemStore) List(_ context.Context, limit int) ([]VersionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []VersionInfo
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
			v.Doc = clone(v.Doc)
			return v, nil
		}
	}
	return Version{}, ErrNotFound
}
