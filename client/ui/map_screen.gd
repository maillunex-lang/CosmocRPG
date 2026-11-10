extends Control
## Карта системы (R-35): границы, зоны, планеты, станции, месторождения, точки, враги, игроки.
## Масштаб: кнопки ±, колесо мыши к курсору, щипок; перетаскивание при увеличении. Кнопка «Я» — к игроку.
## Корабль игрока — пульсирующая метка с курсом; выбранная цель — кольцо, пунктирный курс и дистанция.
## Нажатие на объект выбирает цель; «Автопилот» летит к выбранной цели. Своих расчётов карта не делает.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")
const QuestHints := preload("res://ui/quest_hints.gd")

signal target_selected(id: String)
signal autopilot_requested(id: String)
signal waypoint_requested(pos: Vector2)
signal closed

const ZONE_COLORS := {"pve": Color("#F16C79"), "mining": Color("#F3C46B"), "explore": Color("#9575E8"), "trade": Color("#42C6E5")}
const RESOURCE_COLORS := {"iron_ore": Color("#b08968"), "crystals": Color("#7fe3ff"), "gas": Color("#9be37f"), "scrap": Color("#9aa4ad")}
const PICK_RADIUS := 26.0
const ZOOM_MIN := 1.0
const ZOOM_MAX := 10.0
const PAN_DRAG_PX := 8.0   # меньше — касание, больше — перетаскивание

const Settings := preload("res://ui/ui_settings.gd")
const FILTERS := {"station": "Станции", "deposit": "Добыча", "explore": "Исследование", "hostile": "Враги", "player": "Пилоты", "bodies": "Планеты", "zones": "Зоны"}
var _filters: Dictionary = {}
var _favorite: Button
var _point_stars: Dictionary = {}
var _favorites_signature := ""
var _favorites: OptionButton

var layout: Dictionary = {}
var state: Dictionary = {}
var selected := ""

var _map_rect := Rect2()
var _scale := 1.0
var _center := Vector2.ZERO
var _zoom := 1.0
var _pan := Vector2.ZERO   # смещение центра карты в мировых координатах
var _info: Label
var _coords: Label
var _auto: ActionButton
var _side: ScrollContainer
var _filters_box: GridContainer
var _pickables: Array = []   # [{id, pos(мир), name}]
var _waypoint := Vector2.ZERO
var _has_waypoint := false
var _touches := {}           # index -> позиция (щипок и перетаскивание)
var _pinch_dist := 0.0
var _pinch_zoom := 1.0
var _drag_panning := false
var _drag_start := Vector2.ZERO
var _pan_start := Vector2.ZERO
var _moved := 0.0


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var scroll := ScrollContainer.new()
	scroll.name = "MapSide"
	scroll.set_anchors_and_offsets_preset(Control.PRESET_RIGHT_WIDE)
	scroll.grow_horizontal = Control.GROW_DIRECTION_BEGIN
	scroll.clip_contents = true
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	add_child(scroll)
	_side = scroll
	var pad := MarginContainer.new()
	pad.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	pad.add_theme_constant_override("margin_left", 10)
	pad.add_theme_constant_override("margin_right", 10)
	pad.add_theme_constant_override("margin_top", 8)
	pad.add_theme_constant_override("margin_bottom", 8)
	scroll.add_child(pad)
	var side := VBoxContainer.new()
	side.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	side.add_theme_constant_override("separation", 8)
	pad.add_child(side)
	var title := Label.new()
	title.text = "Карта системы"
	title.theme_type_variation = &"TitleLabel"
	side.add_child(title)
	_coords = Label.new()
	_coords.theme_type_variation = &"MutedLabel"
	_coords.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART   # длинная строка не должна раздвигать панель за край экрана
	_coords.custom_minimum_size = Vector2(0, 44)
	side.add_child(_coords)
	_info = Label.new()
	_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_info.theme_type_variation = &"MutedLabel"
	_info.custom_minimum_size = Vector2(0, 70)
	side.add_child(_info)
	_auto = ActionButton.new()
	_auto.text = "Автопилот к цели"
	_auto.variant = ActionButton.Variant.PRIMARY
	Icons.apply(_auto, "rocket", Tokens.BG, 22)
	_auto.pressed.connect(func() -> void:
		if _has_waypoint and selected == "@point":
			waypoint_requested.emit(_waypoint)
		elif selected != "":
			autopilot_requested.emit(selected))
	side.add_child(_auto)
	_favorite = ActionButton.new()
	_favorite.pressed.connect(_toggle_favorite)
	side.add_child(_favorite)
	_favorites = OptionButton.new()
	_favorites.item_selected.connect(_choose_favorite)
	side.add_child(_favorites)
	var filters := GridContainer.new()
	filters.columns = 1
	filters.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_filters_box = filters
	side.add_child(filters)
	for key in FILTERS:
		var check := CheckBox.new()
		check.text = FILTERS[key]
		check.button_pressed = Settings.get_bool("map_filter_" + key, true)
		check.clip_text = true
		check.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
		check.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		check.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
		_filters[key] = check.button_pressed
		check.toggled.connect(func(on: bool) -> void: set_filter(key, on))
		filters.add_child(check)
	side.add_child(_legend())
	var sp := Control.new()
	sp.size_flags_vertical = Control.SIZE_EXPAND_FILL
	side.add_child(sp)

	# масштаб и возврат к себе
	var zoom_row := HBoxContainer.new()
	zoom_row.add_theme_constant_override("separation", 8)
	side.add_child(zoom_row)
	var zoom_out := ActionButton.new()
	zoom_out.text = "−"
	zoom_out.tooltip_text = "Уменьшить масштаб"
	zoom_out.pressed.connect(func() -> void: zoom_at(1.0 / 1.4, null))
	zoom_row.add_child(zoom_out)
	var me_btn := ActionButton.new()
	me_btn.text = "Я"
	me_btn.tooltip_text = "Показать моё положение (сброс масштаба)"
	Icons.apply(me_btn, "user", Tokens.ACCENT, 20)
	me_btn.pressed.connect(reset_view)
	zoom_row.add_child(me_btn)
	var zoom_in := ActionButton.new()
	zoom_in.text = "+"
	zoom_in.tooltip_text = "Увеличить масштаб"
	zoom_in.pressed.connect(func() -> void: zoom_at(1.4, null))
	zoom_row.add_child(zoom_in)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x", Color.WHITE, 22)
	close.pressed.connect(func() -> void: closed.emit())
	side.add_child(close)
	_fit_panel()
	_refresh_info()
	resized.connect(_fit_panel)


