package design

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ошибки хранилища.
var (
	ErrNotFound  = errors.New("замена не найдена")
	ErrPublished = errors.New("опубликованную замену нельзя удалить: сначала снимите её с публикации")
	ErrState     = errors.New("действие невозможно в текущем состоянии замены")
)

// Store — хранилище замен.
type Store interface {
	// Add сохраняет новую замену черновиком; версия — следующая для элемента.
	Add(ctx context.Context, a Asset) (Asset, error)
	// List возвращает замены без содержимого; elementID пуст — все.
	List(ctx context.Context, elementID string) ([]Asset, error)
	// Get возвращает замену вместе с содержимым.
	Get(ctx context.Context, id int64) (Asset, error)
	// BySHA возвращает опубликованную замену по сумме (для раздачи клиентам).
	BySHA(ctx context.Context, sha string) (Asset, error)
	// Publish публикует замену (прежняя опубликованная архивируется) и увеличивает версию манифеста.
	Publish(ctx context.Context, id int64) (manifest int64, err error)
	// Unpublish снимает опубликованную замену элемента: в игре возвращается встроенный элемент.
	Unpublish(ctx context.Context, elementID string) (manifest int64, err error)
	// Delete удаляет черновик или архивную замену.
	Delete(ctx context.Context, id int64) error
	// Published возвращает опубликованные замены (без содержимого) и версию манифеста.
	Published(ctx context.Context) ([]Asset, int64, error)
}

// PGStore — хранилище в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const assetCols = `id, element_id, version, file_name, mime, sha256, size, status, report, notes, uploaded_by, created_at, published_at`

func scanAsset(row pgx.Row, withData bool) (Asset, error) {
	var a Asset
	var rep []byte
	dest := []any{&a.ID, &a.ElementID, &a.Version, &a.FileName, &a.Mime, &a.SHA256, &a.Size, &a.Status, &rep, &a.Notes, &a.UploadedBy, &a.CreatedAt, &a.PublishedAt}
	if withData {
		dest = append(dest, &a.Data)
	}
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, err
	}
	a.Report = newReport()
	_ = json.Unmarshal(rep, &a.Report)
	return a, nil
}

func (s *PGStore) Add(ctx context.Context, a Asset) (Asset, error) {
	rep, _ := json.Marshal(a.Report)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback(ctx)
	// версию выдаём под блокировкой по элементу, чтобы параллельные загрузки не получили одинаковую
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "design:"+a.ElementID); err != nil {
		return Asset{}, err
	}
	var next int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM design_assets WHERE element_id = $1`, a.ElementID).Scan(&next); err != nil {
		return Asset{}, err
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO design_assets (element_id, version, file_name, mime, sha256, size, data, status, report, notes, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', $8::jsonb, $9, $10)
		RETURNING `+assetCols, a.ElementID, next, a.FileName, a.Mime, a.SHA256, a.Size, a.Data, string(rep), a.Notes, a.UploadedBy)
	out, err := scanAsset(row, false)
	if err != nil {
		return Asset{}, err
	}
	return out, tx.Commit(ctx)
}

func (s *PGStore) List(ctx context.Context, elementID string) ([]Asset, error) {
	q := `SELECT ` + assetCols + ` FROM design_assets`
	args := []any{}
	if elementID != "" {
		q += ` WHERE element_id = $1`
		args = append(args, elementID)
	}
	rows, err := s.pool.Query(ctx, q+` ORDER BY element_id, version DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Asset
	for rows.Next() {
		a, err := scanAsset(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PGStore) Get(ctx context.Context, id int64) (Asset, error) {
	return scanAsset(s.pool.QueryRow(ctx, `SELECT `+assetCols+`, data FROM design_assets WHERE id = $1`, id), true)
}

func (s *PGStore) BySHA(ctx context.Context, sha string) (Asset, error) {
	return scanAsset(s.pool.QueryRow(ctx, `SELECT `+assetCols+`, data FROM design_assets WHERE sha256 = $1 AND status = 'published' LIMIT 1`, sha), true)
}

func bumpManifest(ctx context.Context, tx pgx.Tx) (int64, error) {
	var v int64
	err := tx.QueryRow(ctx, `
		INSERT INTO design_manifest (id, version) VALUES (1, 1)
		ON CONFLICT (id) DO UPDATE SET version = design_manifest.version + 1, updated_at = now()
		RETURNING version`).Scan(&v)
	return v, err
}

func (s *PGStore) Publish(ctx context.Context, id int64) (int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var element, status string
	if err := tx.QueryRow(ctx, `SELECT element_id, status FROM design_assets WHERE id = $1 FOR UPDATE`, id).Scan(&element, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if status == StatusPublished {
		return 0, ErrState
	}
	if _, err := tx.Exec(ctx, `UPDATE design_assets SET status = 'archived' WHERE element_id = $1 AND status = 'published'`, element); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE design_assets SET status = 'published', published_at = now() WHERE id = $1`, id); err != nil {
		return 0, err
	}
	v, err := bumpManifest(ctx, tx)
	if err != nil {
		return 0, err
	}
	return v, tx.Commit(ctx)
}

