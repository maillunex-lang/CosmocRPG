extends Control
## Карта системы: границы, зоны, планеты, станции, месторождения, точки исследования, враги, игроки и корабль игрока.
## Север сверху, масштаб подгоняется под экран. Нажатие на объект выбирает цель; «Автопилот» летит к выбранной цели.
## Данные берутся из раскладки сервера (welcome.layout) и текущего состояния; своих расчётов карта не делает.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")

signal target_selected(id: String)
signal autopilot_requested(id: String)
signal closed

const ZONE_COLORS := {"pve": Color("#F16C79"), "mining": Color("#F3C46B"), "explore": Color("#9575E8"), "trade": Color("#42C6E5")}
const RESOURCE_COLORS := {"iron_ore": Color("#b08968"), "crystals": Color("#7fe3ff"), "gas": Color("#9be37f"), "scrap": Color("#9aa4ad")}
const PICK_RADIUS := 26.0

var layout: Dictionary = {}
var state: Dictionary = {}
var selected := ""

var _map_rect := Rect2()
var _scale := 1.0
var _center := Vector2.ZERO
var _info: Label
var _auto: ActionButton
var _pickables: Array = []   # [{id, pos(мир), name}]


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var side := VBoxContainer.new()
	side.add_theme_constant_override("separation", 8)
	side.anchor_left = 1.0
	side.anchor_right = 1.0
	side.anchor_top = 0.0
	side.anchor_bottom = 1.0
	side.offset_left = -300
	side.offset_right = -16
	side.offset_top = 16
	side.offset_bottom = -16
	add_child(side)
	var title := Label.new()
	title.text = "Карта системы"
	title.theme_type_variation = &"TitleLabel"
	side.add_child(title)
	_info = Label.new()
	_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_info.theme_type_variation = &"MutedLabel"
	_info.custom_minimum_size = Vector2(0, 70)
	side.add_child(_info)
	_auto = ActionButton.new()
	_auto.text = "Автопилот к цели"
	_auto.variant = ActionButton.Variant.PRIMARY
	Icons.apply(_auto, "rocket", Tokens.BG, 22)
	_auto.pressed.connect(func() -> void: if selected != "": autopilot_requested.emit(selected))
	side.add_child(_auto)
	side.add_child(_legend())
	var sp := Control.new()
	sp.size_flags_vertical = Control.SIZE_EXPAND_FILL
	side.add_child(sp)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x", Color.WHITE, 22)
	close.pressed.connect(func() -> void: closed.emit())
	side.add_child(close)
	_refresh_info()


func _legend() -> Control:
	var box := GridContainer.new()
	box.columns = 2
	for it in [["Станция", Tokens.TEXT], ["Планета", Color("#6f8fe8")], ["Добыча", ZONE_COLORS["mining"]], ["Опасно (PvE)", ZONE_COLORS["pve"]],
			["Исследование", ZONE_COLORS["explore"]], ["Торговля", ZONE_COLORS["trade"]], ["Враг", Tokens.DANGER], ["Игрок", Tokens.ACCENT]]:
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 6)
		var sw := ColorRect.new()
		sw.color = it[1]
		sw.custom_minimum_size = Vector2(14, 14)
		sw.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(sw)
		var l := Label.new()
		l.text = it[0]
		l.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
		row.add_child(l)
		box.add_child(row)
	return box


func set_data(lay: Dictionary, st: Dictionary) -> void:
	layout = lay
	state = st
	queue_redraw()
	if _info != null:
		_refresh_info()


func radius() -> float:
	return float(layout.get("radius", 400.0))


## Соответствие мир → экран (север сверху). Вынесено отдельно для тестов.
func to_screen(p: Vector2) -> Vector2:
	return _center + p * _scale


func to_world(s: Vector2) -> Vector2:
	return (s - _center) / _scale


func _compute_rect() -> void:
	var avail := Vector2(size.x - 320.0, size.y) - Vector2(32, 32)
	var side := maxf(minf(avail.x, avail.y), 100.0)
	_map_rect = Rect2(Vector2(16 + (avail.x - side) * 0.5, 16 + (avail.y - side) * 0.5), Vector2(side, side))
	_center = _map_rect.get_center()
	_scale = side / (2.0 * radius() * 1.04)


func _pos(d: Dictionary) -> Vector2:
	var p: Array = d.get("pos", [0, 0])
	return Vector2(float(p[0]), float(p[1]))


