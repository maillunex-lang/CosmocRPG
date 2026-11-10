extends Control
## Вылеты: карточка подземелья, автоподбор ИИ и согласие на прыжок.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal closed

const OPS := [
	{
		"id": "credits", "title": "Казна", "mark": "К",
		"level": "отряд 2–4",
		"about": "Длинный ствол сквозь метеоритный рой. Казначейский конвой бросил сейф в сердце роя.",
		"mech": "Пять залов. Стены из камней держат корабль в стволе, дверь вперёд открывается только после зачистки. Охрана толстая: каждый зал дольше предыдущего.",
		"enemies": "Дозорные роя\nСтражи осыпи\nРезаки\nТяжёлые ломы\nХранитель награды",
		"rewards": [{"name": "Кредиты", "n": "800", "color": Tokens.RARE}],
		"color": Tokens.RARE,
	},
	{
		"id": "gear", "title": "Арсенал", "mark": "А",
		"level": "отряд 2–4",
		"about": "Тот же метеоритный ствол. В сердце роя осталась корабельная обшивка, смотритель её не отдаёт.",
		"mech": "Пять залов между камнями. Обшивку «Сталь» можно надеть сразу после выхода.",
		"enemies": "Дозорные роя\nСтражи осыпи\nРезаки\nТяжёлые ломы\nСмотритель арсенала",
		"rewards": [{"name": "Сталь", "n": "1", "color": Tokens.ACCENT}, {"name": "Кредиты", "n": "200", "color": Tokens.RARE}],
		"color": Tokens.ACCENT,
	},
	{
		"id": "loot", "title": "Трюм", "mark": "Т",
		"level": "отряд 2–4",
		"about": "Грузовой ствол через рой. Мародёры режут контейнеры в осыпи, руда ещё в сердце.",
		"mech": "Пять залов. Награда падает в трюм. Если трюм полон, лишнее не помещается.",
		"enemies": "Дозорные роя\nСтражи осыпи\nРезаки\nГрузчики-мародёры\nХранитель трюма",
		"rewards": [{"name": "Руда", "n": "16", "color": Tokens.SUCCESS}, {"name": "Кристаллы", "n": "8", "color": Tokens.ACCENT_2}, {"name": "Кредиты", "n": "200", "color": Tokens.RARE}],
		"color": Tokens.SUCCESS,
	},
]

var gateway: Node
var _pick := "credits"
var _status: Label
var _squad: Label
var _title: Label
var _level: Label
var _about: Label
var _mech: Label
var _enemies: Label
var _rewards: HBoxContainer
var _diff: HBoxContainer
var _pins: Dictionary = {}


func bind(gw: Node) -> void:
	gateway = gw
	if not gateway.state_updated.is_connected(_on_state):
		gateway.state_updated.connect(_on_state)
	_on_state(gateway.snapshot())


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color("#070d14")
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var margin := MarginContainer.new()
	margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["margin_left", "margin_right", "margin_top", "margin_bottom"]:
		margin.add_theme_constant_override(side, 12)
	add_child(margin)
	var root := VBoxContainer.new()
	root.size_flags_vertical = Control.SIZE_EXPAND_FILL
	root.add_theme_constant_override("separation", 10)
	margin.add_child(root)
	root.add_child(_header())
	var body := HBoxContainer.new()
	body.size_flags_vertical = Control.SIZE_EXPAND_FILL
	body.add_theme_constant_override("separation", 12)
	root.add_child(body)
	body.add_child(_rail())
	body.add_child(_sheet())
	_show(_pick)


func _header() -> Control:
	var bar := PanelContainer.new()
	bar.add_theme_stylebox_override("panel", _box(Color(Tokens.SURFACE, 0.94), Tokens.RARE))
	var row := HBoxContainer.new()
	bar.add_child(row)
	var title := Label.new()
	title.text = "Вылеты"
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(title)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(140, 56)
	close.pressed.connect(func() -> void: closed.emit())
	row.add_child(close)
	return bar


func _rail() -> Control:
	var col := VBoxContainer.new()
	col.custom_minimum_size = Vector2(196, 0)
	col.add_theme_constant_override("separation", 10)
	for op in OPS:
		col.add_child(_pin(op))
	return col


