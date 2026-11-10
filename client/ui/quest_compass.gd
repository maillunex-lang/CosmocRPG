extends Control
## Стрелка к текущему заданию. Рисуется поверх мира и не забирает нажатия.

const Tokens := preload("res://ui/theme/tokens.gd")

var _active := false
var _point := Vector2.ZERO
var _behind := false
var _meters := 0.0
var _caption := ""


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_preset(Control.PRESET_FULL_RECT)


func set_aim(screen_point: Vector2, behind: bool, meters: float, caption: String) -> void:
	_active = true
	_point = screen_point
	_behind = behind
	_meters = meters
	_caption = caption
	queue_redraw()


func clear_aim() -> void:
	if not _active:
		return
	_active = false
	queue_redraw()


func _draw() -> void:
	if not _active or size.x < 10.0:
		return
	var margin := 86.0
	var inner := Rect2(Vector2(margin, margin), size - Vector2(margin * 2.0, margin * 2.0))
	if inner.size.x < 40.0 or inner.size.y < 40.0:
		return
	var font := get_theme_font(&"font", &"Label")
	if font == null:
		font = ThemeDB.fallback_font
	var col := Tokens.RARE
	var on_screen := not _behind and inner.has_point(_point)
	if on_screen:
		_mark(_point, 0.0, col)
		_caption_at(_point + Vector2(16, -6), font, col)
		return
	var center := size * 0.5
	var dir := center - _point if _behind else _point - center
	if dir.length() < 1.0:
		dir = Vector2.UP
	dir = dir.normalized()
	var edge := _ray(center, dir, inner)
	_mark(edge, dir.angle(), col)
	_caption_at(edge + Vector2(0, 22), font, col)


func _mark(at: Vector2, angle: float, col: Color) -> void:
	var tip := at + Vector2(18, 0).rotated(angle)
	var left := at + Vector2(-12, 9).rotated(angle)
	var right := at + Vector2(-12, -9).rotated(angle)
	draw_colored_polygon(PackedVector2Array([tip, left, right]), col)
	draw_arc(at, 20.0, 0.0, TAU, 28, Color(col, 0.45), 2.0, true)


func _caption_at(at: Vector2, font: Font, col: Color) -> void:
	var dist := "%.1f км" % (_meters / 1000.0) if _meters >= 1000.0 else "%d м" % int(round(_meters))
	var text := dist if _caption == "" else "%s  %s" % [_caption, dist]
	var width := font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, 16).x
	var pos := at - Vector2(width * 0.5, 0)
	draw_string(font, pos + Vector2(1, 1), text, HORIZONTAL_ALIGNMENT_LEFT, -1, 16, Color(0, 0, 0, 0.7))
	draw_string(font, pos, text, HORIZONTAL_ALIGNMENT_LEFT, -1, 16, col)


func _ray(origin: Vector2, dir: Vector2, rect: Rect2) -> Vector2:
	var best := 1.0e9
	var hit := origin + dir * 90.0
	if absf(dir.x) > 0.0001:
		for x in [rect.position.x, rect.end.x]:
			var t := (x - origin.x) / dir.x
			if t > 0.0:
				var y := origin.y + dir.y * t
				if y >= rect.position.y and y <= rect.end.y and t < best:
					best = t
					hit = Vector2(x, y)
	if absf(dir.y) > 0.0001:
		for y in [rect.position.y, rect.end.y]:
			var t2 := (y - origin.y) / dir.y
			if t2 > 0.0:
				var x := origin.x + dir.x * t2
				if x >= rect.position.x and x <= rect.end.x and t2 < best:
					best = t2
					hit = Vector2(x, y)
	return hit