func _legend() -> Control:
	var box := GridContainer.new()
	box.columns = 2
	for it in [["Станция", Tokens.TEXT], ["Планета", Color("#6f8fe8")], ["Добыча", ZONE_COLORS["mining"]], ["Опасно (PvE)", ZONE_COLORS["pve"]],
			["Аномалия ◇", ZONE_COLORS["explore"]], ["Обломки □", Color("#9aa4ad")], ["Сигнал ●", Tokens.ACCENT], ["Исследование", ZONE_COLORS["explore"]], ["Торговля", ZONE_COLORS["trade"]], ["Враг", Tokens.DANGER], ["Игрок", Tokens.ACCENT]]:
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 6)
		var sw := ColorRect.new()
		sw.color = it[1]
		sw.custom_minimum_size = Vector2(14, 14)
		sw.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(sw)
		var l := Label.new()
		l.text = it[0]
		l.clip_text = true
		l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
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


func zoom() -> float:
	return _zoom


## Соответствие мир → экран (север сверху). Вынесено отдельно для тестов.
func to_screen(p: Vector2) -> Vector2:
	return _center + (p + _pan) * _scale


func to_world(s: Vector2) -> Vector2:
	return (s - _center) / _scale - _pan


## Увеличение/уменьшение. anchor — экранная точка, которая останется на месте (null: центр).
func zoom_at(factor: float, anchor: Variant) -> void:
	var a: Vector2 = anchor if anchor is Vector2 else _center
	var world_before := to_world(a)
	_zoom = clampf(_zoom * factor, ZOOM_MIN, ZOOM_MAX)
	_compute_rect()
	# после смены масштаба сдвигаем панораму так, чтобы точка под курсором/центром не съехала
	_pan = (a - _center) / _scale - world_before
	queue_redraw()


## Вернуться к игроку и полному обзору.
func reset_view() -> void:
	_zoom = ZOOM_MIN
	_pan = Vector2.ZERO
	_compute_rect()
	queue_redraw()


