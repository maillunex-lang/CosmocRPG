package design

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const clientAssets = "../../../client/assets/"

func readAsset(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(clientAssets, rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func hasErr(r Report, sub string) bool {
	for _, e := range r.Errors {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// buildGLB собирает минимальный GLB из JSON-описания и (необязательного) бинарного блока.
func buildGLB(js map[string]any, bin []byte) []byte {
	j, _ := json.Marshal(js)
	for len(j)%4 != 0 {
		j = append(j, ' ')
	}
	for len(bin)%4 != 0 {
		bin = append(bin, 0)
	}
	total := 12 + 8 + len(j)
	if len(bin) > 0 {
		total += 8 + len(bin)
	}
	out := make([]byte, 12, total)
	binary.LittleEndian.PutUint32(out[0:], glbMagic)
	binary.LittleEndian.PutUint32(out[4:], 2)
	binary.LittleEndian.PutUint32(out[8:], uint32(total))
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:], uint32(len(j)))
	binary.LittleEndian.PutUint32(hdr[4:], chunkJSON)
	out = append(append(out, hdr...), j...)
	if len(bin) > 0 {
		binary.LittleEndian.PutUint32(hdr[0:], uint32(len(bin)))
		binary.LittleEndian.PutUint32(hdr[4:], chunkBIN)
		out = append(append(out, hdr...), bin...)
	}
	return out
}

func simpleModel(tris int, size [3]float64) map[string]any {
	return map[string]any{
		"asset":     map[string]any{"version": "2.0"},
		"meshes":    []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]any{"POSITION": 0}, "indices": 1}}}},
		"accessors": []any{map[string]any{"count": 100, "min": []float64{0, 0, 0}, "max": []float64{size[0], size[1], size[2]}}, map[string]any{"count": tris * 3}},
		"nodes":     []any{map[string]any{"mesh": 0}},
	}
}

var shipModel = Element{ID: "model:FTR-001:a", Category: CatModel, Ship: "FTR-001", Skin: "a", ModelLength: 1.9, NoseAxis: "-x"}

func TestRealStrizGLBAccepted(t *testing.T) {
	mime, rep := Validate(shipModel, "striz_a.glb", readAsset(t, "ships/ftr001/striz_a.glb"))
	if !rep.OK || mime != "model/gltf-binary" {
		t.Fatalf("встроенная модель «Стрижа» должна проходить проверку: %+v", rep)
	}
	if rep.Info["triangles"].(int) <= 0 {
		t.Fatalf("число треугольников не посчитано: %v", rep.Info)
	}
	t.Logf("Стриж: %v", rep.Info)
}

func TestGLBRejections(t *testing.T) {
	ok := buildGLB(simpleModel(1000, [3]float64{1.9, 0.4, 1.0}), nil)
	if _, rep := Validate(shipModel, "m.glb", ok); !rep.OK {
		t.Fatalf("простая корректная модель отклонена: %+v", rep)
	}
	bad := map[string][]byte{}
	bad["не GLB"] = []byte("это просто текст, а не модель.....")
	truncated := append([]byte{}, ok[:len(ok)-8]...)
	bad["обрезан"] = truncated
	ver1 := append([]byte{}, ok...)
	binary.LittleEndian.PutUint32(ver1[4:], 1)
	bad["версия 1"] = ver1
	ext := simpleModel(10, [3]float64{1, 1, 1})
	ext["buffers"] = []any{map[string]any{"uri": "http://evil.example/a.bin", "byteLength": 10}}
	bad["внешний буфер"] = buildGLB(ext, nil)
	img := simpleModel(10, [3]float64{1, 1, 1})
	img["images"] = []any{map[string]any{"uri": "textures/a.png"}}
	bad["внешняя текстура"] = buildGLB(img, nil)
	dr := simpleModel(10, [3]float64{1, 1, 1})
	dr["extensionsRequired"] = []string{"KHR_draco_mesh_compression"}
	bad["draco"] = buildGLB(dr, nil)
	bad["слишком много треугольников"] = buildGLB(simpleModel(200_000, [3]float64{1, 1, 1}), nil)
	bad["нет геометрии"] = buildGLB(map[string]any{"asset": map[string]any{"version": "2.0"}}, nil)
	ver := simpleModel(10, [3]float64{1, 1, 1})
	ver["asset"] = map[string]any{"version": "1.0"}
	bad["версия в описании"] = buildGLB(ver, nil)
	for name, data := range bad {
		if _, rep := Validate(shipModel, "m.glb", data); rep.OK {
			t.Errorf("не отклонено: %s", name)
		}
	}
	// предупреждения о размере и оси носа не блокируют загрузку
	_, rep := Validate(shipModel, "m.glb", buildGLB(simpleModel(100, [3]float64{40, 1, 1}), nil))
	if !rep.OK || len(rep.Warnings) == 0 {
		t.Fatalf("слишком большая модель: ожидалось предупреждение без ошибки: %+v", rep)
	}
	_, rep = Validate(shipModel, "m.glb", buildGLB(simpleModel(100, [3]float64{0.3, 0.3, 1.9}), nil))
	if !rep.OK || len(rep.Warnings) == 0 {
		t.Fatalf("длинная ось не вдоль носа: ожидалось предупреждение: %+v", rep)
	}
	if _, rep := Validate(shipModel, "m.gltf", ok); rep.OK || !hasErr(rep, "расширение") {
		t.Fatalf("расширение должно соответствовать содержимому: %+v", rep)
	}
	if _, rep := Validate(shipModel, "m.glb", make([]byte, MaxModelBytes+1)); rep.OK || !hasErr(rep, "слишком большой") {
		t.Fatalf("предел размера: %+v", rep)
	}
}

