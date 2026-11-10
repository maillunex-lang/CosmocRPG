extends Control
## Ангар на весь экран: коллекция корпусов, фильтр по типу, крупный предпросмотр и слоты модулей.
## Характеристики приходят с сервера. Выбор облика, специализации и корпуса сохраняет сервер.

const Tokens := preload("res://ui/theme/tokens.gd")
const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const TouchScroll := preload("res://ui/touch_scroll.gd")

const FILTERS: Array = [
	["all", "Все"],
	["fighter", "Истребители"],
	["corvette", "Корветы"],
	["frigate", "Фрегаты"],
	["destroyer", "Эсминцы"],
	["cruiser", "Крейсеры"],
	["scout", "Разведка"],
	["support", "Поддержка"],
	["miner", "Добыча"],
	["hauler", "Транспорт"],
	["other", "Прочие"],
]

signal skin_selected(skin_id: String)
signal spec_selected(spec_id: String)
signal ship_selected(ship_id: String)
signal engine_selected(equipped: bool, revision: int)
signal equipment_refresh_requested
signal module_action(op: String, slot_id: String, item_id: String, revision: int, pilot_revision: int)
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
var _fit_box: VBoxContainer
var _fitting: Dictionary = {}
var _passport_info: Label
var _passport: Dictionary = {}
var _max_level := 0
var _stats: Label
var _skin_buttons: Dictionary = {}
var _spec_buttons: Dictionary = {}
var _fleet_box: Container
var _fleet_buttons: Dictionary = {}
var _info: Label
var _ship_title: Label
var _detail_scroll: ScrollContainer
var _detail_box: VBoxContainer
var _filter_row: HBoxContainer
var _filter_buttons: Dictionary = {}
var _filter := "all"
var _data: Dictionary = {}   # характеристики корабля игрока (от сервера или демо)


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
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
	col.add_theme_constant_override("separation", 10)
	margin.add_child(col)

	var head := HBoxContainer.new()
	col.add_child(head)
	var screen_title := Label.new()
	screen_title.text = "Ангар"
	screen_title.theme_type_variation = &"TitleLabel"
	screen_title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(screen_title)

	var filter_scroll := ScrollContainer.new()
	filter_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	filter_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	filter_scroll.custom_minimum_size = Vector2(0, Tokens.TOUCH_MIN + 8)
	col.add_child(filter_scroll)
	_filter_row = HBoxContainer.new()
	_filter_row.add_theme_constant_override("separation", 8)
	filter_scroll.add_child(_filter_row)

	var mid := HBoxContainer.new()
	mid.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	mid.size_flags_vertical = Control.SIZE_EXPAND_FILL
	mid.add_theme_constant_override("separation", 16)
	col.add_child(mid)

	var svc := SubViewportContainer.new()
	svc.stretch = true
	svc.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	svc.size_flags_vertical = Control.SIZE_EXPAND_FILL
	svc.size_flags_stretch_ratio = 1.25
	svc.custom_minimum_size = Vector2(360, 240)
	mid.add_child(svc)
	var vp := SubViewport.new()
	vp.own_world_3d = true
	vp.transparent_bg = false
	vp.size = Vector2i(1024, 768)
	vp.msaa_3d = Viewport.MSAA_DISABLED
	svc.add_child(vp)
	var env := WorldEnvironment.new()
	var e := Environment.new()
	e.background_mode = Environment.BG_COLOR
	e.background_color = Tokens.BG.darkened(0.25)
	e.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	e.ambient_light_color = Color(0.62, 0.68, 0.8)
	env.environment = e
	vp.add_child(env)
	var light := DirectionalLight3D.new()
	light.rotation_degrees = Vector3(-42, -30, 0)
	vp.add_child(light)
	var cam := Camera3D.new()
	cam.position = Vector3(0, 1.5, 14.0)
	cam.rotation_degrees = Vector3(-6, 0, 0)
	cam.fov = 26.0
	vp.add_child(cam)
	_turntable = Node3D.new()
	vp.add_child(_turntable)
	_visual = ShipVisual.new()
	_visual.ship_id = ship_id
	_turntable.add_child(_visual)
	_visual.build(ship_id, skin)

	var side := PanelContainer.new()
	side.custom_minimum_size = Vector2(340, 0)
	side.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	side.size_flags_vertical = Control.SIZE_EXPAND_FILL
	side.size_flags_stretch_ratio = 1.0
	side.clip_contents = true
	mid.add_child(side)
	_detail_scroll = ScrollContainer.new()
	_detail_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_detail_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_detail_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	_detail_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	side.add_child(_detail_scroll)
	_detail_box = VBoxContainer.new()
	_detail_box.add_theme_constant_override("separation", 8)
	_detail_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_detail_scroll.add_child(_detail_box)
	var def := ShipDef.get_def(ship_id)
	_ship_title = Label.new()
	_ship_title.text = "%s — %s" % [def.get("name", ""), def.get("class", "")]
	if bool(def.get("temporary", false)):
		_ship_title.text += " (временная модель)"
	_ship_title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_ship_title.custom_minimum_size = Vector2(280, 0)
	_ship_title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_ship_title.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 4)
	_detail_box.add_child(_ship_title)
	_info = Label.new()
	_info.theme_type_variation = &"MutedLabel"
	_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_info.custom_minimum_size = Vector2(280, 0)
	_info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_detail_box.add_child(_info)
	_stats = Label.new()
	_stats.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_stats.custom_minimum_size = Vector2(280, 0)
	_stats.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_detail_box.add_child(_stats)
	_module_info = Label.new()
	_module_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_module_info.custom_minimum_size = Vector2(280, 0)
	_module_info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_module_info.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	_detail_box.add_child(_module_info)

	_engine_button = ActionButton.new()
	_engine_button.pressed.connect(_toggle_engine)
	_detail_box.add_child(_engine_button)
	_engine_reason = Label.new()
	_engine_reason.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_engine_reason.custom_minimum_size = Vector2(280, 0)
	_engine_reason.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_engine_reason.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	_detail_box.add_child(_engine_reason)
	_equipment_refresh = ActionButton.new()
	_equipment_refresh.text = "Обновить оснащение"
	_equipment_refresh.pressed.connect(func() -> void: equipment_refresh_requested.emit())
	_detail_box.add_child(_equipment_refresh)
	_detail_box.add_child(_caption("Модули"))
	_fit_box = VBoxContainer.new()
	_fit_box.add_theme_constant_override("separation", 8)
	_fit_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_detail_box.add_child(_fit_box)

	_passport_info = Label.new()
	_passport_info.name = "PassportInfo"
	_passport_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_passport_info.custom_minimum_size = Vector2(280, 0)
	_passport_info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_passport_info.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	_detail_box.add_child(_passport_info)
	_detail_box.add_child(_caption("Облик (характеристики одинаковые)"))
	var skins := HBoxContainer.new()
	_detail_box.add_child(skins)
	for k in ShipDef.skin_ids(ship_id):
		var b := ActionButton.new()
		b.text = str(ShipDef.skin(ship_id, k).get("name", k))
		b.toggle_mode = true
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.pressed.connect(_pick_skin.bind(str(k)))
		skins.add_child(b)
		_skin_buttons[str(k)] = b
	_detail_box.add_child(_caption("Специализация"))
	var specs := HBoxContainer.new()
	_detail_box.add_child(specs)
	for k2 in (def.get("specs", {}) as Dictionary).keys():
		var b2 := ActionButton.new()
		b2.text = str(def["specs"][k2].get("name", k2))
		b2.toggle_mode = true
		b2.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b2.pressed.connect(_pick_spec.bind(str(k2)))
		specs.add_child(b2)
		_spec_buttons[str(k2)] = b2
	var dock := ActionButton.new()
	dock.text = "Показать стыковку"
	dock.pressed.connect(func() -> void:
		_visual.undock()
		_visual.play("Dock"))
	_detail_box.add_child(dock)

	col.add_child(_caption("Коллекция"))
	var fleet_scroll := ScrollContainer.new()
	fleet_scroll.custom_minimum_size = Vector2(0, 124)
	fleet_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	fleet_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	col.add_child(fleet_scroll)
	_fleet_box = HBoxContainer.new()
	_fleet_box.add_theme_constant_override("separation", 8)
	fleet_scroll.add_child(_fleet_box)

	TouchScroll.fix_tree(self)
	_refresh()
	_add_exit()


