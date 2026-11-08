package aistudio

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---- помощники ----

type zf struct {
	name string
	data []byte
	mode uint32 // 0 — обычный файл
}

// rawZip собирает архив как есть, без проверок (для атак).
func rawZip(t *testing.T, files []zf) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate}
		if f.mode != 0 {
			h.SetMode(0)
			h.ExternalAttrs = f.mode << 16
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(f.data)
	}
	_ = zw.Close()
	return buf.Bytes()
}

// goodPack возвращает файлы корректного пакета на одного капитана и функцию сборки с манифестом.
func goodFiles(t *testing.T) (map[string][]byte, Config) {
	t.Helper()
	cfg := NewConfig()
	Generate(&cfg, GenerateOptions{Seed: 7, Count: 3})
	data, err := BuildPack(PackOptions{PackID: "test-1", Kind: "partial", CreatedBy: "test"}, cfg, []string{"cpt-0001"}, "2026-11-08T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		rc.Close()
		files[f.Name] = b.Bytes()
	}
	return files, cfg
}

// pack собирает архив из файлов, пересчитывая манифест (mut меняет манифест до подсчёта сумм).
func pack(t *testing.T, files map[string][]byte, mut func(*Manifest)) []byte {
	t.Helper()
	var man Manifest
	if err := json.Unmarshal(files["manifest.json"], &man); err != nil {
		t.Fatal(err)
	}
	man.Files = nil
	var names []string
	for n := range files {
		if n != "manifest.json" {
			names = append(names, n)
		}
	}
	for _, n := range names {
		man.Files = append(man.Files, ManifestFile{Path: n, SHA256: sha(files[n])})
	}
	if mut != nil {
		mut(&man)
	}
	mb, _ := json.Marshal(man)
	list := []zf{{name: "manifest.json", data: mb}}
	for _, n := range names {
		list = append(list, zf{name: n, data: files[n]})
	}
	return rawZip(t, list)
}

func hasCode(r Report, code string) bool {
	for _, i := range r.Errors {
		if i.Code == code {
			return true
		}
	}
	return false
}

// ---- безопасность ZIP ----