func _process(_delta: float) -> void:
	queue_redraw()   # пульс метки игрока


func _panel_width() -> float:
	return clampf(size.x * 0.34, 220.0, 300.0)


func _fit_panel() -> void:
	if _side == null:
		return
	var w := _panel_width()
	_side.offset_left = -w - 8.0
	_side.offset_right = -8.0
	_side.offset_top = 8.0
	_side.offset_bottom = -8.0
	if _filters_box != null:
		_filters_box.columns = 1 if w < 280.0 else 2
	_compute_rect()
	queue_redraw()


func _compute_rect() -> void:
	var reserved := _panel_width() + 28.0
	var avail := Vector2(maxf(size.x - reserved, 80.0), size.y) - Vector2(24, 24)
	var side := maxf(minf(avail.x, avail.y), 80.0)
	_map_rect = Rect2(Vector2(12 + (avail.x - side) * 0.5, 12 + (avail.y - side) * 0.5), Vector2(side, side))
	_center = _map_rect.get_center()
	_scale = side / (2.0 * radius() * 1.04) * _zoom


func _pos(d: Dictionary) -> Vector2:
	var p: Array = d.get("pos", [0, 0])
	return Vector2(float(p[0]), float(p[1]))


func _draw() -> void:
	_compute_rect()
	_pickables.clear()
	draw_rect(Rect2(Vector2.ZERO, size), Tokens.BG)
	var grid_step := _nice_step(70.0 / _scale)
	if grid_step > 0.0:
		var col := Color(Tokens.ACCENT, 0.07)
		var x := -radius()
		while x <= radius():
			draw_line(to_screen(Vector2(x, -radius())), to_screen(Vector2(x, radius())), col, 1.0)
			draw_line(to_screen(Vector2(-radius(), x)), to_screen(Vector2(radius(), x)), col, 1.0)
			x += grid_step
	draw_circle(_center, radius() * _scale, Color(Tokens.SURFACE_SUNKEN, 0.9))
	draw_arc(_center, radius() * _scale, 0, TAU, 128, Color(Tokens.ACCENT, 0.5), 2.0, true)
	var font := UiTheme.make_font(600)
	for z in layout.get("zones", []) if _filters.get("zones", true) else []:
		var c: Color = ZONE_COLORS.get(str(z.get("type", "")), Tokens.TEXT_MUTED)
		var zp := to_screen(_pos(z))
		var zr := float(z.get("radius", 10.0)) * _scale
		draw_circle(zp, zr, Color(c, 0.10))
		draw_arc(zp, zr, 0, TAU, 48, Color(c, 0.55), 1.5, true)
	for b in layout.get("bodies", []) if _filters.get("bodies", true) else []:
		var bp := to_screen(_pos(b))
		var br := maxf(float(b.get("radius", 5.0)) * _scale, 3.0)
		var col2: Array = b.get("color", [0.4, 0.5, 0.8])
		draw_circle(bp, br, Color(float(col2[0]), float(col2[1]), float(col2[2]), 0.9))
		if bool(b.get("ring", false)):
			draw_arc(bp, br * 1.5, 0, TAU, 32, Color(0.85, 0.8, 0.7, 0.6), 1.5, true)
		if str(b.get("kind", "")) == "planet":
			draw_string(font, bp + Vector2(-br, -br - 4), str(b.get("name", "")), HORIZONTAL_ALIGNMENT_LEFT, -1, 12, Color(Tokens.TEXT, 0.7))
	for s in layout.get("stations", []) if _filters.get("station", true) else []:
		var sp := to_screen(_pos(s))
		draw_rect(Rect2(sp - Vector2(5, 5), Vector2(10, 10)), Tokens.TEXT)
		draw_string(font, sp + Vector2(8, 4), str(s.get("name", "")), HORIZONTAL_ALIGNMENT_LEFT, -1, 12, Tokens.TEXT)
		_pickables.append({"id": str(s.get("id", "")), "pos": _pos(s), "name": str(s.get("name", ""))})
		if QuestHints.marked(state, str(s.get("id", "")), "station"):
			_draw_bang(sp)
		elif QuestHints.marker(state) == "station":
			draw_arc(sp, 16, 0, TAU, 24, Color(Tokens.RARE, 0.9), 2.0, true)
	# серверные объекты из состояния: месторождения, точки, враги, игроки
	for o in state.get("objects", []):
		var kind := str(o.get("kind", ""))
		if not object_visible(kind):
			continue
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
			"player", "ai", "convoy":
				draw_circle(sc, 3.5, Tokens.ACCENT)
			"patrol":
				draw_circle(sc, 3.5, Tokens.ACCENT_2)
			_:
				continue
		if kind != "station":
			_pickables.append({"id": str(o.get("id", "")), "pos": p, "name": str(o.get("name", ""))})
		if QuestHints.marker(state) == kind:
			draw_arc(sc, 14, 0, TAU, 24, Color(Tokens.RARE, 0.9), 2.0, true)
		if kind in ["anomaly", "derelict", "signal", "convoy", "patrol"] or str(o.get("id", "")) == selected:
			_draw_name(font, sc + Vector2(8, -8), str(o.get("name", "")))
		if QuestHints.marked(state, str(o.get("id", "")), kind, str(o.get("resource", ""))):
			_draw_bang(sc)
	for g in state.get("gates", []):
		if typeof(g) != TYPE_DICTIONARY:
			continue
		var gp: Array = (g as Dictionary).get("pos", [0, 0])
		var gpos := Vector2(float(gp[0]), float(gp[1]))
		var gs := to_screen(gpos)
		draw_colored_polygon(PackedVector2Array([gs + Vector2(0, -7), gs + Vector2(7, 0), gs + Vector2(0, 7), gs + Vector2(-7, 0)]), Tokens.ACCENT)
		_pickables.append({"id": "gate:" + str((g as Dictionary).get("to", "")), "pos": gpos, "name": "Ворота в " + str((g as Dictionary).get("to", ""))})
		if QuestHints.marked(state, "", "gate", str((g as Dictionary).get("to", ""))):
			_draw_bang(gs)
	var me: Dictionary = state.get("ship", {})
	# курс к цели: пунктир от корабля до выбранного объекта + дистанция посередине
	if selected == "@point" and _has_waypoint and not me.is_empty():
		var a3: Vector2 = to_screen(me["pos"])
		var b3: Vector2 = to_screen(_waypoint)
		draw_dashed_line(a3, b3, Color(Tokens.RARE, 0.8), 2.0, 6.0, true)
		draw_circle(b3, 5.0, Tokens.RARE)
	if selected != "" and selected != "@point" and not me.is_empty():
		for pk in _pickables:
			if pk["id"] == selected:
				var a2: Vector2 = to_screen(me["pos"])
				var b2: Vector2 = to_screen(pk["pos"])
				draw_dashed_line(a2, b2, Color(Tokens.RARE, 0.8), 2.0, 6.0, true)
				var mid := (a2 + b2) * 0.5
				var dist_km := (me["pos"] as Vector2).distance_to(pk["pos"]) / 10.0
				var dist_text := "%.1f км" % dist_km
				var tw := font.get_string_size(dist_text, HORIZONTAL_ALIGNMENT_LEFT, -1, 12).x
				draw_rect(Rect2(mid + Vector2(-tw * 0.5 - 4, -10), Vector2(tw + 8, 16)), Color(Tokens.BG, 0.75))
				draw_string(font, mid + Vector2(-tw * 0.5, 2), dist_text, HORIZONTAL_ALIGNMENT_LEFT, -1, 12, Tokens.RARE)
	if not me.is_empty():
		var mp := to_screen(me["pos"])
		var d: Vector2 = me.get("dir", Vector2(1, 0))
		if d.length() < 0.01:
			d = Vector2(1, 0)
		d = d.normalized()
		var n := Vector2(-d.y, d.x)
		# пульс: расходящееся кольцо, чтобы метку было видно даже в плотной толпе объектов
		var pulse := (Time.get_ticks_msec() % 1400) / 1400.0
		draw_arc(mp, 8.0 + pulse * 14.0, 0, TAU, 32, Color(Tokens.ACCENT, (1.0 - pulse) * 0.5), 2.0, true)
		draw_colored_polygon(PackedVector2Array([mp + d * 10, mp - d * 7 + n * 6, mp - d * 7 - n * 6]), Color.WHITE)
		draw_arc(mp, 12, 0, TAU, 24, Color(Color.WHITE, 0.6), 1.5, true)
	if selected != "":
		for pk in _pickables:
			if pk["id"] == selected:
				var sp2 := to_screen(pk["pos"])
				draw_arc(sp2, 10, 0, TAU, 24, Tokens.RARE, 2.0, true)
	_draw_panel_bg()
	_refresh_info()


