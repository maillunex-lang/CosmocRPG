package aistudio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Service — прикладная логика Studio: версии, экспорт и импорт пакетов, публикация, откат, тесты.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService создаёт сервис.
func NewService(store Store) *Service { return &Service{store: store, now: time.Now} }

// Active возвращает активную конфигурацию и номер версии (0 — версий ещё нет, возвращаются базовые разделы).
func (s *Service) Active(ctx context.Context) (Config, int64, error) {
	v, ok, err := s.store.Active(ctx)
	if err != nil {
		return Config{}, 0, err
	}
	if !ok {
		c := NewConfig()
		EnsureDefaults(&c)
		return c, 0, nil
	}
	EnsureDefaults(&v.Config)
	return v.Config, v.ID, nil
}

// Generate добавляет недостающих капитанов 1..count; существующие не изменяются.
func (s *Service) Generate(ctx context.Context, admin string, seed int64, count int) (GenerateResult, int64, error) {
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return GenerateResult{}, 0, err
	}
	res := Generate(&cfg, GenerateOptions{Seed: seed, Count: count})
	if len(res.Added) == 0 && ver != 0 {
		return res, ver, nil
	}
	if bad := firstErrors(Validate(cfg), 3); len(bad) > 0 {
		return res, 0, fmt.Errorf("сгенерированная конфигурация не проходит проверку: %s", strings.Join(bad, "; "))
	}
	id, err := s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "generated", Config: cfg, ExpectActive: &ver,
		Note: fmt.Sprintf("Генерация капитанов: добавлено %d (seed %d)", len(res.Added), seed)})
	return res, id, err
}

func firstErrors(issues []Issue, n int) []string {
	var out []string
	for _, is := range issues {
		if is.Level == "error" && len(out) < n {
			out = append(out, is.Path+": "+is.Message)
		}
	}
	return out
}

func errorSet(issues []Issue) map[string]bool {
	m := map[string]bool{}
	for _, is := range issues {
		if is.Level == "error" {
			m[is.Path+"|"+is.Message] = true
		}
	}
	return m
}

// ---- экспорт ----

// ExportRequest — запрос на выгрузку пакета.
type ExportRequest struct {
	Kind  string   `json:"kind"`  // template | examples | batch | selected | all
	Size  int      `json:"size"`  // размер партии: 10, 25, 50
	Index int      `json:"index"` // номер партии, с 1
	IDs   []string `json:"ids"`
}

// ExportInfo — описание партий для интерфейса.
type ExportInfo struct {
	Size, Total, Batches int
}

// Batches возвращает число партий при данном размере.
func Batches(total, size int) int {
	if size <= 0 {
		return 0
	}
	return (total + size - 1) / size
}

