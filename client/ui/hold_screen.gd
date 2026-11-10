extends Control
## Трюм на весь экран: слева груз и модули, в центре корабль со слотами, справа параметры.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal closed

const NAMES := {
	"iron_ore": "Руда",
	"crystals": "Кристаллы",
	"gas": "Газ",
	"scrap": "Лом",
}
const SLOTS := {
	"weapon-1": "Орудие 1", "weapon-2": "Орудие 2", "defense-1": "Защита",
	"tech-1": "Двигатель", "tech-2": "Реактор", "tech-3": "Электроника", "special-1": "Особый",
}
const LOOT_COLOR := {
	"iron_ore": Tokens.RARE,
	"crystals": Tokens.ACCENT_2,
	"gas": Tokens.ACCENT,
	"scrap": Tokens.TEXT_MUTED,
}

var gateway: Node
var _ship: Dictionary = {}
var _fitting: Dictionary = {}
var _loot_key := ""
var _sel_slot := ""
var _sel_item := ""
var _ship_title: Label
var _slots: VBoxContainer
var _loot: GridContainer
var _bag: GridContainer
var _stats: VBoxContainer
var _actions: VBoxContainer
var _note: Label


func bind(gw: Node) -> void:
	gateway = gw
	if gateway != null and gateway.has_signal("state_updated") and not gateway.state_updated.is_connected(_on_state):
		gateway.state_updated.connect(_on_state)
	if gateway != null and gateway.has_signal("fitting_updated") and not gateway.fitting_updated.is_connected(_on_fitting):
		gateway.fitting_updated.connect(_on_fitting)
	if gateway != null and gateway.has_method("request_fitting"):
		gateway.request_fitting()
	_on_state(gateway.snapshot() if gateway != null and gateway.has_method("snapshot") else {})


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
	margin.add_theme_constant_override("margin_left", 16)
	margin.add_theme_constant_override("margin_right", 16)
	margin.add_theme_constant_override("margin_top", 12)
	margin.add_theme_constant_override("margin_bottom", 12)
	add_child(margin)
	var root := VBoxContainer.new()
	root.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	root.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	root.size_flags_vertical = Control.SIZE_EXPAND_FILL
	root.add_theme_constant_override("separation", 10)
	margin.add_child(root)
	root.add_child(_header())
	var body := HBoxContainer.new()
	body.size_flags_vertical = Control.SIZE_EXPAND_FILL
	body.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body.add_theme_constant_override("separation", 12)
	root.add_child(body)
	body.add_child(_inventory())
	body.add_child(_doll())
	body.add_child(_sheet())


func _header() -> Control:
	var bar := PanelContainer.new()
	bar.add_theme_stylebox_override("panel", _box(Color(Tokens.SURFACE, 0.92), Tokens.ACCENT))
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 12)
	bar.add_child(row)
	var title := Label.new()
	title.text = "Трюм"
	title.theme_type_variation = &"TitleLabel"
	row.add_child(title)
	_ship_title = Label.new()
	_ship_title.text = "Корабль"
	_ship_title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_ship_title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	row.add_child(_ship_title)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(160, 56)
	close.pressed.connect(func() -> void: closed.emit())
	row.add_child(close)
	return bar


func _inventory() -> Control:
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(360, 0)
	panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_theme_stylebox_override("panel", _box(Color(Tokens.SURFACE_SUNKEN, 0.94), Color(Tokens.RARE, 0.7)))
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	box.add_theme_constant_override("separation", 8)
	scroll.add_child(box)
	box.add_child(_caption("Груз"))
	_loot = GridContainer.new()
	_loot.columns = 2
	_loot.add_theme_constant_override("h_separation", 8)
	_loot.add_theme_constant_override("v_separation", 8)
	box.add_child(_loot)
	box.add_child(_caption("Модули в сумке"))
	_bag = GridContainer.new()
	_bag.columns = 1
	_bag.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_bag.add_theme_constant_override("v_separation", 8)
	box.add_child(_bag)
	return panel


