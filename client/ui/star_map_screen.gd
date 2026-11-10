extends Control
## Звёздная карта v2 — граф галактики (Kimi, 2026-11-10): системы-узлы, линии гиперкоридоров,
## ворота на активном коридоре, выбор системы тапом. Прыжок отправляет серверу jump {to};
## сервер проверяет близость к воротам (no_gate/no_link — тост ошибки).

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

signal closed

const FACTION_COLORS := {
	"Солярный Конкордат": Color("#ffd24a"),
	"Союз Свободных Систем": Color("#42C6E5"),
	"Доминион Веспер": Color("#F16C79"),
}
# Компоновка графа (доли размера экрана): стабильные позиции систем.
const LAYOUT := {"aurora": Vector2(0.26, 0.44), "meridian": Vector2(0.66, 0.26), "redstorm": Vector2(0.62, 0.66)}

var _gateway: UiGateway
var _notice: Label
var _jump_btn: ActionButton
var _selected := ""       # система, выбранная тапом
var _star_seed := 7

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var head := HBoxContainer.new()
	head.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
	head.offset_left = 20
	head.offset_top = 16
	head.offset_right = -20
	head.offset_bottom = 76
	add_child(head)
	head.add_child(Icons.rect("globe", 32, Tokens.ACCENT))
	var title := Label.new()
	title.text = "Карта галактики"
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x")
	close.custom_minimum_size = Vector2(150, 54)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)

	_notice = Label.new()
	_notice.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE)
	_notice.offset_left = 24
	_notice.offset_right = -24
	_notice.offset_top = -76
	_notice.offset_bottom = -24
	_notice.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_notice.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	add_child(_notice)

	_jump_btn = ActionButton.new()
	_jump_btn.text = "Прыгнуть"
	_jump_btn.variant = ActionButton.Variant.PRIMARY
	Icons.apply(_jump_btn, "rocket", Tokens.BG, 24)
	_jump_btn.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_RIGHT)
	_jump_btn.offset_left = -260
	_jump_btn.offset_top = -92
	_jump_btn.offset_right = -24
	_jump_btn.offset_bottom = -28
	_jump_btn.visible = false
	_jump_btn.pressed.connect(_on_jump_pressed)
	add_child(_jump_btn)
	_rebuild()


func bind(gw: UiGateway) -> void:
	_gateway = gw
	if not gw.system_changed.is_connected(_on_system_changed):
		gw.system_changed.connect(_on_system_changed)
	_rebuild()


func _on_system_changed(_info: Dictionary) -> void:
	_selected = ""
	_rebuild()
	set_process(true)


func _system_pos(sys_id: String) -> Vector2:
	var f: Vector2 = LAYOUT.get(sys_id, Vector2(0.5, 0.5))
	return Vector2(size.x * f.x, size.y * f.y)


func _rebuild() -> void:
	if _jump_btn == null:
		return
	_update_notice()
	queue_redraw()


func _update_notice() -> void:
	if _gateway == null:
		_notice.text = ""
		_jump_btn.visible = false
		return
	var current := _current_id()
	if _selected == "" or _selected == current:
		_notice.text = "Нажмите на систему, чтобы построить маршрут."
		_jump_btn.visible = false
		return
	# есть ли коридор current → selected?
	var gate := _gate_to(_selected)
	if gate.is_empty():
		_notice.text = "Прямого коридора нет: долетите до системы-соседа."
		_jump_btn.visible = false
		return
	var ship: Dictionary = _gateway.snapshot().get("ship", {})
	var pos: Vector2 = ship.get("pos", Vector2.ZERO)
	var gpos := Vector2(float(gate.get("pos", [0, 0])[0]), float(gate.get("pos", [0, 0])[1]))
	var dist := pos.distance_to(gpos)
	_jump_btn.visible = true
	if dist <= 70.0:
		_notice.text = "Коридор в %s открыт. Ворота в %d ед. — можно прыгать." % [_system_name(_selected), int(round(dist))]
		_jump_btn.disabled = false
	else:
		_notice.text = "Курс на %s. Подлетите к воротам (%d ед.)." % [_system_name(_selected), int(round(dist))]
		_jump_btn.disabled = false   # сервер всё равно проверит; отказ даст понятный тост


func _on_jump_pressed() -> void:
	if _gateway != null and _selected != "":
		_gateway.jump(_selected)


func _current_id() -> String:
	return str(_gateway.snapshot().get("system", {}).get("id", "")) if _gateway != null else ""


func _system_name(id: String) -> String:
	for sys in _gateway.star_systems():
		if str(sys.get("id", "")) == id:
			return str(sys.get("name", id))
	return id


func _gate_to(sys_id: String) -> Dictionary:
	for g in _gateway.jump_gates():
		if str(g.get("to", "")) == sys_id:
			return g
	return {}


