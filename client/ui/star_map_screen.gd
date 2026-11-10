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

const Settings := preload("res://ui/ui_settings.gd")

var _gateway: UiGateway
var _notice: Label
var _jump_btn: ActionButton
var _search: LineEdit
var _fav_btn: ActionButton
var _favorites_only := false
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

	var bar := HBoxContainer.new()
	bar.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
	bar.offset_left = 20
	bar.offset_top = 84
	bar.offset_right = -20
	bar.offset_bottom = 136
	bar.add_theme_constant_override("separation", 10)
	add_child(bar)
	_search = LineEdit.new()
	_search.placeholder_text = "Поиск системы или фракции"
	_search.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_search.text_changed.connect(func(_t: String) -> void: queue_redraw())
	bar.add_child(_search)
	_fav_btn = ActionButton.new()
	_fav_btn.text = "★"
	_fav_btn.tooltip_text = "Только избранные системы"
	_fav_btn.toggle_mode = true
	_fav_btn.custom_minimum_size = Vector2(64, 52)
	_fav_btn.toggled.connect(func(on: bool) -> void: _favorites_only = on; queue_redraw())
	bar.add_child(_fav_btn)

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


## Поиск по имени/тегу/фракции без учёта регистра и пробелов (используется и тестами).
static func matches_system(sys: Dictionary, query: String) -> bool:
	var q := query.strip_edges().to_lower()
	if q == "":
		return true
	return str(sys.get("name", "")).to_lower().contains(q) \
		or str(sys.get("id", "")).to_lower().contains(q) \
		or str(sys.get("faction", "")).to_lower().contains(q)


func _visible_systems() -> Array:
	var out := []
	for sys in _gateway.star_systems():
		var id := str(sys.get("id", ""))
		if not matches_system(sys, _search.text):
			continue
		if _favorites_only and not Settings.get_bool("map_system_" + id, false):
			continue
		out.append(sys)
	return out


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
	# системы-узлы (поиск/избранное фильтруют: несовпавшие — призрачны)
	var visible := _visible_systems()
	var any_visible := false
	for sys in _gateway.star_systems():
		var idv := str(sys.get("id", ""))
		var dimmed := not visible.has(sys)
		if not dimmed:
			any_visible = true
		_draw_system_node(sys, idv == current, idv == _selected, dimmed)
	if not any_visible:
		var font0 := ThemeDB.fallback_font
		draw_string(font0, Vector2(size.x * 0.5 - 260, size.y * 0.5), "Ничего не найдено — измените запрос", HORIZONTAL_ALIGNMENT_LEFT, 520, 20, Tokens.TEXT_MUTED)


func _draw_system_node(sys: Dictionary, here: bool, sel: bool, dimmed: bool) -> void:
	var id := str(sys.get("id", ""))
	var p := _system_pos(id)
	var faction := str(sys.get("faction", ""))
	var col: Color = FACTION_COLORS.get(faction, Tokens.TEXT_MUTED)
	var fade := 0.25 if dimmed else 1.0
	# свечение
	draw_circle(p, 30, Color(col, (0.10 if not here else 0.22) * fade))
	draw_circle(p, 18, Color(Tokens.BG, 0.85 * fade))
	draw_circle(p, 18, Color(col, 0.9 * fade))
	draw_arc(p, 23, 0, TAU, 40, Color(col, 0.5 * fade), 2.0, true)
	if here:
		draw_arc(p, 30, 0, TAU, 48, Tokens.ACCENT, 2.5, true)
	if sel and not dimmed:
		draw_arc(p, 37, 0, TAU, 48, Color.WHITE, 2.0, true)
	# имена
	var font := ThemeDB.fallback_font
	var name_col := Color(Tokens.TEXT_MUTED if dimmed else Tokens.TEXT)
	draw_string(font, p + Vector2(-90, 46), str(sys.get("name", id)), HORIZONTAL_ALIGNMENT_CENTER, 180, 20, name_col)
	draw_string(font, p + Vector2(-90, 66), faction, HORIZONTAL_ALIGNMENT_CENTER, 180, 14, Color(col, 0.85 * fade))
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
	for sys in _visible_systems():
		var id := str(sys.get("id", ""))
		if _system_pos(id).distance_to(pos) <= 46.0:
			_selected = id
			_update_notice()
			queue_redraw()
			return
	_selected = ""
	_update_notice()
	queue_redraw()
