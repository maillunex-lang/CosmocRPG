extends Control
## Вступление нового капитана: кто он и как попал на «Рубеж», затем стрелки диктора по интерфейсу.
## Показывается один раз, пока контракт «Первый полёт» ещё не начат. «Пропустить» сверху закрывает навсегда.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

const SAVE := "user://briefing.cfg"

const STEPS := [
	{
		"title": "Кто вы",
		"text": "Вы — вольный капитан. Свой корабль вы оставили на прошлой трассе: трюм пуст, допуск просрочен. Станция «Рубеж» в системе Аврора выдала старый истребитель и одно условие — спокойный первый рейс. Это не геройский контракт. Вас просто снова пустили в небо.",
		"node": "",
	},
	{
		"title": "Штурвал",
		"text": "Левая часть экрана — штурвал. Проведите пальцем: корабль полетит в эту сторону. Отпустите — он остановится.",
		"node": "Joystick",
	},
	{
		"title": "Вы и корабль",
		"text": "Слева сверху — имя, уровень и корабль. Ниже щит, корпус и энергия. Если корпус кончится, корабль встанет на ремонт.",
		"node": "ProfileCard",
	},
	{
		"title": "Задание",
		"text": "Карточка «Задания» — ваш контракт. Первый: допуск к вылету. Диспетчер Ирина Ларс ждёт у станции «Рубеж».",
		"node": "QuestsCard",
	},
	{
		"title": "Карта",
		"text": "Глобус справа сверху открывает карту системы. Нажмите на станцию или пустое место — и можно строить маршрут.",
		"node": "MapHudButton",
	},
	{
		"title": "Меню",
		"text": "Кнопка меню: трюм, ангар, вылеты, друзья, станция. Трюм — вещи корабля. Вылеты — закрытые коридоры отрядом.",
		"node": "MenuButton",
	},
	{
		"title": "Цель и огонь",
		"text": "Коснитесь корабля в космосе — он станет целью. «Автопилот» ведёт к ней. «Атаковать» бьёт только врагов. Чужих пилотов бить нельзя.",
		"node": "AttackButton",
	},
]

var _hud: Node
var _pid := ""
var _step := -1
var _card: PanelContainer
var _title: Label
var _body: Label
var _next: Button
var _skip: Button


static func should_offer(quests: Array, already: bool) -> bool:
	if already:
		return false
	var fresh := false
	for q in quests:
		if q is Dictionary and str(q.get("id", "")) == "first_flight" and int(q.get("progress", 0)) == 0:
			fresh = true
		if q is Dictionary and int(q.get("progress", 0)) > 0:
			return false
	return fresh


func start(gateway: Node, hud: Node) -> void:
	_hud = hud
	if gateway != null and gateway.has_signal("state_updated") and not gateway.state_updated.is_connected(_on_state):
		gateway.state_updated.connect(_on_state)
	_on_state(gateway.snapshot() if gateway != null and gateway.has_method("snapshot") else {})


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	visible = false
	z_index = 40
	_skip = ActionButton.new()
	_skip.text = "Пропустить"
	_skip.custom_minimum_size = Vector2(168, 48)
	_skip.pressed.connect(_finish)
	add_child(_skip)
	_card = PanelContainer.new()
	_card.custom_minimum_size = Vector2(360, 0)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 8)
	_card.add_child(box)
	var who := Label.new()
	who.text = "Бортовой канал"
	who.add_theme_color_override("font_color", Tokens.ACCENT)
	who.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	box.add_child(who)
	_title = Label.new()
	_title.add_theme_font_size_override("font_size", Tokens.FONT_H2)
	box.add_child(_title)
	_body = Label.new()
	_body.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_body.custom_minimum_size = Vector2(340, 0)
	box.add_child(_body)
	_next = ActionButton.new()
	_next.text = "Дальше"
	_next.variant = ActionButton.Variant.PRIMARY
	_next.custom_minimum_size = Vector2(0, 56)
	_next.pressed.connect(_advance)
	box.add_child(_next)
	add_child(_card)
	var style := StyleBoxFlat.new()
	style.bg_color = Color(Tokens.SURFACE, 0.96)
	style.border_color = Tokens.ACCENT
	style.set_border_width_all(1)
	style.set_corner_radius_all(16)
	style.content_margin_left = 16
	style.content_margin_right = 16
	style.content_margin_top = 14
	style.content_margin_bottom = 14
	_card.add_theme_stylebox_override("panel", style)