// Export собирает ZIP для обучения. Возвращает содержимое, имя файла и метку.
func (s *Service) Export(ctx context.Context, admin string, req ExportRequest) ([]byte, string, error) {
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return nil, "", err
	}
	all := make([]string, 0, len(cfg.Captains))
	for id := range cfg.Captains {
		all = append(all, id)
	}
	sort.Strings(all)
	now := s.now().UTC()
	stamp := now.Format("20060102-150405")
	base := int(ver)
	opts := PackOptions{CreatedBy: "studio", Base: &base}
	var ids []string
	var label, file string
	switch req.Kind {
	case "template":
		label, file = "пустой шаблон", "ai-pack-template-empty-"+stamp+".zip"
		opts.Kind = "partial"
		opts.Notes = "Пустой шаблон: правила и словари без капитанов."
		opts.Extra = map[string][]byte{"captains/README.md": []byte(captainsReadme), "captains/EMPTY_CAPTAIN.txt": []byte(emptyCaptain())}
	case "examples":
		label, file = "шаблон с примерами", "ai-pack-template-examples-"+stamp+".zip"
		opts.Kind = "partial"
		opts.Notes = "Шаблон с примерами: первые три капитана как образец формата."
		if len(all) == 0 {
			tmp := cfg.Clone()
			Generate(&tmp, GenerateOptions{Seed: 20261108, Count: 3})
			cfg = tmp
			all = []string{"cpt-0001", "cpt-0002", "cpt-0003"}
		}
		ids = all[:min(3, len(all))]
		opts.Extra = map[string][]byte{"captains/README.md": []byte(captainsReadme)}
	case "batch":
		if req.Size < 1 || req.Size > 300 {
			return nil, "", errors.New("размер партии: от 1 до 300")
		}
		total := len(all)
		nb := Batches(total, req.Size)
		if req.Index < 1 || req.Index > nb {
			return nil, "", fmt.Errorf("номер партии: от 1 до %d", nb)
		}
		lo := (req.Index - 1) * req.Size
		ids = all[lo:min(total, lo+req.Size)]
		label = fmt.Sprintf("партия %d из %d по %d", req.Index, nb, req.Size)
		file = fmt.Sprintf("ai-pack-batch-%d-%02d-of-%02d-%s.zip", req.Size, req.Index, nb, stamp)
		opts.Kind = "partial"
		opts.Batch = map[string]any{"size": req.Size, "index": req.Index, "total": nb}
		opts.Notes = BatchNote(req.Size, req.Index, nb, ids)
	case "selected":
		for _, id := range req.IDs {
			if _, ok := cfg.Captains[id]; !ok {
				return nil, "", fmt.Errorf("капитана %s нет в реестре", id)
			}
		}
		if len(req.IDs) == 0 || len(req.IDs) > 300 {
			return nil, "", errors.New("выберите от 1 до 300 капитанов")
		}
		ids = append([]string{}, req.IDs...)
		sort.Strings(ids)
		label, file = fmt.Sprintf("выбранные (%d)", len(ids)), "ai-pack-selected-"+stamp+".zip"
		opts.Kind = "partial"
		opts.Notes = BatchNote(len(ids), 1, 1, ids)
	case "all":
		ids = all
		label, file = fmt.Sprintf("полный пакет (%d)", len(ids)), "ai-pack-full-"+stamp+".zip"
		opts.Kind = "full"
		opts.Notes = "Полный пакет: все капитаны реестра."
	default:
		return nil, "", errors.New("неизвестный вид экспорта")
	}
	if req.Kind != "template" && req.Kind != "examples" && len(ids) == 0 {
		return nil, "", errors.New("в реестре нет капитанов: сначала сгенерируйте их")
	}
	opts.PackID = "studio-" + stamp
	opts.Readme = MasterPrompt(opts.Notes)
	data, err := BuildPack(opts, cfg, ids, now.Format(time.RFC3339))
	if err != nil {
		return nil, "", err
	}
	_, _ = s.store.SaveSession(ctx, Session{Admin: admin, Label: label, BatchSize: len(ids), CaptainIDs: ids, PackID: opts.PackID, SHA256: sha(data)})
	return data, file, nil
}

const captainsReadme = "# Каталог captains/\n\nКаждый капитан — отдельный файл `cpt-NNNN.json` по схеме `schemas/captain.schema.json`.\nПустой шаблон профиля смотрите в `EMPTY_CAPTAIN.txt` (если он есть), примеры — в профилях рядом.\nИдентификатор в имени файла и в поле `id` обязан совпадать.\n"

func emptyCaptain() string {
	tmp := NewConfig()
	Generate(&tmp, GenerateOptions{Seed: 1, Count: 1})
	cp := tmp.Captains["cpt-0001"]
	cp.Name, cp.CallSign = "ЗАПОЛНИТЬ имя", ""
	cp.Bio = "ЗАПОЛНИТЬ: биография на 400-900 знаков."
	b, _ := json.MarshalIndent(cp, "", "  ")
	return "Скелет профиля капитана (скопируйте в captains/cpt-NNNN.json и заполните; значения-образцы заменить):\n\n" + string(b) + "\n"
}

// ---- импорт, предпросмотр, публикация ----

// gateKinds — проверки, без которых публикация запрещена.
var gateKinds = []string{"structure", "facts", "context", "regression"}

