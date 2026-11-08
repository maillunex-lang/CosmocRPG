package aistudio

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrConflict — активная версия изменилась с момента предпросмотра.
var ErrConflict = errors.New("конфигурация изменилась, повторите предпросмотр")

// ErrNotFound — запись не найдена.
var ErrNotFound = errors.New("не найдено")

// VersionInfo — версия конфигурации без содержимого.
type VersionInfo struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Admin     string    `json:"admin"`
	Note      string    `json:"note"`
	Source    string    `json:"source"`
	ImportID  *int64    `json:"import_id"`
	Active    bool      `json:"active"`
	Captains  int       `json:"captains"`
}

// Version — версия с конфигурацией.
type Version struct {
	VersionInfo
	Config Config `json:"-"`
}

// NewVersion — запрос на создание и активацию версии.
type NewVersion struct {
	Admin, Note, Source string
	ImportID            *int64
	Config              Config
	// ExpectActive: если задан, версия создаётся только когда активна именно эта версия (0 — активной нет).
	ExpectActive *int64
}

// ImportRec — загруженный пакет: отчёт, план и разобранное содержимое.
type ImportRec struct {
	ID          int64           `json:"id"`
	CreatedAt   time.Time       `json:"created_at"`
	Admin       string          `json:"admin"`
	Filename    string          `json:"filename"`
	SHA256      string          `json:"sha256"`
	Status      string          `json:"status"` // validated | rejected | published | discarded
	PackID      string          `json:"pack_id"`
	Kind        string          `json:"kind"`
	BaseVersion *int64          `json:"base_version"`
	GateOK      bool            `json:"gate_ok"`
	VersionID   *int64          `json:"version_id"`
	Report      Report          `json:"report"`
	Plan        Plan            `json:"plan"`
	Gate        []TestResult    `json:"gate"`
	Parsed      json.RawMessage `json:"-"`
}

// TestRun — сохранённый прогон экзаменов.
type TestRun struct {
	ID        int64        `json:"id"`
	CreatedAt time.Time    `json:"created_at"`
	Admin     string       `json:"admin"`
	VersionID *int64       `json:"version_id"`
	Sample    int          `json:"sample"`
	Total     int          `json:"total"`
	Passed    int          `json:"passed"`
	Results   []TestResult `json:"results"`
}

// Session — экспортированный обучающий пакет.
type Session struct {
	ID         int64     `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Admin      string    `json:"admin"`
	Label      string    `json:"label"`
	BatchSize  int       `json:"batch_size"`
	CaptainIDs []string  `json:"captain_ids"`
	PackID     string    `json:"pack_id"`
	SHA256     string    `json:"sha256"`
}

// Store — хранилище Studio. Все составные операции атомарны.
type Store interface {
	Active(ctx context.Context) (Version, bool, error)
	SaveVersion(ctx context.Context, v NewVersion) (int64, error)
	ListVersions(ctx context.Context, limit int) ([]VersionInfo, error)
	GetVersion(ctx context.Context, id int64) (Version, error)

	SaveImport(ctx context.Context, rec ImportRec) (int64, error)
	GetImport(ctx context.Context, id int64) (ImportRec, error)
	ListImports(ctx context.Context, limit int) ([]ImportRec, error)
	SetImportStatus(ctx context.Context, id int64, status string) error

	SaveTestRun(ctx context.Context, r TestRun) (int64, error)
	ListTestRuns(ctx context.Context, limit int) ([]TestRun, error)

	SaveSession(ctx context.Context, s Session) (int64, error)
	ListSessions(ctx context.Context, limit int) ([]Session, error)
}

// MemStore — хранилище в памяти (тесты); повторяет семантику PGStore.
type MemStore struct {
	mu       sync.Mutex
	versions []Version
	imports  []ImportRec
	runs     []TestRun
	sessions []Session
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore { return &MemStore{} }

func (m *MemStore) activeIdx() int {
	for i := range m.versions {
		if m.versions[i].Active {
			return i
		}
	}
	return -1
}

func (m *MemStore) Active(_ context.Context) (Version, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i := m.activeIdx(); i >= 0 {
		v := m.versions[i]
		v.Config = v.Config.Clone()
		return v, true, nil
	}
	return Version{}, false, nil
}

func (m *MemStore) SaveVersion(_ context.Context, nv NewVersion) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := int64(0)
	if i := m.activeIdx(); i >= 0 {
		cur = m.versions[i].ID
	}
	if nv.ExpectActive != nil && *nv.ExpectActive != cur {
		return 0, ErrConflict
	}
	for i := range m.versions {
		m.versions[i].Active = false
	}
	id := int64(len(m.versions) + 1)
	m.versions = append(m.versions, Version{
		VersionInfo: VersionInfo{ID: id, CreatedAt: time.Now(), Admin: nv.Admin, Note: nv.Note, Source: nv.Source, ImportID: nv.ImportID, Active: true, Captains: len(nv.Config.Captains)},
		Config:      nv.Config.Clone(),
	})
	if nv.ImportID != nil {
		for i := range m.imports {
			if m.imports[i].ID == *nv.ImportID {
				m.imports[i].Status = "published"
				vid := id
				m.imports[i].VersionID = &vid
			}
		}
	}
	return id, nil
}

func (m *MemStore) ListVersions(_ context.Context, limit int) ([]VersionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []VersionInfo
	for i := len(m.versions) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.versions[i].VersionInfo)
	}
	return out, nil
}

func (m *MemStore) GetVersion(_ context.Context, id int64) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.versions {
		if v.ID == id {
			v.Config = v.Config.Clone()
			return v, nil
		}
	}
	return Version{}, ErrNotFound
}

func (m *MemStore) SaveImport(_ context.Context, rec ImportRec) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec.ID = int64(len(m.imports) + 1)
	rec.CreatedAt = time.Now()
	m.imports = append(m.imports, rec)
	return rec.ID, nil
}

func (m *MemStore) GetImport(_ context.Context, id int64) (ImportRec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.imports {
		if r.ID == id {
			return r, nil
		}
	}
	return ImportRec{}, ErrNotFound
}

func (m *MemStore) ListImports(_ context.Context, limit int) ([]ImportRec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ImportRec
	for i := len(m.imports) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.imports[i])
	}
	return out, nil
}

func (m *MemStore) SetImportStatus(_ context.Context, id int64, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.imports {
		if m.imports[i].ID == id {
			m.imports[i].Status = status
			return nil
		}
	}
	return ErrNotFound
}

func (m *MemStore) SaveTestRun(_ context.Context, r TestRun) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.ID = int64(len(m.runs) + 1)
	r.CreatedAt = time.Now()
	m.runs = append(m.runs, r)
	return r.ID, nil
}

func (m *MemStore) ListTestRuns(_ context.Context, limit int) ([]TestRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []TestRun
	for i := len(m.runs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.runs[i])
	}
	return out, nil
}

func (m *MemStore) SaveSession(_ context.Context, s Session) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.ID = int64(len(m.sessions) + 1)
	s.CreatedAt = time.Now()
	m.sessions = append(m.sessions, s)
	return s.ID, nil
}

func (m *MemStore) ListSessions(_ context.Context, limit int) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Session
	for i := len(m.sessions) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.sessions[i])
	}
	return out, nil
}