func _add_exit() -> void:
	var close := ActionButton.new()
	close.name = "HangarExit"
	close.text = "Выйти"
	close.tooltip_text = "Закрыть ангар и вернуться в космос"
	close.variant = ActionButton.Variant.PRIMARY
	close.z_index = 40
	close.mouse_filter = Control.MOUSE_FILTER_STOP
	close.custom_minimum_size = Vector2(168, 56)
	close.set_anchors_preset(Control.PRESET_TOP_RIGHT)
	close.grow_horizontal = Control.GROW_DIRECTION_BEGIN
	close.offset_left = -184.0
	close.offset_top = 12.0
	close.offset_right = -16.0
	close.offset_bottom = 68.0
	close.pressed.connect(func() -> void: closed.emit())
	add_child(close)


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_cancel"):
		closed.emit()
		get_viewport().set_input_as_handled()


func _caption(t: String) -> Label:
	var l := Label.new()
	l.text = t
	l.theme_type_variation = &"MutedLabel"
	return l


func _group_of_class(kind: String) -> String:
	var c := kind.to_lower()
	if "истреб" in c:
		return "fighter"
	if "корвет" in c:
		return "corvette"
	if "фрегат" in c:
		return "frigate"
	if "эсминец" in c:
		return "destroyer"
	if "крейсер" in c:
		return "cruiser"
	if "развед" in c:
		return "scout"
	if "поддерж" in c:
		return "support"
	if "добыт" in c:
		return "miner"
	if "транспорт" in c or "груз" in c:
		return "hauler"
	return "other"