func _pin(op: Dictionary) -> Button:
	var b := Button.new()
	b.text = "%s\n%s" % [op["mark"], op["title"]]
	b.custom_minimum_size = Vector2(180, 92)
	b.add_theme_font_size_override("font_size", Tokens.FONT_BODY)
	var id := str(op["id"])
	_pins[id] = b
	b.pressed.connect(func() -> void:
		_pick = id
		_show(id)
		_arm(id)
	)
	return b


func _sheet() -> Control:
	var panel := PanelContainer.new()
	panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_theme_stylebox_override("panel", _box(Color("#101820"), Color(Tokens.RARE, 0.85)))
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	box.custom_minimum_size = Vector2(420, 0)
	box.add_theme_constant_override("separation", 8)
	scroll.add_child(box)
	_title = Label.new()
	_title.add_theme_font_size_override("font_size", Tokens.FONT_TITLE)
	box.add_child(_title)
	_level = Label.new()
	_level.theme_type_variation = &"MutedLabel"
	box.add_child(_level)
	_about = Label.new()
	_about.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_about)
	box.add_child(_caption("Механика"))
	_mech = Label.new()
	_mech.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_mech.theme_type_variation = &"MutedLabel"
	box.add_child(_mech)
	box.add_child(_caption("Враги"))
	_enemies = Label.new()
	box.add_child(_enemies)
	box.add_child(_caption("Награда"))
	_rewards = HBoxContainer.new()
	_rewards.add_theme_constant_override("separation", 8)
	box.add_child(_rewards)
	box.add_child(_caption("Живучесть охраны"))
	_diff = HBoxContainer.new()
	_diff.add_theme_constant_override("separation", 8)
	box.add_child(_diff)
	_squad = Label.new()
	_squad.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_squad)
	_status = Label.new()
	_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_status.theme_type_variation = &"MutedLabel"
	box.add_child(_status)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	var fill := ActionButton.new()
	fill.text = "Автоподбор"
	fill.custom_minimum_size = Vector2(0, 56)
	fill.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	fill.pressed.connect(func() -> void: _call("party_fill"))
	row.add_child(fill)
	var go := ActionButton.new()
	go.text = "Войти"
	go.variant = ActionButton.Variant.PRIMARY
	go.custom_minimum_size = Vector2(0, 56)
	go.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	go.pressed.connect(_ready_vote)
	row.add_child(go)
	box.add_child(row)
	var leave := ActionButton.new()
	leave.text = "Покинуть отряд"
	leave.custom_minimum_size = Vector2(0, 52)
	leave.pressed.connect(func() -> void: _call("party_leave"))
	box.add_child(leave)
	return panel


func _show(id: String) -> void:
	var op := OPS[0]
	for item in OPS:
		if str(item["id"]) == id:
			op = item
	if _title == null:
		return
	_title.text = str(op["title"])
	_title.add_theme_color_override("font_color", op["color"])
	_level.text = "Рекомендуется: %s. Прыжок после вашего согласия. ИИ соглашаются сами." % str(op["level"])
	_about.text = str(op["about"])
	_mech.text = str(op["mech"])
	_enemies.text = str(op["enemies"])
	_paint_rewards(op["rewards"])
	for pin_id in _pins:
		var b := _pins[pin_id] as Button
		var on: bool = pin_id == id
		var tint: Color = Tokens.TEXT_MUTED
		for item in OPS:
			if str(item["id"]) == pin_id:
				tint = item["color"]
		var fill := Color(tint, 0.38 if on else 0.16)
		var box := _box(fill, tint if on else Color(Tokens.TEXT_MUTED, 0.6))
		b.add_theme_stylebox_override("normal", box)
		b.add_theme_stylebox_override("hover", box)
		b.add_theme_stylebox_override("pressed", box)
		b.add_theme_color_override("font_color", Tokens.TEXT)