func TestZipSecurity(t *testing.T) {
	files, _ := goodFiles(t)
	mk := func(extra ...zf) []byte {
		list := []zf{}
		for n, b := range files {
			list = append(list, zf{name: n, data: b})
		}
		return rawZip(t, append(list, extra...))
	}
	zeros := func(n int) []byte { return make([]byte, n) }
	cases := []struct {
		name string
		data []byte
		code string
	}{
		{"ZIP Slip: ../", mk(zf{name: "../evil.json", data: []byte("{}")}), "unsafe_path"},
		{"ZIP Slip: вложенный ..", mk(zf{name: "captains/../../evil.json", data: []byte("{}")}), "unsafe_path"},
		{"абсолютный путь", mk(zf{name: "/etc/passwd.txt", data: []byte("x")}), "unsafe_path"},
		{"обратная косая черта", mk(zf{name: "captains\\x.json", data: []byte("{}")}), "unsafe_path"},
		{"диск Windows", mk(zf{name: "C:/x.txt", data: []byte("x")}), "unsafe_path"},
		{"символическая ссылка", mk(zf{name: "reports/link.txt", data: []byte("/etc/passwd"), mode: 0o120777}), "special_file"},
		{"исполняемый файл .exe", mk(zf{name: "reports/run.exe", data: []byte("MZ....")}), "forbidden_type"},
		{"скрипт .sh", mk(zf{name: "reports/run.sh", data: []byte("echo hi")}), "forbidden_type"},
		{"скрипт .py", mk(zf{name: "reports/run.py", data: []byte("print(1)")}), "forbidden_type"},
		{"ELF под видом .txt", mk(zf{name: "reports/a.txt", data: []byte("\x7fELF\x02\x01")}), "executable"},
		{"вложенный архив под видом .txt", mk(zf{name: "reports/a.txt", data: []byte("PK\x03\x04abc")}), "executable"},
		{"shebang в .md", mk(zf{name: "reports/a.md", data: []byte("#!/bin/sh\necho hi")}), "executable"},
		{"<script> в .md", mk(zf{name: "reports/a.md", data: []byte("текст <script>alert(1)</script>")}), "script"},
		{"неизвестный каталог", mk(zf{name: "evil/a.txt", data: []byte("x")}), "unexpected_dir"},
		{"файл в корне", mk(zf{name: "a.json", data: []byte("{}")}), "unexpected_file"},
		{"скрытый файл", mk(zf{name: "reports/.hidden.txt", data: []byte("x")}), "unsafe_path"},
		{"слишком глубоко", mk(zf{name: "reports/a/b/c.txt", data: []byte("x")}), "unsafe_path"},
		{"ZIP bomb: огромный файл", mk(zf{name: "reports/big.txt", data: zeros(MaxFileBytes + 10)}), "file_too_large"},
		{"ZIP bomb: степень сжатия", mk(zf{name: "reports/bomb.txt", data: zeros(1500000)}), "zip_bomb"},
		{"ключ API в файле", mk(zf{name: "reports/k.txt", data: []byte("мой ключ sk-abcdefghijklmnopqrstuvwxyz123456")}), "secret"},
		{"пароль в файле", mk(zf{name: "reports/p.txt", data: []byte("password: hunter2hunter2")}), "secret"},
		{"приватный ключ", mk(zf{name: "reports/p.txt", data: []byte("-----BEGIN RSA PRIVATE KEY-----")}), "secret"},
		{"не UTF-8", mk(zf{name: "reports/a.txt", data: []byte{0xff, 0xfe, 0xfd}}), "not_text"},
		{"нулевой байт", mk(zf{name: "reports/a.txt", data: []byte("a\x00b")}), "not_text"},
	}
	for _, c := range cases {
		_, rep := ParsePack(c.data)
		if rep.OK || !hasCode(rep, c.code) {
			t.Errorf("%s: ожидалась ошибка %s, отчёт: ok=%v %+v", c.name, c.code, rep.OK, first2(rep.Errors))
		}
	}
	// дубликаты имён
	dup := rawZip(t, []zf{{name: "manifest.json", data: files["manifest.json"]}, {name: "reports/a.txt", data: []byte("1")}, {name: "reports/A.txt", data: []byte("2")}})
	if _, rep := ParsePack(dup); !hasCode(rep, "duplicate") {
		t.Error("дубликаты имён не обнаружены")
	}
	// слишком много файлов
	var many []zf
	for i := 0; i < MaxFiles+5; i++ {
		many = append(many, zf{name: "reports/f" + itoa(i) + ".txt", data: []byte("x")})
	}
	if _, rep := ParsePack(rawZip(t, many)); !hasCode(rep, "too_many_files") {
		t.Error("превышение числа файлов не обнаружено")
	}
	// не ZIP, пустой, слишком большой
	if _, rep := ParsePack([]byte("это не архив")); !hasCode(rep, "bad_zip") {
		t.Error("не-ZIP не отклонён")
	}
	if _, rep := ParsePack(nil); !hasCode(rep, "empty") {
		t.Error("пустой файл не отклонён")
	}
	if _, rep := ParsePack(make([]byte, MaxZipBytes+1)); !hasCode(rep, "too_large") {
		t.Error("слишком большой архив не отклонён")
	}
}

func first2(i []Issue) []Issue {
	if len(i) > 3 {
		return i[:3]
	}
	return i
}

func TestGoodPackParses(t *testing.T) {
	files, _ := goodFiles(t)
	parsed, rep := ParsePack(pack(t, files, nil))
	if !rep.OK || parsed == nil {
		t.Fatalf("корректный пакет отклонён: %+v", rep.Errors)
	}
	if rep.Integrity != "verified" || len(parsed.Partial.Captains) != 1 || rep.Kind != "partial" {
		t.Fatalf("итог: %+v captains=%d", rep, len(parsed.Partial.Captains))
	}
}