func _on_state(state: Dictionary) -> void:
	if _step >= 0 or not visible and _pid == "done":
		return
	var gw: Node = get_parent().get("gateway") if get_parent() != null else null
	var pid := ""
	if gw != null:
		pid = str(gw.get("player_id"))
	if pid == "":
		return
	var quests: Array = state.get("quests", []) if state.get("quests") is Array else []
	if not should_offer(quests, _saved(pid)):
		return
	_pid = pid
	_step = 0
	visible = true
	mouse_filter = Control.MOUSE_FILTER_STOP
	_apply()


func _advance() -> void:
	_step += 1
	if _step >= STEPS.size():
		_finish()
		return
	_apply()


func _apply() -> void:
	var step: Dictionary = STEPS[_step]
	_title.text = str(step["title"])
	_body.text = str(step["text"])
	_next.text = "В полёт" if _step == STEPS.size() - 1 else "Дальше"
	_layout()
	queue_redraw()


func _layout() -> void:
	_skip.position = Vector2(size.x * 0.5 - 84.0, 10.0)
	var node := _target()
	var wide := minf(460.0, size.x - 32.0)
	_card.custom_minimum_size = Vector2(wide, 0)
	_card.size = Vector2(wide, 0)
	_card.reset_size()
	if node == null:
		_card.position = Vector2((size.x - wide) * 0.5, size.y * 0.28)
		return
	var r := node.get_global_rect()
	var center := r.position + r.size * 0.5
	var x := 16.0 if center.x > size.x * 0.5 else size.x - wide - 16.0
	var y := clampf(center.y - 40.0, 68.0, maxf(68.0, size.y - 280.0))
	_card.position = Vector2(x, y)


func _target() -> Control:
	if _hud == null or _step < 0 or _step >= STEPS.size():
		return null
	var name := str(STEPS[_step].get("node", ""))
	if name == "":
		return null
	var n := _hud.get_node_or_null("%" + name)
	return n as Control


func _draw() -> void:
	if _step < 0:
		return
	draw_rect(Rect2(Vector2.ZERO, size), Color(0.02, 0.04, 0.07, 0.72))
	var node := _target()
	if node == null or _card == null:
		return
	var r := node.get_global_rect()
	var origin := get_global_rect().position
	var local := r.position - origin
	var ring := Rect2(local - Vector2(8, 8), r.size + Vector2(16, 16))
	draw_rect(ring, Color(Tokens.ACCENT, 0.95), false, 3.0)
	var tip := local + r.size * 0.5
	var from := _card.position + Vector2(_card.size.x * 0.5, _card.size.y * 0.5)
	draw_line(from, tip, Color(Tokens.ACCENT, 0.95), 3.0)
	var dir := (tip - from).normalized()
	if dir.length() < 0.5:
		return
	var side := Vector2(-dir.y, dir.x)
	draw_colored_polygon(PackedVector2Array([tip, tip - dir * 18.0 + side * 8.0, tip - dir * 18.0 - side * 8.0]), Color(Tokens.ACCENT, 0.95))


func _notification(what: int) -> void:
	if what == NOTIFICATION_RESIZED and _step >= 0:
		_layout()


func _finish() -> void:
	if _pid != "" and _pid != "done":
		var f := ConfigFile.new()
		f.load(SAVE)
		f.set_value("done", _pid, true)
		f.save(SAVE)
	_pid = "done"
	_step = -1
	visible = false
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	queue_redraw()


func _saved(pid: String) -> bool:
	var f := ConfigFile.new()
	if f.load(SAVE) != OK:
		return false
	return bool(f.get_value("done", pid, false))