func gatePassed(res []TestResult) bool {
	for _, r := range res {
		if r.Gate && r.Passed != r.Total {
			return false
		}
	}
	return true
}

// Preview разбирает и проверяет пакет, строит план изменений и сохраняет результат. Конфигурацию не меняет.
func (s *Service) Preview(ctx context.Context, admin, filename string, data []byte, opt MergeOptions) (ImportRec, error) {
	rec := ImportRec{Admin: admin, Filename: filename, SHA256: sha(data), Status: "rejected", Gate: []TestResult{}}
	parsed, rep := ParsePack(data)
	rec.Report = rep
	if parsed != nil {
		rec.PackID, rec.Kind = parsed.Manifest.PackID, parsed.Manifest.Kind
	}
	if !rep.OK || parsed == nil {
		id, err := s.store.SaveImport(ctx, rec)
		rec.ID = id
		return rec, err
	}
	base, ver, err := s.Active(ctx)
	if err != nil {
		return rec, err
	}
	rec.BaseVersion = &ver
	merged, plan := Merge(base, parsed, opt)
	rec.Plan = plan
	before := errorSet(Validate(base))
	for _, is := range Validate(merged) {
		if is.Level == "error" && before[is.Path+"|"+is.Message] {
			continue // ошибка уже была в активной версии, пакет её не вносит
		}
		if is.Level == "error" {
			rec.Report.Errors = append(rec.Report.Errors, is)
		} else {
			rec.Report.Warnings = append(rec.Report.Warnings, is)
		}
	}
	for _, ch := range plan.Captains {
		if ch.Kind == "conflict" {
			rec.Report.Warnings = append(rec.Report.Warnings, Issue{"warning", "manual_conflict", "captains/" + ch.ID + ".json", ch.Reason})
		}
	}
	rec.Report.Stats["added"], rec.Report.Stats["changed"], rec.Report.Stats["unchanged"], rec.Report.Stats["conflicts"] = plan.Added, plan.Changed, plan.Unchanged, plan.Conflicts
	rec.Report.OK = len(rec.Report.Errors) == 0
	if rec.Report.OK {
		rec.Gate = RunSuite(&merged, gateKinds, 25)
		rec.GateOK = gatePassed(rec.Gate)
		if rec.GateOK {
			rec.Status = "validated"
		} else {
			for _, r := range rec.Gate {
				if r.Gate && r.Passed != r.Total {
					for _, f := range r.Failures {
						rec.Report.Errors = append(rec.Report.Errors, Issue{"error", "gate_" + r.Kind, "", r.Title + ": " + f})
					}
				}
			}
			rec.Report.OK = false
		}
	}
	pj, err := json.Marshal(parsed)
	if err != nil {
		return rec, err
	}
	rec.Parsed = pj
	id, err := s.store.SaveImport(ctx, rec)
	rec.ID = id
	return rec, err
}

// PublishOptions — подтверждение публикации.
type PublishOptions struct {
	AcceptWarnings  bool `json:"accept_warnings"`
	OverwriteManual bool `json:"overwrite_manual"`
}

// Publish применяет проверенный пакет атомарно как новую версию. Живую память, экономику и прогресс не трогает.
func (s *Service) Publish(ctx context.Context, admin string, importID int64, opt PublishOptions) (int64, error) {
	rec, err := s.store.GetImport(ctx, importID)
	if err != nil {
		return 0, err
	}
	if rec.Status != "validated" {
		return 0, fmt.Errorf("пакет нельзя опубликовать: статус «%s»", rec.Status)
	}
	if len(rec.Report.Warnings) > 0 && !opt.AcceptWarnings {
		return 0, errors.New("в пакете есть предупреждения: подтвердите, что приняли их")
	}
	var parsed Parsed
	if err := json.Unmarshal(rec.Parsed, &parsed); err != nil {
		return 0, fmt.Errorf("содержимое пакета повреждено: %w", err)
	}
	base, ver, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	if rec.BaseVersion == nil || *rec.BaseVersion != ver {
		return 0, ErrConflict
	}
	merged, plan := Merge(base, &parsed, MergeOptions{OverwriteManual: opt.OverwriteManual})
	before := errorSet(Validate(base))
	for _, is := range Validate(merged) {
		if is.Level == "error" && !before[is.Path+"|"+is.Message] {
			return 0, fmt.Errorf("публикация отменена: %s: %s", is.Path, is.Message)
		}
	}
	if gate := RunSuite(&merged, gateKinds, 25); !gatePassed(gate) {
		return 0, errors.New("публикация отменена: не пройдены обязательные проверки качества")
	}
	imp := importID
	return s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "import", ImportID: &imp, Config: merged, ExpectActive: &ver,
		Note: fmt.Sprintf("Импорт «%s»: %s", rec.Filename, plan.Summary())})
}

