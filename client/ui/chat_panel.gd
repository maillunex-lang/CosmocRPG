extends VBoxContainer
## Окно чата в карточке HUD: вкладки «Общий» и «Личные», журнал сообщений, ввод. Сервер единственный источник сообщений:
## панель только показывает то, что прислал сервер, и отправляет текст игрока. Сообщения ИИ-капитанов помечены иконкой.
## Текст игроков экранируется: разметка BBCode в сообщениях не работает.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")

signal unread_changed(count: int)

const MAX_LINES := 100
const MAX_TEXT := 200

var gateway: Node
var tab := "general"
var unread := 0
var dm_unread := 0
var enabled := true

var _general: Array = []
var _dms: Array = []
var _log: RichTextLabel
var _edit: LineEdit
var _to: LineEdit
var _send: ActionButton
var _error: Label
var _tab_buttons: Dictionary = {}
var _error_timer: SceneTreeTimer


## Экранирование: «[» превращается в «[lb]», чтобы игрок не мог вставить теги разметки.
static func escape(s: String) -> String:
	return s.replace("[", "[lb]")


static func clock(at_ms: int) -> String:
	var bias := int(Time.get_time_zone_from_system().get("bias", 0))
	var d := Time.get_datetime_dict_from_unix_time(int(at_ms / 1000.0) + bias * 60)
	return "%02d:%02d" % [int(d["hour"]), int(d["minute"])]


## Строка журнала в BBCode. my_id — идентификатор своего игрока.
static func format_line(m: Dictionary, my_id: String) -> String:
	var t := "[color=%s]%s[/color] " % [Tokens.hex(Tokens.TEXT_MUTED), clock(int(m.get("at", 0)))]
	var text := escape(str(m.get("text", "")))
	var from := escape(str(m.get("from_name", "")))
	var icon := "[img=16]res://assets/icons/bot.svg[/img] " if bool(m.get("from_ai", false)) else ""
	match str(m.get("channel", "")):
		"system":
			return t + "[color=%s]%s[/color]" % [Tokens.hex(Tokens.RARE), text]
		"dm":
			if str(m.get("from_id", "")) == my_id:
				return t + "[color=%s]→ %s:[/color] %s" % [Tokens.hex(Tokens.ACCENT_2), escape(str(m.get("to_name", ""))), text]
			return t + "%s[color=%s]%s:[/color] %s" % [icon, Tokens.hex(Tokens.ACCENT_2), from, text]
	var col := Tokens.ACCENT if str(m.get("from_id", "")) == my_id else Tokens.TEXT
	return t + "%s[color=%s]%s:[/color] %s" % [icon, Tokens.hex(col), from, text]


func _ready() -> void:
	add_theme_constant_override("separation", 6)
	var tabs := HBoxContainer.new()
	tabs.add_theme_constant_override("separation", 6)
	for pair in [["general", "Общий"], ["dm", "Личные"]]:
		var b := ActionButton.new()
		b.toggle_mode = true
		b.text = pair[1]
		b.custom_minimum_size = Vector2(0, 40)
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
		b.pressed.connect(_select_tab.bind(str(pair[0])))
		tabs.add_child(b)
		_tab_buttons[pair[0]] = b
	add_child(tabs)
	_log = RichTextLabel.new()
	_log.bbcode_enabled = true
	_log.scroll_following = true
	_log.fit_content = false
	_log.custom_minimum_size = Vector2(0, 150)
	_log.add_theme_font_size_override("normal_font_size", Tokens.FONT_SMALL)
	_log.add_theme_font_size_override("bold_font_size", Tokens.FONT_SMALL)
	_log.mouse_filter = Control.MOUSE_FILTER_PASS
	add_child(_log)
	_error = Label.new()
	_error.add_theme_color_override("font_color", Tokens.DANGER)
	_error.add_theme_font_size_override("font_size", Tokens.FONT_SMALL - 2)
	_error.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_error.visible = false
	add_child(_error)
	_to = LineEdit.new()
	_to.placeholder_text = "Кому (ник)"
	_to.max_length = 24
	_to.custom_minimum_size = Vector2(0, 40)
	_to.visible = false
	add_child(_to)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	_edit = LineEdit.new()
	_edit.placeholder_text = "Сообщение…"
	_edit.max_length = MAX_TEXT
	_edit.custom_minimum_size = Vector2(0, 44)
	_edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_edit.text_submitted.connect(func(_t: String) -> void: submit())
	row.add_child(_edit)
	_send = ActionButton.new()
	_send.text = ""
	Icons.apply(_send, "send", Color.WHITE, 22)
	_send.custom_minimum_size = Vector2(52, 44)
	_send.pressed.connect(submit)
	row.add_child(_send)
	add_child(row)
	_select_tab("general")