func _draw() -> void:
	_compute_rect()
	_pickables.clear()
	draw_rect(Rect2(Vector2.ZERO, size), Color(Tokens.BG, 0.96))
	draw_circle(_center, radius() * _scale, Color(Tokens.SURFACE_SUNKEN, 0.9))
	draw_arc(_center, radius() * _scale, 0, TAU, 128, Color(Tokens.ACCENT, 0.5), 2.0, true)
	var font := UiTheme.make_font(600)
	for z in layout.get("zones", []):
		var c: Color = ZONE_COLORS.get(str(z.get("type", "")), Tokens.TEXT_MUTED)
		var zp := to_screen(_pos(z))
		var zr := float(z.get("radius", 10.0)) * _scale
		draw_circle(zp, zr, Color(c, 0.10))
		draw_arc(zp, zr, 0, TAU, 48, Color(c, 0.55), 1.5, true)
	for b in layout.get("bodies", []):
		var bp := to_screen(_pos(b))
		var br := maxf(float(b.get("radius", 5.0)) * _scale, 3.0)
		var col: Array = b.get("color", [0.4, 0.5, 0.8])
		draw_circle(bp, br, Color(float(col[0]), float(col[1]), float(col[2]), 0.9))
		if bool(b.get("ring", false)):
			draw_arc(bp, br * 1.5, 0, TAU, 32, Color(0.85, 0.8, 0.7, 0.6), 1.5, true)
		if str(b.get("kind", "")) == "planet":
			draw_string(font, bp + Vector2(-br, -br - 4), str(b.get("name", "")), HORIZONTAL_ALIGNMENT_LEFT, -1, 12, Color(Tokens.TEXT, 0.7))
	for s in layout.get("stations", []):
		var sp := to_screen(_pos(s))
		draw_rect(Rect2(sp - Vector2(5, 5), Vector2(10, 10)), Tokens.TEXT)
		draw_string(font, sp + Vector2(8, 4), str(s.get("name", "")), HORIZONTAL_ALIGNMENT_LEFT, -1, 12, Tokens.TEXT)
		_pickables.append({"id": str(s.get("id", "")), "pos": _pos(s), "name": str(s.get("name", ""))})
	# серверные объекты из состояния: месторождения, точки, враги, игроки
	var me: Dictionary = state.get("ship", {})
	for o in state.get("objects", []):
		var kind := str(o.get("kind", ""))
		var p: Vector2 = o["pos"]
		var sc := to_screen(p)
		match kind:
			"deposit":
				var rc: Color = RESOURCE_COLORS.get(str(o.get("resource", "")), Color("#b08968"))
				draw_circle(sc, 3.0, rc)
			"anomaly":
				draw_colored_polygon(PackedVector2Array([sc + Vector2(0, -6), sc + Vector2(6, 0), sc + Vector2(0, 6), sc + Vector2(-6, 0)]), ZONE_COLORS["explore"])
			"derelict":
				draw_rect(Rect2(sc - Vector2(4, 4), Vector2(8, 8)), Color("#9aa4ad"), false, 2.0)
			"signal":
				draw_circle(sc, 3.0, Tokens.ACCENT)
			"hostile":
				if bool(o.get("dead", false)):
					continue   # побеждённый враг не показывается и не выбирается
				draw_circle(sc, 3.5, Tokens.DANGER)
			"player", "ai":
				draw_circle(sc, 3.5, Tokens.ACCENT)
			_:
				continue
		if kind != "station":
			_pickables.append({"id": str(o.get("id", "")), "pos": p, "name": str(o.get("name", ""))})
	if not me.is_empty():
		var mp := to_screen(me["pos"])
		var d: Vector2 = me.get("dir", Vector2(1, 0))
		if d.length() < 0.01:
			d = Vector2(1, 0)
		d = d.normalized()
		var n := Vector2(-d.y, d.x)
		draw_colored_polygon(PackedVector2Array([mp + d * 9, mp - d * 6 + n * 5, mp - d * 6 - n * 5]), Color.WHITE)
		draw_arc(mp, 11, 0, TAU, 24, Color(Color.WHITE, 0.5), 1.0, true)
	if selected != "":
		for pk in _pickables:
			if pk["id"] == selected:
				var sp2 := to_screen(pk["pos"])
				draw_arc(sp2, 10, 0, TAU, 24, Tokens.RARE, 2.0, true)


func _gui_input(ev: InputEvent) -> void:
	var pos := Vector2.ZERO
	if ev is InputEventMouseButton and (ev as InputEventMouseButton).pressed:
		pos = (ev as InputEventMouseButton).position
	elif ev is InputEventScreenTouch and (ev as InputEventScreenTouch).pressed:
		pos = (ev as InputEventScreenTouch).position
	else:
		return
	pick_at(pos)


## Выбор объекта по точке экрана. Возвращает идентификатор или "".
func pick_at(screen_pos: Vector2) -> String:
	var best := ""
	var best_d := PICK_RADIUS
	for pk in _pickables:
		var d := to_screen(pk["pos"]).distance_to(screen_pos)
		if d <= best_d:
			best_d = d
			best = str(pk["id"])
	if best != "":
		select(best)
	return best


func select(id: String) -> void:
	selected = id
	_refresh_info()
	queue_redraw()
	target_selected.emit(id)


func _refresh_info() -> void:
	if _info == null:
		return
	var name_ := ""
	var dist_km := -1.0
	var me: Dictionary = state.get("ship", {})
	for pk in _pickables:
		if pk["id"] == selected:
			name_ = str(pk["name"])
			if not me.is_empty():
				dist_km = (me["pos"] as Vector2).distance_to(pk["pos"]) / 10.0
	if name_ == "":
		for s in layout.get("stations", []):
			if str(s.get("id", "")) == selected:
				name_ = str(s.get("name", ""))
				if not me.is_empty():
					dist_km = (me["pos"] as Vector2).distance_to(_pos(s)) / 10.0
	if selected == "" or name_ == "":
		_info.text = "Нажмите на станцию, месторождение, точку исследования или врага."
		_auto.disabled = true
	else:
		_info.text = "%s\nРасстояние: %.1f км" % [name_, dist_km] if dist_km >= 0.0 else name_
		_auto.disabled = false
