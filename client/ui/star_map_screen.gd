extends Control
## Звёздная карта (Э-1): системы галактики прототипа, фракции, точки прыжков текущей системы.
## Прыжок отправляет серверу jump {to}; сервер проверяет близость к точке (no_gate/no_link — тост ошибки).

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

signal closed

var _gateway: UiGateway
var _rows: VBoxContainer
var _notice: Label


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 0.96)
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

	var head := HBoxContainer.new()
	col.add_child(head)
	head.add_child(Icons.rect("globe", 32, Tokens.ACCENT))
	var title := Label.new()
	title.text = "Звёздная карта"
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x")
	close.custom_minimum_size = Vector2(140, 52)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)

	_notice = Label.new()
	_notice.theme_type_variation = &"MutedLabel"
	_notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(_notice)

	var scroll := ScrollContainer.new()
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	col.add_child(scroll)
	_rows = VBoxContainer.new()
	_rows.add_theme_constant_override("separation", 8)
	_rows.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	scroll.add_child(_rows)
	_rebuild()


func bind(gw: UiGateway) -> void:
	_gateway = gw
	if not gw.system_changed.is_connected(_on_system_changed):
		gw.system_changed.connect(_on_system_changed)
	_rebuild()


func _on_system_changed(_info: Dictionary) -> void:
	_rebuild()   # после прыжка: новая текущая система и точки
	_notice.text = "Гиперпереход выполнен."


func _rebuild() -> void:
	if _rows == null:
		return
	for c in _rows.get_children():
		c.queue_free()
	if _gateway == null:
		return
	var current := str(_gateway.snapshot().get("system", {}).get("id", ""))
	var ship: Dictionary = _gateway.snapshot().get("ship", {})
	var pos: Vector2 = ship.get("pos", Vector2.ZERO)
	for sys in _gateway.star_systems():
		var panel := PanelContainer.new()
		panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_rows.add_child(panel)
		var box := HBoxContainer.new()
		box.add_theme_constant_override("separation", 10)
		panel.add_child(box)
		var here := str(sys.get("id", "")) == current
		box.add_child(Icons.rect("globe", 26, Tokens.ACCENT if here else Tokens.TEXT_MUTED))
		var col := VBoxContainer.new()
		col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		box.add_child(col)
		var name := Label.new()
		name.text = str(sys.get("name", sys.get("id", ""))) + ("  · вы здесь" if here else "")
		name.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
		col.add_child(name)
		var info := Label.new()
		info.theme_type_variation = &"MutedLabel"
		info.text = str(sys.get("faction", ""))
		col.add_child(info)
		if here:
			# точки прыжков текущей системы: расстояние и кнопка
			for g in _gateway.jump_gates():
				var to := str(g.get("to", ""))
				var gpos := Vector2(float(g.get("pos", [0, 0])[0]), float(g.get("pos", [0, 0])[1]))
				var row := HBoxContainer.new()
				row.add_theme_constant_override("separation", 8)
				col.add_child(row)
				var gl := Label.new()
				gl.size_flags_horizontal = Control.SIZE_EXPAND_FILL
				gl.text = "Переход: %s · %.0f ед." % [_system_name(to), pos.distance_to(gpos)]
				row.add_child(gl)
				var jb := ActionButton.new()
				jb.text = "Прыгнуть"
				Icons.apply(jb, "rocket", Tokens.ACCENT, 20)
				jb.pressed.connect(_on_jump.bind(to))
				row.add_child(jb)
	if _gateway.jump_gates().is_empty():
		_notice.text = "В этой системе нет точек гиперперехода."
	elif _notice.text == "":
		_notice.text = "Подлетите к точке перехода и нажмите «Прыгнуть»."


func _system_name(id: String) -> String:
	for sys in _gateway.star_systems():
		if str(sys.get("id", "")) == id:
			return str(sys.get("name", id))
	return id


func _on_jump(to: String) -> void:
	if _gateway != null:
		_gateway.jump(to)
