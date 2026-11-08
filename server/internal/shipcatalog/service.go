package shipcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Service — каталог кораблей: версии, правки, патчи, откат. onChange вызывается после каждой смены активной версии
// (сервер применяет каталог к миру на лету).
type Service struct {
	store    Store
	onChange func(world.Catalog)
	now      func() time.Time

	mu       sync.Mutex
	previews map[string]*Preview
}

// NewService создаёт сервис.
func NewService(store Store, onChange func(world.Catalog)) *Service {
	if onChange == nil {
		onChange = func(world.Catalog) {}
	}
	return &Service{store: store, onChange: onChange, now: time.Now, previews: map[string]*Preview{}}
}

// Init создаёт начальную версию из встроенного каталога (если версий нет) и применяет активный каталог.
func (s *Service) Init(ctx context.Context) error {
	v, ok, err := s.store.Active(ctx)
	if err != nil {
		return err
	}
	if !ok {
		zero := int64(0)
		if _, err := s.store.Save(ctx, NewVersion{Admin: "система", Note: "Начальный каталог из встроенных данных", Source: "seed", Doc: world.DefaultDoc(), ExpectActive: &zero}); err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
		if v, ok, err = s.store.Active(ctx); err != nil || !ok {
			return fmt.Errorf("не удалось создать начальный каталог: %v", err)
		}
	}
	cat, err := world.CatalogFromDoc(v.Doc)
	if err != nil {
		return fmt.Errorf("активный каталог кораблей повреждён: %w", err)
	}
	s.onChange(cat)
	return nil
}

// Active возвращает активный документ каталога и номер версии.
func (s *Service) Active(ctx context.Context) (world.Doc, int64, error) {
	v, ok, err := s.store.Active(ctx)
	if err != nil {
		return world.Doc{}, 0, err
	}
	if !ok {
		return world.DefaultDoc(), 0, nil
	}
	return v.Doc, v.ID, nil
}

func (s *Service) commit(ctx context.Context, admin, note, source string, doc world.Doc, base int64) (int64, error) {
	cat, err := world.CatalogFromDoc(doc)
	if err != nil {
		return 0, err
	}
	id, err := s.store.Save(ctx, NewVersion{Admin: admin, Note: note, Source: source, Doc: doc, ExpectActive: &base})
	if err != nil {
		return 0, err
	}
	s.onChange(cat)
	return id, nil
}

// UpdateShip заменяет паспорт одного корабля (новый корабль добавляется) после проверки диапазонов и согласованности.
func (s *Service) UpdateShip(ctx context.Context, admin string, ship world.ShipSpec) (int64, error) {
	if errs := ValidateShip(ship); len(errs) > 0 {
		return 0, errors.New(strings.Join(errs, "; "))
	}
	doc, ver, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	doc = replaceShip(doc, ship)
	return s.commit(ctx, admin, "Правка корабля "+ship.ID, "edit", doc, ver)
}

func replaceShip(doc world.Doc, ship world.ShipSpec) world.Doc {
	out := world.Doc{Starters: doc.Starters}
	found := false
	for _, sp := range doc.Ships {
		if sp.ID == ship.ID {
			out.Ships = append(out.Ships, ship)
			found = true
		} else {
			out.Ships = append(out.Ships, sp)
		}
	}
	if !found {
		out.Ships = append(out.Ships, ship)
	}
	sort.Slice(out.Ships, func(i, j int) bool { return out.Ships[i].ID < out.Ships[j].ID })
	return out
}

// Rollback возвращает каталог к содержимому прежней версии (создаётся новая версия, история сохраняется).
func (s *Service) Rollback(ctx context.Context, admin string, id int64) (int64, error) {
	v, err := s.store.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	_, cur, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	if cur == id {
		return 0, errors.New("эта версия уже активна")
	}
	return s.commit(ctx, admin, fmt.Sprintf("Откат к версии %d", id), "rollback", v.Doc, cur)
}

// Versions возвращает историю версий.
func (s *Service) Versions(ctx context.Context) ([]VersionInfo, error) { return s.store.List(ctx, 50) }

// ---- предпросмотр патча ----

// FieldChange — изменение одного параметра.
type FieldChange struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// ShipChange — изменения корабля.
type ShipChange struct {
	ID     string        `json:"id"`
	Kind   string        `json:"kind"` // new | changed | unchanged
	Fields []FieldChange `json:"fields"`
}

// Preview — результат проверки патча до применения.
type Preview struct {
	ID          string       `json:"id"`
	Filename    string       `json:"filename"`
	SHA256      string       `json:"sha256"`
	Status      string       `json:"status"` // ready | rejected | published | discarded
	PackID      string       `json:"pack_id"`
	Integrity   string       `json:"integrity"`
	Errors      []Issue      `json:"errors"`
	Warnings    []Issue      `json:"warnings"`
	Changes     []ShipChange `json:"changes"`
	BaseVersion int64        `json:"base_version"`
	CreatedAt   time.Time    `json:"created_at"`

	doc world.Doc
}

// Issue — замечание проверки.
type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (s *Service) putPreview(p *Preview) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, old := range s.previews { // предпросмотры живут час
		if now.Sub(old.CreatedAt) > time.Hour || len(s.previews) > 50 {
			delete(s.previews, id)
		}
	}
	s.previews[p.ID] = p
}

// PublishPreview применяет проверенный патч атомарно как новую версию каталога.
func (s *Service) PublishPreview(ctx context.Context, admin, id string) (int64, error) {
	s.mu.Lock()
	p := s.previews[id]
	s.mu.Unlock()
	if p == nil {
		return 0, ErrNotFound
	}
	if p.Status != "ready" {
		return 0, fmt.Errorf("патч нельзя опубликовать: статус «%s»", p.Status)
	}
	ver, err := s.commit(ctx, admin, fmt.Sprintf("ZIP-патч «%s» (%s)", p.Filename, summary(p.Changes)), "patch", p.doc, p.BaseVersion)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	p.Status = "published"
	s.mu.Unlock()
	return ver, nil
}

// DiscardPreview отклоняет патч.
func (s *Service) DiscardPreview(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.previews[id]
	if p == nil {
		return ErrNotFound
	}
	if p.Status == "published" {
		return errors.New("патч уже опубликован")
	}
	p.Status = "discarded"
	return nil
}

func summary(ch []ShipChange) string {
	n, c := 0, 0
	for _, x := range ch {
		switch x.Kind {
		case "new":
			n++
		case "changed":
			c++
		}
	}
	return fmt.Sprintf("новых %d, изменено %d", n, c)
}

func flatten(prefix string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flatten(p, x, out)
		}
	default:
		b, _ := json.Marshal(t)
		out[prefix] = string(b)
	}
}

func shipMap(s world.ShipSpec) map[string]string {
	b, _ := json.Marshal(s)
	var generic any
	_ = json.Unmarshal(b, &generic)
	out := map[string]string{}
	flatten("", generic, out)
	return out
}

// diffShips сравнивает паспорта по параметрам.
func diffShips(old, new world.ShipSpec, existed bool) ShipChange {
	ch := ShipChange{ID: new.ID, Kind: "unchanged", Fields: []FieldChange{}}
	if !existed {
		ch.Kind = "new"
		return ch
	}
	a, b := shipMap(old), shipMap(new)
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		if a[k] != b[k] {
			ch.Fields = append(ch.Fields, FieldChange{Path: k, Old: a[k], New: b[k]})
		}
	}
	if len(ch.Fields) > 0 {
		ch.Kind = "changed"
	}
	return ch
}
