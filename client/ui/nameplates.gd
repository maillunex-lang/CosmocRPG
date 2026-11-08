extends Control
## Подписи объектов в космосе как в MMO: экранный слой, а не 3D-текст. Подписываются только важные объекты (цель, свой корабль,
## ближайшие враги и игроки, станции, находки рядом), не больше MAX_PLATES; подписи **не пересекаются** (сдвигаются по вертикали,
## при нехватке места лишние скрываются, цель и свой корабль показываются всегда); дальние бледнее. Чистая раскладка вынесена в
## статическую функцию place() и проверяется тестом.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")

const MAX_PLATES := 7
const PADDING := 3.0
const MAX_NAME_CHARS := 22

var _labels: Dictionary = {}   # id -> Label (пул по идентификатору)
var _font := UiTheme.make_font(600)


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_IGNORE


static func short_name(n: String) -> String:
	return n if n.length() <= MAX_NAME_CHARS else n.left(MAX_NAME_CHARS - 1) + "…"


## Раскладка подписей. items: [{id, anchor: Vector2 (низ-центр подписи на экране), size: Vector2, priority: float, force: bool}].
## Возвращает {id: Rect2} только для показанных подписей. Порядок по приоритету; конфликт решается сдвигом вверх и вниз.
static func place(items: Array, area: Rect2, max_plates: int = MAX_PLATES) -> Dictionary:
	var sorted := items.duplicate()
	sorted.sort_custom(func(a: Dictionary, b: Dictionary) -> bool:
		if float(a["priority"]) == float(b["priority"]):
			return str(a["id"]) < str(b["id"])   # детерминированность при равных приоритетах
		return float(a["priority"]) > float(b["priority"]))
	var out := {}
	var rects: Array = []
	for it in sorted:
		var forced := bool(it.get("force", false))
		if out.size() >= max_plates and not forced:
			continue
		var size: Vector2 = it["size"]
		var anchor: Vector2 = it["anchor"]
		var base := Rect2(Vector2(anchor.x - size.x * 0.5, anchor.y - size.y), size)
		var chosen := Rect2()
		var found := false
		for k in 9:
			var step := (int((k + 1) / 2)) * (size.y + PADDING) * (-1.0 if k % 2 == 1 else 1.0)
			var cand := _clamp(Rect2(base.position + Vector2(0, step), size), area)
			var clash := false
			for r in rects:
				if cand.grow(PADDING * 0.5).intersects(r):
					clash = true
					break
			if not clash:
				chosen = cand
				found = true
				break
		if not found:
			if not forced:
				continue
			chosen = _clamp(base, area)   # цель и свой корабль подписаны всегда, даже внахлёст
		out[str(it["id"])] = chosen
		rects.append(chosen.grow(PADDING * 0.5))
	return out


static func _clamp(r: Rect2, area: Rect2) -> Rect2:
	var p := r.position
	p.x = clampf(p.x, area.position.x, maxf(area.position.x, area.end.x - r.size.x))
	p.y = clampf(p.y, area.position.y, maxf(area.position.y, area.end.y - r.size.y))
	return Rect2(p, r.size)


func _plate(id: String) -> Label:
	var l: Label = _labels.get(id)
	if l == null:
		l = Label.new()
		l.mouse_filter = Control.MOUSE_FILTER_IGNORE
		l.add_theme_font_override("font", _font)
		l.add_theme_color_override("font_outline_color", Color(Tokens.BG, 0.9))
		l.add_theme_constant_override("outline_size", 7)
		l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		add_child(l)
		_labels[id] = l
	return l


func _measure(text: String, font_size: int) -> Vector2:
	return _font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size) + Vector2(10, 6)


## Обновление подписей. items: [{id, text, anchor, priority, force, alpha, color, big}] (без size: размер считается по тексту).
func update_plates(items: Array, area: Rect2) -> void:
	var prepared: Array = []
	var by_id := {}
	for it in items:
		var fs := 22 if bool(it.get("big", false)) else 17
		var text := short_name(str(it["text"]))
		var d := {"id": str(it["id"]), "anchor": it["anchor"], "size": _measure(text, fs), "priority": float(it["priority"]), "force": bool(it.get("force", false))}
		prepared.append(d)
		by_id[d["id"]] = {"text": text, "fs": fs, "alpha": float(it.get("alpha", 1.0)), "color": it.get("color", Tokens.TEXT)}
	var placed := place(prepared, area)
	for id in _labels.keys():
		if not placed.has(id):
			(_labels[id] as Label).visible = false
	for id in placed:
		var l := _plate(id)
		var info: Dictionary = by_id[id]
		var r: Rect2 = placed[id]
		l.text = info["text"]
		l.add_theme_font_size_override("font_size", info["fs"])
		l.add_theme_color_override("font_color", info["color"])
		l.modulate.a = info["alpha"]
		l.position = r.position
		l.size = r.size
		l.visible = true
	# подписи исчезнувших объектов освобождаются
	var alive := {}
	for it in items:
		alive[str(it["id"])] = true
	for id in _labels.keys():
		if not alive.has(id):
			(_labels[id] as Label).queue_free()
			_labels.erase(id)


func visible_count() -> int:
	var n := 0
	for l in _labels.values():
		if (l as Label).visible:
			n += 1
	return n
