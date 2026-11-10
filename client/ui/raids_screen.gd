extends Control
## Коридоры Рубежа: три закрытых вылета. Отряд прыгает только после согласия каждого.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal closed

const OPS := [
	{"id": "credits", "title": "Казна", "about": "Три зала. Награда — кредиты."},
	{"id": "gear", "title": "Арсенал", "about": "Три зала. Награда — обшивка «Сталь»."},
	{"id": "loot", "title": "Трюм", "about": "Три зала. Награда — руда и кристаллы."},
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
	about.text = "Как партийное подземелье: собрали отряд, каждый дал согласие, общий гиперпрыжок. Коридор из трёх залов. Дверь в следующий открывается, только когда текущий зал зачищен."
	about.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	about.theme_type_variation = &"MutedLabel"
	box.add_child(about)
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
	var b := ActionButton.new()
	b.text = "%s\n%s" % [str(op["title"]), str(op["about"])]
	b.custom_minimum_size = Vector2(0, 72)
	b.alignment = HORIZONTAL_ALIGNMENT_LEFT
	var id := str(op["id"])
	_cards[id] = b
	b.pressed.connect(func() -> void: _arm(id))
	return b


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
