package media

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Track — запись библиотеки. Файл на диске называется sha256.ext.
type Track struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	DurationMS  int       `json:"duration_ms"`
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	Ext         string    `json:"ext"`
	MIME        string    `json:"mime"`
	License     string    `json:"license"`
	Source      string    `json:"source"`
	AIGenerated bool      `json:"ai_generated"`
	Tags        string    `json:"tags"`
	Enabled     bool      `json:"enabled"`
	UploadedBy  string    `json:"uploaded_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	URL         string    `json:"url,omitempty"`
}

// Store — хранилище метаданных.
type Store interface {
	Insert(ctx context.Context, t Track) (Track, error)
	UpdateMeta(ctx context.Context, t Track) (Track, error)
	Delete(ctx context.Context, id int64) (Track, error)
	ByID(ctx context.Context, id int64) (Track, error)
	BySHA(ctx context.Context, sha string) (Track, error)
	List(ctx context.Context) ([]Track, error)
}

// PGStore — метаданные в PostgreSQL.
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const trackCols = `id, kind, title, artist, duration_ms, sha256, size, ext, mime, license, source, ai_generated, tags, enabled, uploaded_by, created_at`

func scanTrack(row pgx.Row) (Track, error) {
	var t Track
	err := row.Scan(&t.ID, &t.Kind, &t.Title, &t.Artist, &t.DurationMS, &t.SHA256, &t.Size, &t.Ext, &t.MIME, &t.License, &t.Source, &t.AIGenerated, &t.Tags, &t.Enabled, &t.UploadedBy, &t.CreatedAt)
	return t, err
}

func (s *PGStore) Insert(ctx context.Context, t Track) (Track, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO media_tracks (kind, title, artist, duration_ms, sha256, size, ext, mime, license, source, ai_generated, tags, enabled, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING `+trackCols,
		t.Kind, t.Title, t.Artist, t.DurationMS, t.SHA256, t.Size, t.Ext, t.MIME, t.License, t.Source, t.AIGenerated, t.Tags, t.Enabled, t.UploadedBy)
	out, err := scanTrack(row)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return Track{}, ErrDuplicate
	}
	return out, err
}

func (s *PGStore) UpdateMeta(ctx context.Context, t Track) (Track, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE media_tracks
		SET kind=$2, title=$3, artist=$4, license=$5, source=$6, ai_generated=$7, tags=$8, enabled=$9
		WHERE id=$1
		RETURNING `+trackCols,
		t.ID, t.Kind, t.Title, t.Artist, t.License, t.Source, t.AIGenerated, t.Tags, t.Enabled)
	out, err := scanTrack(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Track{}, ErrNotFound
	}
	return out, err
}

func (s *PGStore) Delete(ctx context.Context, id int64) (Track, error) {
	row := s.pool.QueryRow(ctx, `DELETE FROM media_tracks WHERE id=$1 RETURNING `+trackCols, id)
	t, err := scanTrack(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Track{}, ErrNotFound
	}
	return t, err
}

func (s *PGStore) ByID(ctx context.Context, id int64) (Track, error) {
	t, err := scanTrack(s.pool.QueryRow(ctx, `SELECT `+trackCols+` FROM media_tracks WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Track{}, ErrNotFound
	}
	return t, err
}

func (s *PGStore) BySHA(ctx context.Context, sha string) (Track, error) {
	t, err := scanTrack(s.pool.QueryRow(ctx, `SELECT `+trackCols+` FROM media_tracks WHERE sha256=$1`, sha))
	if errors.Is(err, pgx.ErrNoRows) {
		return Track{}, ErrNotFound
	}
	return t, err
}

func (s *PGStore) List(ctx context.Context) ([]Track, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+trackCols+` FROM media_tracks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
