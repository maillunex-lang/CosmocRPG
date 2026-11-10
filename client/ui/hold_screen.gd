extends Control
## Трюм: лут и модули корабля. Надеть, сменить и заточить можно в любой момент.

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
	"tech-1": "Двигатель", "tech-2": "Реактор", "tech-3": "Электроника", "special-1": "Особый слот",
}

var gateway: Node
var _loot: VBoxContainer
var _gear: VBoxContainer
var _note: Label
var _fitting: Dictionary = {}


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
	bg.color = Color(Tokens.BG, 0.96)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(center)
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(720, 0)
	center.add_child(panel)
	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(680, 640)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	box.custom_minimum_size = Vector2(660, 0)
	scroll.add_child(box)
	var title := Label.new()
	title.text = "Трюм"
	title.theme_type_variation = &"TitleLabel"
	box.add_child(title)
	var loot_title := Label.new()
	loot_title.text = "Лут"
	box.add_child(loot_title)
	_loot = VBoxContainer.new()
	box.add_child(_loot)
	var gear_title := Label.new()
	gear_title.text = "Модули корабля"
	box.add_child(gear_title)
	_note = Label.new()
	_note.text = "Надеть, сменить и заточить можно прямо здесь, даже в полёте."
	_note.theme_type_variation = &"MutedLabel"
	_note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_note)
	_gear = VBoxContainer.new()
	_gear.add_theme_constant_override("separation", 8)
	box.add_child(_gear)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(0, 56)
	close.pressed.connect(func() -> void: closed.emit())
	box.add_child(close)


func _on_state(state: Dictionary) -> void:
	_fill_loot(state)


func _on_fitting(data: Dictionary) -> void:
	_fitting = data
	_fill_gear()


func _fill_loot(state: Dictionary) -> void:
	if _loot == null:
		return
	for c in _loot.get_children():
		c.queue_free()
	var ship: Dictionary = state.get("ship", {}) if state.get("ship") is Dictionary else {}
	var hold: Dictionary = ship.get("hold", {}) if ship.get("hold") is Dictionary else {}
	var any_loot := false
	for key in hold.keys():
		var n := float(hold[key])
		if n < 0.05:
			continue
		any_loot = true
		var row := Label.new()
		row.text = "%s  ·  %s" % [str(NAMES.get(str(key), key)), str(snapped(n, 0.1))]
		_loot.add_child(row)
	if not any_loot:
		var empty := Label.new()
		empty.text = "Трюм пуст. Руда, газ и лом появятся здесь."
		empty.theme_type_variation = &"MutedLabel"
		_loot.add_child(empty)


func _fill_gear() -> void:
	if _gear == null:
		return
	for c in _gear.get_children():
		c.queue_free()
	if _fitting.is_empty():
		var wait := Label.new()
		wait.text = "Модули читаются с сервера."
		wait.theme_type_variation = &"MutedLabel"
		_gear.add_child(wait)
		return
	var can := bool(_fitting.get("can_change", false))
	if _note != null:
		_note.text = "Надеть, сменить и заточить можно прямо здесь, даже в полёте." if can else "Сейчас модули заняты: корабль уничтожен или оснащение ещё сохраняется."
	var revision := int(_fitting.get("revision", 0))
	var pilot_revision := int(_fitting.get("pilot_revision", 0))
	var bag: Array = _fitting.get("bag", []) if _fitting.get("bag") is Array else []
	var slots: Array = _fitting.get("slots", []) if _fitting.get("slots") is Array else []
	if slots.is_empty() and bag.is_empty():
		var none := Label.new()
		none.text = "Съёмных модулей на этом корабле пока нет."
		none.theme_type_variation = &"MutedLabel"
		none.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		_gear.add_child(none)
		return
	for raw in slots:
		if raw is Dictionary:
			_gear.add_child(_slot_card(raw, bag, can, revision, pilot_revision))
	if bag.is_empty():
		return
	var bag_title := Label.new()
	bag_title.text = "В сумке"
	_gear.add_child(bag_title)
	for raw in bag:
		if raw is Dictionary:
			_gear.add_child(_bag_row(raw, can, revision, pilot_revision))


func _slot_card(slot: Dictionary, bag: Array, can: bool, revision: int, pilot_revision: int) -> Control:
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 4)
	var item: Dictionary = slot.get("item", {}) if slot.get("item") is Dictionary else {}
	var title := Label.new()
	var slot_name := str(SLOTS.get(str(slot.get("id", "")), slot.get("id", "")))
	title.text = slot_name + ": " + (_item_name(item) if not item.is_empty() else "пусто")
	if bool(slot.get("locked", false)):
		title.text += "  ·  штатный"
	title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(title)
	if bool(slot.get("locked", false)):
		return box
	var actions := HBoxContainer.new()
	actions.add_theme_constant_override("separation", 8)
	if not item.is_empty():
		actions.add_child(_act("Снять", can, "unequip", str(slot.get("id", "")), "", revision, pilot_revision))
		actions.add_child(_act("Точить", can, "engineer", "", str(item.get("id", "")), revision, pilot_revision))
	for raw in bag:
		if not (raw is Dictionary):
			continue
		var verb := "Сменить" if not item.is_empty() else "Надеть"
		actions.add_child(_act("%s %s" % [verb, _item_name(raw)], can, "equip", str(slot.get("id", "")), str(raw.get("id", "")), revision, pilot_revision))
	if actions.get_child_count() > 0:
		box.add_child(actions)
	return box


func _bag_row(item: Dictionary, can: bool, revision: int, pilot_revision: int) -> Control:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	var name := Label.new()
	name.text = _item_name(item)
	name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(name)
	row.add_child(_act("Точить", can, "engineer", "", str(item.get("id", "")), revision, pilot_revision))
	return row


func _item_name(item: Dictionary) -> String:
	var name := str(item.get("name", "Модуль"))
	var rank := int(item.get("engineering", 0))
	if rank > 0:
		name += " +%d" % rank
	return name


func _act(text: String, can: bool, op: String, slot_id: String, item_id: String, revision: int, pilot_revision: int) -> Button:
	var b := ActionButton.new()
	b.text = text
	b.disabled = not can
	b.custom_minimum_size = Vector2(0, 56)
	b.pressed.connect(func() -> void:
		if gateway != null and gateway.has_method("set_module"):
			gateway.set_module(op, slot_id, item_id, revision, pilot_revision)
	)
	return b