func _doll() -> Control:
	var panel := PanelContainer.new()
	panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_theme_stylebox_override("panel", _box(Color("#101820"), Color(Tokens.ACCENT, 0.35)))
	var row := HBoxContainer.new()
	row.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_theme_constant_override("separation", 8)
	panel.add_child(row)
	var slot_scroll := ScrollContainer.new()
	slot_scroll.custom_minimum_size = Vector2(230, 0)
	slot_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	slot_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_child(slot_scroll)
	_slots = VBoxContainer.new()
	_slots.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_slots.add_theme_constant_override("separation", 8)
	slot_scroll.add_child(_slots)
	var plate := CenterContainer.new()
	plate.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	plate.size_flags_vertical = Control.SIZE_EXPAND_FILL
	row.add_child(plate)
	var ship := ShipMark.new()
	plate.add_child(ship)
	return panel


func _sheet() -> Control:
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(280, 0)
	panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_theme_stylebox_override("panel", _box(Color(Tokens.SURFACE, 0.94), Color(Tokens.TEXT_MUTED, 0.45)))
	var scroll := ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	box.add_theme_constant_override("separation", 8)
	scroll.add_child(box)
	box.add_child(_caption("Параметры"))
	_stats = VBoxContainer.new()
	_stats.add_theme_constant_override("separation", 4)
	box.add_child(_stats)
	_actions = VBoxContainer.new()
	_actions.add_theme_constant_override("separation", 8)
	box.add_child(_actions)
	_note = Label.new()
	_note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_note.theme_type_variation = &"MutedLabel"
	_note.text = "Выберите слот, затем модуль в сумке. Точить можно в любом месте."
	box.add_child(_note)
	return panel


func _on_state(state: Dictionary) -> void:
	_ship = state.get("ship", {}) if state.get("ship") is Dictionary else {}
	_paint_title()
	_paint_stats()
	var hold: Dictionary = _ship.get("hold", {}) if _ship.get("hold") is Dictionary else {}
	var key := ""
	var names: Array = hold.keys()
	names.sort()
	for k in names:
		key += "%s=%.1f;" % [str(k), float(hold[k])]
	if key != _loot_key:
		_loot_key = key
		_paint_loot(hold)


func _on_fitting(data: Dictionary) -> void:
	_fitting = data
	_paint_stats()
	_paint_slots()
	_paint_bag()
	_paint_actions()


func _paint_title() -> void:
	if _ship_title == null:
		return
	var name := str(_ship.get("name", "Корабль"))
	var level := int(_ship.get("level", 0))
	_ship_title.text = name if level <= 0 else "%s   ·   уровень %d" % [name, level]


func _paint_loot(hold: Dictionary) -> void:
	if _loot == null:
		return
	_clear(_loot)
	var any := false
	for key in NAMES.keys():
		var n := float(hold.get(key, 0.0))
		if n < 0.05:
			continue
		any = true
		_loot.add_child(_cell(str(NAMES[key]), str(snapped(n, 0.1)), LOOT_COLOR.get(key, Tokens.ACCENT), false, ""))
	for key in hold.keys():
		if NAMES.has(str(key)) or float(hold[key]) < 0.05:
			continue
		any = true
		_loot.add_child(_cell(str(key), str(snapped(float(hold[key]), 0.1)), Tokens.TEXT_MUTED, false, ""))
	if not any:
		var empty := Label.new()
		empty.text = "Груз пуст"
		empty.theme_type_variation = &"MutedLabel"
		_loot.add_child(empty)


func _paint_slots() -> void:
	if _slots == null:
		return
	_clear(_slots)
	var slots: Array = _fitting.get("slots", []) if _fitting.get("slots") is Array else []
	if slots.is_empty():
		var wait := Label.new()
		wait.text = "Слоты читаются с сервера"
		wait.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		wait.theme_type_variation = &"MutedLabel"
		_slots.add_child(wait)
		return
	for raw in slots:
		if raw is Dictionary:
			_slots.add_child(_slot_button(raw))


