package aistudio

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/*.json
var schemaFS embed.FS

// Ограничения безопасного импорта.
const (
	MaxZipBytes   = 16 << 20 // размер самого архива
	MaxFiles      = 800
	MaxFileBytes  = 2 << 20  // один файл после распаковки
	MaxTotalBytes = 48 << 20 // всё содержимое после распаковки
	MaxRatio      = 300      // степень сжатия одного файла (защита от ZIP bomb)
	MaxDepth      = 3
)

// Issue — замечание проверки.
type Issue struct {
	Level   string `json:"level"` // error | warning
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Report — отчёт проверки пакета.
type Report struct {
	OK        bool           `json:"ok"`
	Errors    []Issue        `json:"errors"`
	Warnings  []Issue        `json:"warnings"`
	Stats     map[string]int `json:"stats"`
	PackID    string         `json:"pack_id"`
	Kind      string         `json:"kind"`
	Integrity string         `json:"integrity"` // verified | partial | none | failed
}

func (r *Report) errf(code, path, format string, a ...any) {
	r.Errors = append(r.Errors, Issue{"error", code, path, fmt.Sprintf(format, a...)})
}

func (r *Report) warnf(code, path, format string, a ...any) {
	r.Warnings = append(r.Warnings, Issue{"warning", code, path, fmt.Sprintf(format, a...)})
}

func newReport() Report {
	return Report{Errors: []Issue{}, Warnings: []Issue{}, Stats: map[string]int{}}
}

// Manifest — manifest.json.
type Manifest struct {
	Schema        string         `json:"schema"`
	SchemaVersion int            `json:"schema_version"`
	PackID        string         `json:"pack_id"`
	Kind          string         `json:"kind"`
	CreatedAt     string         `json:"created_at"`
	CreatedBy     string         `json:"created_by"`
	BaseVersion   *int           `json:"base_version"`
	Batch         map[string]any `json:"batch"`
	Captains      []string       `json:"captains"`
	Files         []ManifestFile `json:"files"`
	Notes         string         `json:"notes"`
}

// ManifestFile — файл и его SHA-256.
type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Parsed — разобранный и проверенный по структуре пакет (до слияния с конфигурацией).
type Parsed struct {
	Manifest Manifest
	Partial  Config          // только то, что есть в пакете
	Sections map[string]bool // какие разделы пакета присутствуют
	Notes    map[string]string
}

// fileKind сопоставляет путь файла схеме и разделу конфигурации.
var fileKinds = map[string]string{
	"dialogue/intents.json":     "intents",
	"dialogue/synonyms.json":    "synonyms",
	"dialogue/templates.json":   "templates",
	"dialogue/rules.json":       "rules",
	"dialogue/emotions.json":    "emotions",
	"knowledge/facts.json":      "facts",
	"society/factions.json":     "factions",
	"society/corporations.json": "corporations",
	"society/relations.json":    "relations",
	"behavior/strategies.json":  "strategies",
	"learning/lessons.json":     "lessons",
	"memory/memory_rules.json":  "memory_rules",
	"scheduler/schedules.json":  "schedules",
	"tests/dialogue_tests.json": "dialogue_tests",
}

var allowedDirs = map[string]bool{
	"schemas": true, "captains": true, "dialogue": true, "knowledge": true, "society": true,
	"behavior": true, "learning": true, "memory": true, "scheduler": true, "tests": true, "reports": true,
}

var (
	rootDocRe   = regexp.MustCompile(`^[A-Za-z0-9_\-]+\.(md|txt)$`)
	captainFile = regexp.MustCompile(`^captains/(cpt-\d{4})\.json$`)
	secretRes   = []*regexp.Regexp{
		regexp.MustCompile(`sk-[A-Za-z0-9_\-]{20,}`),
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
		regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`),
		regexp.MustCompile(`(?i)\bapi[_\- ]?key\b\s*[:=]\s*\S{6,}`),
		regexp.MustCompile(`(?i)\bpassword\b\s*[:=]\s*\S{4,}`),
		regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]{20,}`),
	}
	execMagic = [][]byte{[]byte("MZ"), []byte("\x7fELF"), []byte("#!"), []byte("PK\x03\x04"), []byte("\xca\xfe\xba\xbe"), []byte("<?php")}
	scriptRe  = regexp.MustCompile(`(?i)<script\b|javascript:|<iframe\b|powershell|cmd\.exe|/bin/(ba)?sh`)
	// запрещённые ключи JSON: пароли, ключи, приватные сообщения, живой игровой прогресс
	forbiddenKeys = map[string]bool{
		"password": true, "passwd": true, "secret": true, "api_key": true, "apikey": true, "token": true,
		"authorization": true, "private_message": true, "private_messages": true, "chat_log": true,
		"player_progress": true, "inventory": true, "credits_balance": true, "session": true, "email": true,
	}
)

var (
	schemaOnce sync.Once
	schemas    map[string]*jsonschema.Schema
	schemaErr  error
	schemaRaw  map[string][]byte // имя файла схемы -> содержимое
)

func loadSchemas() {
	schemas = map[string]*jsonschema.Schema{}
	schemaRaw = map[string][]byte{}
	c := jsonschema.NewCompiler()
	entries, err := schemaFS.ReadDir("schemas")
	if err != nil {
		schemaErr = err
		return
	}
	names := []string{}
	for _, e := range entries {
		b, err := schemaFS.ReadFile("schemas/" + e.Name())
		if err != nil {
			schemaErr = err
			return
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			schemaErr = fmt.Errorf("схема %s: %w", e.Name(), err)
			return
		}
		if err := c.AddResource("mem:///"+e.Name(), doc); err != nil {
			schemaErr = err
			return
		}
		schemaRaw[e.Name()] = b
		names = append(names, e.Name())
	}
	for _, n := range names {
		s, err := c.Compile("mem:///" + n)
		if err != nil {
			schemaErr = fmt.Errorf("схема %s: %w", n, err)
			return
		}
		schemas[strings.TrimSuffix(n, ".schema.json")] = s
	}
}

// SchemaFiles возвращает содержимое схем (для включения в пакеты): имя файла -> JSON.
func SchemaFiles() map[string][]byte {
	schemaOnce.Do(loadSchemas)
	out := map[string][]byte{}
	for k, v := range schemaRaw {
		out[k] = v
	}
	return out
}

// validateAgainst проверяет JSON по схеме; возвращает список сообщений.
func validateAgainst(kind string, data []byte) []string {
	schemaOnce.Do(loadSchemas)
	if schemaErr != nil {
		return []string{"внутренняя ошибка схем: " + schemaErr.Error()}
	}
	s, ok := schemas[kind]
	if !ok {
		return []string{"неизвестная схема " + kind}
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return []string{"некорректный JSON: " + err.Error()}
	}
	if err := s.Validate(inst); err != nil {
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return []string{err.Error()}
		}
		var msgs []string
		for _, u := range ve.BasicOutput().Errors {
			if u.Error == nil {
				continue
			}
			b, _ := json.Marshal(u.Error)
			msgs = append(msgs, fmt.Sprintf("%s: %s", u.InstanceLocation, strings.Trim(string(b), `"`)))
			if len(msgs) >= 8 {
				break
			}
		}
		if len(msgs) == 0 {
			msgs = []string{ve.Error()}
		}
		return msgs
	}
	return nil
}

// ValidateJSON проверяет документ раздела по схеме (используется и редакторами в админке).
func ValidateJSON(kind string, data []byte) []string { return validateAgainst(kind, data) }

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// safeName проверяет путь записи архива. Возвращает нормализованный путь или причину отказа.
func safeName(name string) (string, string) {
	if name == "" || !utf8.ValidString(name) {
		return "", "пустое или некорректное имя"
	}
	if strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") {
		return "", "абсолютный путь, обратная косая черта или нулевой байт"
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", "управляющие символы в имени"
		}
	}
	clean := path.Clean(name)
	if clean != name || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", "выход за пределы каталога (ZIP Slip) или неканоничный путь"
	}
	parts := strings.Split(clean, "/")
	if len(parts) > MaxDepth {
		return "", "слишком глубокая вложенность"
	}
	for _, p := range parts {
		if p == "" || len(p) > 100 || strings.Contains(p, ":") || strings.HasPrefix(p, ".") {
			return "", "недопустимый сегмент пути"
		}
	}
	return clean, ""
}

func scanJSONKeys(v any, path string, report *Report, file string) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if forbiddenKeys[strings.ToLower(k)] {
				report.errf("forbidden_field", file, "запрещённое поле «%s» (пароли, ключи, приватные сообщения и игровой прогресс в пакетах недопустимы)", k)
			}
			scanJSONKeys(val, path+"/"+k, report, file)
		}
	case []any:
		for _, e := range t {
			scanJSONKeys(e, path, report, file)
		}
	}
}

// ReadSafe безопасно читает ZIP: лимиты размера и числа файлов, защита от ZIP Slip и ZIP bomb, запрет символических ссылок,
// шифрования, не-текстовых и исполняемых файлов, секретов. allowed — допустимые каталоги верхнего уровня.
// Содержимое не выполняется и не пишется на диск. ok=false: дальше разбирать нечего.
func ReadSafe(data []byte, allowed map[string]bool) (files map[string][]byte, total int64, rep Report, ok bool) {
	rep = newReport()
	if len(data) == 0 {
		rep.errf("empty", "", "пустой файл")
		return nil, 0, rep, false
	}
	if len(data) > MaxZipBytes {
		rep.errf("too_large", "", "архив больше %d МБ", MaxZipBytes>>20)
		return nil, 0, rep, false
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		rep.errf("bad_zip", "", "это не ZIP-архив или он повреждён: %v", err)
		return nil, 0, rep, false
	}
	if len(zr.File) > MaxFiles {
		rep.errf("too_many_files", "", "в архиве %d записей, максимум %d", len(zr.File), MaxFiles)
		return nil, 0, rep, false
	}

	files = map[string][]byte{}
	seen := map[string]bool{}
	for _, f := range zr.File {
		name, why := safeName(f.Name)
		if why != "" {
			if !strings.HasSuffix(f.Name, "/") { // записи-каталоги вида "dir/" допустимы
				rep.errf("unsafe_path", f.Name, "%s", why)
			}
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 || f.Mode()&os.ModeDevice != 0 || f.Mode()&os.ModeNamedPipe != 0 || f.Mode()&os.ModeSocket != 0 {
			rep.errf("special_file", name, "символические ссылки и специальные файлы запрещены")
			continue
		}
		if f.Flags&1 != 0 {
			rep.errf("encrypted", name, "зашифрованные записи не поддерживаются")
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			rep.errf("duplicate", name, "дубликат имени файла в архиве")
			continue
		}
		seen[key] = true
		if f.UncompressedSize64 > MaxFileBytes {
			rep.errf("file_too_large", name, "файл больше %d МБ после распаковки", MaxFileBytes>>20)
			continue
		}
		if f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 > MaxRatio && f.UncompressedSize64 > 64<<10 {
			rep.errf("zip_bomb", name, "подозрительная степень сжатия (защита от ZIP bomb)")
			continue
		}
		rc, err := f.Open()
		if err != nil {
			rep.errf("bad_entry", name, "не удалось открыть запись: %v", err)
			continue
		}
		// размер проверяем по факту распаковки, а не по заголовку
		b, err := io.ReadAll(io.LimitReader(rc, MaxFileBytes+1))
		rc.Close()
		if err != nil {
			rep.errf("bad_entry", name, "ошибка чтения записи: %v", err)
			continue
		}
		if int64(len(b)) > MaxFileBytes {
			rep.errf("file_too_large", name, "файл больше %d МБ после распаковки", MaxFileBytes>>20)
			continue
		}
		total += int64(len(b))
		if total > MaxTotalBytes {
			rep.errf("zip_bomb", "", "суммарный размер содержимого больше %d МБ", MaxTotalBytes>>20)
			return nil, 0, rep, false
		}
		files[name] = b
	}
	if len(rep.Errors) > 0 && files["manifest.json"] == nil {
		return nil, 0, rep, false
	}

	// структура, типы файлов, содержимое
	for name, b := range files {
		parts := strings.Split(name, "/")
		if len(parts) == 1 {
			if name != "manifest.json" && !rootDocRe.MatchString(name) {
				rep.errf("unexpected_file", name, "в корне допускаются только manifest.json и документы .md/.txt")
			}
		} else if !allowed[parts[0]] {
			rep.errf("unexpected_dir", name, "каталог «%s» не входит в стандарт %s", parts[0], SchemaName)
		}
		ext := strings.ToLower(path.Ext(name))
		if ext != ".json" && ext != ".md" && ext != ".txt" {
			rep.errf("forbidden_type", name, "тип файла «%s» запрещён: разрешены только .json, .md, .txt (исполняемый код недопустим)", ext)
			continue
		}
		for _, m := range execMagic {
			if bytes.HasPrefix(b, m) {
				rep.errf("executable", name, "файл похож на исполняемый или вложенный архив")
			}
		}
		if bytes.IndexByte(b, 0) >= 0 || !utf8.Valid(b) {
			rep.errf("not_text", name, "файл не является текстом UTF-8")
			continue
		}
		if scriptRe.Match(b) {
			rep.errf("script", name, "в файле найден код или команды (скрипты запрещены)")
		}
		for _, re := range secretRes {
			if re.Match(b) {
				rep.errf("secret", name, "в файле найдены признаки секретов (ключи, пароли, токены)")
				break
			}
		}
	}
	return files, total, rep, true
}

// ParsePack безопасно читает ZIP, проверяет структуру, целостность и схемы.
// Ничего не пишет на диск и не выполняет содержимое. Ссылочная целостность проверяется позже (Validate).
func ParsePack(data []byte) (*Parsed, Report) {
	files, total, rep, ok := ReadSafe(data, allowedDirs)
	fail := func() (*Parsed, Report) { rep.OK = false; return nil, rep }
	if !ok {
		return fail()
	}
	mraw, ok := files["manifest.json"]
	if !ok {
		rep.errf("no_manifest", "manifest.json", "нет manifest.json")
		return fail()
	}
	if msgs := validateAgainst("manifest", mraw); len(msgs) > 0 {
		for _, m := range msgs {
			rep.errf("schema", "manifest.json", "%s", m)
		}
		return fail()
	}
	var man Manifest
	if err := json.Unmarshal(mraw, &man); err != nil {
		rep.errf("schema", "manifest.json", "%v", err)
		return fail()
	}
	rep.PackID, rep.Kind = man.PackID, man.Kind

	// целостность
	listed := map[string]string{}
	for _, mf := range man.Files {
		p, why := safeName(mf.Path)
		if why != "" {
			rep.errf("unsafe_path", mf.Path, "в манифесте: %s", why)
			continue
		}
		listed[p] = mf.SHA256
	}
	switch {
	case len(man.Files) == 0:
		rep.Integrity = "none"
		rep.warnf("integrity", "manifest.json", "список файлов и контрольных сумм SHA-256 пуст: целостность не подтверждена")
	default:
		rep.Integrity = "verified"
		for p, want := range listed {
			b, ok := files[p]
			if !ok {
				rep.errf("integrity", p, "файл указан в манифесте, но отсутствует в архиве")
				rep.Integrity = "failed"
				continue
			}
			if sha(b) != want {
				rep.errf("integrity", p, "контрольная сумма SHA-256 не совпадает: файл изменён или повреждён")
				rep.Integrity = "failed"
			}
		}
		for p := range files {
			if p == "manifest.json" {
				continue
			}
			if _, ok := listed[p]; !ok {
				rep.warnf("integrity", p, "файл не указан в манифесте, целостность не проверена")
				if rep.Integrity == "verified" {
					rep.Integrity = "partial"
				}
			}
		}
	}

	// схемы пакета: должны быть валидным JSON; расхождение с серверными только предупреждение
	embedded := SchemaFiles()
	for name, b := range files {
		if strings.HasPrefix(name, "schemas/") {
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				rep.errf("schema_file", name, "схема в пакете не является JSON")
				continue
			}
			base := strings.TrimPrefix(name, "schemas/")
			if eb, ok := embedded[base]; ok && sha(eb) != sha(b) {
				rep.warnf("schema_outdated", name, "схема в пакете отличается от серверной: проверка идёт по серверной")
			}
		}
	}

	out := &Parsed{Manifest: man, Partial: NewConfig(), Sections: map[string]bool{}, Notes: map[string]string{}}
	if man.Schema != SchemaName || man.SchemaVersion != SchemaVersion {
		rep.errf("schema_version", "manifest.json", "ожидается %s версии %d", SchemaName, SchemaVersion)
	}

	// файлы данных
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	found := map[string]bool{}
	for _, name := range names {
		b := files[name]
		if name == "manifest.json" || strings.HasPrefix(name, "schemas/") || !strings.HasSuffix(name, ".json") {
			if strings.HasPrefix(name, "reports/") && !strings.HasSuffix(name, ".json") {
				out.Notes[name] = string(b)
			}
			continue
		}
		if strings.HasPrefix(name, "reports/") {
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				rep.errf("json", name, "некорректный JSON: %v", err)
			}
			continue
		}
		var generic any
		if err := json.Unmarshal(b, &generic); err != nil {
			rep.errf("json", name, "некорректный JSON: %v", err)
			continue
		}
		scanJSONKeys(generic, "", &rep, name)
		if m := captainFile.FindStringSubmatch(name); m != nil {
			for _, msg := range validateAgainst("captain", b) {
				rep.errf("schema", name, "%s", msg)
			}
			var c Captain
			if err := json.Unmarshal(b, &c); err != nil {
				rep.errf("json", name, "%v", err)
				continue
			}
			if c.ID != m[1] {
				rep.errf("captain_id", name, "идентификатор в файле (%s) не совпадает с именем файла", c.ID)
				continue
			}
			if c.Meta.Source == "generator" || c.Meta.Source == "" {
				// профиль, прошедший через ChatGPT, помечается источником chatgpt; сохраняем как есть
			}
			out.Partial.Captains[c.ID] = c
			found[c.ID] = true
			continue
		}
		kind, ok := fileKinds[name]
		if !ok {
			rep.errf("unexpected_file", name, "неизвестный файл данных: в этом каталоге разрешены только файлы стандарта")
			continue
		}
		if msgs := validateAgainst(kind, b); len(msgs) > 0 {
			for _, m := range msgs {
				rep.errf("schema", name, "%s", m)
			}
			continue
		}
		out.Sections[kind] = true
		decodeSection(kind, b, &out.Partial, &rep, name)
	}
	for name := range files {
		if strings.HasPrefix(name, "captains/") && !captainFile.MatchString(name) && strings.HasSuffix(name, ".json") {
			rep.errf("captain_file", name, "имя файла капитана должно быть captains/cpt-NNNN.json")
		}
	}

	// манифест и содержимое captains/ должны совпадать
	mset := map[string]bool{}
	for _, id := range man.Captains {
		mset[id] = true
		if !found[id] {
			rep.errf("manifest_captains", "captains/"+id+".json", "капитан указан в манифесте, но файла нет или он не прошёл проверку")
		}
	}
	for id := range found {
		if !mset[id] {
			rep.errf("manifest_captains", "captains/"+id+".json", "файл капитана не указан в списке captains манифеста")
		}
	}
	if man.Kind == "full" && len(man.Captains) == 0 && len(out.Sections) == 0 {
		rep.warnf("empty_pack", "", "пакет не содержит ни капитанов, ни разделов данных")
	}

	out.Manifest = man
	rep.Stats["files"] = len(files)
	rep.Stats["captains"] = len(found)
	rep.Stats["sections"] = len(out.Sections)
	rep.Stats["bytes"] = int(total)
	rep.OK = len(rep.Errors) == 0
	return out, rep
}

func decodeSection(kind string, b []byte, cfg *Config, rep *Report, file string) {
	fail := func(err error) { rep.errf("json", file, "%v", err) }
	switch kind {
	case "intents":
		var d struct {
			Intents []Intent `json:"intents"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Intents = d.Intents
	case "synonyms":
		if err := json.Unmarshal(b, &cfg.Synonyms); err != nil {
			fail(err)
		}
	case "templates":
		var d struct {
			Templates []Template `json:"templates"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Templates = d.Templates
	case "rules":
		var d struct {
			Rules []Rule `json:"rules"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Rules = d.Rules
	case "emotions":
		var d struct {
			Emotions []Emotion `json:"emotions"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Emotions = d.Emotions
	case "facts":
		var d struct {
			Facts []Fact `json:"facts"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Facts = d.Facts
	case "factions":
		var d struct {
			Factions []Faction `json:"factions"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Factions = d.Factions
	case "corporations":
		var d struct {
			Corporations []Corporation `json:"corporations"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Corporations = d.Corporations
	case "relations":
		var d struct {
			Relations []FactionRelation `json:"relations"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Relations = d.Relations
	case "strategies":
		var d struct {
			Strategies []Strategy `json:"strategies"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Strategies = d.Strategies
	case "lessons":
		var d struct {
			Lessons []Lesson `json:"lessons"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Lessons = d.Lessons
	case "memory_rules":
		var d struct {
			Rules []MemoryRule `json:"rules"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.MemoryRules = d.Rules
	case "schedules":
		var d struct {
			Schedules []Schedule `json:"schedules"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Schedules = d.Schedules
	case "dialogue_tests":
		var d struct {
			Tests []DialogueTest `json:"tests"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			fail(err)
			return
		}
		cfg.Tests = d.Tests
	}
}

// ---- сборка пакетов ----

// PackOptions — параметры экспорта.
type PackOptions struct {
	PackID    string
	Kind      string // full | partial
	CreatedBy string
	Base      *int
	Batch     map[string]any
	Notes     string
	Readme    string            // README_FOR_CHATGPT.md
	Extra     map[string][]byte // дополнительные документы (.md/.txt)
}

// BuildPack собирает пакет: данные конфигурации, выбранные капитаны, схемы, документы и манифест с SHA-256.
func BuildPack(opts PackOptions, cfg Config, captainIDs []string, now string) ([]byte, error) {
	files := map[string][]byte{}
	put := func(name string, v any) error {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		files[name] = append(b, '\n')
		return nil
	}
	sort.Strings(captainIDs)
	for _, id := range captainIDs {
		c, ok := cfg.Captains[id]
		if !ok {
			return nil, fmt.Errorf("капитана %s нет в конфигурации", id)
		}
		if err := put("captains/"+id+".json", c); err != nil {
			return nil, err
		}
	}
	sec := []struct {
		name string
		v    any
	}{
		{"dialogue/intents.json", map[string]any{"intents": nz(cfg.Intents)}},
		{"dialogue/synonyms.json", map[string]any{"synonyms": nz(cfg.Synonyms.Synonyms), "slang": nz(cfg.Synonyms.Slang), "typos": nz(cfg.Synonyms.Typos)}},
		{"dialogue/templates.json", map[string]any{"templates": nz(cfg.Templates)}},
		{"dialogue/rules.json", map[string]any{"rules": nz(cfg.Rules)}},
		{"dialogue/emotions.json", map[string]any{"emotions": nz(cfg.Emotions)}},
		{"knowledge/facts.json", map[string]any{"facts": nz(cfg.Facts)}},
		{"society/factions.json", map[string]any{"factions": nz(cfg.Factions)}},
		{"society/corporations.json", map[string]any{"corporations": nz(cfg.Corporations)}},
		{"society/relations.json", map[string]any{"relations": nz(cfg.Relations)}},
		{"behavior/strategies.json", map[string]any{"strategies": nz(cfg.Strategies)}},
		{"learning/lessons.json", map[string]any{"lessons": nz(cfg.Lessons)}},
		{"memory/memory_rules.json", map[string]any{"rules": nz(cfg.MemoryRules)}},
		{"scheduler/schedules.json", map[string]any{"schedules": nz(cfg.Schedules)}},
		{"tests/dialogue_tests.json", map[string]any{"tests": nz(cfg.Tests)}},
	}
	for _, s := range sec {
		if err := put(s.name, s.v); err != nil {
			return nil, err
		}
	}
	for n, b := range SchemaFiles() {
		files["schemas/"+n] = b
	}
	if opts.Readme != "" {
		files["README_FOR_CHATGPT.md"] = []byte(opts.Readme)
	}
	for n, b := range opts.Extra {
		files[n] = b
	}
	files["reports/README.md"] = []byte("# Отчёты\n\nСюда ChatGPT кладёт короткие заметки: что изменено и почему (файлы .md или .txt). Содержимое не влияет на игру.\n")

	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	man := Manifest{
		Schema: SchemaName, SchemaVersion: SchemaVersion, PackID: opts.PackID, Kind: opts.Kind, CreatedAt: now,
		CreatedBy: opts.CreatedBy, BaseVersion: opts.Base, Batch: opts.Batch, Captains: append([]string{}, captainIDs...), Notes: opts.Notes,
	}
	if man.Captains == nil {
		man.Captains = []string{}
	}
	for _, n := range names {
		man.Files = append(man.Files, ManifestFile{Path: n, SHA256: sha(files[n])})
	}
	mb, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name string, b []byte) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetModTime(parseTime(now))
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	if err := write("manifest.json", append(mb, '\n')); err != nil {
		return nil, err
	}
	for _, n := range names {
		if err := write(n, files[n]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// nz заменяет nil-срез пустым, чтобы в JSON был [] а не null.
func nz[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Unix(0, 0).UTC()
	}
	return t
}