func _rebuild_filters() -> void:
	if _filter_row == null or _fleet_box == null:
		return
	for child in _filter_row.get_children():
		_filter_row.remove_child(child)
		child.free()
	_filter_buttons.clear()
	var counts: Dictionary = {}
	for child in _fleet_box.get_children():
		var group: String = str(child.get_meta("group", "other"))
		counts["all"] = int(counts.get("all", 0)) + 1
		counts[group] = int(counts.get(group, 0)) + 1
	for pair in FILTERS:
		var id: String = str(pair[0])
		var title: String = str(pair[1])
		var n: int = int(counts.get(id, 0))
		if id != "all" and n <= 0:
			continue
		if id == "all" and int(counts.get("all", 0)) <= 0:
			continue
		var b := ActionButton.new()
		b.toggle_mode = true
		b.text = title
		b.custom_minimum_size = Vector2(0, Tokens.TOUCH_MIN)
		b.set_pressed_no_signal(id == _filter)
		b.pressed.connect(_apply_filter.bind(id))
		_filter_row.add_child(b)
		_filter_buttons[id] = b
	if not _filter_buttons.has(_filter):
		_filter = "all"
	_apply_filter(_filter)


func _apply_filter(id: String) -> void:
	_filter = id
	for k in _filter_buttons:
		var chip := _filter_buttons[k] as Button
		if chip != null:
			chip.set_pressed_no_signal(k == id)
	if _fleet_box == null:
		return
	for child in _fleet_box.get_children():
		var group: String = str(child.get_meta("group", "other"))
		child.visible = id == "all" or group == id


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
		_fleet_box.remove_child(c)
		c.free()
	_fleet_buttons.clear()
	for sh in ships:
		if not (sh is Dictionary):
			continue
		if str(sh.get("id", "")) == ship_id:
			_max_level = int(sh.get("max_level", 0))
			if sh.get("rpg_passport") is Dictionary and str(sh["rpg_passport"].get("id", "")) == ship_id:
				_passport = sh["rpg_passport"]
		var hull_class: String = str(sh.get("class", ""))
		if hull_class == "":
			hull_class = str(ShipDef.get_def(str(sh.get("id", ""))).get("class", ""))
		var b := ActionButton.new()
		b.toggle_mode = true
		var lvl := (" · ур. %d" % int(sh["level"])) if sh.has("level") else ""
		var temp := " · врем." if bool(sh.get("temporary", false)) else ""
		var flying := "\nВ полёте" if str(sh.get("id", "")) == ship_id else ""
		b.text = "%s%s\n%s%s%s" % [str(sh.get("name", "")), lvl, hull_class, temp, flying]
		b.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
		b.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
		b.custom_minimum_size = Vector2(176, 104)
		b.set_meta("group", _group_of_class(hull_class))
		b.set_pressed_no_signal(str(sh.get("id", "")) == ship_id)
		b.pressed.connect(_pick_ship.bind(str(sh.get("id", ""))))
		_fleet_box.add_child(b)
		_fleet_buttons[str(sh.get("id", ""))] = b
	_rebuild_filters()
	_refresh()


