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
			route_text = "О#f#0$1#d#0я з#e#d#0: цель появится с$0#5#4и в$0#0#6еских меток."
		elif kind in ["sell", "reach", "talk", "report"]:
			route_text = "С$2#0#d$6#8я: з#0#2#5$0$8#8$2#5 д#5#9$1$2#2#8#5 п#e$1#b#5 в$5#e#4а в б#5#7о#f#0$1н$3ю з#e#dу."
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