// Discard отклоняет предпросмотренный пакет без применения.
func (s *Service) Discard(ctx context.Context, importID int64) error {
	rec, err := s.store.GetImport(ctx, importID)
	if err != nil {
		return err
	}
	if rec.Status == "published" {
		return errors.New("пакет уже опубликован")
	}
	return s.store.SetImportStatus(ctx, importID, "discarded")
}

// Rollback возвращает конфигурацию к содержимому прежней версии, создавая новую версию (история сохраняется).
func (s *Service) Rollback(ctx context.Context, admin string, versionID int64) (int64, error) {
	v, err := s.store.GetVersion(ctx, versionID)
	if err != nil {
		return 0, err
	}
	_, cur, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	if cur == versionID {
		return 0, errors.New("эта версия уже активна")
	}
	return s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "rollback", Config: v.Config, ExpectActive: &cur,
		Note: fmt.Sprintf("Откат к версии %d", versionID)})
}

// ---- редакторы ----

// EditCaptain сохраняет правку профиля (помечает его как правленный вручную).
func (s *Service) EditCaptain(ctx context.Context, admin string, cp Captain) (int64, error) {
	b, _ := json.Marshal(cp)
	if msgs := ValidateJSON("captain", b); len(msgs) > 0 {
		return 0, fmt.Errorf("профиль не проходит схему: %s", strings.Join(msgs, "; "))
	}
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	old, exists := cfg.Captains[cp.ID]
	if !exists {
		return 0, fmt.Errorf("капитана %s нет в реестре", cp.ID)
	}
	before := errorSet(Validate(cfg))
	cp.Revision = old.Revision + 1
	cp.Meta.Manual, cp.Meta.Source, cp.Meta.Seed = true, "manual", old.Meta.Seed
	cfg.Captains[cp.ID] = cp
	for _, is := range Validate(cfg) {
		if is.Level == "error" && !before[is.Path+"|"+is.Message] {
			return 0, fmt.Errorf("%s: %s", is.Path, is.Message)
		}
	}
	return s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "edit", Config: cfg, ExpectActive: &ver,
		Note: fmt.Sprintf("Правка капитана %s (%s)", cp.ID, cp.Name)})
}

// SectionDoc возвращает документ раздела в формате пакета (для редактора).
func SectionDoc(cfg Config, kind string) (any, bool) {
	switch kind {
	case "intents":
		return map[string]any{"intents": nz(cfg.Intents)}, true
	case "synonyms":
		return map[string]any{"synonyms": nz(cfg.Synonyms.Synonyms), "slang": nz(cfg.Synonyms.Slang), "typos": nz(cfg.Synonyms.Typos)}, true
	case "templates":
		return map[string]any{"templates": nz(cfg.Templates)}, true
	case "rules":
		return map[string]any{"rules": nz(cfg.Rules)}, true
	case "emotions":
		return map[string]any{"emotions": nz(cfg.Emotions)}, true
	case "facts":
		return map[string]any{"facts": nz(cfg.Facts)}, true
	case "memory_rules":
		return map[string]any{"rules": nz(cfg.MemoryRules)}, true
	case "dialogue_tests":
		return map[string]any{"tests": nz(cfg.Tests)}, true
	case "lessons":
		return map[string]any{"lessons": nz(cfg.Lessons)}, true
	case "strategies":
		return map[string]any{"strategies": nz(cfg.Strategies)}, true
	case "factions":
		return map[string]any{"factions": nz(cfg.Factions)}, true
	case "corporations":
		return map[string]any{"corporations": nz(cfg.Corporations)}, true
	case "relations":
		return map[string]any{"relations": nz(cfg.Relations)}, true
	case "schedules":
		return map[string]any{"schedules": nz(cfg.Schedules)}, true
	}
	return nil, false
}

