package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ManifestSchema — идентификатор формата манифеста ресурсов.
const ManifestSchema = "Cosmoc_Assets_v1"

// Ошибки сервиса.
var (
	ErrNoElement  = errors.New("такого элемента нет в каталоге")
	ErrNoSigner   = errors.New("ключ подписи ресурсов не подключён: публикация недоступна (загрузка черновиков работает)")
	ErrDuplicate  = errors.New("такой файл уже загружен для этого элемента")
	ErrBadRequest = errors.New("некорректный запрос")
)

// InvalidError — файл не прошёл проверку; Report содержит причины.
type InvalidError struct{ Report Report }

func (e *InvalidError) Error() string {
	if len(e.Report.Errors) > 0 {
		return "файл не прошёл проверку: " + e.Report.Errors[0]
	}
	return "файл не прошёл проверку"
}

// Service — библиотека дизайна.
type Service struct {
	store   Store
	catalog *Catalog
	signer  *Signer
}

// NewService создаёт сервис; signer может быть nil (публикация выключена).
func NewService(store Store, signer *Signer) (*Service, error) {
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	return &Service{store: store, catalog: cat, signer: signer}, nil
}

// CanPublish сообщает, подключён ли ключ подписи.
func (s *Service) CanPublish() bool { return s.signer != nil }

// SignerPublicPEM возвращает публичный ключ подписи ресурсов ("" без ключа).
func (s *Service) SignerPublicPEM() string {
	if s.signer == nil {
		return ""
	}
	return s.signer.PublicPEM()
}

// ElementView — элемент каталога с состоянием замены.
type ElementView struct {
	Element
	State     string `json:"state"` // builtin | draft | replaced
	Published *Asset `json:"published,omitempty"`
	Drafts    int    `json:"drafts"`
	Versions  int    `json:"versions"`
}

// Filter — условия отбора элементов.
type Filter struct {
	Category, Group, Tag, State, Q string
}

// Facets — доступные значения фильтров с количеством.
type Facets struct {
	Categories map[string]int `json:"categories"`
	Groups     map[string]int `json:"groups"`
	States     map[string]int `json:"states"`
	Tags       map[string]int `json:"tags"`
}

func (s *Service) views(ctx context.Context) ([]ElementView, error) {
	all, err := s.store.List(ctx, "")
	if err != nil {
		return nil, err
	}
	by := map[string][]Asset{}
	for _, a := range all {
		by[a.ElementID] = append(by[a.ElementID], a)
	}
	var out []ElementView
	for _, e := range s.catalog.All() {
		v := ElementView{Element: e, State: "builtin"}
		for _, a := range by[e.ID] {
			v.Versions++
			switch a.Status {
			case StatusPublished:
				c := a
				v.Published = &c
				v.State = "replaced"
			case StatusDraft:
				v.Drafts++
			}
		}
		if v.State == "builtin" && v.Drafts > 0 {
			v.State = "draft"
		}
		out = append(out, v)
	}
	return out, nil
}

func match(v ElementView, f Filter) bool {
	if f.Category != "" && v.Category != f.Category || f.Group != "" && v.Group != f.Group || f.State != "" && v.State != f.State {
		return false
	}
	if f.Tag != "" {
		ok := false
		for _, t := range v.Tags {
			ok = ok || strings.EqualFold(t, f.Tag)
		}
		if !ok {
			return false
		}
	}
	if q := strings.ToLower(strings.TrimSpace(f.Q)); q != "" {
		hay := strings.ToLower(v.ID + " " + v.Name + " " + v.Group + " " + v.Ship + " " + strings.Join(v.Tags, " ") + " " + strings.Join(v.UsedBy, " "))
		for _, w := range strings.Fields(q) {
			if !strings.Contains(hay, w) {
				return false
			}
		}
	}
	return true
}

// Elements возвращает элементы по фильтру и значения фильтров (счётчики считаются по всему каталогу, кроме самого фильтруемого поля).
func (s *Service) Elements(ctx context.Context, f Filter) ([]ElementView, Facets, error) {
	all, err := s.views(ctx)
	if err != nil {
		return nil, Facets{}, err
	}
	fc := Facets{Categories: map[string]int{}, Groups: map[string]int{}, States: map[string]int{}, Tags: map[string]int{}}
	var out []ElementView
	for _, v := range all {
		if match(v, f) {
			out = append(out, v)
		}
		g := f
		g.Category = ""
		if match(v, g) {
			fc.Categories[v.Category]++
		}
		g = f
		g.Group = ""
		if match(v, g) {
			fc.Groups[v.Group]++
		}
		g = f
		g.State = ""
		if match(v, g) {
			fc.States[v.State]++
		}
		g = f
		g.Tag = ""
		if match(v, g) {
			for _, t := range v.Tags {
				if t != "" {
					fc.Tags[t]++
				}
			}
		}
	}
	return out, fc, nil
}

