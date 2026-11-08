package shipcatalog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

// Формат ZIP-патча параметров кораблей.
const (
	PatchSchema        = "Ship_Patch_Pack_v1"
	PatchSchemaVersion = 1
)

var (
	patchDirs  = map[string]bool{"schemas": true, "ships": true, "reports": true}
	packIDRe   = regexp.MustCompile(`^[A-Za-z0-9_\-]{4,64}$`)
	shipFileRe = regexp.MustCompile(`^ships/([A-Z]{3}-[A-Z0-9]{3,12})\.json$`)

	schemaOnce sync.Once
	schemaObj  *jsonschema.Schema
	schemaErr  error
)

func shipSchema() (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		c := jsonschema.NewCompiler()
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON()))
		if err != nil {
			schemaErr = err
			return
		}
		if err := c.AddResource("mem:///ship.schema.json", doc); err != nil {
			schemaErr = err
			return
		}
		schemaObj, schemaErr = c.Compile("mem:///ship.schema.json")
	})
	return schemaObj, schemaErr
}

func validateShipJSON(b []byte) []string {
	sch, err := shipSchema()
	if err != nil {
		return []string{"внутренняя ошибка схемы: " + err.Error()}
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return []string{"некорректный JSON: " + err.Error()}
	}
	if err := sch.Validate(inst); err != nil {
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return []string{err.Error()}
		}
		var out []string
		for _, u := range ve.BasicOutput().Errors {
			if u.Error == nil {
				continue
			}
			m, _ := json.Marshal(u.Error)
			out = append(out, u.InstanceLocation+": "+strings.Trim(string(m), `"`))
			if len(out) >= 8 {
				break
			}
		}
		if len(out) == 0 {
			out = []string{ve.Error()}
		}
		return out
	}
	return nil
}

type manifest struct {
	Schema        string   `json:"schema"`
	SchemaVersion int      `json:"schema_version"`
	PackID        string   `json:"pack_id"`
	Kind          string   `json:"kind"`
	CreatedAt     string   `json:"created_at"`
	CreatedBy     string   `json:"created_by"`
	Ships         []string `json:"ships"`
	Files         []mfile  `json:"files"`
	Notes         string   `json:"notes"`
}

