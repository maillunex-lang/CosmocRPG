package design

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg" // регистрация декодеров для image.DecodeConfig
	_ "image/png"
	"io"
	"math"
	"strings"
)

// Ограничения размера по категориям (байты).
const (
	MaxModelBytes    = 8 << 20
	MaxImageBytes    = 6 << 20
	MaxIconBytes     = 256 << 10
	MaxShipDataBytes = 64 << 10
)

// MaxUploadBytes — наибольший допустимый размер загрузки для категории.
func MaxUploadBytes(cat string) int64 {
	switch cat {
	case CatModel:
		return MaxModelBytes
	case CatBackground:
		return MaxImageBytes
	case CatIcon:
		return MaxIconBytes
	case CatShipData:
		return MaxShipDataBytes
	}
	return 0
}

func extForMime(m string) string {
	switch m {
	case "model/gltf-binary":
		return "glb"
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	case "application/json":
		return "json"
	}
	return "bin"
}

// Validate проверяет файл замены для элемента. Тип определяется по содержимому, а не по имени; имя и расширение должны ему соответствовать.
func Validate(el Element, fileName string, data []byte) (mime string, rep Report) {
	rep = newReport()
	if max := MaxUploadBytes(el.Category); max == 0 {
		rep.errf("неизвестная категория элемента")
		return "", rep.done()
	} else if int64(len(data)) > max {
		rep.errf(fmt.Sprintf("файл слишком большой: %s, предел для этой категории %s", human(int64(len(data))), human(max)))
		return "", rep.done()
	}
	if len(data) == 0 {
		rep.errf("файл пустой")
		return "", rep.done()
	}
	rep.Info["bytes"] = len(data)
	switch el.Category {
	case CatModel:
		mime = "model/gltf-binary"
		validateGLB(el, data, &rep)
	case CatIcon:
		if looksLikeSVG(data) {
			mime = "image/svg+xml"
			validateSVG(data, &rep)
		} else {
			mime = validateRaster(data, &rep, 1024, false)
		}
	case CatBackground:
		mime = validateRaster(data, &rep, 4096, true)
	case CatShipData:
		mime = "application/json"
		validateShipData(el, data, &rep)
	}
	ext := strings.ToLower(fileName)
	if i := strings.LastIndex(ext, "."); i >= 0 {
		ext = ext[i+1:]
	} else {
		ext = ""
	}
	want := extForMime(mime)
	if mime != "" && ext != want && !(want == "jpg" && ext == "jpeg") {
		rep.errf(fmt.Sprintf("расширение файла «.%s» не соответствует содержимому (%s)", ext, want))
	}
	return mime, rep.done()
}

func human(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f МБ", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f КБ", float64(n)/1024)
	}
	return fmt.Sprintf("%d Б", n)
}

// ---------- GLB ----------

const (
	glbMagic     = 0x46546C67
	chunkJSON    = 0x4E4F534A
	chunkBIN     = 0x004E4942
	maxTriangles = 120_000
)

// Расширения glTF, которые Godot не поддерживает при импорте во время работы: такие модели в игре не откроются.
var unsupportedRequired = map[string]string{
	"KHR_draco_mesh_compression": "сжатие Draco не поддерживается: экспортируйте без сжатия",
	"EXT_meshopt_compression":    "сжатие meshopt не поддерживается: экспортируйте без сжатия",
	"KHR_texture_basisu":         "текстуры Basis не поддерживаются: используйте PNG или JPEG",
}

