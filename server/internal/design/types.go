// Package design — библиотека дизайна (R-53): каталог встроенных элементов (иконки, фоны, 3D-модели, паспорта кораблей),
// замены, загруженные из админки (проверка файлов, версии, публикация), подписанный манифест ресурсов для клиента
// и промпты для создания элементов (R-54). Ключ элемента не привязан к пути файла: «model:FTR-001:a» — модель облика «a» корабля FTR-001.
package design

import (
	"embed"
	"encoding/json"
	"sort"
	"time"
)

//go:embed builtin.json
var builtinFS embed.FS

// Категории элементов.
const (
	CatIcon       = "icon"
	CatBackground = "background"
	CatModel      = "model"
	CatShipData   = "ship_data"
)

// Статусы замен.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

// Element — встроенный элемент дизайна (описание из каталога, собранного из файлов клиента).
type Element struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Group       string   `json:"group"`
	Tags        []string `json:"tags"`
	Builtin     string   `json:"builtin"` // путь встроенного файла в клиенте
	Bytes       int64    `json:"bytes"`
	Dims        []int    `json:"dims,omitempty"`
	UsedBy      []string `json:"used_by,omitempty"`
	PreviewSVG  string   `json:"preview_svg,omitempty"`
	Ship        string   `json:"ship,omitempty"`
	Skin        string   `json:"skin,omitempty"`
	Class       string   `json:"class,omitempty"`
	Placeholder bool     `json:"placeholder,omitempty"`
	ModelLength float64  `json:"model_length,omitempty"`
	NoseAxis    string   `json:"nose_axis,omitempty"`
	Current     string   `json:"current,omitempty"` // текущий паспорт корабля (JSON), только для ship_data
	Note        string   `json:"note,omitempty"`
}

// Asset — замена элемента, загруженная из админки.
type Asset struct {
	ID          int64          `json:"id"`
	ElementID   string         `json:"element_id"`
	Version     int            `json:"version"`
	FileName    string         `json:"file_name"`
	Mime        string         `json:"mime"`
	SHA256      string         `json:"sha256"`
	Size        int64          `json:"size"`
	Status      string         `json:"status"`
	Report      Report         `json:"report"`
	Notes       string         `json:"notes"`
	UploadedBy  string         `json:"uploaded_by"`
	CreatedAt   time.Time      `json:"created_at"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	Data        []byte         `json:"-"`
	Extra       map[string]any `json:"-"`
}

// Ext возвращает расширение файла замены без точки (по типу содержимого).
func (a Asset) Ext() string { return extForMime(a.Mime) }

// Report — результат проверки загруженного файла.
type Report struct {
	OK       bool           `json:"ok"`
	Errors   []string       `json:"errors"`
	Warnings []string       `json:"warnings"`
	Info     map[string]any `json:"info"`
}

func newReport() Report {
	return Report{Errors: []string{}, Warnings: []string{}, Info: map[string]any{}}
}

func (r *Report) errf(msg string)  { r.Errors = append(r.Errors, msg) }
func (r *Report) warnf(msg string) { r.Warnings = append(r.Warnings, msg) }
func (r *Report) done() Report {
	r.OK = len(r.Errors) == 0
	return *r
}

// Catalog — встроенный каталог.
type Catalog struct {
	byID  map[string]Element
	order []Element
}

// LoadCatalog читает встроенный каталог.
func LoadCatalog() (*Catalog, error) {
	raw, err := builtinFS.ReadFile("builtin.json")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Elements []Element `json:"elements"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	c := &Catalog{byID: map[string]Element{}}
	for _, e := range doc.Elements {
		c.byID[e.ID] = e
		c.order = append(c.order, e)
	}
	sort.SliceStable(c.order, func(i, j int) bool {
		if c.order[i].Category != c.order[j].Category {
			return c.order[i].Category < c.order[j].Category
		}
		return c.order[i].ID < c.order[j].ID
	})
	return c, nil
}

// Get возвращает элемент по ключу.
func (c *Catalog) Get(id string) (Element, bool) {
	e, ok := c.byID[id]
	return e, ok
}

// All возвращает все элементы в стабильном порядке.
func (c *Catalog) All() []Element { return append([]Element(nil), c.order...) }