func _draw_panel_bg() -> void:
	if _side == null:
		return
	var r := _side.get_rect()
	if r.size.x < 8.0 or r.size.y < 8.0:
		return
	draw_rect(r, Tokens.SURFACE)
	draw_rect(r, Color(Tokens.ACCENT, 0.35), false, 1.0)


func _draw_name(font: Font, at: Vector2, text: String) -> void:
	if text == "":
		return
	draw_string(font, at, text, HORIZONTAL_ALIGNMENT_LEFT, 160, 12, Tokens.TEXT)


func _draw_bang(at: Vector2) -> void:
	var p := at + Vector2(0, -16)
	draw_circle(p, 9.0, Color("#F3C46B"))
	draw_circle(p, 9.0, Color("#081421"), false, 1.5)
	var font := ThemeDB.fallback_font
	draw_string(font, p + Vector2(-3, 5), "!", HORIZONTAL_ALIGNMENT_LEFT, -1, 14, Color("#081421"))


## Шаг сетки координат в мирах: ближайший «красивый» шаг, чтобы сетка была не чаще ~70 px.
func _nice_step(target: float) -> float:
	for st in [1.0, 2.0, 5.0, 10.0, 20.0, 50.0, 100.0, 200.0, 500.0]:
		if st >= target:
			return st
	return 0.0