func (s *PGStore) Unpublish(ctx context.Context, elementID string) (int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	t, err := tx.Exec(ctx, `UPDATE design_assets SET status = 'archived' WHERE element_id = $1 AND status = 'published'`, elementID)
	if err != nil {
		return 0, err
	}
	if t.RowsAffected() == 0 {
		return 0, ErrNotFound
	}
	v, err := bumpManifest(ctx, tx)
	if err != nil {
		return 0, err
	}
	return v, tx.Commit(ctx)
}

func (s *PGStore) Delete(ctx context.Context, id int64) error {
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT status FROM design_assets WHERE id = $1`, id).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status == StatusPublished {
		return ErrPublished
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM design_assets WHERE id = $1 AND status <> 'published'`, id)
	return err
}

func (s *PGStore) Published(ctx context.Context) ([]Asset, int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+assetCols+` FROM design_assets WHERE status = 'published' ORDER BY element_id`)
	if err != nil {
		return nil, 0, err
	}
	var out []Asset
	for rows.Next() {
		a, err := scanAsset(rows, false)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var v int64
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT version FROM design_manifest WHERE id = 1), 0)`).Scan(&v); err != nil {
		return nil, 0, err
	}
	return out, v, nil
}

// MemStore — хранилище в памяти (тесты).
type MemStore struct {
	mu       sync.Mutex
	assets   []*Asset
	nextID   int64
	manifest int64
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore { return &MemStore{} }

func (m *MemStore) find(id int64) *Asset {
	for _, a := range m.assets {
		if a.ID == id {
			return a
		}
	}
	return nil
}

func (m *MemStore) Add(_ context.Context, a Asset) (Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := 1
	for _, x := range m.assets {
		if x.ElementID == a.ElementID && x.Version >= next {
			next = x.Version + 1
		}
	}
	m.nextID++
	a.ID, a.Version, a.Status, a.CreatedAt = m.nextID, next, StatusDraft, time.Now()
	cp := a
	m.assets = append(m.assets, &cp)
	a.Data = nil
	return a, nil
}

func (m *MemStore) List(_ context.Context, elementID string) ([]Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Asset
	for _, a := range m.assets {
		if elementID == "" || a.ElementID == elementID {
			c := *a
			c.Data = nil
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ElementID != out[j].ElementID {
			return out[i].ElementID < out[j].ElementID
		}
		return out[i].Version > out[j].Version
	})
	return out, nil
}

func (m *MemStore) Get(_ context.Context, id int64) (Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a := m.find(id); a != nil {
		return *a, nil
	}
	return Asset{}, ErrNotFound
}

func (m *MemStore) BySHA(_ context.Context, sha string) (Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.assets {
		if a.SHA256 == sha && a.Status == StatusPublished {
			return *a, nil
		}
	}
	return Asset{}, ErrNotFound
}

func (m *MemStore) Publish(_ context.Context, id int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.find(id)
	if a == nil {
		return 0, ErrNotFound
	}
	if a.Status == StatusPublished {
		return 0, ErrState
	}
	for _, x := range m.assets {
		if x.ElementID == a.ElementID && x.Status == StatusPublished {
			x.Status = StatusArchived
		}
	}
	now := time.Now()
	a.Status, a.PublishedAt = StatusPublished, &now
	m.manifest++
	return m.manifest, nil
}

func (m *MemStore) Unpublish(_ context.Context, elementID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := false
	for _, x := range m.assets {
		if x.ElementID == elementID && x.Status == StatusPublished {
			x.Status, found = StatusArchived, true
		}
	}
	if !found {
		return 0, ErrNotFound
	}
	m.manifest++
	return m.manifest, nil
}

func (m *MemStore) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, a := range m.assets {
		if a.ID == id {
			if a.Status == StatusPublished {
				return ErrPublished
			}
			m.assets = append(m.assets[:i], m.assets[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *MemStore) Published(_ context.Context) ([]Asset, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Asset
	for _, a := range m.assets {
		if a.Status == StatusPublished {
			c := *a
			c.Data = nil
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ElementID < out[j].ElementID })
	return out, m.manifest, nil
}
