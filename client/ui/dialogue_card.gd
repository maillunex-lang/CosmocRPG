extends Control
## Короткий разговор у станции. Реплики и ответы присылает сервер, клиент их не выдумывает.

const Rules := preload("res://ui/dialogue_rules.gd")
const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal route_ready

var _gateway: Node
var _open_btn: Button
var _panel: PanelContainer
var _speaker: Label
var _text: Label
var _choices: VBoxContainer
var _skip: Button
var _close_btn: Button
var _open := false
var _completed := false


func bind(gateway: Node) -> void:
	_gateway = gateway
	if gateway.has_signal("state_updated"):
		gateway.state_updated.connect(_on_state)
	if gateway.has_signal("dialogue_opened"):
		gateway.dialogue_opened.connect(_show)


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_open_btn = ActionButton.new()
	_open_btn.text = "Поговорить"
	_open_btn.custom_minimum_size = Vector2(220, 56)
	_open_btn.position = Vector2(24, 24)
	_open_btn.set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	_open_btn.offset_left = -110
	_open_btn.offset_right = 110
	_open_btn.offset_top = -150
	_open_btn.offset_bottom = -94
	_open_btn.visible = false
	_open_btn.pressed.connect(_ask_open)
	add_child(_open_btn)

	_panel = PanelContainer.new()
	_panel.visible = false
	_panel.mouse_filter = Control.MOUSE_FILTER_STOP
	_panel.set_anchors_preset(Control.PRESET_CENTER)
	_panel.offset_left = -300
	_panel.offset_right = 300
	_panel.offset_top = -280
	_panel.offset_bottom = 280
	_panel.add_theme_stylebox_override("panel", _box())
	add_child(_panel)
	var margin := MarginContainer.new()
	for side in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 16)
	_panel.add_child(margin)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 10)
	margin.add_child(col)
	_speaker = Label.new()
	_speaker.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	col.add_child(_speaker)
	var scroll := ScrollContainer.new()
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.custom_minimum_size = Vector2(520, 260)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	col.add_child(scroll)
	var inner := VBoxContainer.new()
	inner.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	inner.add_theme_constant_override("separation", 10)
	scroll.add_child(inner)
	_text = Label.new()
	_text.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_text.custom_minimum_size = Vector2(520, 0)
	inner.add_child(_text)
	_choices = VBoxContainer.new()
	_choices.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_choices.add_theme_constant_override("separation", 8)
	inner.add_child(_choices)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	col.add_child(row)
	_skip = ActionButton.new()
	_skip.text = "Пропустить"
	_skip.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_skip.pressed.connect(_skip_talk)
	row.add_child(_skip)
	_close_btn = ActionButton.new()
	_close_btn.text = "Закрыть"
	_close_btn.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_close_btn.pressed.connect(_close)
	row.add_child(_close_btn)


func _on_state(state: Dictionary) -> void:
	var q := Rules.talk_quest(state)
	_open_btn.visible = not q.is_empty() and not _open
	if not q.is_empty():
		_open_btn.text = "Поговорить"


func _ask_open() -> void:
	if _gateway != null and _gateway.has_method("talk"):
		_gateway.talk("")


func _show(data: Dictionary) -> void:
	_open = true
	_completed = bool(data.get("done", false))
	_open_btn.visible = false
	_panel.visible = true
	_speaker.text = str(data.get("speaker", ""))
	_text.text = str(data.get("text", ""))
	for child in _choices.get_children():
		child.queue_free()
	if bool(data.get("done", false)):
		_skip.visible = false
		var ok := ActionButton.new()
		ok.text = "Понятно"
		ok.pressed.connect(_close)
		_choices.add_child(ok)
		return
	_skip.visible = true
	for raw in data.get("choices", []):
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var choice: Dictionary = raw
		var b := ActionButton.new()
		b.text = str(choice.get("text", "Ответить"))
		b.custom_minimum_size = Vector2(0, 52)
		var id := str(choice.get("id", ""))
		b.pressed.connect(func() -> void:
			if _gateway != null and _gateway.has_method("talk"):
				_gateway.talk(id))
		_choices.add_child(b)


func _skip_talk() -> void:
	if _gateway != null and _gateway.has_method("talk"):
		_gateway.talk("skip")


func _close() -> void:
	var go := _completed
	_completed = false
	_open = false
	_panel.visible = false
	if _gateway != null and _gateway.has_method("snapshot"):
		_on_state(_gateway.snapshot())
	if go:
		route_ready.emit()


func _box() -> StyleBoxFlat:
	var s := StyleBoxFlat.new()
	s.bg_color = Color(Tokens.BG, 0.96)
	s.border_color = Color(Tokens.ACCENT, Tokens.BORDER_ALPHA)
	s.set_border_width_all(1)
	s.set_corner_radius_all(12)
	s.content_margin_left = 8
	s.content_margin_right = 8
	return s