// PutSection заменяет раздел целиком после проверки по схеме и согласованности; создаёт новую версию.
func (s *Service) PutSection(ctx context.Context, admin, kind string, raw []byte) (int64, error) {
	if _, ok := SectionDoc(NewConfig(), kind); !ok {
		return 0, fmt.Errorf("неизвестный раздел «%s»", kind)
	}
	if msgs := ValidateJSON(kind, raw); len(msgs) > 0 {
		return 0, fmt.Errorf("раздел не проходит схему: %s", strings.Join(msgs, "; "))
	}
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	before := errorSet(Validate(cfg))
	var rep Report
	rep = newReport()
	decodeSection(kind, raw, &cfg, &rep, kind)
	if len(rep.Errors) > 0 {
		return 0, errors.New(rep.Errors[0].Message)
	}
	for _, is := range Validate(cfg) {
		if is.Level == "error" && !before[is.Path+"|"+is.Message] {
			return 0, fmt.Errorf("%s: %s", is.Path, is.Message)
		}
	}
	if gate := RunSuite(&cfg, []string{"facts", "regression"}, 15); !gatePassed(gate) {
		return 0, errors.New("правка отклонена: после неё не проходят проверки фактов или регрессионные диалоги")
	}
	return s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "edit", Config: cfg, ExpectActive: &ver, Note: "Правка раздела «" + kind + "»"})
}

// ---- тесты и сводка ----

// RunTests запускает экзамены по активной конфигурации и сохраняет результат.
func (s *Service) RunTests(ctx context.Context, admin string, kinds []string, sample int) (TestRun, error) {
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return TestRun{}, err
	}
	res := RunSuite(&cfg, kinds, sample)
	run := TestRun{Admin: admin, Sample: sample, Results: res}
	for _, r := range res {
		run.Total += r.Total
		run.Passed += r.Passed
	}
	if ver != 0 {
		v := ver
		run.VersionID = &v
	}
	id, err := s.store.SaveTestRun(ctx, run)
	run.ID = id
	return run, err
}

// Dashboard — сводка для главной страницы Studio.
type Dashboard struct {
	Captains       int            `json:"captains"`
	Target         int            `json:"target"`
	BySource       map[string]int `json:"by_source"`
	ActiveVersion  int64          `json:"active_version"`
	Versions       []VersionInfo  `json:"versions"`
	Imports        []ImportRec    `json:"imports"`
	Sessions       []Session      `json:"sessions"`
	LastTests      *TestRun       `json:"last_tests"`
	ValidationErrs int            `json:"validation_errors"`
	Quality        map[string]any `json:"quality"`
	Statuses       map[string]int `json:"statuses"`
}

// Dashboard собирает сводку.
func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	d := Dashboard{Captains: len(cfg.Captains), Target: 300, ActiveVersion: ver, Quality: Quality(&cfg), Statuses: map[string]int{}}
	d.BySource = d.Quality["by_source"].(map[string]int)
	if d.Versions, err = s.store.ListVersions(ctx, 15); err != nil {
		return d, err
	}
	if d.Imports, err = s.store.ListImports(ctx, 15); err != nil {
		return d, err
	}
	if d.Sessions, err = s.store.ListSessions(ctx, 15); err != nil {
		return d, err
	}
	for _, im := range d.Imports {
		d.Statuses[im.Status]++
		d.ValidationErrs += len(im.Report.Errors)
	}
	runs, err := s.store.ListTestRuns(ctx, 1)
	if err != nil {
		return d, err
	}
	if len(runs) > 0 {
		d.LastTests = &runs[0]
	}
	return d, nil
}