type gltfDoc struct {
	Asset struct {
		Version string `json:"version"`
	} `json:"asset"`
	ExtensionsRequired []string `json:"extensionsRequired"`
	Buffers            []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	BufferViews []struct {
		Buffer     int `json:"buffer"`
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
	} `json:"bufferViews"`
	Accessors []struct {
		Count int       `json:"count"`
		Min   []float64 `json:"min"`
		Max   []float64 `json:"max"`
	} `json:"accessors"`
	Images []struct {
		URI        string `json:"uri"`
		MimeType   string `json:"mimeType"`
		BufferView *int   `json:"bufferView"`
	} `json:"images"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
			Indices    *int           `json:"indices"`
			Mode       *int           `json:"mode"`
		} `json:"primitives"`
	} `json:"meshes"`
	Nodes      []json.RawMessage `json:"nodes"`
	Materials  []json.RawMessage `json:"materials"`
	Textures   []json.RawMessage `json:"textures"`
	Animations []json.RawMessage `json:"animations"`
}

func validateGLB(el Element, data []byte, rep *Report) {
	if len(data) < 20 {
		rep.errf("файл слишком короткий для GLB")
		return
	}
	if binary.LittleEndian.Uint32(data[0:4]) != glbMagic {
		rep.errf("это не GLB: нет заголовка glTF (нужен .glb — бинарный glTF 2.0)")
		return
	}
	if v := binary.LittleEndian.Uint32(data[4:8]); v != 2 {
		rep.errf(fmt.Sprintf("версия glTF %d не поддерживается, нужна 2", v))
		return
	}
	total := int(binary.LittleEndian.Uint32(data[8:12]))
	if total != len(data) {
		rep.errf(fmt.Sprintf("длина в заголовке (%d) не равна размеру файла (%d): файл повреждён", total, len(data)))
		return
	}
	var jsonChunk, binChunk []byte
	pos := 12
	for pos+8 <= len(data) {
		clen := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		ctype := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		pos += 8
		if clen < 0 || pos+clen > len(data) {
			rep.errf("блок GLB выходит за границы файла: файл повреждён")
			return
		}
		switch ctype {
		case chunkJSON:
			if jsonChunk == nil {
				jsonChunk = data[pos : pos+clen]
			}
		case chunkBIN:
			if binChunk == nil {
				binChunk = data[pos : pos+clen]
			}
		}
		pos += clen + (4-clen%4)%4
	}
	if jsonChunk == nil {
		rep.errf("в GLB нет блока JSON")
		return
	}
	var g gltfDoc
	if err := json.Unmarshal(bytes.TrimRight(jsonChunk, " \x00"), &g); err != nil {
		rep.errf("блок JSON в GLB повреждён: " + err.Error())
		return
	}
	if !strings.HasPrefix(g.Asset.Version, "2") {
		rep.errf("версия glTF в описании не 2.x")
	}
	for _, ext := range g.ExtensionsRequired {
		if msg, bad := unsupportedRequired[ext]; bad {
			rep.errf(msg)
		}
	}
	for i, b := range g.Buffers {
		if b.URI != "" && !strings.HasPrefix(b.URI, "data:") {
			rep.errf(fmt.Sprintf("буфер %d ссылается на внешний файл «%s»: всё должно быть внутри GLB", i, clip(b.URI)))
		}
	}
	for i, im := range g.Images {
		if im.URI != "" && !strings.HasPrefix(im.URI, "data:") {
			rep.errf(fmt.Sprintf("текстура %d ссылается на внешний файл «%s»: встройте её в GLB", i, clip(im.URI)))
		}
	}
	prims, tris := 0, 0
	var minP, maxP [3]float64
	have := false
	for _, m := range g.Meshes {
		for _, p := range m.Primitives {
			prims++
			mode := 4
			if p.Mode != nil {
				mode = *p.Mode
			}
			if mode != 4 {
				continue
			}
			n := 0
			if p.Indices != nil && *p.Indices >= 0 && *p.Indices < len(g.Accessors) {
				n = g.Accessors[*p.Indices].Count
			} else if pi, ok := p.Attributes["POSITION"]; ok && pi >= 0 && pi < len(g.Accessors) {
				n = g.Accessors[pi].Count
			}
			tris += n / 3
			if pi, ok := p.Attributes["POSITION"]; ok && pi >= 0 && pi < len(g.Accessors) {
				a := g.Accessors[pi]
				if len(a.Min) == 3 && len(a.Max) == 3 {
					for k := 0; k < 3; k++ {
						if !have || a.Min[k] < minP[k] {
							minP[k] = a.Min[k]
						}
						if !have || a.Max[k] > maxP[k] {
							maxP[k] = a.Max[k]
						}
					}
					have = true
				}
			}
		}
	}
	rep.Info["meshes"], rep.Info["primitives"], rep.Info["triangles"] = len(g.Meshes), prims, tris
	rep.Info["materials"], rep.Info["textures"], rep.Info["nodes"] = len(g.Materials), len(g.Textures), len(g.Nodes)
	rep.Info["animations"] = len(g.Animations)
	if len(g.Meshes) == 0 || prims == 0 {
		rep.errf("в модели нет геометрии (мешей)")
	}
	if len(g.Meshes) > 64 || prims > 256 || len(g.Nodes) > 512 || len(g.Materials) > 32 || len(g.Textures) > 16 || len(g.Images) > 16 {
		rep.errf("слишком сложная структура: не больше 64 мешей, 256 примитивов, 512 узлов, 32 материалов, 16 текстур")
	}
	if tris > maxTriangles {
		rep.errf(fmt.Sprintf("слишком много треугольников: %d, предел %d (для телефона оптимально 20–40 тысяч)", tris, maxTriangles))
	} else if tris > 60_000 {
		rep.warnf(fmt.Sprintf("много треугольников (%d): на слабых телефонах возможны тормоза", tris))
	}
	if have {
		size := [3]float64{maxP[0] - minP[0], maxP[1] - minP[1], maxP[2] - minP[2]}
		rep.Info["bbox"] = []float64{round3(size[0]), round3(size[1]), round3(size[2])}
		longest := math.Max(size[0], math.Max(size[1], size[2]))
		if longest <= 0 || math.IsNaN(longest) || math.IsInf(longest, 0) {
			rep.errf("размеры модели некорректны")
		} else if el.ModelLength > 0 {
			ratio := longest / el.ModelLength
			if ratio < 0.5 || ratio > 2.0 {
				rep.warnf(fmt.Sprintf("длина модели %.2f заметно отличается от ожидаемой %.2f: в игре её придётся масштабировать (visual_scale)", longest, el.ModelLength))
			}
			axis := strings.TrimPrefix(strings.ToLower(el.NoseAxis), "-")
			idx := map[string]int{"x": 0, "y": 1, "z": 2}[axis]
			if axis != "" && size[idx] < longest*0.9 {
				rep.warnf(fmt.Sprintf("самая длинная ось модели не совпадает с осью носа «%s»: проверьте, что нос смотрит вдоль неё", el.NoseAxis))
			}
		}
	} else {
		rep.warnf("у вершин нет границ (min/max): размер модели не проверен")
	}
	for i, im := range g.Images {
		var raw []byte
		switch {
		case im.BufferView != nil && *im.BufferView >= 0 && *im.BufferView < len(g.BufferViews) && binChunk != nil:
			bv := g.BufferViews[*im.BufferView]
			if bv.ByteOffset < 0 || bv.ByteLength < 0 || bv.ByteOffset+bv.ByteLength > len(binChunk) {
				rep.errf(fmt.Sprintf("текстура %d выходит за границы данных: файл повреждён", i))
				continue
			}
			raw = binChunk[bv.ByteOffset : bv.ByteOffset+bv.ByteLength]
		case strings.HasPrefix(im.URI, "data:"):
			continue // встроенные data-URI проверяются только по общему размеру файла
		default:
			continue
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			rep.warnf(fmt.Sprintf("текстура %d: формат не удалось проверить (%s)", i, im.MimeType))
			continue
		}
		if cfg.Width > 4096 || cfg.Height > 4096 {
			rep.errf(fmt.Sprintf("текстура %d слишком большая: %d×%d, предел 4096", i, cfg.Width, cfg.Height))
		} else if cfg.Width > 2048 || cfg.Height > 2048 {
			rep.warnf(fmt.Sprintf("текстура %d крупная (%d×%d): для телефона достаточно 2048", i, cfg.Width, cfg.Height))
		}
		rep.Info[fmt.Sprintf("texture_%d", i)] = fmt.Sprintf("%s %d×%d", format, cfg.Width, cfg.Height)
	}
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// ---------- растровые изображения ----------

// validateRaster проверяет PNG, JPEG и (если allowWebP) WebP. Возвращает тип содержимого.
func validateRaster(data []byte, rep *Report, maxSide int, allowWebP bool) string {
	var w, h int
	var mime string
	switch {
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		if !allowWebP {
			rep.errf("для этого элемента формат WebP не принимается: загрузите PNG или SVG")
			return ""
		}
		var ok bool
		if w, h, ok = webpSize(data); !ok {
			rep.errf("не удалось прочитать заголовок WebP: файл повреждён")
			return "image/webp"
		}
		mime = "image/webp"
	default:
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			rep.errf("не удалось определить формат изображения: допустимы PNG, JPEG, WebP (фоны) и SVG (иконки)")
			return ""
		}
		w, h = cfg.Width, cfg.Height
		switch format {
		case "png":
			mime = "image/png"
		case "jpeg":
			mime = "image/jpeg"
		}
		if w*h <= 1<<20 { // небольшие изображения декодируем целиком: ловим обрезанные файлы
			if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
				rep.errf("изображение повреждено или обрезано: " + err.Error())
			}
		}
	}
	rep.Info["width"], rep.Info["height"] = w, h
	if w < 1 || h < 1 {
		rep.errf("размер изображения некорректен")
	}
	if w > maxSide || h > maxSide {
		rep.errf(fmt.Sprintf("изображение слишком большое: %d×%d, предел %d по стороне", w, h, maxSide))
	}
	return mime
}

// webpSize читает размеры из заголовка WebP (VP8, VP8L, VP8X).
func webpSize(d []byte) (int, int, bool) {
	if len(d) < 30 {
		return 0, 0, false
	}
	switch string(d[12:16]) {
	case "VP8 ":
		if len(d) < 30 || d[23] != 0x9d || d[24] != 0x01 || d[25] != 0x2a {
			return 0, 0, false
		}
		return int(binary.LittleEndian.Uint16(d[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(d[28:30]) & 0x3fff), true
	case "VP8L":
		if d[20] != 0x2f {
			return 0, 0, false
		}
		b := binary.LittleEndian.Uint32(d[21:25])
		return int(b&0x3fff) + 1, int((b>>14)&0x3fff) + 1, true
	case "VP8X":
		w := int(d[24]) | int(d[25])<<8 | int(d[26])<<16
		h := int(d[27]) | int(d[28])<<8 | int(d[29])<<16
		return w + 1, h + 1, true
	}
	return 0, 0, false
}

// ---------- SVG ----------

func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	return bytes.Contains(bytes.ToLower(head), []byte("<svg")) || bytes.HasPrefix(bytes.TrimSpace(head), []byte("<?xml"))
}

var forbiddenSVGElements = map[string]bool{"script": true, "foreignobject": true, "iframe": true, "object": true, "embed": true, "audio": true, "video": true, "image": true}

func validateSVG(data []byte, rep *Report) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	dec.Entity = nil
	root := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			rep.errf("SVG не разобрался как XML: " + err.Error())
			return
		}
		switch t := tok.(type) {
		case xml.Directive:
			rep.errf("в SVG нельзя объявления DOCTYPE и ENTITY")
			return
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if root == "" {
				root = name
				if name != "svg" {
					rep.errf("корневой элемент должен быть svg")
					return
				}
			}
			if forbiddenSVGElements[name] {
				rep.errf(fmt.Sprintf("в SVG нельзя элемент «%s»", name))
			}
			for _, a := range t.Attr {
				an := strings.ToLower(a.Name.Local)
				v := strings.ToLower(strings.TrimSpace(a.Value))
				if strings.HasPrefix(an, "on") {
					rep.errf(fmt.Sprintf("в SVG нельзя обработчик событий «%s»", an))
				}
				if (an == "href" || an == "src") && !strings.HasPrefix(v, "#") {
					rep.errf("в SVG нельзя внешние ссылки (допустимы только #id)")
				}
				if strings.Contains(v, "javascript:") || strings.Contains(v, "@import") || (strings.Contains(v, "url(") && !strings.Contains(v, "url(#")) {
					rep.errf("в SVG нельзя внешние ресурсы и скрипты в значениях атрибутов")
				}
			}
		}
	}
	if root == "" {
		rep.errf("в файле нет SVG")
	}
}

// ---------- паспорт корабля ----------

func validateShipData(el Element, data []byte, rep *Report) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		rep.errf("JSON некорректен: " + err.Error())
		return
	}
	if id, _ := doc["id"].(string); id != el.Ship {
		rep.errf(fmt.Sprintf("идентификатор корабля в файле «%v» должен быть «%s»", doc["id"], el.Ship))
	}
	skins, ok := doc["skins"].(map[string]any)
	if !ok || len(skins) == 0 {
		rep.errf("в паспорте нет раздела skins (облики)")
	}
	if _, ok := doc["points"].(map[string]any); !ok {
		rep.warnf("нет раздела points: точки двигателей, оружия и стыковки возьмутся по умолчанию")
	}
	for _, key := range []string{"model_length", "nose_axis"} {
		if _, ok := doc[key]; !ok {
			rep.warnf("нет поля «" + key + "»: будет принято значение по умолчанию")
		}
	}
	rep.Info["skins"] = len(skins)
}
