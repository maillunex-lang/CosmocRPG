extends Control
## Создание персонажа при первом входе: ник, пол, стартовое направление пилота (определяет стартовый корабль).
## Направление пилота не заменяет классификацию кораблей: у каждого корабля остаются свои специализации и роли.
## Окончательную проверку ника делает сервер; здесь только быстрая проверка формата.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal submitted(nick: String, gender: String, pilot: String)

const PILOTS := {
	"interceptor": {"name": "Перехватчик", "about": "Боевые действия, скорость, манёвренность."},
	"scout": {"name": "Разведчик", "about": "Исследование, сканирование, поиск аномалий."},
	"trader": {"name": "Торговец", "about": "Торговля, логистика, грузоперевозки."},
	"miner": {"name": "Шахтёр", "about": "Добыча и переработка ресурсов."},
}

var options: Array = []   # стартовые корабли от сервера: [{pilot, ship, ship_name, class, role, temporary, hull, shield, cargo, speed}]
var gender := ""
var pilot := ""

var _nick: LineEdit
var _error: Label
var _start: ActionButton
var _gender_buttons: Dictionary = {}
var _pilot_buttons: Dictionary = {}


## Быстрая проверка формата ника (как на сервере: 3-16 символов, буквы, цифры, «_» и «-» внутри). "" — ник подходит.
static func nick_problem(s: String) -> String:
	s = s.strip_edges()
	if s.length() < 3 or s.length() > 16:
		return "Ник: от 3 до 16 символов."
	var letters := 0
	for i in s.length():
		var c := s.unicode_at(i)
		var is_letter := (c >= 97 and c <= 122) or (c >= 65 and c <= 90) or (c >= 0x430 and c <= 0x44F) or (c >= 0x410 and c <= 0x42F) or c == 0x451 or c == 0x401
		if is_letter:
			letters += 1
		elif c >= 48 and c <= 57:
			pass
		elif (c == 95 or c == 45) and i > 0 and i < s.length() - 1:
			pass
		else:
			return "В нике допустимы буквы, цифры, «_» и «-» внутри."
	if letters == 0:
		return "В нике нужна хотя бы одна буква."
	return ""


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 1.0)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var margin := MarginContainer.new()
	margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 20)
	add_child(margin)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 10)
	margin.add_child(col)
	var title := Label.new()
	title.text = "Создание пилота"
	title.theme_type_variation = &"TitleLabel"
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	col.add_child(title)

	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 12)
	col.add_child(row)
	_nick = LineEdit.new()
	_nick.placeholder_text = "Ник (3-16 символов)"
	_nick.max_length = 16
	_nick.custom_minimum_size = Vector2(0, 56)
	_nick.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_nick.text_changed.connect(func(_t: String) -> void: _update())
	row.add_child(_nick)
	for g in [["m", "Мужчина"], ["f", "Женщина"]]:
		var b := ActionButton.new()
		b.text = g[1]
		b.toggle_mode = true
		b.custom_minimum_size = Vector2(130, 56)
		b.pressed.connect(_pick_gender.bind(str(g[0])))
		row.add_child(b)
		_gender_buttons[g[0]] = b

	var hint := Label.new()
	hint.text = "Стартовое направление определяет первый корабль. Позже вы сможете развиваться в любую сторону."
	hint.theme_type_variation = &"MutedLabel"
	hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(hint)
	var grid := GridContainer.new()
	grid.columns = 2
	grid.add_theme_constant_override("h_separation", 10)
	grid.add_theme_constant_override("v_separation", 10)
	grid.size_flags_vertical = Control.SIZE_EXPAND_FILL
	col.add_child(grid)
	for k in PILOTS.keys():
		var o := _option(str(k))
		var b2 := ActionButton.new()
		b2.toggle_mode = true
		b2.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b2.size_flags_vertical = Control.SIZE_EXPAND_FILL
		b2.custom_minimum_size = Vector2(0, 84)
		var ship_line := ""
		if not o.is_empty():
			ship_line = "\nКорабль: %s%s" % [str(o.get("ship_name", "")), " (временная модель)" if bool(o.get("temporary", false)) else ""]
		b2.text = "%s\n%s%s" % [PILOTS[k]["name"], PILOTS[k]["about"], ship_line]
		b2.pressed.connect(_pick_pilot.bind(str(k)))
		grid.add_child(b2)
		_pilot_buttons[k] = b2

	_error = Label.new()
	_error.add_theme_color_override("font_color", Tokens.DANGER)
	_error.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(_error)
	_start = ActionButton.new()
	_start.text = "Начать игру"
	_start.variant = ActionButton.Variant.PRIMARY
	_start.custom_minimum_size = Vector2(0, 56)
	_start.pressed.connect(_submit)
	col.add_child(_start)
	_update()


func _option(pilot_id: String) -> Dictionary:
	for o in options:
		if o is Dictionary and str(o.get("pilot", "")) == pilot_id:
			return o
	return {}


func _pick_gender(g: String) -> void:
	gender = g
	for k in _gender_buttons:
		(_gender_buttons[k] as Button).set_pressed_no_signal(k == g)
	_update()


func _pick_pilot(p: String) -> void:
	pilot = p
	for k in _pilot_buttons:
		(_pilot_buttons[k] as Button).set_pressed_no_signal(k == p)
	_update()


func ready_to_submit() -> bool:
	return nick_problem(_nick.text) == "" and gender != "" and pilot != ""


func _update() -> void:
	if _start != null:
		_start.disabled = not ready_to_submit()


func _submit() -> void:
	var p := nick_problem(_nick.text)
	if p != "":
		show_error(p)
		return
	if gender == "" or pilot == "":
		show_error("Выберите пол и стартовое направление.")
		return
	_error.text = ""
	_start.begin_busy("Создание…")
	submitted.emit(_nick.text.strip_edges(), gender, pilot)


## Ошибка от сервера (ник занят и т. п.): кнопка снова доступна.
func show_error(text: String) -> void:
	_error.text = text
	_start.end_busy()
	_update()


func set_nick(s: String) -> void:
	_nick.text = s
	_update()
