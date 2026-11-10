extends Control
## Ангар (минимальная совместимая часть): 3D-предпросмотр корабля, выбор облика и специализации, характеристики.
## Показывает данные сервера (характеристики приходят в состоянии игрока); выбор сохраняет сервер, в демо он только локальный.

const Tokens := preload("res://ui/theme/tokens.gd")
const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const TouchScroll := preload("res://ui/touch_scroll.gd")

signal skin_selected(skin_id: String)
signal spec_selected(spec_id: String)
signal ship_selected(ship_id: String)
signal engine_selected(equipped: bool, revision: int)
signal equipment_refresh_requested
signal closed

var ship_id := ShipDef.DEFAULT_ID
var skin := "a"
var spec := "interceptor"

var _visual: Node3D
var _turntable: Node3D
var _module_info: Label
var _engine_button: Button
var _engine_reason: Label
var _equipment_refresh: Button
var _passport_info: Label
var _passport: Dictionary = {}
var _max_level := 0
var _stats: Label
var _skin_buttons: Dictionary = {}
var _spec_buttons: Dictionary = {}
var _fleet_box: GridContainer
var _fleet_buttons: Dictionary = {}
var _info: Label
var _data: Dictionary = {}   # характеристики корабля игрока (от сервера или демо)


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP   # всё, что под ангаром, не получает касания
	var bg := ColorRect.new()
	bg.color = Tokens.BG
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var margin := MarginContainer.new()
	margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side_name in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side_name, 16)
	add_child(margin)
	var col := VBoxContainer.new()
	col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	col.size_flags_vertical = Control.SIZE_EXPAND_FILL
	col.add_theme_constant_override("separation", 12)
	margin.add_child(col)

	var head := HBoxContainer.new()
	col.add_child(head)
	var screen_title := Label.new()
	screen_title.text = "Ангар"
	screen_title.theme_type_variation = &"TitleLabel"
	screen_title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(screen_title)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.variant = ActionButton.Variant.PRIMARY
	close.custom_minimum_size = Vector2(150, 54)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)

	var row := HBoxContainer.new()
	row.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_theme_constant_override("separation", 16)
	col.add_child(row)

	# 3D-предпросмотр на всю высоту колонки, а не маленьким чёрным прямоугольником.
	var svc := SubViewportContainer.new()
	svc.stretch = true
	svc.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	svc.size_flags_vertical = Control.SIZE_EXPAND_FILL
	svc.size_flags_stretch_ratio = 1.15
	svc.custom_minimum_size = Vector2(240, 180)
	row.add_child(svc)
	var vp := SubViewport.new()
	vp.own_world_3d = true
	vp.transparent_bg = false
	vp.msaa_3d = Viewport.MSAA_DISABLED
	svc.add_child(vp)
	var env := WorldEnvironment.new()
	var e := Environment.new()
	e.background_mode = Environment.BG_COLOR
	e.background_color = Tokens.BG.darkened(0.2)
	e.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	e.ambient_light_color = Color(0.55, 0.6, 0.75)
	env.environment = e
	vp.add_child(env)
	var light := DirectionalLight3D.new()
	light.rotation_degrees = Vector3(-45, -35, 0)
	vp.add_child(light)
	var cam := Camera3D.new()
	cam.position = Vector3(0, 2.6, 6.0)
	cam.rotation_degrees = Vector3(-18, 0, 0)
	cam.fov = 38.0
	vp.add_child(cam)
	_turntable = Node3D.new()
	vp.add_child(_turntable)
	_visual = ShipVisual.new()
	_visual.ship_id = ship_id
	_turntable.add_child(_visual)
	_visual.build(ship_id, skin)

	# панель справа: заголовок снаружи, остальное в ограниченной прокрутке
	var side := PanelContainer.new()
	side.custom_minimum_size = Vector2(300, 0)
	side.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	side.size_flags_vertical = Control.SIZE_EXPAND_FILL
	side.size_flags_stretch_ratio = 0.9
	side.clip_contents = true
	row.add_child(side)
	var side_scroll := ScrollContainer.new()
	side_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	side_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	side_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	side_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_SHOW_ALWAYS
	side.add_child(side_scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 8)
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	side_scroll.add_child(box)
	var def := ShipDef.get_def(ship_id)
	var title := Label.new()
	title.text = "%s — %s" % [def.get("name", ""), def.get("class", "")]
	if bool(def.get("temporary", false)):
		title.text += " (временная модель)"
	title.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 4)
	title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(title)
	_info = Label.new()
	_info.theme_type_variation = &"MutedLabel"
	_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_info)
	_stats = Label.new()
	_stats.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_stats)
	_module_info = Label.new()
	_module_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_module_info.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	box.add_child(_module_info)

	_engine_button = ActionButton.new()
	_engine_button.pressed.connect(_toggle_engine)
	box.add_child(_engine_button)
	_engine_reason = Label.new()
	_engine_reason.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_engine_reason.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	box.add_child(_engine_reason)
	_equipment_refresh = ActionButton.new()
	_equipment_refresh.text = "Обновить оснащение"
	_equipment_refresh.pressed.connect(func() -> void: equipment_refresh_requested.emit())
	box.add_child(_equipment_refresh)

	_passport_info = Label.new()
	_passport_info.name = "PassportInfo"
	_passport_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_passport_info.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	box.add_child(_passport_info)
	box.add_child(_caption("Корабль (смена у станции)"))
	var fleet_scroll := ScrollContainer.new()
	fleet_scroll.custom_minimum_size = Vector2(0, 112)
	fleet_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	box.add_child(fleet_scroll)
	_fleet_box = GridContainer.new()
	_fleet_box.columns = 2
	_fleet_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	fleet_scroll.add_child(_fleet_box)
	box.add_child(_caption("Облик (характеристики одинаковые)"))
	var skins := HBoxContainer.new()
	box.add_child(skins)
	for k in ShipDef.skin_ids(ship_id):
		var b := ActionButton.new()
		b.text = str(ShipDef.skin(ship_id, k).get("name", k))
		b.toggle_mode = true
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.pressed.connect(_pick_skin.bind(str(k)))
		skins.add_child(b)
		_skin_buttons[str(k)] = b
	box.add_child(_caption("Специализация"))
	var specs := HBoxContainer.new()
	box.add_child(specs)
	for k in (def.get("specs", {}) as Dictionary).keys():
		var b2 := ActionButton.new()
		b2.text = str(def["specs"][k].get("name", k))
		b2.toggle_mode = true
		b2.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b2.pressed.connect(_pick_spec.bind(str(k)))
		specs.add_child(b2)
		_spec_buttons[str(k)] = b2
	var dock := ActionButton.new()
	dock.text = "Показать стыковку"
	dock.pressed.connect(func() -> void:
		_visual.undock()
		_visual.play("Dock"))
	box.add_child(dock)
	# Рамки и подписи внутри прокрутки не должны глотать жест пальца.
	TouchScroll.fix_tree(self)
	_refresh()