func _paint_rewards(items: Array) -> void:
	if _rewards == null:
		return
	_clear(_rewards)
	for raw in items:
		if raw is Dictionary:
			var chip := PanelContainer.new()
			chip.add_theme_stylebox_override("panel", _box(Color(raw["color"], 0.22), raw["color"]))
			var text := Label.new()
			text.text = "%s\n%s" % [raw["n"], raw["name"]]
			text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
			chip.add_child(text)
			chip.custom_minimum_size = Vector2(108, 64)
			_rewards.add_child(chip)


func _on_state(state: Dictionary) -> void:
	if _squad == null:
		return
	var party: Dictionary = state.get("party", {}) if state.get("party") is Dictionary else {}
	if bool(party.get("instance", false)):
		closed.emit()
		return
	var op := str(party.get("op", ""))
	if op != "" and op != _pick:
		_pick = op
		_show(op)
	var members: Array = party.get("members", []) if party.get("members") is Array else []
	var lines := "Отряд ещё не собран. Выберите вылет и нажмите «Автоподбор»."
	if members.size() > 0:
		lines = "Отряд · %d" % members.size()
		for m in members:
			if m is Dictionary:
				var nm := str(m.get("name", "пилот"))
				if bool(m.get("leader", false)):
					nm += " · ведущий"
				if bool(m.get("ai", false)):
					nm += " · ИИ"
				if bool(m.get("pending", false)):
					nm += " · ждёт ответ"
				elif bool(m.get("ready", false)):
					nm += " · согласен"
				lines += "\n" + nm
	_squad.text = lines
	_paint_diff(members.size())
	var goal := str(party.get("goal", ""))
	if goal != "":
		_status.text = goal
	elif members.size() >= 2:
		_status.text = "ИИ уже согласны. «Войти» — ваше согласие, и отряд прыгает."
	else:
		_status.text = "Одному в коридор нельзя. Автоподбор позовёт пилотов этой системы."


func _paint_diff(size: int) -> void:
	if _diff == null:
		return
	_clear(_diff)
	var tiers := [
		{"name": "Обычный", "at": 2, "mul": "×1.6"},
		{"name": "Трудный", "at": 3, "mul": "×2.1"},
		{"name": "Тяжёлый", "at": 4, "mul": "×2.7"},
	]
	var hot := 2
	if size >= 4:
		hot = 4
	elif size == 3:
		hot = 3
	for tier in tiers:
		var on: bool = int(tier["at"]) == hot and size >= 2
		var b := Label.new()
		b.text = "%s\n%s" % [tier["name"], tier["mul"]]
		b.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		b.custom_minimum_size = Vector2(110, 56)
		b.add_theme_color_override("font_color", Tokens.TEXT if on else Tokens.TEXT_MUTED)
		var holder := PanelContainer.new()
		holder.add_theme_stylebox_override("panel", _box(Color(Tokens.ACCENT, 0.35) if on else Color(Tokens.SURFACE_SUNKEN, 0.9), Tokens.ACCENT if on else Tokens.TEXT_MUTED))
		holder.add_child(b)
		_diff.add_child(holder)


func _arm(id: String) -> void:
	_call("party_arm", id)


func _ready_vote() -> void:
	_call("party_ready")


func _call(method: String, arg: String = "") -> void:
	if gateway == null or not gateway.has_method(method):
		if _status != null:
			_status.text = "Вылеты доступны в игре на сервере."
		return
	if arg == "":
		gateway.call(method)
	else:
		gateway.call(method, arg)


func _caption(text: String) -> Label:
	var l := Label.new()
	l.text = text
	l.add_theme_font_size_override("font_size", Tokens.FONT_BODY)
	l.add_theme_color_override("font_color", Tokens.RARE)
	return l


func _box(bg: Color, edge: Color) -> StyleBoxFlat:
	var s := StyleBoxFlat.new()
	s.bg_color = bg
	s.border_color = edge
	s.set_border_width_all(1)
	s.set_corner_radius_all(14)
	s.content_margin_left = 10
	s.content_margin_right = 10
	s.content_margin_top = 8
	s.content_margin_bottom = 8
	return s


func _clear(node: Node) -> void:
	for c in node.get_children():
		node.remove_child(c)
		c.free()