// Store возвращает хранилище (для обработчиков чтения).
func (s *Service) Store() Store { return s.store }

// ImportCaptain добавляет нового капитана или заменяет существующего из отдельного JSON-файла профиля.
// Замена существующего возможна только с overwrite; результат помечается как правленный вручную.
func (s *Service) ImportCaptain(ctx context.Context, admin string, cp Captain, overwrite bool) (int64, error) {
	b, _ := json.Marshal(cp)
	if msgs := ValidateJSON("captain", b); len(msgs) > 0 {
		return 0, fmt.Errorf("профиль не проходит схему: %s", strings.Join(msgs, "; "))
	}
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return 0, err
	}
	old, exists := cfg.Captains[cp.ID]
	if exists && !overwrite {
		return 0, fmt.Errorf("капитан %s уже есть: подтвердите замену", cp.ID)
	}
	before := errorSet(Validate(cfg))
	if exists {
		cp.Revision = old.Revision + 1
	} else if cp.Revision < 1 {
		cp.Revision = 1
	}
	cp.Meta.Manual, cp.Meta.Source = true, "manual"
	cfg.Captains[cp.ID] = cp
	for _, is := range Validate(cfg) {
		if is.Level == "error" && !before[is.Path+"|"+is.Message] {
			return 0, fmt.Errorf("%s: %s", is.Path, is.Message)
		}
	}
	note := "Импорт капитана " + cp.ID
	if exists {
		note = "Замена капитана " + cp.ID
	}
	return s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "edit", Config: cfg, ExpectActive: &ver, Note: note + " (" + cp.Name + ")"})
}

// CaptainSummary — строка реестра.
type CaptainSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CallSign    string `json:"call_sign"`
	Profession  string `json:"profession"`
	Faction     string `json:"faction"`
	Corporation string `json:"corporation"`
	Formality   string `json:"formality"`
	Humor       string `json:"humor"`
	Source      string `json:"source"`
	Manual      bool   `json:"manual"`
	Revision    int    `json:"revision"`
}

// CaptainFilter — условия поиска.
type CaptainFilter struct {
	Q, Faction, Profession, Corporation, Source, Formality string
}