func _process(_delta: float) -> void:
	# живое обновление дистанции до ворот, пока открыта карта
	if visible and _selected != "" and _gateway != null:
		_update_notice()
		queue_redraw()


func _draw() -> void:
	# Непрозрачный фон: иначе карта рисуется поверх живого космоса и HUD и «сливается» с ними.
	draw_rect(Rect2(Vector2.ZERO, size), Tokens.BG)
	draw_rect(Rect2(0, 0, size.x, 92), Color(0, 0, 0, 0.28))
	draw_rect(Rect2(0, size.y - 108, size.x, 108), Color(0, 0, 0, 0.40))
	if _gateway == null:
		return
	var current := _current_id()
	_draw_stars()
	# коридоры
	for ln in _gateway.gate_links():
		var a := _system_pos(str(ln[0]))
		var b := _system_pos(str(ln[1]))
		var active := str(ln[0]) == current or str(ln[1]) == current
		var col := Color(Tokens.ACCENT, 0.55) if active else Color(Tokens.TEXT_MUTED, 0.22)
		draw_line(a, b, col, 2.5 if active else 1.5, true)
		if active:
			# «энергия» коридора: пунктирные вспышки
			for k in range(1, 6):
				var p := a.lerp(b, k / 6.0)
				draw_circle(p, 2.2, Color(Tokens.ACCENT, 0.5))
	# ворота текущей системы — ромб на коридоре
	for g in _gateway.jump_gates():
		var to_id := str(g.get("to", ""))
		var gp := _system_pos(to_id)
		var cur := _system_pos(current)
		var dir := (gp - cur).normalized()
		var gate_screen := cur + dir * 62.0
		var gp2: Vector2 = Vector2(float(g.get("pos", [0, 0])[0]), float(g.get("pos", [0, 0])[1]))
		var ship: Dictionary = _gateway.snapshot().get("ship", {})
		var sp: Vector2 = ship.get("pos", Vector2.ZERO)
		var near := sp.distance_to(gp2) <= 70.0
		var col := Tokens.RARE if near else Color(Tokens.ACCENT, 0.8)
		var pts := PackedVector2Array([gate_screen + Vector2(0, -9), gate_screen + Vector2(9, 0), gate_screen + Vector2(0, 9), gate_screen + Vector2(-9, 0)])
		draw_colored_polygon(pts, col)
		draw_arc(gate_screen, 13, 0, TAU, 20, Color(col, 0.5), 1.5, true)
	# системы-узлы
	for sys in _gateway.star_systems():
		var id := str(sys.get("id", ""))
		var p := _system_pos(id)
		var here := id == current
		var sel := id == _selected
		var faction := str(sys.get("faction", ""))
		var col: Color = FACTION_COLORS.get(faction, Tokens.TEXT_MUTED)
		# свечение
		draw_circle(p, 30, Color(col, 0.10 if not here else 0.22))
		draw_circle(p, 18, Color(Tokens.BG, 0.85))
		draw_circle(p, 18, Color(col, 0.9))
		draw_arc(p, 23, 0, TAU, 40, Color(col, 0.5), 2.0, true)
		if here:
			draw_arc(p, 30, 0, TAU, 48, Tokens.ACCENT, 2.5, true)
		if sel:
			draw_arc(p, 37, 0, TAU, 48, Color.WHITE, 2.0, true)
		# имена
		var font := ThemeDB.fallback_font
		draw_string(font, p + Vector2(-90, 46), str(sys.get("name", id)), HORIZONTAL_ALIGNMENT_CENTER, 180, 20, Tokens.TEXT)
		draw_string(font, p + Vector2(-90, 66), faction, HORIZONTAL_ALIGNMENT_CENTER, 180, 14, Color(col, 0.85))
		if here:
			draw_string(font, p + Vector2(-90, 86), "вы здесь", HORIZONTAL_ALIGNMENT_CENTER, 180, 13, Color(Tokens.ACCENT, 0.9))


func _draw_stars() -> void:
	var rng := RandomNumberGenerator.new()
	rng.seed = _star_seed
	for i in range(90):
		var x := rng.randf() * size.x
		var y := rng.randf() * size.y
		var r := rng.randf_range(0.6, 1.8)
		var a := rng.randf_range(0.15, 0.6)
		draw_circle(Vector2(x, y), r, Color(1, 1, 1, a))


func _gui_input(ev: InputEvent) -> void:
	var pos := Vector2.ZERO
	if ev is InputEventMouseButton and ev.pressed and ev.button_index == MOUSE_BUTTON_LEFT:
		pos = ev.position
	elif ev is InputEventScreenTouch and ev.pressed:
		pos = ev.position
	else:
		return
	for sys in _gateway.star_systems():
		var id := str(sys.get("id", ""))
		if _system_pos(id).distance_to(pos) <= 46.0:
			_selected = id
			_update_notice()
			queue_redraw()
			return
	_selected = ""
	_update_notice()
	queue_redraw()