// Detail возвращает элемент и историю его замен.
func (s *Service) Detail(ctx context.Context, id string) (ElementView, []Asset, error) {
	if _, ok := s.catalog.Get(id); !ok {
		return ElementView{}, nil, ErrNoElement
	}
	all, err := s.views(ctx)
	if err != nil {
		return ElementView{}, nil, err
	}
	hist, err := s.store.List(ctx, id)
	if err != nil {
		return ElementView{}, nil, err
	}
	for _, v := range all {
		if v.ID == id {
			return v, hist, nil
		}
	}
	return ElementView{}, nil, ErrNoElement
}

// Upload проверяет файл и сохраняет его черновиком замены элемента.
func (s *Service) Upload(ctx context.Context, elementID, fileName string, data []byte, notes, admin string) (Asset, error) {
	el, ok := s.catalog.Get(elementID)
	if !ok {
		return Asset{}, ErrNoElement
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" || len(fileName) > 120 || strings.ContainsAny(fileName, "/\\\x00") {
		return Asset{}, fmt.Errorf("%w: имя файла", ErrBadRequest)
	}
	if len(notes) > 500 {
		notes = notes[:500]
	}
	mime, rep := Validate(el, fileName, data)
	if !rep.OK {
		return Asset{}, &InvalidError{Report: rep}
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	hist, err := s.store.List(ctx, elementID)
	if err != nil {
		return Asset{}, err
	}
	for _, a := range hist {
		if a.SHA256 == sha {
			return Asset{}, fmt.Errorf("%w (версия %d)", ErrDuplicate, a.Version)
		}
	}
	return s.store.Add(ctx, Asset{ElementID: elementID, FileName: fileName, Mime: mime, SHA256: sha, Size: int64(len(data)), Data: data, Report: rep, Notes: notes, UploadedBy: admin})
}

// Publish публикует замену; манифест получает новую версию.
func (s *Service) Publish(ctx context.Context, id int64) (int64, error) {
	if s.signer == nil {
		return 0, ErrNoSigner
	}
	return s.store.Publish(ctx, id)
}

// Unpublish снимает замену с публикации: в игре снова встроенный элемент.
func (s *Service) Unpublish(ctx context.Context, elementID string) (int64, error) {
	if s.signer == nil {
		return 0, ErrNoSigner
	}
	return s.store.Unpublish(ctx, elementID)
}

// Delete удаляет черновик или архивную замену.
func (s *Service) Delete(ctx context.Context, id int64) error { return s.store.Delete(ctx, id) }

// File возвращает замену вместе с содержимым (для предпросмотра и скачивания в админке).
func (s *Service) File(ctx context.Context, id int64) (Asset, error) { return s.store.Get(ctx, id) }

// ManifestEntry — запись манифеста.
type ManifestEntry struct {
	Element  string `json:"element"`
	Category string `json:"category"`
	File     string `json:"file"` // sha256.расширение: клиент сам строит адрес, адресов в манифесте нет
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Mime     string `json:"mime"`
	Version  int    `json:"version"`
}

// Manifest собирает манифест опубликованных замен и подписывает его.
func (s *Service) Manifest(ctx context.Context) (body []byte, sig string, version int64, err error) {
	if s.signer == nil {
		return nil, "", 0, ErrNoSigner
	}
	pub, v, err := s.store.Published(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	entries := make([]ManifestEntry, 0, len(pub))
	for _, a := range pub {
		el, ok := s.catalog.Get(a.ElementID)
		if !ok {
			continue // элемент убрали из каталога: клиенту он не нужен
		}
		entries = append(entries, ManifestEntry{Element: a.ElementID, Category: el.Category, File: a.SHA256 + "." + a.Ext(), SHA256: a.SHA256, Size: a.Size, Mime: a.Mime, Version: a.Version})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Element < entries[j].Element })
	body, err = json.Marshal(map[string]any{"schema": ManifestSchema, "version": v, "assets": entries})
	if err != nil {
		return nil, "", 0, err
	}
	sig, err = s.signer.Sign(body)
	if err != nil {
		return nil, "", 0, err
	}
	return body, sig, v, nil
}

// Status — состояние библиотеки для админки.
type Status struct {
	CanPublish      bool   `json:"can_publish"`
	ManifestVersion int64  `json:"manifest_version"`
	Published       int    `json:"published"`
	Elements        int    `json:"elements"`
	PublicKey       string `json:"public_key,omitempty"`
}

// Status возвращает сводку.
func (s *Service) Status(ctx context.Context) (Status, error) {
	pub, v, err := s.store.Published(ctx)
	if err != nil {
		return Status{}, err
	}
	return Status{CanPublish: s.signer != nil, ManifestVersion: v, Published: len(pub), Elements: len(s.catalog.All()), PublicKey: s.SignerPublicPEM()}, nil
}