func _gui_input(ev: InputEvent) -> void:
	if ev is InputEventMouseButton:
		var mb := ev as InputEventMouseButton
		if mb.button_index == MOUSE_BUTTON_WHEEL_UP and mb.pressed:
			zoom_at(1.25, mb.position)
			return
		if mb.button_index == MOUSE_BUTTON_WHEEL_DOWN and mb.pressed:
			zoom_at(1.0 / 1.25, mb.position)
			return
		if mb.button_index == MOUSE_BUTTON_LEFT:
			if mb.pressed:
				_drag_panning = true
				_drag_start = mb.position
				_pan_start = _pan
				_moved = 0.0
			elif _drag_panning:
				_drag_panning = false
				if _moved <= PAN_DRAG_PX:
					pick_at(mb.position)   # короткий клик — выбор
			return
	elif ev is InputEventMouseMotion and _drag_panning:
		var mm := ev as InputEventMouseMotion
		_moved = maxf(_moved, mm.position.distance_to(_drag_start))
		if _zoom > ZOOM_MIN and _moved > PAN_DRAG_PX:
			_pan = _pan_start + (mm.position - _drag_start) / _scale
			queue_redraw()
		return
	elif ev is InputEventScreenTouch:
		var st := ev as InputEventScreenTouch
		if st.pressed:
			_touches[st.index] = st.position
			if _touches.size() == 2:
				var pts := _touches.values()
				_pinch_dist = (pts[0] as Vector2).distance_to(pts[1])
				_pinch_zoom = _zoom
				_drag_panning = false
			elif _touches.size() == 1:
				_drag_panning = true
				_drag_start = st.position
				_pan_start = _pan
				_moved = 0.0
		else:
			_touches.erase(st.index)
			if _touches.size() < 2:
				_pinch_dist = 0.0
			if _touches.is_empty():
				var was_pan := _drag_panning
				_drag_panning = false
				if was_pan and _moved <= PAN_DRAG_PX:
					pick_at(st.position)   # короткий тап — выбор
		return
	elif ev is InputEventScreenDrag:
		var sd := ev as InputEventScreenDrag
		if _touches.has(sd.index):
			_touches[sd.index] = sd.position
		if _touches.size() == 2 and _pinch_dist > 0.0:
			var pts2 := _touches.values()
			var dist := (pts2[0] as Vector2).distance_to(pts2[1])
			zoom_at(_pinch_zoom * dist / _pinch_dist / _zoom, null)
			return
		if _drag_panning and _zoom > ZOOM_MIN:
			_moved = maxf(_moved, sd.position.distance_to(_drag_start))
			if _moved > PAN_DRAG_PX:
				_pan = _pan_start + (sd.position - _drag_start) / _scale
				queue_redraw()
		return


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
		_has_waypoint = false
		select(best)
		return best
	var world := to_world(screen_pos)
	if world.length() <= radius() and _map_rect.has_point(screen_pos):
		_waypoint = world
		_has_waypoint = true
		selected = "@point"
		_refresh_info()
		queue_redraw()
		return "@point"
	return ""