func TestBuiltinIconsAndBackgroundsAccepted(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	icons, bgs := 0, 0
	for _, el := range cat.All() {
		switch el.Category {
		case CatIcon:
			data := readAsset(t, strings.TrimPrefix(el.Builtin, "res://assets/"))
			if _, rep := Validate(el, filepath.Base(el.Builtin), data); !rep.OK {
				t.Errorf("встроенная иконка %s отклонена: %+v", el.ID, rep)
			}
			icons++
		case CatBackground:
			data := readAsset(t, strings.TrimPrefix(el.Builtin, "res://assets/"))
			mime, rep := Validate(el, filepath.Base(el.Builtin), data)
			if !rep.OK || mime != "image/webp" {
				t.Errorf("встроенный фон %s отклонён: %s %+v", el.ID, mime, rep)
			}
			if w, ok := rep.Info["width"].(int); !ok || w < 100 {
				t.Errorf("размеры фона %s не прочитаны: %v", el.ID, rep.Info)
			}
			bgs++
		}
	}
	if icons < 20 || bgs != 3 {
		t.Fatalf("проверено иконок %d, фонов %d", icons, bgs)
	}
}

func TestSVGRejections(t *testing.T) {
	icon := Element{ID: "icon:x", Category: CatIcon}
	good := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0z" fill="none" stroke="currentColor"/></svg>`
	if _, rep := Validate(icon, "x.svg", []byte(good)); !rep.OK {
		t.Fatalf("корректный SVG отклонён: %+v", rep)
	}
	bad := map[string]string{
		"скрипт":         `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"foreignObject":  `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject/></svg>`,
		"onload":         `<svg xmlns="http://www.w3.org/2000/svg" onload="x()"><path d="M0 0"/></svg>`,
		"внешняя ссылка": `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="http://evil.example/a.svg#x"/></svg>`,
		"внешний url":    `<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill:url(http://evil.example/x)"/></svg>`,
		"javascript":     `<svg xmlns="http://www.w3.org/2000/svg"><a href="javascript:alert(1)"/></svg>`,
		"DOCTYPE":        `<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]><svg xmlns="http://www.w3.org/2000/svg"><text>&x;</text></svg>`,
		"image":          `<svg xmlns="http://www.w3.org/2000/svg"><image href="data:image/png;base64,AAAA"/></svg>`,
		"не XML":         `<svg><path`,
		"корень":         `<html><svg/></html>`,
	}
	for name, s := range bad {
		if _, rep := Validate(icon, "x.svg", []byte(s)); rep.OK {
			t.Errorf("не отклонено: %s", name)
		}
	}
}

func TestRasterRejections(t *testing.T) {
	bg := Element{ID: "background:x", Category: CatBackground}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a} // только сигнатура
	if _, rep := Validate(bg, "x.png", png); rep.OK {
		t.Fatal("обрезанный PNG принят")
	}
	if _, rep := Validate(bg, "x.png", []byte("не картинка")); rep.OK {
		t.Fatal("текст принят как изображение")
	}
	icon := Element{ID: "icon:x", Category: CatIcon}
	if _, rep := Validate(icon, "x.webp", readAsset(t, "space/backgrounds/stars_deep.webp")); rep.OK || !hasErr(rep, "WebP") {
		t.Fatalf("иконка в WebP не принимается: %+v", rep)
	}
}

func TestShipDataValidation(t *testing.T) {
	cat, _ := LoadCatalog()
	el, ok := cat.Get("ship_data:FTR-001")
	if !ok {
		t.Fatal("нет паспорта FTR-001 в каталоге")
	}
	if _, rep := Validate(el, "ship.json", []byte(el.Current)); !rep.OK {
		t.Fatalf("текущий паспорт должен проходить проверку: %+v", rep)
	}
	for name, body := range map[string]string{
		"не JSON":       `{`,
		"чужой корабль": `{"id":"FTR-002","skins":{"a":{}}}`,
		"нет обликов":   `{"id":"FTR-001"}`,
		"массив":        `[]`,
	} {
		if _, rep := Validate(el, "ship.json", []byte(body)); rep.OK {
			t.Errorf("не отклонено: %s", name)
		}
	}
}

func TestCatalogShape(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]int{}
	seen := map[string]bool{}
	for _, e := range cat.All() {
		if seen[e.ID] || e.ID == "" || e.Name == "" || e.Group == "" || e.Builtin == "" {
			t.Fatalf("некорректный элемент: %+v", e)
		}
		seen[e.ID] = true
		by[e.Category]++
	}
	if by[CatIcon] != 28 || by[CatModel] != 14 || by[CatShipData] != 12 || by[CatBackground] != 3 {
		t.Fatalf("состав каталога: %v", by)
	}
	if e, _ := cat.Get("model:COR-001:a"); !e.Placeholder || e.Ship != "COR-001" {
		t.Fatalf("модель Барса должна быть заглушкой: %+v", e)
	}
	if e, _ := cat.Get("model:FTR-001:a"); e.Placeholder {
		t.Fatalf("модель Стрижа — не заглушка: %+v", e)
	}
}