func _pick_ship(id: String) -> void:
	for k in _fleet_buttons:
		(_fleet_buttons[k] as Button).set_pressed_no_signal(k == ship_id)
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
	if _ship_title != null:
		_ship_title.text = "%s — %s" % [str(def.get("name", "")), str(def.get("class", ""))]
		if bool(def.get("temporary", false)):
			_ship_title.text += " (временная модель)"
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
	_refresh_fitting()
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
	_turntable.rotation.y += delta * 0.35
	_visual.update_motion(delta, 0.4, 0.0, false)
	_sync_detail_width()


func _sync_detail_width() -> void:
	if _detail_scroll == null or _detail_box == null:
		return
	var w: float = _detail_scroll.size.x
	var bar := _detail_scroll.get_v_scroll_bar()
	if bar != null and bar.visible:
		w -= bar.size.x
	if absf(_detail_box.custom_minimum_size.x - w) > 1.0:
		_detail_box.custom_minimum_size.x = maxf(280.0, w)


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


func set_fitting(data: Dictionary) -> void:
	_fitting = data
	_refresh_fitting()


func _slot_title(slot_id: String) -> String:
	var names := {
		"weapon-1": "Орудие 1", "weapon-2": "Орудие 2", "defense-1": "Защита",
		"tech-1": "Двигатель", "tech-2": "Реактор", "tech-3": "Электроника", "special-1": "Особый слот"
	}
	return str(names.get(slot_id, slot_id))


func _refresh_fitting() -> void:
	if _fit_box == null:
		return
	for child in _fit_box.get_children():
		_fit_box.remove_child(child)
		child.free()
	if _fitting.is_empty():
		var wait := Label.new()
		wait.text = "Модули читаются с сервера. Ресурсы в эти слоты не кладутся."
		wait.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		wait.custom_minimum_size = Vector2(280, 0)
		wait.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_fit_box.add_child(wait)
		return
	var can := bool(_fitting.get("can_change", false))
	var summary := Label.new()
	summary.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	summary.custom_minimum_size = Vector2(280, 0)
	summary.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	summary.text = "Масса %.0f/%.0f т · реактор %.0f/%.0f" % [float(_fitting.get("mass_tons", 0)), float(_fitting.get("mass_budget_tons", 0)), float(_fitting.get("power_draw_mw", 0)), float(_fitting.get("power_capacity_mw", 0))]
	if not can:
		summary.text += "\nМенять модули можно у станции, стоя на месте и вне боя."
	_fit_box.add_child(summary)
	var revision := int(_fitting.get("revision", 0))
	var pilot_revision := int(_fitting.get("pilot_revision", 0))
	for slot in _fitting.get("slots", []):
		if not (slot is Dictionary):
			continue
		var slot_box := VBoxContainer.new()
		slot_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		slot_box.add_theme_constant_override("separation", 4)
		var label := Label.new()
		label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		label.custom_minimum_size = Vector2(280, 0)
		label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		var item: Dictionary = slot.get("item", {}) if slot.get("item") is Dictionary else {}
		var title := _slot_title(str(slot.get("id", "")))
		label.text = title + ": " + (str(item.get("name", "")) if not item.is_empty() else "пусто")
		if bool(slot.get("locked", false)):
			label.text += " · закрыт"
		slot_box.add_child(label)
		var actions := HBoxContainer.new()
		actions.add_theme_constant_override("separation", 8)
		if not bool(slot.get("locked", false)) and not item.is_empty():
			var off := ActionButton.new()
			off.text = "Снять"
			off.disabled = not can
			off.pressed.connect(module_action.emit.bind("unequip", str(slot.get("id", "")), "", revision, pilot_revision))
			actions.add_child(off)
		else:
			for bag_item in _fitting.get("bag", []):
				if not (bag_item is Dictionary):
					continue
				var on := ActionButton.new()
				on.text = str(bag_item.get("name", "Вставить"))
				on.disabled = not can
				on.pressed.connect(module_action.emit.bind("equip", str(slot.get("id", "")), str(bag_item.get("id", "")), revision, pilot_revision))
				actions.add_child(on)
		if actions.get_child_count() > 0:
			slot_box.add_child(actions)
		_fit_box.add_child(slot_box)
	var bag: Array = _fitting.get("bag", [])
	if bag.size() > 0:
		var bag_label := Label.new()
		var names: PackedStringArray = []
		for bag_item in bag:
			if bag_item is Dictionary:
				names.append(str(bag_item.get("name", "")))
		bag_label.text = "В сумке: " + ", ".join(names)
		bag_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		bag_label.custom_minimum_size = Vector2(280, 0)
		bag_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_fit_box.add_child(bag_label)
