extends Control
## Журнал заданий: описание и автопилот к месту, где задание выполняется.

const QuestHints := preload("res://ui/quest_hints.gd")

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


func set_quests(quests: Array, state: Dictionary = {}) -> void:
	if _list == null:
		return
	for c in _list.get_children():
		c.queue_free()
	var first_hour: Dictionary = state.get("first_hour", {}) if state.get("first_hour") is Dictionary else {}
	var steps: Array = first_hour.get("steps", []) if first_hour.get("steps") is Array else []
	if not steps.is_empty():
		var guide := PanelContainer.new()
		_list.add_child(guide)
		var guide_box := VBoxContainer.new()
		guide_box.add_theme_constant_override("separation", 3)
		guide.add_child(guide_box)
		var guide_title := Label.new()
		guide_title.text = "Первый час"
		guide_title.theme_type_variation = &"TitleLabel"
		guide_box.add_child(guide_title)
		var current := int(first_hour.get("current", 0))
		for i in steps.size():
			var step: Dictionary = steps[i] if steps[i] is Dictionary else {}
			var line := Label.new()
			line.text = "%s %s" % ["✓" if i < current else ("▶" if i == current else "○"), str(step.get("title", ""))]
			line.theme_type_variation = &"MutedLabel" if i != current else &"RareLabel"
			guide_box.add_child(line)
	if quests.is_empty():
		var empty := Label.new()
		empty.text = "Нет активных заданий"
		_list.add_child(empty)
		return
	for raw in quests:
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var q: Dictionary = QuestHints.delivery_step(raw, state)
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
		var route := Label.new()
		var kind := str(q.get("kind", ""))
		var goal := int(q.get("goal", 1))
		var ship: Variant = state.get("ship", {})
		var cargo := int(round(float((ship as Dictionary).get("cargo", 0.0)))) if ship is Dictionary else 0
		var route_text := "Маршрут: следуйте жёлтой метке на карте."
		if kind in ["mine", "deliver"]:
			var target := str(q.get("target", "ресурс"))
			route_text = "Ресурс: %s · цель: %d ед." % [target, goal]
			if cargo > 0 and cargo < goal:
				route_text += " · трюм: %d — по$2ребует$1я не$1колько рейсов и#bи гру#7овой модуль" % cargo
		elif kind == "kill":
			route_text = "Бой: цель появится среди вражеских меток."
		elif kind in ["sell", "reach", "talk", "report"]:
			route_text = "Станция: следуйте жёлтой метке и войдите в безопасную зону."
		route.text = route_text
		route.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		route.theme_type_variation = &"RareLabel"
		box.add_child(route)
		var fly := ActionButton.new()
		fly.text = "Лететь к цели"
		fly.variant = ActionButton.Variant.PRIMARY
		fly.pressed.connect(_emit_fly.bind(q))
		box.add_child(fly)
	TouchScroll.fix_tree(self)


func _emit_fly(q: Dictionary) -> void:
	fly_requested.emit(q)
