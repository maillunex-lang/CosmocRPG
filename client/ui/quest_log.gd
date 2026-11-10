extends Control
## Журнал заданий: описание и автопилот к месту, где задание выполняется.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const TouchScroll := preload("res://ui/touch_scroll.gd")

signal fly_requested(quest: Dictionary)
signal closed

var _list: VBoxContainer


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
		margin.add_theme_constant_override("margin_" + side_name, 20)
	add_child(margin)
	var col := VBoxContainer.new()
	col.size_flags_vertical = Control.SIZE_EXPAND_FILL
	col.add_theme_constant_override("separation", 10)
	margin.add_child(col)
	var head := HBoxContainer.new()
	col.add_child(head)
	var title := Label.new()
	title.text = "Задания"
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(140, 52)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)
	var scroll := ScrollContainer.new()
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	col.add_child(scroll)
	_list = VBoxContainer.new()
	_list.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_list.add_theme_constant_override("separation", 12)
	scroll.add_child(_list)


func set_quests(quests: Array) -> void:
	if _list == null:
		return
	for c in _list.get_children():
		c.queue_free()
	if quests.is_empty():
		var empty := Label.new()
		empty.text = "Нет активных заданий"
		_list.add_child(empty)
		return
	for raw in quests:
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var q: Dictionary = raw
		var card := PanelContainer.new()
		_list.add_child(card)
		var box := VBoxContainer.new()
		box.add_theme_constant_override("separation", 6)
		card.add_child(box)
		var name := Label.new()
		name.text = "%s (%d/%d)" % [str(q.get("title", "")), int(q.get("progress", 0)), int(q.get("goal", 1))]
		name.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		box.add_child(name)
		var body := Label.new()
		body.text = str(q.get("text", ""))
		body.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		body.theme_type_variation = &"MutedLabel"
		box.add_child(body)
		var fly := ActionButton.new()
		fly.text = "Лететь к цели"
		fly.variant = ActionButton.Variant.PRIMARY
		fly.pressed.connect(_emit_fly.bind(q))
		box.add_child(fly)
	TouchScroll.fix_tree(self)


func _emit_fly(q: Dictionary) -> void:
	fly_requested.emit(q)