// ListCaptains возвращает отфильтрованный реестр (по возрастанию идентификатора).
func ListCaptains(cfg Config, f CaptainFilter) []CaptainSummary {
	ids := make([]string, 0, len(cfg.Captains))
	for id := range cfg.Captains {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	q := strings.ToLower(strings.TrimSpace(f.Q))
	var out []CaptainSummary
	for _, id := range ids {
		c := cfg.Captains[id]
		corp := ""
		if c.Corporation != nil {
			corp = *c.Corporation
		}
		src := c.Meta.Source
		if c.Meta.Manual {
			src = "manual"
		}
		if f.Faction != "" && c.Faction != f.Faction || f.Profession != "" && c.Profession != f.Profession ||
			f.Corporation != "" && corp != f.Corporation && !(f.Corporation == "none" && corp == "") ||
			f.Source != "" && src != f.Source || f.Formality != "" && c.Speech.Formality != f.Formality {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(id+" "+c.Name+" "+c.CallSign+" "+c.Bio), q) {
			continue
		}
		out = append(out, CaptainSummary{ID: id, Name: c.Name, CallSign: c.CallSign, Profession: c.Profession, Faction: c.Faction,
			Corporation: corp, Formality: c.Speech.Formality, Humor: c.Speech.Humor, Source: src, Manual: c.Meta.Manual, Revision: c.Revision})
	}
	return out
}

// UniquenessReport — отчёт о повторах и близких парах.
type UniquenessReport struct {
	Captains   int           `json:"captains"`
	Duplicates []string      `json:"duplicates"`
	Similar    []SimilarPair `json:"similar"`
	MaxSim     float64       `json:"max_similarity"`
}

// SimilarPair — пара похожих капитанов.
type SimilarPair struct {
	A     string  `json:"a"`
	B     string  `json:"b"`
	Score float64 `json:"score"`
}

// Uniqueness проверяет уникальность персонажей: повторы имён, биографий, характеров и самые похожие пары.
func Uniqueness(cfg Config) UniquenessReport {
	rep := UniquenessReport{Captains: len(cfg.Captains), Duplicates: []string{}, Similar: []SimilarPair{}}
	ids := make([]string, 0, len(cfg.Captains))
	for id := range cfg.Captains {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	names, bios, sigs := map[string]string{}, map[string]string{}, map[string]string{}
	for _, id := range ids {
		c := cfg.Captains[id]
		for _, p := range []struct {
			m    map[string]string
			k, w string
		}{{names, strings.ToLower(c.Name), "имя"}, {bios, c.Bio, "биография"}, {sigs, c.Signature(), "характер и речь"}} {
			if o, dup := p.m[p.k]; dup {
				rep.Duplicates = append(rep.Duplicates, fmt.Sprintf("%s и %s: совпадает %s", o, id, p.w))
			}
			p.m[p.k] = id
		}
	}
	var pairs []SimilarPair
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if sc := Similarity(cfg.Captains[ids[i]], cfg.Captains[ids[j]]); sc >= 0.8 {
				pairs = append(pairs, SimilarPair{ids[i], ids[j], sc})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Score > pairs[j].Score })
	if len(pairs) > 0 {
		rep.MaxSim = pairs[0].Score
	}
	if len(pairs) > 20 {
		pairs = pairs[:20]
	}
	rep.Similar = pairs
	return rep
}

// RenameResult — итог переименования.
type RenameResult struct {
	Renamed int   `json:"renamed"`
	Skipped int   `json:"skipped"` // правленные вручную и уже переименованные
	Version int64 `json:"version"`
}

// ApplyNicknames заменяет имена капитанов игровыми никами (русские и английские вперемешку). Профили, правленные вручную,
// не затрагиваются. Первое имя в биографии («Марта — …») заменяется местоимением. Повторный запуск безопасен.
func (s *Service) ApplyNicknames(ctx context.Context, admin string, seed int64) (RenameResult, error) {
	cfg, ver, err := s.Active(ctx)
	if err != nil {
		return RenameResult{}, err
	}
	taken := map[string]bool{}
	for _, c := range cfg.Captains {
		if c.Meta.Manual {
			taken[strings.ToLower(c.Name)] = true
		}
	}
	ids := make([]string, 0, len(cfg.Captains))
	for id := range cfg.Captains {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var res RenameResult
	for _, id := range ids {
		c := cfg.Captains[id]
		if c.Meta.Manual {
			res.Skipped++
			continue
		}
		n := 0
		fmt.Sscanf(strings.TrimPrefix(id, "cpt-"), "%d", &n)
		nick := ""
		for shift := 0; shift < 50; shift++ {
			cand := NickFor(seed, max(n, 1), shift)
			if !taken[strings.ToLower(cand)] {
				nick = cand
				break
			}
		}
		if nick == "" || c.Name == nick {
			res.Skipped++
			continue
		}
		old := c.Name
		first := strings.Fields(old)
		if len(first) > 0 {
			pron := "Он"
			if c.Gender == "f" {
				pron = "Она"
			}
			c.Bio = strings.Replace(c.Bio, first[0]+" — ", pron+" — ", 1)
		}
		c.Name, c.CallSign = nick, ""
		c.Revision++
		taken[strings.ToLower(nick)] = true
		cfg.Captains[id] = c
		res.Renamed++
	}
	if res.Renamed == 0 {
		res.Version = ver
		return res, nil
	}
	if bad := firstErrors(Validate(cfg), 3); len(bad) > 0 {
		return res, fmt.Errorf("после переименования конфигурация не проходит проверку: %s", strings.Join(bad, "; "))
	}
	id, err := s.store.SaveVersion(ctx, NewVersion{Admin: admin, Source: "edit", Config: cfg, ExpectActive: &ver,
		Note: fmt.Sprintf("Игровые ники: переименовано %d капитанов", res.Renamed)})
	res.Version = id
	return res, err
}
