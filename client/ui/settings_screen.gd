extends Control
## Настройки игры. Переключатели сохраняются на устройстве (user://settings.cfg); игровые (автоответ на атаку) дополнительно
## отправляются серверу при входе и при изменении. Экран только показывает и сообщает об изменениях сигналом.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const FrameStats := preload("res://ui/debug/frame_stats.gd")

signal option_changed(name: String, value: bool)
signal closed

## Описание переключателей: ключ, подпись, пояснение, значение по умолчанию.
const OPTIONS := [
	{"key": "auto_retaliate", "title": "Автоответ на атаку", "about": "Когда вас атакуют, вы сразу автоматически атакуете нападающего (если не заняты другой целью).", "default": true, "icon": "zap"},
	{"key": "content_auto", "title": "Автообновление контента", "about": "Игра сама докачивает новые корабли, картинки и данные, когда они появляются, без переустановки. Применяется при следующем запуске.", "default": true, "icon": "package"},
	{"key": "debug_overlay2", "title": "Панель FPS (для тестов)", "about": "Показывает частоту кадров и время загрузки. Также включается тремя пальцами или F3.", "default": false, "icon": "gauge"},
]

var _toggles: Dictionary = {}


## Значение переключателя (с учётом сохранённого).
static func get_value(key: String) -> bool:
	for o in OPTIONS:
		if o["key"] == key:
			return UiSettings.get_bool(key, bool(o["default"]))
	return false


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 0.94)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(center)
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(720, 0)
	center.add_child(panel)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 14)
	panel.add_child(box)
	var head := HBoxContainer.new()
	head.add_theme_constant_override("separation", 10)
	head.add_child(Icons.rect("settings", 32, Tokens.ACCENT))
	var title := Label.new()
	title.text = "Настройки"
	title.theme_type_variation = &"TitleLabel"
	head.add_child(title)
	box.add_child(head)
	for o in OPTIONS:
		box.add_child(_row(o))
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.variant = ActionButton.Variant.PRIMARY
	close.custom_minimum_size = Vector2(0, 56)
	close.pressed.connect(func() -> void: closed.emit())
	box.add_child(close)


func _row(o: Dictionary) -> Control:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 14)
	var texts := VBoxContainer.new()
	texts.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var t := Label.new()
	t.text = str(o["title"])
	t.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	texts.add_child(t)
	var a := Label.new()
	a.text = str(o["about"])
	a.theme_type_variation = &"MutedLabel"
	a.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	texts.add_child(a)
	row.add_child(texts)
	var b := ActionButton.new()
	b.toggle_mode = true
	b.custom_minimum_size = Vector2(120, 56)
	var key := str(o["key"])
	_toggles[key] = b
	_sync_button(b, get_value(key))
	b.pressed.connect(func() -> void:
		var v := b.button_pressed
		UiSettings.set_bool(key, v)
		_sync_button(b, v)
		option_changed.emit(key, v))
	row.add_child(b)
	return row


func _sync_button(b: Button, on: bool) -> void:
	b.set_pressed_no_signal(on)
	b.text = "Вкл" if on else "Выкл"
	(b as ActionButton).variant = ActionButton.Variant.PRIMARY if on else ActionButton.Variant.SECONDARY
