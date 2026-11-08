package aistudio

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — хранилище в PostgreSQL (миграция 0005_ai_studio.sql).
type PGStore struct{ pool *pgxpool.Pool }

// NewPGStore создаёт хранилище.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func decodeVersion(row pgx.Row) (Version, error) {
	var v Version
	var snap []byte
	if err := row.Scan(&v.ID, &v.CreatedAt, &v.Admin, &v.Note, &v.Source, &v.ImportID, &v.Active, &v.Captains, &snap); err != nil {
		return v, err
	}
	v.Config = NewConfig()
	if err := json.Unmarshal(snap, &v.Config); err != nil {
		return v, fmt.Errorf("снимок версии %d: %w", v.ID, err)
	}
	if v.Config.Captains == nil {
		v.Config.Captains = map[string]Captain{}
	}
	return v, nil
}

const versionCols = `id, created_at, admin, note, source, import_id, is_active, captains, snapshot`

func (s *PGStore) Active(ctx context.Context) (Version, bool, error) {
	v, err := decodeVersion(s.pool.QueryRow(ctx, `SELECT `+versionCols+` FROM ai_versions WHERE is_active`))
	if err == pgx.ErrNoRows {
		return Version{}, false, nil
	}
	return v, err == nil, err
}

// SaveVersion создаёт и активирует версию одной транзакцией; при ExpectActive проверяет, что активная не изменилась.
func (s *PGStore) SaveVersion(ctx context.Context, nv NewVersion) (int64, error) {
	snap, err := json.Marshal(nv.Config)
	if err != nil {
		return 0, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	// сериализуем конкурирующие публикации
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(724001)`); err != nil {
		return 0, err
	}
	var cur int64
	err = tx.QueryRow(ctx, `SELECT id FROM ai_versions WHERE is_active`).Scan(&cur)
	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}
	if nv.ExpectActive != nil && *nv.ExpectActive != cur {
		return 0, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_versions SET is_active = false WHERE is_active`); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO ai_versions (admin, note, source, import_id, is_active, captains, snapshot)
		VALUES ($1, $2, $3, $4, true, $5, $6) RETURNING id`,
		nv.Admin, nv.Note, nv.Source, nv.ImportID, len(nv.Config.Captains), snap).Scan(&id); err != nil {
		return 0, err
	}
	if nv.ImportID != nil {
		if _, err := tx.Exec(ctx, `UPDATE ai_imports SET status = 'published', version_id = $2 WHERE id = $1`, *nv.ImportID, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit(ctx)
}

func (s *PGStore) ListVersions(ctx context.Context, limit int) ([]VersionInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, created_at, admin, note, source, import_id, is_active, captains FROM ai_versions ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VersionInfo
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.ID, &v.CreatedAt, &v.Admin, &v.Note, &v.Source, &v.ImportID, &v.Active, &v.Captains); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *PGStore) GetVersion(ctx context.Context, id int64) (Version, error) {
	v, err := decodeVersion(s.pool.QueryRow(ctx, `SELECT `+versionCols+` FROM ai_versions WHERE id = $1`, id))
	if err == pgx.ErrNoRows {
		return v, ErrNotFound
	}
	return v, err
}

func (s *PGStore) SaveImport(ctx context.Context, r ImportRec) (int64, error) {
	rep, _ := json.Marshal(r.Report)
	plan, _ := json.Marshal(r.Plan)
	gate, _ := json.Marshal(nzGate(r.Gate))
	parsed := r.Parsed
	if len(parsed) == 0 {
		parsed = []byte("{}")
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO ai_imports (admin, filename, sha256, status, pack_id, kind, base_version, gate_ok, report, plan, gate, parsed)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		r.Admin, r.Filename, r.SHA256, r.Status, r.PackID, r.Kind, r.BaseVersion, r.GateOK, rep, plan, gate, parsed).Scan(&id)
	return id, err
}

func nzGate(g []TestResult) []TestResult {
	if g == nil {
		return []TestResult{}
	}
	return g
}

const importCols = `id, created_at, admin, filename, sha256, status, pack_id, kind, base_version, gate_ok, version_id, report, plan, gate, parsed`

func scanImport(row pgx.Row) (ImportRec, error) {
	var r ImportRec
	var rep, plan, gate, parsed []byte
	if err := row.Scan(&r.ID, &r.CreatedAt, &r.Admin, &r.Filename, &r.SHA256, &r.Status, &r.PackID, &r.Kind, &r.BaseVersion, &r.GateOK, &r.VersionID, &rep, &plan, &gate, &parsed); err != nil {
		return r, err
	}
	_ = json.Unmarshal(rep, &r.Report)
	_ = json.Unmarshal(plan, &r.Plan)
	_ = json.Unmarshal(gate, &r.Gate)
	r.Parsed = parsed
	return r, nil
}

func (s *PGStore) GetImport(ctx context.Context, id int64) (ImportRec, error) {
	r, err := scanImport(s.pool.QueryRow(ctx, `SELECT `+importCols+` FROM ai_imports WHERE id = $1`, id))
	if err == pgx.ErrNoRows {
		return r, ErrNotFound
	}
	return r, err
}

func (s *PGStore) ListImports(ctx context.Context, limit int) ([]ImportRec, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+importCols+` FROM ai_imports ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImportRec
	for rows.Next() {
		r, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		r.Parsed = nil // в списке содержимое не нужно
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGStore) SetImportStatus(ctx context.Context, id int64, status string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE ai_imports SET status = $2 WHERE id = $1`, id, status)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *PGStore) SaveTestRun(ctx context.Context, r TestRun) (int64, error) {
	res, _ := json.Marshal(r.Results)
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO ai_test_runs (admin, version_id, sample, total, passed, results) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		r.Admin, r.VersionID, r.Sample, r.Total, r.Passed, res).Scan(&id)
	return id, err
}

func (s *PGStore) ListTestRuns(ctx context.Context, limit int) ([]TestRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, created_at, admin, version_id, sample, total, passed, results FROM ai_test_runs ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestRun
	for rows.Next() {
		var r TestRun
		var res []byte
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.Admin, &r.VersionID, &r.Sample, &r.Total, &r.Passed, &res); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(res, &r.Results)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGStore) SaveSession(ctx context.Context, x Session) (int64, error) {
	ids, _ := json.Marshal(x.CaptainIDs)
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO ai_sessions (admin, label, batch_size, captain_ids, pack_id, sha256) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		x.Admin, x.Label, x.BatchSize, ids, x.PackID, x.SHA256).Scan(&id)
	return id, err
}

func (s *PGStore) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, created_at, admin, label, batch_size, captain_ids, pack_id, sha256 FROM ai_sessions ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var x Session
		var ids []byte
		if err := rows.Scan(&x.ID, &x.CreatedAt, &x.Admin, &x.Label, &x.BatchSize, &ids, &x.PackID, &x.SHA256); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(ids, &x.CaptainIDs)
		out = append(out, x)
	}
	return out, rows.Err()
}