func bind(gw: Node) -> void:
	gateway = gw
	enabled = gw.has_chat()
	_edit.editable = enabled
	_send.disabled = not enabled
	_edit.placeholder_text = "Сообщение…" if enabled else "Чат только при игре на сервере"
	gw.chat_received.connect(on_message)
	gw.chat_history_loaded.connect(on_history)
	gw.chat_failed.connect(show_error)


func _my_id() -> String:
	return str(gateway.own_id()) if gateway != null and gateway.has_method("own_id") else ""


func on_history(general: Array, dms: Array, _settings: Dictionary) -> void:
	_general = general.slice(maxi(0, general.size() - MAX_LINES))
	_dms = dms.slice(maxi(0, dms.size() - MAX_LINES))
	_redraw()


func on_message(m: Dictionary) -> void:
	var is_dm := str(m.get("channel", "")) == "dm"
	var arr: Array = _dms if is_dm else _general
	arr.append(m)
	if arr.size() > MAX_LINES:
		arr.pop_front()
	var mine := str(m.get("from_id", "")) == _my_id()
	if is_dm and not mine and _to.text.strip_edges() == "":
		_to.text = str(m.get("from_name", ""))   # ответить одним нажатием
	if (is_dm and tab != "dm") or (not is_dm and tab != "general" and not mine):
		if is_dm:
			dm_unread += 1
		else:
			unread += 1
		_refresh_tabs()
		unread_changed.emit(unread + dm_unread)
	_redraw()


func _select_tab(t: String) -> void:
	tab = t
	if t == "dm":
		dm_unread = 0
	else:
		unread = 0
	_to.visible = t == "dm"
	_refresh_tabs()
	unread_changed.emit(unread + dm_unread)
	_redraw()


func _refresh_tabs() -> void:
	for k in _tab_buttons:
		var b: Button = _tab_buttons[k]
		b.set_pressed_no_signal(k == tab)
		var n := dm_unread if k == "dm" else unread
		b.text = ("Личные" if k == "dm" else "Общий") + (" (%d)" % n if n > 0 else "")


func _redraw() -> void:
	if _log == null:
		return
	_log.clear()
	var arr: Array = _dms if tab == "dm" else _general
	var id := _my_id()
	if arr.is_empty():
		_log.append_text("[color=%s]Сообщений пока нет[/color]" % Tokens.hex(Tokens.TEXT_MUTED))
		return
	var lines: PackedStringArray = []
	for m in arr:
		lines.append(format_line(m, id))
	_log.append_text("\n".join(lines))


## Отправка введённого текста. Пустой текст и недоступный чат игнорируются; проверки (длина, частота, фильтр) делает сервер.
func submit() -> void:
	if not enabled or gateway == null:
		return
	var text := _edit.text.strip_edges()
	if text == "":
		return
	var to := _to.text.strip_edges() if tab == "dm" else ""
	if tab == "dm" and to == "":
		show_error("Укажите, кому написать")
		return
	gateway.chat_send("dm" if tab == "dm" else "general", to, text)
	_edit.text = ""
	_error.visible = false


func show_error(text: String) -> void:
	_error.text = text
	_error.visible = true
	_error_timer = get_tree().create_timer(4.0) if is_inside_tree() else null
	if _error_timer != null:
		var mine := _error_timer
		_error_timer.timeout.connect(func() -> void:
			if _error_timer == mine and is_instance_valid(_error):
				_error.visible = false)
