package shipcatalog

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func newSvc(t *testing.T) (*Service, *world.World) {
	t.Helper()
	w := world.New()
	s := NewService(NewMemStore(), w.SetCatalog)
	s.now = func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }
	if err := s.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, w
}

func unzip(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = b
	}
	return out
}

// rebuild собирает патч заново с верными суммами (как скрипт из промпта).
func rebuild(t *testing.T, files map[string][]byte, mut func(*manifest)) []byte {
	t.Helper()
	var m manifest
	_ = json.Unmarshal(files["manifest.json"], &m)
	m.Files = []mfile{}
	var ships []string
	for n, b := range files {
		if n != "manifest.json" {
			m.Files = append(m.Files, mfile{n, sum(b)})
		}
		if mm := shipFileRe.FindStringSubmatch(n); mm != nil {
			ships = append(ships, mm[1])
		}
	}
	m.Ships = ships
	if m.Ships == nil {
		m.Ships = []string{}
	}
	if mut != nil {
		mut(&m)
	}
	mb, _ := json.Marshal(m)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write(mb)
	for n, b := range files {
		if n == "manifest.json" {
			continue
		}
		w, _ := zw.Create(n)
		_, _ = w.Write(b)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func modShip(t *testing.T, files map[string][]byte, id string, f func(m map[string]any)) {
	t.Helper()
	var m map[string]any
	_ = json.Unmarshal(files["ships/"+id+".json"], &m)
	f(m)
	files["ships/"+id+".json"], _ = json.Marshal(m)
}

func TestValidateShipRanges(t *testing.T) {
	doc := world.DefaultDoc()
	good := doc.Ships[0]
	if errs := ValidateShip(good); len(errs) > 0 {
		t.Fatalf("встроенный корабль не проходит: %v", errs)
	}
	cases := map[string]func(s *world.ShipSpec){
		"hull":          func(s *world.ShipSpec) { s.Hull = 5 },
		"hull_big":      func(s *world.ShipSpec) { s.Hull = 1e9 },
		"speed":         func(s *world.ShipSpec) { s.Speed = 500 },
		"armor":         func(s *world.ShipSpec) { s.Armor = -1 },
		"name":          func(s *world.ShipSpec) { s.Name = "" },
		"no_pulse":      func(s *world.ShipSpec) { delete(s.Abilities, "pulse") },
		"boost_mul":     func(s *world.ShipSpec) { a := s.Abilities["boost"]; a.SpeedMul = 0.5; s.Abilities["boost"] = a },
		"damage":        func(s *world.ShipSpec) { a := s.Abilities["pulse"]; a.Damage = 1e6; s.Abilities["pulse"] = a },
		"default_spec":  func(s *world.ShipSpec) { s.DefaultSpec = "нет" },
		"no_skins":      func(s *world.ShipSpec) { s.Skins = nil },
		"pulse_period0": func(s *world.ShipSpec) { a := s.Abilities["pulse"]; a.Period = 0; s.Abilities["pulse"] = a },
	}
	for name, f := range cases {
		s := good
		s.Abilities = map[string]world.AbilityCfg{}
		for k, v := range good.Abilities {
			s.Abilities[k] = v
		}
		f(&s)
		if len(ValidateShip(s)) == 0 {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

func TestUpdateShipAppliesLive(t *testing.T) {
	ctx := context.Background()
	s, w := newSvc(t)
	p, _ := w.Join(world.Record{ID: "a", Name: "a"})
	doc, v1, _ := s.Active(ctx)
	if v1 != 1 {
		t.Fatalf("начальная версия %d", v1)
	}
	var sp world.ShipSpec
	for _, x := range doc.Ships {
		if x.ID == "FTR-001" {
			sp = x
		}
	}
	sp.Hull, sp.Speed = 900, 18
	if _, err := s.UpdateShip(ctx, "owner", sp); err != nil {
		t.Fatal(err)
	}
	if p.Hull != 900 || w.Catalog().Ships["FTR-001"].Speed != 18 {
		t.Fatalf("правка не применена в мире: hull=%.0f", p.Hull)
	}
	sp.Hull = 5
	if _, err := s.UpdateShip(ctx, "owner", sp); err == nil {
		t.Fatal("значение вне диапазона должно отклоняться")
	}
	if w.Catalog().Ships["FTR-001"].Hull != 900 {
		t.Fatal("отклонённая правка изменила каталог")
	}
	// откат
	if _, err := s.Rollback(ctx, "owner", 1); err != nil {
		t.Fatal(err)
	}
	if w.Catalog().Ships["FTR-001"].Speed != 14 || p.Hull != 1200-0 && p.Hull > 1200 {
		t.Fatalf("откат: скорость %.1f", w.Catalog().Ships["FTR-001"].Speed)
	}
	if _, err := s.Rollback(ctx, "owner", 3); err == nil {
		t.Fatal("откат на активную версию")
	}
	vs, _ := s.Versions(ctx)
	if len(vs) != 3 || !vs[0].Active || vs[0].Source != "rollback" {
		t.Fatalf("история: %+v", vs)
	}
}

func TestPatchRoundTripPreviewPublishRollback(t *testing.T) {
	ctx := context.Background()
	s, w := newSvc(t)
	data, name, err := s.ExportPatch(ctx, nil)
	if err != nil || !strings.HasSuffix(name, ".zip") {
		t.Fatalf("экспорт: %v", err)
	}
	files := unzip(t, data)
	if len(files) < 6 || !strings.Contains(string(files["README_FOR_CHATGPT.md"]), PatchSchema) || files["schemas/ship.schema.json"] == nil {
		t.Fatalf("состав патча: %d файлов", len(files))
	}
	// без изменений: предупреждение, публикация допустима
	pv, _ := s.PreviewPatch(ctx, "o", "same.zip", data)
	if pv.Status != "ready" || pv.Integrity != "verified" || len(pv.Errors) != 0 {
		t.Fatalf("экспортированный патч должен проходить: %+v", pv.Errors)
	}
	for _, c := range pv.Changes {
		if c.Kind != "unchanged" {
			t.Fatalf("без правок изменений быть не должно: %+v", c)
		}
	}
	// правка: скорость и урон импульса «Стрижа»
	modShip(t, files, "FTR-001", func(m map[string]any) {
		m["speed"] = 16.5
		m["abilities"].(map[string]any)["pulse"].(map[string]any)["damage"] = 30
	})
	patch := rebuild(t, files, func(m *manifest) { m.PackID = "chatgpt-test01"; m.CreatedBy = "chatgpt" })
	pv, _ = s.PreviewPatch(ctx, "o", "p.zip", patch)
	if pv.Status != "ready" || len(pv.Errors) != 0 {
		t.Fatalf("предпросмотр: %+v", pv.Errors)
	}
	var diff []string
	for _, c := range pv.Changes {
		if c.ID == "FTR-001" {
			for _, f := range c.Fields {
				diff = append(diff, f.Path+":"+f.Old+"->"+f.New)
			}
		}
	}
	joined := strings.Join(diff, " ")
	if !strings.Contains(joined, "speed:14->16.5") || !strings.Contains(joined, "abilities.pulse.damage:22->30") || len(diff) != 2 {
		t.Fatalf("различия: %v", diff)
	}
	if w.Catalog().Ships["FTR-001"].Speed != 14 {
		t.Fatal("предпросмотр изменил мир до подтверждения")
	}
	ver, err := s.PublishPreview(ctx, "o", pv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if w.Catalog().Ships["FTR-001"].Speed != 16.5 || w.Catalog().Ships["FTR-001"].Abilities["pulse"].Damage != 30 {
		t.Fatal("патч не применён")
	}
	if _, err := s.PublishPreview(ctx, "o", pv.ID); err == nil {
		t.Fatal("повторная публикация")
	}
	if _, err := s.Rollback(ctx, "o", 1); err != nil || w.Catalog().Ships["FTR-001"].Speed != 14 {
		t.Fatalf("откат патча: %v (версия %d)", err, ver)
	}
}

func TestPatchRejections(t *testing.T) {
	ctx := context.Background()
	s, w := newSvc(t)
	data, _, _ := s.ExportPatch(ctx, []string{"FTR-001"})
	base := unzip(t, data)
	clone := func() map[string][]byte {
		c := map[string][]byte{}
		for k, v := range base {
			c[k] = append([]byte{}, v...)
		}
		return c
	}
	type tc struct {
		name string
		data func() []byte
		code string
	}
	cases := []tc{
		{"hull вне диапазона", func() []byte {
			f := clone()
			modShip(t, f, "FTR-001", func(m map[string]any) { m["hull"] = 5 })
			return rebuild(t, f, nil)
		}, "schema"},
		{"скорость вне диапазона", func() []byte {
			f := clone()
			modShip(t, f, "FTR-001", func(m map[string]any) { m["speed"] = 999 })
			return rebuild(t, f, nil)
		}, "schema"},
		{"лишнее поле", func() []byte {
			f := clone()
			modShip(t, f, "FTR-001", func(m map[string]any) { m["god_mode"] = true })
			return rebuild(t, f, nil)
		}, "schema"},
		{"id не совпадает с файлом", func() []byte {
			f := clone()
			modShip(t, f, "FTR-001", func(m map[string]any) { m["id"] = "FTR-002" })
			return rebuild(t, f, nil)
		}, "ship_id"},
		{"нет способности pulse", func() []byte {
			f := clone()
			modShip(t, f, "FTR-001", func(m map[string]any) { delete(m["abilities"].(map[string]any), "pulse") })
			return rebuild(t, f, nil)
		}, "range"},
		{"подмена файла после сумм", func() []byte {
			f := clone()
			data := rebuild(t, f, nil)
			zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, x := range zr.File {
				rc, _ := x.Open()
				b, _ := io.ReadAll(rc)
				rc.Close()
				if x.Name == "ships/FTR-001.json" {
					b = bytes.Replace(b, []byte(`"hull": 1200`), []byte(`"hull": 9999`), 1)
				}
				w, _ := zw.Create(x.Name)
				_, _ = w.Write(b)
			}
			_ = zw.Close()
			return buf.Bytes()
		}, "integrity"},
		{"ZIP Slip", func() []byte { f := clone(); f["../evil.json"] = []byte("{}"); return rawWith(t, f) }, "unsafe_path"},
		{"исполняемый файл", func() []byte { f := clone(); f["reports/run.exe"] = []byte("MZ.."); return rebuild(t, f, nil) }, "forbidden_type"},
		{"секрет в заметке", func() []byte {
			f := clone()
			f["reports/n.txt"] = []byte("api_key: sk-abcdefghijklmnopqrstuvwxyz123456")
			return rebuild(t, f, nil)
		}, "secret"},
		{"неизвестный каталог", func() []byte { f := clone(); f["other/x.txt"] = []byte("x"); return rebuild(t, f, nil) }, "unexpected_dir"},
		{"не архив", func() []byte { return []byte("мусор") }, "bad_zip"},
		{"пустой патч", func() []byte {
			f := clone()
			delete(f, "ships/FTR-001.json")
			return rebuild(t, f, nil)
		}, "empty"},
		{"неверная версия схемы", func() []byte { return rebuild(t, clone(), func(m *manifest) { m.SchemaVersion = 2 }) }, "schema_version"},
		{"неверное имя файла корабля", func() []byte { f := clone(); f["ships/ftr.json"] = f["ships/FTR-001.json"]; return rebuild(t, f, nil) }, "ship_file"},
	}
	for _, c := range cases {
		pv, err := s.PreviewPatch(ctx, "o", "x.zip", c.data())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		found := false
		for _, e := range pv.Errors {
			found = found || e.Code == c.code
		}
		if pv.Status == "ready" || !found {
			t.Errorf("%s: ожидалась ошибка %s, статус %s, ошибки %+v", c.name, c.code, pv.Status, pv.Errors)
		}
		if _, err := s.PublishPreview(ctx, "o", pv.ID); err == nil {
			t.Errorf("%s: отклонённый патч опубликован", c.name)
		}
	}
	if w.Catalog().Ships["FTR-001"].Hull != 1200 {
		t.Fatal("отклонённые патчи изменили каталог")
	}
}

func rawWith(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, b := range files {
		w, _ := zw.Create(n)
		_, _ = w.Write(b)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestPatchAddsNewShipAndConflict(t *testing.T) {
	ctx := context.Background()
	s, w := newSvc(t)
	data, _, _ := s.ExportPatch(ctx, []string{"TMP-HAUL"})
	files := unzip(t, data)
	var m map[string]any
	_ = json.Unmarshal(files["ships/TMP-HAUL.json"], &m)
	m["id"], m["name"], m["temporary"], m["hull"] = "TMP-NEW", "Новый корабль", true, 2500
	nb, _ := json.Marshal(m)
	files["ships/TMP-NEW.json"] = nb
	patch := rebuild(t, files, nil)
	pv, _ := s.PreviewPatch(ctx, "o", "n.zip", patch)
	if pv.Status != "ready" {
		t.Fatalf("%+v", pv.Errors)
	}
	kinds := map[string]string{}
	for _, c := range pv.Changes {
		kinds[c.ID] = c.Kind
	}
	if kinds["TMP-NEW"] != "new" || kinds["TMP-HAUL"] != "unchanged" {
		t.Fatalf("виды изменений: %v", kinds)
	}
	// пока предпросмотр открыт, каталог изменили другой правкой: публикация отклоняется
	doc, _, _ := s.Active(ctx)
	sp := doc.Ships[0]
	sp.Cargo = 11
	if _, err := s.UpdateShip(ctx, "o", sp); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishPreview(ctx, "o", pv.ID); err != ErrConflict {
		t.Fatalf("ожидался конфликт: %v", err)
	}
	pv, _ = s.PreviewPatch(ctx, "o", "n.zip", patch)
	if _, err := s.PublishPreview(ctx, "o", pv.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.Catalog().Ships["TMP-NEW"]; !ok {
		t.Fatal("новый корабль не появился в каталоге мира")
	}
	// отбрасывание предпросмотра
	pv2, _ := s.PreviewPatch(ctx, "o", "n.zip", patch)
	if err := s.DiscardPreview(pv2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishPreview(ctx, "o", pv2.ID); err == nil {
		t.Fatal("отброшенный патч опубликован")
	}
}

func TestSchemaIsValidAndMatchesRanges(t *testing.T) {
	if _, err := shipSchema(); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(SchemaJSON(), &d); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(world.DefaultDoc().Ships[0])
	if msgs := validateShipJSON(b); len(msgs) > 0 {
		t.Fatalf("встроенный корабль не проходит схему: %v", msgs)
	}
	if !strings.Contains(PatchPrompt(), "hull: от 100 до 100000") {
		t.Fatal("в промпте нет диапазонов")
	}
}

func TestPGStore(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	w := world.New()
	s := NewService(NewPGStore(pool), w.SetCatalog)
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(ctx); err != nil { // повторный запуск не создаёт вторую версию
		t.Fatal(err)
	}
	vs, _ := s.Versions(ctx)
	if len(vs) != 1 || vs[0].Source != "seed" || vs[0].Ships != 12 {
		t.Fatalf("версии: %+v", vs)
	}
	doc, _, _ := s.Active(ctx)
	sp := doc.Ships[0]
	sp.Hull = 1111
	if _, err := s.UpdateShip(ctx, "o", sp); err != nil {
		t.Fatal(err)
	}
	// «перезапуск сервера»: новый мир получает активный каталог из базы
	w2 := world.New()
	s2 := NewService(NewPGStore(pool), w2.SetCatalog)
	if err := s2.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if w2.Catalog().Ships[sp.ID].Hull != 1111 {
		t.Fatal("каталог после перезапуска не сохранился")
	}
	if _, err := s2.Rollback(ctx, "o", 1); err != nil {
		t.Fatal(err)
	}
	var active int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM ship_catalog_versions WHERE is_active`).Scan(&active)
	if active != 1 {
		t.Fatalf("активных версий %d", active)
	}
}
