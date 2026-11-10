extends Control
## Настройки игры. Переключатели сохраняются на устройстве (user://settings.cfg); игровые (автоответ на атаку) дополнительно
## отправляются серверу при входе и при изменении. Экран только показывает и сообщает об изменениях сигналом.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const FrameStats := preload("res://ui/debug/frame_stats.gd")
const AutoBattle := preload("res://ui/auto_battle.gd")

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
	var scroll := ScrollContainer.new()
	scroll.name = "SettingsScroll"
	scroll.custom_minimum_size = Vector2(680, 600)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 14)
	box.custom_minimum_size = Vector2(660, 0)
	scroll.add_child(box)
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
		if o["key"] == "auto_retaliate":
			box.add_child(_auto_target_row())
	box.add_child(_music_section())
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
	_sync_button(b, UiSettings.get_bool(key, bool(o.get("default", false))))
	b.pressed.connect(func() -> void:
		var v := b.button_pressed
		UiSettings.set_bool(key, v)
		_sync_button(b, v)
		if key == "music_enabled":
			_apply_music()
		option_changed.emit(key, v))
	row.add_child(b)
	return row


const AUTO_TARGET_CHOICES := [
	{"mode": "enemies", "text": "Враги"},
	{"mode": "enemies_players", "text": "Враги и игроки"},
	{"mode": "all", "text": "Все подряд"},
]

var _mode_buttons: Dictionary = {}


## Выбор целей автобоя: кнопка «Авто» в бою сама выделяет и атакует цели этого вида.
func _auto_target_row() -> Control:
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 8)
	var t := Label.new()
	t.text = "Автобой: кого атаковать"
	t.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	col.add_child(t)
	var a := Label.new()
	a.text = "Когда включена кнопка «Авто», игра сама выбирает ближайшую цель выбранного вида и атакует. Бой между игроками пока выключен на сервере: такие цели будут пропускаться."
	a.theme_type_variation = &"MutedLabel"
	a.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(a)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 10)
	for c in AUTO_TARGET_CHOICES:
		var b := ActionButton.new()
		b.text = str(c["text"])
		b.toggle_mode = true
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.custom_minimum_size = Vector2(0, 52)
		var mode_id := str(c["mode"])
		_mode_buttons[mode_id] = b
		b.pressed.connect(func() -> void:
			UiSettings.set_string(AutoBattle.SETTING_KEY, mode_id)
			_sync_modes()
			option_changed.emit(AutoBattle.SETTING_KEY, true))
		row.add_child(b)
	col.add_child(row)
	_sync_modes()
	return col


func _sync_modes() -> void:
	var cur := AutoBattle.mode()
	for m in _mode_buttons:
		var b := _mode_buttons[m] as ActionButton
		b.set_pressed_no_signal(m == cur)
		b.variant = ActionButton.Variant.PRIMARY if m == cur else ActionButton.Variant.SECONDARY


func _sync_button(b: Button, on: bool) -> void:
	b.set_pressed_no_signal(on)
	b.text = "Вкл" if on else "Выкл"
	(b as ActionButton).variant = ActionButton.Variant.PRIMARY if on else ActionButton.Variant.SECONDARY


func _music() -> Node:
	return get_node_or_null("/root/Music")


func _apply_music() -> void:
	var m := _music()
	if m != null and m.has_method("apply_mix"):
		m.apply_mix()


func _music_section() -> Control:
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 8)
	var t := Label.new()
	t.text = "Музыка"
	t.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	col.add_child(t)
	var a := Label.new()
	a.text = "Играет фоном с сервера, к которому вы подключены. По умолчанию включена на половине громкости. Радиоэфир появится позже."
	a.theme_type_variation = &"MutedLabel"
	a.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(a)
	col.add_child(_row({"key": "music_enabled", "title": "Музыка", "about": "Выключите, если хотите тишину. Выбор запоминается на этом устройстве.", "default": true}))
	col.add_child(_volume("music_volume", "Громкость музыки", 0.5))
	col.add_child(_volume("radio_volume", "Громкость радио", 0.5))
	col.add_child(_volume("sfx_volume", "Громкость эффектов", 0.5))
	var now := Label.new()
	now.name = "NowPlaying"
	now.theme_type_variation = &"MutedLabel"
	now.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(now)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 10)
	var prev := ActionButton.new()
	prev.text = "Предыдущий"
	prev.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	prev.custom_minimum_size = Vector2(0, 52)
	prev.pressed.connect(func() -> void:
		var m := _music()
		if m != null and m.has_method("prev_track"):
			m.prev_track())
	var nxt := ActionButton.new()
	nxt.text = "Следующий"
	nxt.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	nxt.custom_minimum_size = Vector2(0, 52)
	nxt.pressed.connect(func() -> void:
		var m := _music()
		if m != null and m.has_method("next_track"):
			m.next_track())
	row.add_child(prev)
	row.add_child(nxt)
	col.add_child(row)
	var timer := Timer.new()
	timer.wait_time = 0.5
	timer.timeout.connect(func() -> void:
		var m := _music()
		if m == null:
			now.text = ""
			return
		var title := str(m.get("track_title"))
		var st := str(m.get("status"))
		if not UiSettings.get_bool("music_enabled", true):
			now.text = "Музыка выключена"
		elif title != "":
			now.text = "Сейчас играет: %s" % title
		else:
			now.text = st
	)
	timer.autostart = true
	add_child(timer)
	return col


func _volume(key: String, title: String, default: float) -> Control:
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 4)
	var t := Label.new()
	t.text = title
	col.add_child(t)
	var s := HSlider.new()
	s.min_value = 0
	s.max_value = 100
	s.step = 1
	s.custom_minimum_size = Vector2(0, 36)
	s.set_value_no_signal(round(UiSettings.get_float(key, default) * 100.0))
	s.value_changed.connect(func(v: float) -> void:
		UiSettings.set_float(key, v / 100.0)
		_apply_music())
	col.add_child(s)
	return col