type mfile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func randID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ExportPatch собирает ZIP с паспортами выбранных кораблей (пустой список — все), схемой и мастер-промптом для ChatGPT.
func (s *Service) ExportPatch(ctx context.Context, ids []string) ([]byte, string, error) {
	doc, ver, err := s.Active(ctx)
	if err != nil {
		return nil, "", err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	files := map[string][]byte{}
	var listed []string
	for _, sp := range doc.Ships {
		if len(want) > 0 && !want[sp.ID] {
			continue
		}
		b, _ := json.MarshalIndent(sp, "", "  ")
		files["ships/"+sp.ID+".json"] = append(b, '\n')
		listed = append(listed, sp.ID)
	}
	if len(listed) == 0 {
		return nil, "", errors.New("нет кораблей для экспорта")
	}
	for id := range want {
		found := false
		for _, l := range listed {
			found = found || l == id
		}
		if !found {
			return nil, "", fmt.Errorf("корабля %s нет в каталоге", id)
		}
	}
	files["schemas/ship.schema.json"] = SchemaJSON()
	files["README_FOR_CHATGPT.md"] = []byte(PatchPrompt())
	files["reports/README.md"] = []byte("# Отчёты\n\nСюда кладите короткие заметки: что изменено и почему (.md или .txt). Содержимое не влияет на игру.\n")
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	now := s.now().UTC()
	m := manifest{Schema: PatchSchema, SchemaVersion: PatchSchemaVersion, PackID: "shipspatch-" + now.Format("20060102-150405"), Kind: "patch",
		CreatedAt: now.Format(time.RFC3339), CreatedBy: "studio", Ships: listed, Notes: fmt.Sprintf("Параметры кораблей из каталога версии %d", ver)}
	for _, n := range names {
		m.Files = append(m.Files, mfile{Path: n, SHA256: sum(files[n])})
	}
	mb, _ := json.MarshalIndent(m, "", "  ")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	put := func(name string, b []byte) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetModTime(now)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	if err := put("manifest.json", append(mb, '\n')); err != nil {
		return nil, "", err
	}
	for _, n := range names {
		if err := put(n, files[n]); err != nil {
			return nil, "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "ships-patch-" + now.Format("20060102-150405") + ".zip", nil
}

// PreviewPatch безопасно разбирает патч и строит предпросмотр различий. Каталог не меняется.
func (s *Service) PreviewPatch(ctx context.Context, admin, filename string, data []byte) (*Preview, error) {
	p := &Preview{ID: randID(), Filename: filename, SHA256: sum(data), Status: "rejected", Errors: []Issue{}, Warnings: []Issue{}, Changes: []ShipChange{}, CreatedAt: s.now()}
	bad := func(code, path, f string, a ...any) {
		p.Errors = append(p.Errors, Issue{code, path, fmt.Sprintf(f, a...)})
	}
	files, _, rep, ok := aistudio.ReadSafe(data, patchDirs)
	for _, e := range rep.Errors {
		p.Errors = append(p.Errors, Issue{e.Code, e.Path, e.Message})
	}
	for _, w := range rep.Warnings {
		p.Warnings = append(p.Warnings, Issue{w.Code, w.Path, w.Message})
	}
	if !ok || len(p.Errors) > 0 {
		s.putPreview(p)
		return p, nil
	}
	var m manifest
	mraw, has := files["manifest.json"]
	if !has {
		bad("no_manifest", "manifest.json", "нет manifest.json")
	} else if err := json.Unmarshal(mraw, &m); err != nil {
		bad("manifest", "manifest.json", "некорректный манифест: %v", err)
	} else {
		if m.Schema != PatchSchema || m.SchemaVersion != PatchSchemaVersion {
			bad("schema_version", "manifest.json", "ожидается %s версии %d", PatchSchema, PatchSchemaVersion)
		}
		if !packIDRe.MatchString(m.PackID) {
			bad("pack_id", "manifest.json", "pack_id: 4-64 символа (латиница, цифры, «_», «-»)")
		}
		p.PackID = m.PackID
	}
	// целостность
	listed := map[string]string{}
	for _, f := range m.Files {
		listed[f.Path] = f.SHA256
	}
	p.Integrity = "verified"
	if len(m.Files) == 0 {
		p.Integrity = "none"
		p.Warnings = append(p.Warnings, Issue{"integrity", "manifest.json", "список контрольных сумм SHA-256 пуст: целостность не подтверждена"})
	} else {
		for pth, want := range listed {
			b, ok := files[pth]
			switch {
			case !ok:
				bad("integrity", pth, "файл указан в манифесте, но отсутствует в архиве")
				p.Integrity = "failed"
			case sum(b) != want:
				bad("integrity", pth, "контрольная сумма SHA-256 не совпадает: файл изменён или повреждён")
				p.Integrity = "failed"
			}
		}
		for pth := range files {
			if _, ok := listed[pth]; !ok && pth != "manifest.json" {
				p.Warnings = append(p.Warnings, Issue{"integrity", pth, "файл не указан в манифесте, целостность не проверена"})
				if p.Integrity == "verified" {
					p.Integrity = "partial"
				}
			}
		}
	}
	// корабли
	patched := map[string]world.ShipSpec{}
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if !strings.HasPrefix(n, "ships/") {
			continue
		}
		mm := shipFileRe.FindStringSubmatch(n)
		if mm == nil || path.Ext(n) != ".json" {
			bad("ship_file", n, "файл корабля должен называться ships/ИДЕНТИФИКАТОР.json (например ships/FTR-001.json)")
			continue
		}
		b := files[n]
		if msgs := validateShipJSON(b); len(msgs) > 0 {
			for _, x := range msgs {
				bad("schema", n, "%s", x)
			}
			continue
		}
		var sp world.ShipSpec
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&sp); err != nil {
			bad("json", n, "%v", err)
			continue
		}
		if sp.ID != mm[1] {
			bad("ship_id", n, "идентификатор в файле (%s) не совпадает с именем файла", sp.ID)
			continue
		}
		for _, e := range ValidateShip(sp) {
			bad("range", n, "%s", e)
		}
		patched[sp.ID] = sp
	}
	if len(patched) == 0 && len(p.Errors) == 0 {
		bad("empty", "", "в патче нет ни одного корабля (ships/*.json)")
	}
	for _, id := range m.Ships {
		if _, ok := patched[id]; !ok && len(p.Errors) == 0 {
			bad("manifest_ships", "ships/"+id+".json", "корабль указан в манифесте, но файла нет")
		}
	}
	if len(p.Errors) > 0 {
		s.putPreview(p)
		return p, nil
	}
	// слияние и проверка каталога целиком
	base, ver, err := s.Active(ctx)
	if err != nil {
		return nil, err
	}
	p.BaseVersion = ver
	merged := base
	existing := map[string]world.ShipSpec{}
	for _, sp := range base.Ships {
		existing[sp.ID] = sp
	}
	for _, id := range sortedKeys(patched) {
		old, ex := existing[id]
		p.Changes = append(p.Changes, diffShips(old, patched[id], ex))
		merged = replaceShip(merged, patched[id])
	}
	if _, err := world.CatalogFromDoc(merged); err != nil {
		bad("catalog", "", "каталог после патча не проходит проверку: %v", err)
		s.putPreview(p)
		return p, nil
	}
	changed := 0
	for _, c := range p.Changes {
		if c.Kind != "unchanged" {
			changed++
		}
	}
	if changed == 0 {
		p.Warnings = append(p.Warnings, Issue{"no_changes", "", "патч не меняет ни одного параметра"})
	}
	p.doc = merged
	p.Status = "ready"
	s.putPreview(p)
	return p, nil
}

// PatchPrompt — промпт для ChatGPT (и README внутри ZIP): как менять параметры кораблей в ZIP-патче.
func PatchPrompt() string {
	var r strings.Builder
	for _, k := range sortedKeys(shipRanges) {
		r.WriteString(fmt.Sprintf("- %s: от %v до %v\n", k, shipRanges[k].lo, shipRanges[k].hi))
	}
	var a strings.Builder
	for _, k := range sortedKeys(abilityRanges) {
		a.WriteString(fmt.Sprintf("- %s: от %v до %v\n", k, abilityRanges[k].lo, abilityRanges[k].hi))
	}
	return fmt.Sprintf(`# Мастер-промпт: изменение параметров кораблей «Звёздных рубежей»

Версия формата ZIP: **%s**, schema_version **%d**.

## Задача
Я прикладываю ZIP с паспортами кораблей (ships/*.json). Измени параметры так, как я попрошу (баланс, скорость, урон, трюм и т. п.), и верни новый ZIP. Игра работает без языковых моделей: пакет — это готовые числа, которые проверяет и применяет сервер.

## Содержимое пакета
- manifest.json: паспорт пакета (schema, schema_version, pack_id, created_at, ships[], files[{path, sha256}]).
- schemas/ship.schema.json: JSON Schema паспорта корабля (менять нельзя).
- ships/ИДЕНТИФИКАТОР.json: паспорт одного корабля (например ships/FTR-001.json). Имя файла и поле id обязаны совпадать.
- reports/: твои заметки .md или .txt (что изменено и почему).

## Правила
1. Менять можно только числовые параметры и тексты корабля; поле id не менять; новые поля не добавлять (лишние поля отклоняются).
2. Поля: name, class, role, temporary (временная модель-заглушка), specs, default_spec, skins, dimensions_m, crew, engines, weapons, hull, shield, armor, energy, cargo, speed, shield_regen, shield_regen_delay, energy_regen, abilities.
3. abilities: обязательны pulse (автоатака) и boost (форсаж); специализации (intercept, scan) необязательны. Параметры способностей: name, spec, damage, shots, range, period, energy, cooldown, duration, speed_mul, max_dist, min_dist, damage_mul, radius, reveal.
4. default_spec должна входить в specs. Облик (skins) и специализация корабля не меняют его характеристики.
5. Диапазоны корабля:
%s
Диапазоны способностей:
%s
6. Баланс: щит и корпус задают живучесть, броня снижает урон по корпусу в 1+брони/400 раз; не делай корабль неуязвимым (корпус и щит одновременно у верхней границы) и не ставь перезарядку 0 вместе с большим уроном.
7. В пакет нельзя класть пароли, ключи, исполняемый код, вложенные архивы; допустимы только .json, .md, .txt.

## Как собрать ZIP
Выполни скрипт Python (пересчитает SHA-256 и соберёт архив):

`+"```python"+`
import hashlib, json, os, zipfile, datetime, uuid, glob
root = "pack"          # папка с распакованным пакетом
out = "ships_patch.zip"
mpath = os.path.join(root, "manifest.json")
man = json.load(open(mpath, encoding="utf-8"))
man["pack_id"] = "chatgpt-" + uuid.uuid4().hex[:10]
man["created_by"] = "chatgpt"
man["created_at"] = datetime.datetime.now(datetime.timezone.utc).strftime("%%Y-%%m-%%dT%%H:%%M:%%SZ")
man["ships"] = sorted(os.path.basename(p)[:-5] for p in glob.glob(os.path.join(root, "ships", "*.json")))
files = []
for dp, _, fn in os.walk(root):
    for f in fn:
        full = os.path.join(dp, f)
        rel = os.path.relpath(full, root).replace(os.sep, "/")
        if rel != "manifest.json":
            files.append({"path": rel, "sha256": hashlib.sha256(open(full, "rb").read()).hexdigest()})
man["files"] = sorted(files, key=lambda x: x["path"])
json.dump(man, open(mpath, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
    z.write(mpath, "manifest.json")
    for f in man["files"]:
        z.write(os.path.join(root, f["path"]), f["path"])
print("готово", out, len(man["ships"]), "кораблей")
`+"```"+`

Верни один файл ships_patch.zip. В админке (раздел «Корабли») система проверит файл, покажет, какие параметры изменятся, и применит только после моего подтверждения. Прежнюю версию можно откатить.
`, PatchSchema, PatchSchemaVersion, r.String(), a.String())
}