func select(id: String) -> void:
	if id != "@point":
		_has_waypoint = false
	selected = id
	_refresh_info()
	queue_redraw()
	target_selected.emit(id)


func _refresh_info() -> void:
	if _info == null:
		return
	var me: Dictionary = state.get("ship", {})
	if _coords != null:
		if me.is_empty():
			_coords.text = ""
		else:
			var mp: Vector2 = me["pos"]
			_coords.text = "Мои координаты: X %d · Y %d · масштаб ×%.1f" % [int(round(mp.x)), int(round(mp.y)), _zoom]
	var name_ := ""
	var dist_km := -1.0
	for pk in _pickables:
		if pk["id"] == selected:
			name_ = str(pk["name"])
			if not me.is_empty():
				dist_km = (me["pos"] as Vector2).distance_to(pk["pos"]) / 10.0
	if selected == "@point" and _has_waypoint:
		name_ = "Точка маршрута"
		if not me.is_empty():
			dist_km = (me["pos"] as Vector2).distance_to(_waypoint) / 10.0
	_favorite.disabled = name_ == "" or selected == "@point"
	_favorite.text = "★ Убрать точку" if _is_favorite(selected) else "☆ В избранные точки"
	_refresh_favorites()
	if selected == "" or name_ == "":
		_info.text = "Нажмите на объект или пустое место карты, чтобы построить маршрут."
		var hint := QuestHints.prefix(state)
		if hint != "":
			_info.text = hint + "\n" + _info.text
		_auto.disabled = true
	else:
		_info.text = "%s\nРасстояние: %.1f км" % [name_, dist_km] if dist_km >= 0.0 else name_
		_auto.disabled = false


func object_visible(kind: String) -> bool:
	var category := kind
	if kind in ["anomaly", "derelict", "signal"]:
		category = "explore"
	elif kind in ["ai", "convoy", "patrol"]:
		category = "player"
	return bool(_filters.get(category, true))


func set_filter(category: String, enabled: bool) -> void:
	_filters[category] = enabled
	Settings.set_bool("map_filter_" + category, enabled)
	# До следующего кадра скрытые объекты тоже не должны выбираться.
	_pickables.clear()
	_auto.disabled = true
	_refresh_info()
	queue_redraw()


func _favorite_key(id: String) -> String:
	return "map_point_" + str(state.get("system", {}).get("id", layout.get("id", "local"))) + "_" + id


func _toggle_favorite() -> void:
	if selected.is_empty() or _favorite.disabled:
		return
	var key := _favorite_key(selected)
	_point_stars[key] = not _is_favorite(selected)
	Settings.set_bool(key, _point_stars[key])
	_refresh_info()


func _refresh_favorites() -> void:
	var ids: Array = []
	for point in _pickables:
		if _is_favorite(str(point["id"])):
			ids.append(point)
	var signature := ""
	for point in ids:
		signature += str(point["id"]) + ":" + str(point["name"]) + "|"
	if signature == _favorites_signature:
		return
	_favorites_signature = signature
	_favorites.clear()
	_favorites.add_item("Избранные точки (%d)" % ids.size())
	for point in ids:
		_favorites.add_item(str(point["name"]))
		_favorites.set_item_metadata(_favorites.item_count - 1, point)
	_favorites.disabled = ids.is_empty()


func _choose_favorite(index: int) -> void:
	if index <= 0:
		return
	var saved: Dictionary = _favorites.get_item_metadata(index)
	for point in _pickables:
		if point["id"] == saved["id"]:
			_pan = -(point["pos"] as Vector2)
			select(str(point["id"]))
			return


func _is_favorite(id: String) -> bool:
	var key := _favorite_key(id)
	if not _point_stars.has(key):
		_point_stars[key] = Settings.get_bool(key, false)
	return bool(_point_stars[key])