func TestIntegrityAndManifest(t *testing.T) {
	files, _ := goodFiles(t)
	// подмена файла после расчёта сумм
	good := pack(t, files, nil)
	zr, _ := zip.NewReader(bytes.NewReader(good), int64(len(good)))
	var list []zf
	for _, f := range zr.File {
		rc, _ := f.Open()
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		rc.Close()
		d := b.Bytes()
		if f.Name == "knowledge/facts.json" {
			d = bytes.Replace(d, []byte("Кси"), []byte("Омега"), 1)
		}
		list = append(list, zf{name: f.Name, data: d})
	}
	if _, rep := ParsePack(rawZip(t, list)); !hasCode(rep, "integrity") || rep.Integrity != "failed" {
		t.Errorf("подмена файла не обнаружена: %s %+v", rep.Integrity, first2(rep.Errors))
	}
	// пустой список сумм: пакет принимается, но целостность не подтверждена
	p, rep := ParsePack(pack(t, files, func(m *Manifest) { m.Files = []ManifestFile{} }))
	if p == nil || !rep.OK || rep.Integrity != "none" || len(rep.Warnings) == 0 {
		t.Errorf("пустой список сумм: ok=%v integrity=%s", rep.OK, rep.Integrity)
	}
	// манифест указывает файл, которого нет
	_, rep = ParsePack(pack(t, files, func(m *Manifest) {
		m.Files = append(m.Files, ManifestFile{Path: "reports/нет.txt", SHA256: strings.Repeat("a", 64)})
	}))
	if !hasCode(rep, "integrity") {
		t.Error("отсутствующий файл из манифеста не обнаружен")
	}
	// список капитанов не совпадает с каталогом
	_, rep = ParsePack(pack(t, files, func(m *Manifest) { m.Captains = []string{"cpt-0002"} }))
	if !hasCode(rep, "manifest_captains") {
		t.Error("расхождение списка капитанов не обнаружено")
	}
	// неподдерживаемая версия схемы
	bad := map[string][]byte{}
	for k, v := range files {
		bad[k] = v
	}
	bad["manifest.json"] = bytes.Replace(files["manifest.json"], []byte(`"schema_version": 1`), []byte(`"schema_version": 2`), 1)
	if _, rep := ParsePack(pack(t, bad, nil)); rep.OK {
		t.Error("схема версии 2 должна отклоняться")
	}
}

func TestSchemaAndForbiddenFields(t *testing.T) {
	files, _ := goodFiles(t)
	mod := func(f func(c map[string]any)) []byte {
		var c map[string]any
		_ = json.Unmarshal(files["captains/cpt-0001.json"], &c)
		f(c)
		b, _ := json.Marshal(c)
		cp := map[string][]byte{}
		for k, v := range files {
			cp[k] = v
		}
		cp["captains/cpt-0001.json"] = b
		return pack(t, cp, nil)
	}
	_ = mod
	cases := []struct {
		name string
		f    func(c map[string]any)
		code string
	}{
		{"короткая биография", func(c map[string]any) { c["bio"] = "коротко" }, "schema"},
		{"неизвестное поле", func(c map[string]any) { c["лишнее"] = 1 }, "schema"},
		{"неверная формальность", func(c map[string]any) { c["speech"].(map[string]any)["formality"] = "cool" }, "schema"},
		{"неверная профессия", func(c map[string]any) { c["profession"] = "волшебник" }, "schema"},
		{"возраст вне диапазона", func(c map[string]any) { c["age"] = 5 }, "schema"},
		{"запрещённое поле password", func(c map[string]any) { c["password"] = "x" }, "forbidden_field"},
		{"запрещённое поле private_messages", func(c map[string]any) { c["private_messages"] = []any{} }, "forbidden_field"},
		{"игровой прогресс", func(c map[string]any) { c["player_progress"] = 5 }, "forbidden_field"},
		{"другой идентификатор в файле", func(c map[string]any) { c["id"] = "cpt-0099" }, "captain_id"},
	}
	for _, c := range cases {
		_, rep := ParsePack(mod(c.f))
		if rep.OK || !hasCode(rep, c.code) {
			t.Errorf("%s: ожидалась ошибка %s: %+v", c.name, c.code, first2(rep.Errors))
		}
	}
}

// ---- сквозной цикл через сервис ----

func newSvc() *Service {
	s := NewService(NewMemStore())
	s.now = func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }
	return s
}

func readPack(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		rc.Close()
		files[f.Name] = b.Bytes()
	}
	return files
}