func _paint_bag() -> void:
	if _bag == null:
		return
	_clear(_bag)
	var bag: Array = _fitting.get("bag", []) if _fitting.get("bag") is Array else []
	if bag.is_empty():
		var empty := Label.new()
		empty.text = "Сумка пуста"
		empty.theme_type_variation = &"MutedLabel"
		_bag.add_child(empty)
		return
	for raw in bag:
		if raw is Dictionary:
			var id := str(raw.get("id", ""))
			var b := _cell(_item_name(raw), str(raw.get("category", "")), Tokens.ACCENT_2, id == _sel_item, id)
			b.pressed.connect(_pick_item.bind(id))
			_bag.add_child(b)


func _paint_stats() -> void:
	if _stats == null:
		return
	_clear(_stats)
	_stats.add_child(_stat("Корпус", _pair(_ship, "hull", "hull_max")))
	_stats.add_child(_stat("Щит", _pair(_ship, "shield", "shield_max")))
	_stats.add_child(_stat("Броня", _one(_ship.get("armor", null))))
	_stats.add_child(_stat("Энергия", _pair(_ship, "energy", "energy_max")))
	_stats.add_child(_stat("Груз", _cargo()))
	_stats.add_child(_stat("Масса", _fit_pair("mass_tons", "mass_budget_tons", "т")))
	_stats.add_child(_stat("Реактор", _fit_pair("power_draw_mw", "power_capacity_mw", "МВт")))


func _paint_actions() -> void:
	if _actions == null:
		return
	_clear(_actions)
	var can := bool(_fitting.get("can_change", false))
	var revision := int(_fitting.get("revision", 0))
	var pilot := int(_fitting.get("pilot_revision", 0))
	var slot := _slot_by_id(_sel_slot)
	var locked := bool(slot.get("locked", false))
	var worn: Dictionary = slot.get("item", {}) if slot.get("item") is Dictionary else {}
	if _note != null:
		if _fitting.is_empty():
			_note.text = "Модули ещё читаются с сервера."
		elif not can:
			_note.text = "Сейчас оснащение занято: корабль уничтожен или изменение ещё сохраняется."
		elif locked:
			_note.text = "Штатный двигатель снимается в ангаре."
		elif _sel_slot != "" and _sel_item != "":
			_note.text = "Модуль из сумки встанет в выбранный слот."
		else:
			_note.text = "Слот слева от корабля, модуль — в сумке. Надеть и точить можно на лету."
	if not worn.is_empty() and not locked:
		_actions.add_child(_act("Снять", can, "unequip", _sel_slot, "", revision, pilot))
		_actions.add_child(_act("Точить", can, "engineer", "", str(worn.get("id", "")), revision, pilot))
	elif _sel_item != "" and _sel_slot == "":
		_actions.add_child(_act("Точить", can, "engineer", "", _sel_item, revision, pilot))
	if _sel_slot != "" and _sel_item != "" and not locked:
		_actions.add_child(_act("Надеть в слот", can, "equip", _sel_slot, _sel_item, revision, pilot))


func _slot_button(slot: Dictionary) -> Button:
	var id := str(slot.get("id", ""))
	var item: Dictionary = slot.get("item", {}) if slot.get("item") is Dictionary else {}
	var title := str(SLOTS.get(id, id))
	var body := "пусто"
	if bool(slot.get("locked", false)):
		body = "штатный"
	elif not item.is_empty():
		body = _item_name(item)
	var b := _cell(title, body, Tokens.ACCENT if id == _sel_slot else Tokens.SURFACE_RAISED, id == _sel_slot, id)
	b.disabled = bool(slot.get("locked", false))
	b.pressed.connect(func() -> void:
		_sel_slot = id
		_paint_slots.call_deferred()
		_paint_actions.call_deferred()
	)
	return b


func _pick_item(id: String) -> void:
	_sel_item = "" if _sel_item == id else id
	_paint_bag.call_deferred()
	_paint_actions.call_deferred()


func _slot_by_id(id: String) -> Dictionary:
	for raw in _fitting.get("slots", []):
		if raw is Dictionary and str(raw.get("id", "")) == id:
			return raw
	return {}


