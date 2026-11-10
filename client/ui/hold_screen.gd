extends Control
## Трюм: лут в отсеках и модули, которые можно надеть на корабль. Надевание — в ангаре у станции.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal closed

const NAMES := {
	"iron_ore": "Руда",
	"crystals": "Кристаллы",
	"gas": "Газ",
	"scrap": "Лом",
}

var _loot: VBoxContainer
var _gear: VBoxContainer


func bind(state: Dictionary) -> void:
	_fill(state)


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
	panel.custom_minimum_size = Vector2(680, 0)
	center.add_child(panel)
	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(640, 560)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	box.custom_minimum_size = Vector2(620, 0)
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
	gear_title.text = "Шмот на корабль"
	box.add_child(gear_title)
	var note := Label.new()
	note.text = "Надеть модуль можно в ангаре, когда корабль у станции."
	note.theme_type_variation = &"MutedLabel"
	note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(note)
	_gear = VBoxContainer.new()
	box.add_child(_gear)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(0, 56)
	close.pressed.connect(func() -> void: closed.emit())
	box.add_child(close)


func _fill(state: Dictionary) -> void:
	if _loot == null:
		return
	for c in _loot.get_children():
		c.queue_free()
	for c in _gear.get_children():
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
		var title := str(NAMES.get(str(key), key))
		row.text = "%s  ·  %s" % [title, str(snapped(n, 0.1))]
		_loot.add_child(row)
	if not any_loot:
		var empty := Label.new()
		empty.text = "Трюм пуст. Руда, газ и лом появятся здесь."
		empty.theme_type_variation = &"MutedLabel"
		_loot.add_child(empty)
	var modules: Array = ship.get("modules", []) if ship.get("modules") is Array else []
	if modules.is_empty():
		var none := Label.new()
		none.text = "Съёмных модулей на этом корабле пока нет."
		none.theme_type_variation = &"MutedLabel"
		none.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		_gear.add_child(none)
		return
	for raw in modules:
		if not (raw is Dictionary):
			continue
		var row := Label.new()
		var worn := "надет" if bool(raw.get("equipped", false)) else "в трюме"
		row.text = "%s  ·  %s" % [str(raw.get("name", "Модуль")), worn]
		_gear.add_child(row)
