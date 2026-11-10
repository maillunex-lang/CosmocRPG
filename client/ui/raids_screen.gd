extends Control
## Коридоры Рубежа: три закрытых вылета. Отряд прыгает только после согласия каждого.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal closed

const OPS := [
	{"id": "credits", "title": "Казна", "about": "Три зала. В конце — кредиты.", "pay": "Награда: кредиты", "color": Tokens.RARE},
	{"id": "gear", "title": "Арсенал", "about": "Три зала. В конце — обшивка «Сталь».", "pay": "Награда: обшивка", "color": Tokens.ACCENT},
	{"id": "loot", "title": "Трюм", "about": "Три зала. В конце — руда и кристаллы.", "pay": "Награда: руда и кристаллы", "color": Tokens.SUCCESS},
]

var gateway: Node
var _status: Label
var _squad: Label
var _cards: Dictionary = {}
var _agree: Button


func bind(gw: Node) -> void:
	gateway = gw
	if not gateway.state_updated.is_connected(_on_state):
		gateway.state_updated.connect(_on_state)
	_on_state(gateway.snapshot())


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
	panel.custom_minimum_size = Vector2(720, 0)
	center.add_child(panel)
	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(680, 640)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 12)
	box.custom_minimum_size = Vector2(660, 0)
	scroll.add_child(box)
	var title := Label.new()
	title.text = "Коридоры Рубежа"
	title.theme_type_variation = &"TitleLabel"
	box.add_child(title)
	var about := Label.new()
	about.text = "Закрытый вылет отряда. Три зала подряд: дверь открывается, когда зал зачищен. Прыжок — после согласия каждого."
	about.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	about.theme_type_variation = &"MutedLabel"
	box.add_child(about)
	var halls := HBoxContainer.new()
	halls.add_theme_constant_override("separation", 8)
	for i in 3:
		var hall := Label.new()
		hall.text = "Зал %d" % (i + 1)
		hall.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
		hall.add_theme_color_override("font_color", Tokens.ACCENT)
		halls.add_child(hall)
	box.add_child(halls)
	for op in OPS:
		box.add_child(_card(op))
	_squad = Label.new()
	_squad.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_squad)
	_status = Label.new()
	_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_status.theme_type_variation = &"MutedLabel"
	box.add_child(_status)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	var invite := ActionButton.new()
	invite.text = "Позвать цель"
	invite.custom_minimum_size = Vector2(0, 56)
	invite.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	invite.pressed.connect(_invite)
	row.add_child(invite)
	_agree = ActionButton.new()
	_agree.text = "Дать согласие"
	_agree.variant = ActionButton.Variant.PRIMARY
	_agree.custom_minimum_size = Vector2(0, 56)
	_agree.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_agree.pressed.connect(_ready_vote)
	row.add_child(_agree)
	box.add_child(row)
	var leave := ActionButton.new()
	leave.text = "Покинуть отряд"
	leave.custom_minimum_size = Vector2(0, 52)
	leave.pressed.connect(func() -> void: _call("party_leave"))
	box.add_child(leave)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(0, 56)
	close.pressed.connect(func() -> void: closed.emit())
	box.add_child(close)


func _card(op: Dictionary) -> Control:
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(0, 116)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 12)
	panel.add_child(row)
	var stripe := ColorRect.new()
	stripe.custom_minimum_size = Vector2(8, 0)
	stripe.color = op["color"]
	stripe.mouse_filter = Control.MOUSE_FILTER_IGNORE
	row.add_child(stripe)
	var col := VBoxContainer.new()
	col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	col.add_theme_constant_override("separation", 2)
	row.add_child(col)
	var name := Label.new()
	name.text = str(op["title"])
	name.add_theme_font_size_override("font_size", Tokens.FONT_H2)
	col.add_child(name)
	var about := Label.new()
	about.text = str(op["about"])
	about.theme_type_variation = &"MutedLabel"
	about.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(about)
	var pay := Label.new()
	pay.text = str(op["pay"])
	pay.add_theme_color_override("font_color", op["color"])
	col.add_child(pay)
	var pick := ActionButton.new()
	pick.text = "Выбрать"
	pick.custom_minimum_size = Vector2(148, 56)
	var id := str(op["id"])
	_cards[id] = pick
	pick.pressed.connect(func() -> void: _arm(id))
	row.add_child(pick)
	return panel


func _arm(id: String) -> void:
	_call("party_arm", id)


func _invite() -> void:
	var st: Dictionary = gateway.snapshot() if gateway != null else {}
	var target := str(st.get("target_id", ""))
	if target == "":
		_status.text = "Сначала выберите пилота или капитана рядом."
		return
	_call("party_invite", target)


func _ready_vote() -> void:
	_call("party_ready")


func _call(method: String, arg: String = "") -> void:
	if gateway == null or not gateway.has_method(method):
		_status.text = "Коридоры доступны в игре на сервере."
		return
	if arg == "":
		gateway.call(method)
	else:
		gateway.call(method, arg)


func _on_state(state: Dictionary) -> void:
	if _squad == null:
		return
	var party: Dictionary = state.get("party", {}) if state.get("party") is Dictionary else {}
	if bool(party.get("instance", false)):
		closed.emit()
		return
	var op := str(party.get("op", ""))
	for id in _cards:
		var b := _cards[id] as ActionButton
		var on: bool = id == op
		b.variant = ActionButton.Variant.PRIMARY if on else ActionButton.Variant.SECONDARY
	var lines := "Отряд ещё не собран. Позовите цель рядом, затем выберите коридор."
	var members: Array = party.get("members", []) if party.get("members") is Array else []
	if members.size() > 0:
		lines = "Отряд"
		for m in members:
			if m is Dictionary:
				var nm := str(m.get("name", "пилот"))
				if bool(m.get("leader", false)):
					nm += " · ведущий"
				if bool(m.get("pending", false)):
					nm += " · ждёт ответ"
				elif bool(m.get("ready", false)):
					nm += " · согласен"
				lines += "\n" + nm
	_squad.text = lines
	var goal := str(party.get("goal", ""))
	if goal != "":
		_status.text = goal
	elif _status.text == "":
		_status.text = "Выберите казну, арсенал или трюм. Прыжок начнётся сам, когда согласны все."
	if _agree != null:
		_agree.disabled = bool(party.get("you_ready", false)) or bool(party.get("instance", false))
		_agree.text = "Согласие принято" if bool(party.get("you_ready", false)) else "Дать согласие"