func _caption(t: String) -> Label:
	var l := Label.new()
	l.text = t
	l.theme_type_variation = &"MutedLabel"
	return l


## Данные корабля игрока: type, skin, spec, hull_max, shield_max, armor, energy_max, cargo (если есть).
func set_data(ship: Dictionary) -> void:
	_data = ship
	if str(ship.get("skin", "")) != "":
		skin = str(ship["skin"])
		_visual.set_skin(skin)
	if str(ship.get("spec", "")) != "":
		spec = str(ship["spec"])
	_refresh()


## Список кораблей каталога от сервера (сообщение ships); текущий корабль выделен.
func set_fleet(ships: Array) -> void:
	_passport = {}
	_max_level = 0
	for c in _fleet_box.get_children():
		c.queue_free()
	_fleet_buttons.clear()
	for sh in ships:
		if not (sh is Dictionary):
			continue
		if str(sh.get("id", "")) == ship_id:
			_max_level = int(sh.get("max_level", 0))
			if sh.get("rpg_passport") is Dictionary and str(sh["rpg_passport"].get("id", "")) == ship_id:
				_passport = sh["rpg_passport"]
		var b := ActionButton.new()
		b.toggle_mode = true
		var lvl := (" · ур. %d" % int(sh["level"])) if sh.has("level") else ""   # уровень корабля — от сервера
		b.text = "%s%s\n%s%s" % [str(sh.get("name", "")), lvl, str(sh.get("class", "")), " · врем." if bool(sh.get("temporary", false)) else ""]
		b.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.custom_minimum_size = Vector2(0, 52)
		b.set_pressed_no_signal(str(sh.get("id", "")) == ship_id)
		b.pressed.connect(_pick_ship.bind(str(sh.get("id", ""))))
		_fleet_box.add_child(b)
		_fleet_buttons[str(sh.get("id", ""))] = b

	_refresh()

func _pick_ship(id: String) -> void:
	for k in _fleet_buttons:
		(_fleet_buttons[k] as Button).set_pressed_no_signal(k == ship_id)   # выбор подтверждает сервер: пока остаётся прежний корабль
	if id != ship_id:
		ship_selected.emit(id)


func _pick_skin(k: String) -> void:
	skin = k
	_visual.undock()
	_visual.set_skin(k)
	_refresh()
	skin_selected.emit(k)


func _pick_spec(k: String) -> void:
	spec = k
	_refresh()
	spec_selected.emit(k)


