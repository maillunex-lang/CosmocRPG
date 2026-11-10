extends Control
## Друзья: добавить пилота, написать лично, позвать в пати.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal closed
signal write_requested(nick: String)

var gateway: Node
var _list: VBoxContainer
var _status: Label
var _nick: LineEdit


func bind(gw: Node) -> void:
	gateway = gw
	if gateway.has_signal("friends_updated") and not gateway.friends_updated.is_connected(_show):
		gateway.friends_updated.connect(_show)
	if gateway.has_method("friend_list"):
		gateway.friend_list()


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
	panel.custom_minimum_size = Vector2(680, 0)
	center.add_child(panel)
	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(640, 560)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	box.custom_minimum_size = Vector2(620, 0)
	scroll.add_child(box)
	var title := Label.new()
	title.text = "Друзья"
	title.theme_type_variation = &"TitleLabel"
	box.add_child(title)
	var about := Label.new()
	about.text = "Добавьте пилота по нику или текущую цель. Ему можно написать лично и позвать в пати."
	about.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	about.theme_type_variation = &"MutedLabel"
	box.add_child(about)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	_nick = LineEdit.new()
	_nick.placeholder_text = "Ник пилота"
	_nick.custom_minimum_size = Vector2(0, 52)
	_nick.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(_nick)
	var add := ActionButton.new()
	add.text = "Добавить"
	add.custom_minimum_size = Vector2(160, 52)
	add.pressed.connect(_add_nick)
	row.add_child(add)
	box.add_child(row)
	var target := ActionButton.new()
	target.text = "Добавить выбранную цель"
	target.custom_minimum_size = Vector2(0, 52)
	target.pressed.connect(_add_target)
	box.add_child(target)
	_status = Label.new()
	_status.theme_type_variation = &"MutedLabel"
	_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_status)
	_list = VBoxContainer.new()
	_list.add_theme_constant_override("separation", 8)
	box.add_child(_list)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.custom_minimum_size = Vector2(0, 56)
	close.pressed.connect(func() -> void: closed.emit())
	box.add_child(close)


func _add_nick() -> void:
	var nick := _nick.text.strip_edges()
	if nick == "":
		_status.text = "Введите ник."
		return
	_call_add(nick)


func _add_target() -> void:
	var st: Dictionary = gateway.snapshot() if gateway != null and gateway.has_method("snapshot") else {}
	var id := str(st.get("target_id", ""))
	if id == "":
		_status.text = "Сначала выберите пилота рядом."
		return
	_call_add(id)


func _call_add(q: String) -> void:
	if gateway == null or not gateway.has_method("friend_add"):
		_status.text = "Друзья доступны в игре на сервере."
		return
	gateway.friend_add(q)
	_nick.text = ""


func _show(list: Array) -> void:
	if _list == null:
		return
	for c in _list.get_children():
		c.queue_free()
	if list.is_empty():
		var empty := Label.new()
		empty.text = "Список пуст."
		empty.theme_type_variation = &"MutedLabel"
		_list.add_child(empty)
		return
	for raw in list:
		if not (raw is Dictionary):
			continue
		var f: Dictionary = raw
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 6)
		var name := Label.new()
		name.text = str(f.get("name", "пилот"))
		name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(name)
		var write := ActionButton.new()
		write.text = "Написать"
		write.custom_minimum_size = Vector2(120, 48)
		var nick := str(f.get("name", ""))
		var id := str(f.get("id", ""))
		write.pressed.connect(_write.bind(nick))
		row.add_child(write)
		var party := ActionButton.new()
		party.text = "В пати"
		party.custom_minimum_size = Vector2(110, 48)
		party.pressed.connect(_invite.bind(id))
		row.add_child(party)
		var drop := ActionButton.new()
		drop.text = "Убрать"
		drop.custom_minimum_size = Vector2(100, 48)
		drop.pressed.connect(_remove.bind(id))
		row.add_child(drop)
		_list.add_child(row)


func _write(nick: String) -> void:
	write_requested.emit(nick)


func _invite(id: String) -> void:
	if gateway != null and gateway.has_method("party_invite"):
		gateway.party_invite(id)


func _remove(id: String) -> void:
	if gateway != null and gateway.has_method("friend_remove"):
		gateway.friend_remove(id)