func _cell(title: String, sub: String, tint: Color, on: bool, _id: String) -> Button:
	var b := Button.new()
	b.text = title if sub == "" else "%s\n%s" % [title, sub]
	b.alignment = HORIZONTAL_ALIGNMENT_LEFT
	b.custom_minimum_size = Vector2(160, 64)
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	var box := _box(Color(tint, 0.28 if not on else 0.55), Tokens.RARE if on else Color(tint, 0.8))
	b.add_theme_stylebox_override("normal", box)
	b.add_theme_stylebox_override("hover", box)
	b.add_theme_stylebox_override("pressed", box)
	b.add_theme_stylebox_override("disabled", _box(Color(Tokens.SURFACE_SUNKEN, 0.7), Color(Tokens.TEXT_MUTED, 0.3)))
	b.add_theme_color_override("font_color", Tokens.TEXT)
	b.add_theme_color_override("font_disabled_color", Tokens.TEXT_MUTED)
	return b


func _act(text: String, can: bool, op: String, slot_id: String, item_id: String, revision: int, pilot: int) -> Button:
	var b := ActionButton.new()
	b.text = text
	b.disabled = not can
	b.custom_minimum_size = Vector2(0, 56)
	b.pressed.connect(func() -> void:
		if gateway != null and gateway.has_method("set_module"):
			gateway.set_module(op, slot_id, item_id, revision, pilot)
	)
	return b


func _stat(title: String, value: String) -> Control:
	var row := HBoxContainer.new()
	var name := Label.new()
	name.text = title
	name.theme_type_variation = &"MutedLabel"
	name.custom_minimum_size = Vector2(110, 0)
	row.add_child(name)
	var num := Label.new()
	num.text = value
	num.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	num.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	row.add_child(num)
	return row


func _pair(src: Dictionary, now: String, cap: String) -> String:
	if not src.has(now):
		return "—"
	return "%s / %s" % [_num(src.get(now, 0)), _num(src.get(cap, 0))]


func _one(v: Variant) -> String:
	if v == null:
		return "—"
	return _num(v)


func _cargo() -> String:
	if not _ship.has("cargo"):
		return "—"
	return "%s / %s" % [_num(_ship.get("cargo_used", 0)), _num(_ship.get("cargo", 0))]


func _fit_pair(now: String, cap: String, unit: String) -> String:
	if _fitting.is_empty():
		return "—"
	return "%s / %s %s" % [_num(_fitting.get(now, 0)), _num(_fitting.get(cap, 0)), unit]


func _num(v: Variant) -> String:
	var n := float(v)
	if absf(n - roundf(n)) < 0.05:
		return str(int(roundf(n)))
	return str(snapped(n, 0.1))


func _item_name(item: Dictionary) -> String:
	var name := str(item.get("name", "Модуль"))
	var rank := int(item.get("engineering", 0))
	if rank > 0:
		name += " +%d" % rank
	return name


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
	s.set_corner_radius_all(12)
	s.content_margin_left = 10
	s.content_margin_right = 10
	s.content_margin_top = 8
	s.content_margin_bottom = 8
	return s


func _clear(node: Node) -> void:
	for c in node.get_children():
		node.remove_child(c)
		c.free()


class ShipMark extends Control:
	func _init() -> void:
		custom_minimum_size = Vector2(220, 320)
		mouse_filter = Control.MOUSE_FILTER_IGNORE

	func _draw() -> void:
		var c := size * 0.5
		var nose := c + Vector2(0, -120)
		var pts := PackedVector2Array([
			nose,
			c + Vector2(62, 96),
			c + Vector2(22, 64),
			c + Vector2(0, 118),
			c + Vector2(-22, 64),
			c + Vector2(-62, 96),
		])
		draw_colored_polygon(pts, Color("#42C6E5"))
		draw_colored_polygon(PackedVector2Array([
			c + Vector2(-18, -10), c + Vector2(18, -10), c + Vector2(8, 36), c + Vector2(-8, 36),
		]), Color("#F3C46B"))
		draw_arc(c + Vector2(0, 8), 78, 0, TAU, 48, Color("#42C6E5", 0.35), 2.0)