func _refresh() -> void:
	var def := ShipDef.get_def(ship_id)
	var base: Dictionary = _passport.get("base", {}) if _passport.get("base") is Dictionary else def
	var d: Array = base.get("dimensions_m", [0, 0, 0])
	_info.text = "Экипаж %d · двигателей %d · %s\nГабариты %s × %s × %s м" % [int(base.get("crew", 1)), int(base.get("engines", 2)), str(base.get("weapons", "")),
		str(d[0]), str(d[1]), str(d[2])]
	if _data.has("hull_max"):
		_stats.text = (("Уровень корабля %d" % int(_data["level"]) + (" из %d" % _max_level if _max_level > 0 else "") + "\n") if _data.has("level") else "") + "Корпус %d · щит %d · броня %d\nЭнергия %d · груз %d\nХарактеристики определяет сервер, значения балансируются." % [
			int(_data.get("hull_max", 0)), int(_data.get("shield_max", 0)), int(_data.get("armor", 0)), int(_data.get("energy_max", 0)), int(_data.get("cargo", 0))]
		var bays: Array = _data.get("bays", [])
		if bays.size() > 0:
			var bits: PackedStringArray = []
			for bay in bays:
				if bay is Dictionary:
					bits.append("%s %d/%d" % [str(bay.get("name", "")), int(round(float(bay.get("used", 0.0)))), int(round(float(bay.get("cap", 0.0))))])
			_stats.text += "\nОтсеки ресурсов: " + " · ".join(bits) + "\nМодули в эти отсеки не кладутся: их ставят в слоты корабля."
	else:
		_stats.text = "Характеристики недоступны: сервер их не прислал."
	_module_info.text = ""
	for module in _data.get("modules", []):
		if module is Dictionary and str(module.get("prototype_id", "")) == "ENG-S-START":
			var installed := bool(module.get("owned", false)) and bool(module.get("equipped", false))
			_module_info.text = "Штатный двигатель: %s\nОткрывает M01 · Форсаж" % ("установлен" if installed else "не установлен")
	_module_info.visible = _module_info.text != ""
	_refresh_engine_controls()
	_passport_info.visible = not _passport.is_empty()
	if not _passport.is_empty():
		var groups := {"weapon": 0, "defensive": 0, "technical": 0, "special": 0}
		for slot in _passport.get("slots", []):
			if slot is Dictionary and groups.has(str(slot.get("group", ""))):
				groups[str(slot["group"])] += 1
		_passport_info.text = "%s\nПлан оснащения (предварительно)\nМасса корпуса %s т · реактор %s МВт\nБюджет оборудования %s т\nСлоты: оружие %d · защита %d · техника %d · спец. %d" % [
			str(_passport.get("description", "")), str(_passport.get("mass_tons", "")), str(_passport.get("reactor_power_mw", "")),
			str(_passport.get("module_mass_budget_tons", "")), groups["weapon"], groups["defensive"], groups["technical"], groups["special"]]
	for k in _skin_buttons:
		(_skin_buttons[k] as Button).set_pressed_no_signal(k == skin)
	for k in _spec_buttons:
		var b := _spec_buttons[k] as Button
		b.set_pressed_no_signal(k == spec)
		b.tooltip_text = str((def.get("specs", {}) as Dictionary).get(k, {}).get("ability", ""))


func _process(delta: float) -> void:
	_turntable.rotation.y += delta * 0.6
	_visual.update_motion(delta, 0.4, 0.0, false)


func _native_engine() -> Dictionary:
	for module in _data.get("modules", []):
		if module is Dictionary and str(module.get("prototype_id", "")) == "ENG-S-START":
			return module
	return {}


func _toggle_engine() -> void:
	var status: Dictionary = _data.get("equipment", {})
	var module := _native_engine()
	if not bool(status.get("can_change", false)) or not status.has("revision") or not bool(module.get("owned", false)):
		return
	# Keep presentation unchanged until an authoritative state arrives.
	engine_selected.emit(not bool(module.get("equipped", false)), int(status["revision"]))


func _refresh_engine_controls() -> void:
	var status: Dictionary = _data.get("equipment", {})
	var module := _native_engine()
	var supported := ship_id == "FTR-001" and not status.is_empty()
	_engine_button.visible = supported
	_engine_reason.visible = supported
	_equipment_refresh.visible = supported and not status.has("revision")
	_engine_button.text = "Снять двигатель" if bool(module.get("equipped", false)) else "Установить двигатель"
	_engine_button.disabled = not bool(status.get("can_change", false)) or not status.has("revision") or not bool(module.get("owned", false))
	var reasons := {
		"not_at_station": "Подлетите к станции для смены оснащения.",
		"equipment_in_combat": "Недоступно в бою и 10 секунд после него.",
		"equipment_busy": "Остановитесь, отключите автопилот и дождитесь окончания форсажа.",
		"equipment_unavailable": "Не удалось подтвердить оснащение. Обновите состояние.",
		"equipment_unsupported_ship": "Смена оснащения пока доступна только Стрижу.",
		"dead": "Корабль уничтожен."
	}
	_engine_reason.text = str(reasons.get(str(status.get("reason", "")), "Смена у станции вне боя. Снятие отключит M01 · Форсаж."))
	if bool(status.get("pending", false)):
		_engine_reason.text = "Сохраняем оснащение…"
