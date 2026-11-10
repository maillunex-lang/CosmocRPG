package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound  = errors.New("трек не найден")
	ErrDuplicate = errors.New("этот файл уже есть в библиотеке")
	ErrBad       = errors.New("некорректные данные трека")
)

// InvalidError — ошибка, которую можно показать администратору.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

var fileNameRe = regexp.MustCompile(`^[0-9a-f]{64}\.(mp3|ogg|opus)$`)

// Meta — поля, которые задаёт администратор. Длительность и сумма считает сервер.
type Meta struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	License     string `json:"license"`
	Source      string `json:"source"`
	AIGenerated bool   `json:"ai_generated"`
	Tags        string `json:"tags"`
	Enabled     bool   `json:"enabled"`
}

// Service — библиотека: проверка файла, диск и база.
type Service struct {
	store Store
	dir   string
}

func NewService(store Store, dir string) (*Service, error) {
	if dir == "" {
		return nil, errors.New("каталог библиотеки не задан")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("каталог библиотеки: %w", err)
	}
	return &Service{store: store, dir: dir}, nil
}

func (s *Service) Upload(ctx context.Context, name string, data []byte, m Meta, user string) (Track, error) {
	if err := normalize(&m, true); err != nil {
		return Track{}, err
	}
	ext, mime, sum, dur, err := Probe(name, data)
	if err != nil {
		return Track{}, &InvalidError{Msg: err.Error()}
	}
	if _, err := s.store.BySHA(ctx, sum); err == nil {
		return Track{}, ErrDuplicate
	} else if !errors.Is(err, ErrNotFound) {
		return Track{}, err
	}
	fname := sum + "." + ext
	if err := writeFileAtomic(filepath.Join(s.dir, fname), data); err != nil {
		return Track{}, err
	}
	t, err := s.store.Insert(ctx, Track{
		Kind: m.Kind, Title: m.Title, Artist: m.Artist, DurationMS: int(dur / time.Millisecond),
		SHA256: sum, Size: int64(len(data)), Ext: ext, MIME: mime, License: m.License, Source: m.Source,
		AIGenerated: m.AIGenerated, Tags: m.Tags, Enabled: m.Enabled, UploadedBy: user,
	})
	if err != nil {
		_ = os.Remove(filepath.Join(s.dir, fname))
		return Track{}, err
	}
	t.URL = PublicPath(t)
	return t, nil
}

func (s *Service) Update(ctx context.Context, id int64, m Meta) (Track, error) {
	cur, err := s.store.ByID(ctx, id)
	if err != nil {
		return Track{}, err
	}
	if err := normalize(&m, false); err != nil {
		return Track{}, err
	}
	cur.Kind, cur.Title, cur.Artist = m.Kind, m.Title, m.Artist
	cur.License, cur.Source, cur.AIGenerated, cur.Tags, cur.Enabled = m.License, m.Source, m.AIGenerated, m.Tags, m.Enabled
	out, err := s.store.UpdateMeta(ctx, cur)
	if err != nil {
		return Track{}, err
	}
	out.URL = PublicPath(out)
	return out, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	t, err := s.store.Delete(ctx, id)
	if err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(s.dir, t.SHA256+"."+t.Ext))
	return nil
}

func (s *Service) Get(ctx context.Context, id int64) (Track, error) {
	t, err := s.store.ByID(ctx, id)
	if err != nil {
		return Track{}, err
	}
	t.URL = PublicPath(t)
	return t, nil
}

func (s *Service) List(ctx context.Context) ([]Track, error) {
	items, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Track{}
	}
	for i := range items {
		items[i].URL = PublicPath(items[i])
	}
	return items, nil
}

// PublicLists — только включённые треки, без имени администратора.
func (s *Service) PublicList(ctx context.Context) ([]Track, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Track, 0, len(items))
	for _, t := range items {
		if !t.Enabled {
			continue
		}
		t.UploadedBy = ""
		out = append(out, t)
	}
	return out, nil
}

func (s *Service) Read(shaExt string) ([]byte, string, error) {
	if !fileNameRe.MatchString(shaExt) {
		return nil, "", ErrNotFound
	}
	b, err := os.ReadFile(filepath.Join(s.dir, shaExt))
	if err != nil {
		return nil, "", ErrNotFound
	}
	ext := strings.TrimPrefix(filepath.Ext(shaExt), ".")
	mime := "audio/ogg"
	if ext == "mp3" {
		mime = "audio/mpeg"
	}
	return b, mime, nil
}

func PublicPath(t Track) string {
	return "/media/files/" + t.SHA256 + "." + t.Ext
}

func normalize(m *Meta, upload bool) error {
	m.Title = strings.TrimSpace(m.Title)
	m.Artist = strings.TrimSpace(m.Artist)
	m.License = strings.TrimSpace(m.License)
	m.Source = strings.TrimSpace(m.Source)
	m.Tags = strings.TrimSpace(m.Tags)
	m.Kind = strings.TrimSpace(m.Kind)
	if m.Kind == "" && upload {
		m.Kind = "music"
	}
	switch m.Kind {
	case "music", "ad", "news", "sfx":
	default:
		return &InvalidError{Msg: "вид трека: music, ad, news или sfx"}
	}
	if m.Title == "" || utf8.RuneCountInString(m.Title) > 120 {
		return &InvalidError{Msg: "нужно название до 120 символов"}
	}
	if utf8.RuneCountInString(m.Artist) > 120 || utf8.RuneCountInString(m.License) > 200 || utf8.RuneCountInString(m.Source) > 120 || utf8.RuneCountInString(m.Tags) > 200 {
		return &InvalidError{Msg: "слишком длинное поле описания"}
	}
	if m.Source == "" {
		m.Source = "Google Gemini"
	}
	return nil
}

func writeFileAtomic(dst string, data []byte) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