func TestFullTrainingCycle(t *testing.T) {
	ctx := context.Background()
	s := newSvc()
	if _, ver, err := s.Generate(ctx, "owner", 20261108, 60); err != nil || ver == 0 {
		t.Fatalf("генерация: %v", err)
	}
	// шаблоны выгружаются и сами проходят проверку
	for _, k := range []string{"template", "examples"} {
		data, name, err := s.Export(ctx, "owner", ExportRequest{Kind: k})
		if err != nil || !strings.HasSuffix(name, ".zip") {
			t.Fatalf("экспорт %s: %v", k, err)
		}
		if _, rep := ParsePack(data); !rep.OK || rep.Integrity != "verified" {
			t.Fatalf("шаблон %s не проходит проверку: %+v", k, first2(rep.Errors))
		}
	}
	// партия из 25
	data, name, err := s.Export(ctx, "owner", ExportRequest{Kind: "batch", Size: 25, Index: 2})
	if err != nil || !strings.Contains(name, "batch-25-02-of-03") {
		t.Fatalf("партия: %v %s", err, name)
	}
	files := readPack(t, data)
	if !strings.Contains(string(files["README_FOR_CHATGPT.md"]), "Партия 2 из 3") || !strings.Contains(string(files["README_FOR_CHATGPT.md"]), SchemaName) {
		t.Fatal("в README нет описания партии или версии схемы")
	}
	var man Manifest
	_ = json.Unmarshal(files["manifest.json"], &man)
	if len(man.Captains) != 25 || man.Captains[0] != "cpt-0026" || man.Captains[24] != "cpt-0050" {
		t.Fatalf("состав партии: %v", man.Captains)
	}
	// «ChatGPT» обучает партию: улучшает биографии и добавляет тесты
	for _, id := range man.Captains {
		var c Captain
		_ = json.Unmarshal(files["captains/"+id+".json"], &c)
		c.Bio = c.Bio + " Дополнено при обучении: капитан хорошо запоминает маршруты и людей, с которыми летал."
		c.Revision++
		c.Meta.Source = "chatgpt"
		b, _ := json.MarshalIndent(c, "", "  ")
		files["captains/"+id+".json"] = b
	}
	files["reports/NOTES.md"] = []byte("Партия 2 обработана.")
	trained := pack(t, files, func(m *Manifest) { m.PackID = "chatgpt-test01"; m.CreatedBy = "chatgpt" })

	rec, err := s.Preview(ctx, "owner", "trained.zip", trained, MergeOptions{})
	if err != nil || rec.Status != "validated" || !rec.GateOK {
		t.Fatalf("предпросмотр: %v статус=%s ошибки=%+v", err, rec.Status, first2(rec.Report.Errors))
	}
	if rec.Plan.Changed != 25 || rec.Plan.Added != 0 || rec.Report.Integrity != "verified" {
		t.Fatalf("план: %s, целостность %s", rec.Plan.Summary(), rec.Report.Integrity)
	}
	if cfg, _, _ := s.Active(ctx); strings.Contains(cfg.Captains["cpt-0026"].Bio, "Дополнено при обучении") {
		t.Fatal("предпросмотр изменил конфигурацию до подтверждения")
	}
	// публикация
	ver, err := s.Publish(ctx, "owner", rec.ID, PublishOptions{AcceptWarnings: true})
	if err != nil {
		t.Fatalf("публикация: %v", err)
	}
	cfg, active, _ := s.Active(ctx)
	if active != ver || !strings.Contains(cfg.Captains["cpt-0026"].Bio, "Дополнено при обучении") {
		t.Fatal("версия не применена")
	}
	if cfg.Captains["cpt-0026"].Revision != 2 || cfg.Captains["cpt-0026"].Meta.Source != "chatgpt" {
		t.Fatalf("ревизия/источник: %+v", cfg.Captains["cpt-0026"].Meta)
	}
	if cfg.Captains["cpt-0001"].Revision != 1 || len(cfg.Captains) != 60 {
		t.Fatal("капитаны вне партии изменились")
	}
	// повторная публикация того же импорта запрещена
	if _, err := s.Publish(ctx, "owner", rec.ID, PublishOptions{AcceptWarnings: true}); err == nil {
		t.Fatal("повторная публикация должна отклоняться")
	}
	// откат к версии 1
	rb, err := s.Rollback(ctx, "owner", 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, active, _ = s.Active(ctx)
	if active != rb || strings.Contains(cfg.Captains["cpt-0026"].Bio, "Дополнено при обучении") || cfg.Captains["cpt-0026"].Revision != 1 {
		t.Fatal("откат не восстановил прежнюю конфигурацию")
	}
	vs, _ := s.store.ListVersions(ctx, 10)
	if len(vs) != 3 || !vs[0].Active || vs[0].Source != "rollback" {
		t.Fatalf("история версий: %+v", vs)
	}
	if _, err := s.Rollback(ctx, "owner", rb); err == nil {
		t.Fatal("откат на активную версию должен отклоняться")
	}
	// сессии обучения записаны
	ss, _ := s.store.ListSessions(ctx, 10)
	if len(ss) != 3 {
		t.Fatalf("сессий экспорта %d", len(ss))
	}
}

func TestManualProtectionAndConflict(t *testing.T) {
	ctx := context.Background()
	s := newSvc()
	if _, _, err := s.Generate(ctx, "o", 5, 12); err != nil {
		t.Fatal(err)
	}
	data, _, _ := s.Export(ctx, "o", ExportRequest{Kind: "batch", Size: 10, Index: 1})
	files := readPack(t, data)
	// владелец правит капитана вручную после выгрузки
	cfg, _, _ := s.Active(ctx)
	cp := cfg.Captains["cpt-0003"]
	cp.Speech.Catchphrases = []string{"Моя собственная фраза.", "И ещё одна моя фраза."}
	if _, err := s.EditCaptain(ctx, "o", cp); err != nil {
		t.Fatal(err)
	}
	if c, _, _ := s.Active(ctx); !c.Captains["cpt-0003"].Meta.Manual {
		t.Fatal("правка должна помечать профиль как ручной")
	}
	// импорт старой выгрузки с изменением того же капитана: конфликт, перезапись по умолчанию запрещена
	var c3 Captain
	_ = json.Unmarshal(files["captains/cpt-0003.json"], &c3)
	c3.Bio += " Новая строка биографии от обучения, достаточно длинная для проверки схемы."
	b, _ := json.Marshal(c3)
	files["captains/cpt-0003.json"] = b
	trained := pack(t, files, func(m *Manifest) { m.PackID = "chatgpt-c" })
	rec, err := s.Preview(ctx, "o", "t.zip", trained, MergeOptions{})
	if err != nil || rec.Plan.Conflicts != 1 {
		t.Fatalf("ожидался один конфликт: %v %s", err, rec.Plan.Summary())
	}
	if len(rec.Report.Warnings) == 0 {
		t.Fatal("конфликт должен быть предупреждением")
	}
	if _, err := s.Publish(ctx, "o", rec.ID, PublishOptions{}); err == nil {
		t.Fatal("публикация с предупреждениями требует подтверждения")
	}
	if _, err := s.Publish(ctx, "o", rec.ID, PublishOptions{AcceptWarnings: true}); err != nil {
		t.Fatalf("публикация: %v", err)
	}
	after, _, _ := s.Active(ctx)
	if after.Captains["cpt-0003"].Speech.Catchphrases[0] != "Моя собственная фраза." || strings.Contains(after.Captains["cpt-0003"].Bio, "Новая строка биографии") {
		t.Fatal("ручная правка перезаписана без разрешения")
	}
	// с разрешением перезаписи
	rec2, _ := s.Preview(ctx, "o", "t2.zip", pack(t, files, func(m *Manifest) { m.PackID = "chatgpt-d" }), MergeOptions{OverwriteManual: true})
	if rec2.Plan.Conflicts != 0 || rec2.Plan.Changed != 1 {
		t.Fatalf("с разрешением: %s", rec2.Plan.Summary())
	}
	// оптимистическая блокировка: пока открыт предпросмотр, версия сменилась
	rec3, _ := s.Preview(ctx, "o", "t3.zip", pack(t, files, func(m *Manifest) { m.PackID = "chatgpt-e" }), MergeOptions{OverwriteManual: true})
	if _, err := s.Publish(ctx, "o", rec2.ID, PublishOptions{AcceptWarnings: true, OverwriteManual: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, "o", rec3.ID, PublishOptions{AcceptWarnings: true, OverwriteManual: true}); err != ErrConflict {
		t.Fatalf("ожидался ErrConflict, получено %v", err)
	}
}

func TestRejectedImportsNeverTouchConfig(t *testing.T) {
	ctx := context.Background()
	s := newSvc()
	_, _, _ = s.Generate(ctx, "o", 5, 12)
	_, before, _ := s.Active(ctx)
	files, _ := goodFiles(t)
	// ссылка на неизвестный факт в шаблоне -> проверка ссылочной целостности
	var tpl struct {
		Templates []Template `json:"templates"`
	}
	_ = json.Unmarshal(files["dialogue/templates.json"], &tpl)
	tpl.Templates = append(tpl.Templates, Template{ID: "evil", Intent: "greeting", Text: "Тут {fact:выдуманный_факт}", Weight: 5})
	files["dialogue/templates.json"], _ = json.Marshal(tpl)
	rec, err := s.Preview(ctx, "o", "bad.zip", pack(t, files, nil), MergeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "rejected" || rec.Report.OK {
		t.Fatalf("пакет с выдуманным фактом должен отклоняться: %s", rec.Status)
	}
	if _, err := s.Publish(ctx, "o", rec.ID, PublishOptions{AcceptWarnings: true}); err == nil {
		t.Fatal("отклонённый пакет нельзя опубликовать")
	}
	// ссылка на несуществующую фракцию
	files2, _ := goodFiles(t)
	var c Captain
	_ = json.Unmarshal(files2["captains/cpt-0001.json"], &c)
	c.Faction = "fac-nonexistent"
	files2["captains/cpt-0001.json"], _ = json.Marshal(c)
	if rec, _ := s.Preview(ctx, "o", "bad2.zip", pack(t, files2, nil), MergeOptions{}); rec.Status != "rejected" {
		t.Fatal("несуществующая фракция должна отклоняться")
	}
	// не архив
	if rec, _ := s.Preview(ctx, "o", "x.zip", []byte("мусор"), MergeOptions{}); rec.Status != "rejected" {
		t.Fatal("мусор должен отклоняться")
	}
	if _, after, _ := s.Active(ctx); after != before {
		t.Fatal("отклонённые пакеты изменили активную версию")
	}
}

func TestEditSectionGates(t *testing.T) {
	ctx := context.Background()
	s := newSvc()
	_, _, _ = s.Generate(ctx, "o", 5, 10)
	cfg, _, _ := s.Active(ctx)
	doc, _ := SectionDoc(cfg, "facts")
	raw, _ := json.Marshal(doc)
	// удаляем используемый факт: ссылки в шаблонах ломаются, правка отклоняется
	var d struct {
		Facts []Fact `json:"facts"`
	}
	_ = json.Unmarshal(raw, &d)
	d.Facts = d.Facts[:1]
	broken, _ := json.Marshal(d)
	if _, err := s.PutSection(ctx, "o", "facts", broken); err == nil {
		t.Fatal("удаление используемых фактов должно отклоняться")
	}
	// корректная правка: дополнительный факт
	d2 := struct {
		Facts []Fact `json:"facts"`
	}{}
	_ = json.Unmarshal(raw, &d2)
	d2.Facts = append(d2.Facts, Fact{ID: "extra_fact", Text: "Тестовый факт для проверки редактора", Tags: []string{"тест"}})
	ok, _ := json.Marshal(d2)
	if _, err := s.PutSection(ctx, "o", "facts", ok); err != nil {
		t.Fatalf("корректная правка отклонена: %v", err)
	}
	if _, err := s.PutSection(ctx, "o", "facts", []byte(`{"facts":[{"id":"BAD ID","text":"x"}]}`)); err == nil {
		t.Fatal("нарушение схемы должно отклоняться")
	}
	if _, err := s.PutSection(ctx, "o", "несуществует", []byte(`{}`)); err == nil {
		t.Fatal("неизвестный раздел")
	}
}

func TestDashboardAndTests(t *testing.T) {
	ctx := context.Background()
	s := newSvc()
	_, _, _ = s.Generate(ctx, "o", 5, 30)
	run, err := s.RunTests(ctx, "o", nil, 10)
	if err != nil || run.Total == 0 || run.Passed != run.Total {
		t.Fatalf("прогон: %v %d/%d", err, run.Passed, run.Total)
	}
	d, err := s.Dashboard(ctx)
	if err != nil || d.Captains != 30 || d.LastTests == nil || len(d.Versions) != 1 || d.Quality["captains"].(int) != 30 {
		t.Fatalf("сводка: %v %+v", err, d)
	}
}

func TestGenerateRerunNoNewVersion(t *testing.T) {
	ctx := context.Background()
	s := newSvc()
	_, v1, _ := s.Generate(ctx, "o", 5, 20)
	res, v2, err := s.Generate(ctx, "o", 5, 20)
	if err != nil || len(res.Added) != 0 || v1 != v2 {
		t.Fatalf("повторная генерация создала версию: %v %v %v", err, v1, v2)
	}
	res, v3, _ := s.Generate(ctx, "o", 5, 40)
	if len(res.Added) != 20 || v3 == v2 {
		t.Fatal("расширение реестра должно создавать версию")
	}
}

func TestPublishedConfigKeepsOtherData(t *testing.T) {
	// публикация меняет только раздел AI: игровые данные в этом пакете не затрагиваются (проверка на PostgreSQL см. pg_test.go)
	cfg := NewConfig()
	Generate(&cfg, GenerateOptions{Seed: 1, Count: 5})
	b, _ := json.Marshal(cfg)
	for _, forbidden := range []string{"credits", "inventory", "password", "token"} {
		if strings.Contains(strings.ToLower(string(b)), "\""+forbidden+"\"") {
			t.Errorf("в конфигурации AI встретилось игровое или секретное поле %q", forbidden)
		}
	}
}
