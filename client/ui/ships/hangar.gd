extends Control
## Ангар (минимальная совместимая часть): 3D-предпросмотр корабля, выбор облика и специализации, характеристики.
## Показывает данные сервера (характеристики приходят в состоянии игрока); выбор сохраняет сервер, в демо он только локальный.

const Tokens := preload("res://ui/theme/tokens.gd")
const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal skin_selected(skin_id: String)
signal spec_selected(spec_id: String)
signal closed

var ship_id := ShipDef.DEFAULT_ID
var skin := "a"
var spec := "interceptor"

var _visual: Node3D
var _turntable: Node3D
var _stats: Label
var _skin_buttons: Dictionary = {}
var _spec_buttons: Dictionary = {}
var _info: Label
var _data: Dictionary = {}   # характеристики корабля игрока (от сервера или демо)


func _ready() -> void:
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP   # всё, что под ангаром, не получает касания
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 0.94)
	bg.set_anchors_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var margin := MarginContainer.new()
	margin.set_anchors_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 24)
	add_child(margin)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 20)
	margin.add_child(row)

	# 3D-предпросмотр
	var svc := SubViewportContainer.new()
	svc.stretch = true
	svc.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	svc.size_flags_vertical = Control.SIZE_EXPAND_FILL
	svc.custom_minimum_size = Vector2(420, 300)
	row.add_child(svc)
	var vp := SubViewport.new()
	vp.own_world_3d = true
	vp.transparent_bg = false
	vp.msaa_3d = Viewport.MSAA_DISABLED
	svc.add_child(vp)
	var env := WorldEnvironment.new()
	var e := Environment.new()
	e.background_mode = Environment.BG_COLOR
	e.background_color = Tokens.BG.darkened(0.2)
	e.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	e.ambient_light_color = Color(0.55, 0.6, 0.75)
	env.environment = e
	vp.add_child(env)
	var light := DirectionalLight3D.new()
	light.rotation_degrees = Vector3(-45, -35, 0)
	vp.add_child(light)
	var cam := Camera3D.new()
	cam.position = Vector3(0, 2.6, 6.0)
	cam.rotation_degrees = Vector3(-18, 0, 0)
	cam.fov = 38.0
	vp.add_child(cam)
	_turntable = Node3D.new()
	vp.add_child(_turntable)
	_visual = ShipVisual.new()
	_visual.ship_id = ship_id
	_turntable.add_child(_visual)
	_visual.build(ship_id, skin)

	# панель справа
	var side := PanelContainer.new()
	side.custom_minimum_size = Vector2(340, 0)
	row.add_child(side)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 8)
	side.add_child(box)
	var def := ShipDef.get_def(ship_id)
	var title := Label.new()
	title.text = "%s — %s" % [def.get("name", ""), def.get("class", "")]
	if bool(def.get("temporary", false)):
		title.text += " (временная модель)"
	title.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 4)
	box.add_child(title)
	_info = Label.new()
	_info.theme_type_variation = &"MutedLabel"
	_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_info)
	_stats = Label.new()
	_stats.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_stats)
	box.add_child(_caption("Облик (характеристики одинаковые)"))
	var skins := HBoxContainer.new()
	box.add_child(skins)
	for k in ShipDef.skin_ids(ship_id):
		var b := ActionButton.new()
		b.text = str(ShipDef.skin(ship_id, k).get("name", k))
		b.toggle_mode = true
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.pressed.connect(_pick_skin.bind(str(k)))
		skins.add_child(b)
		_skin_buttons[str(k)] = b
	box.add_child(_caption("Специализация"))
	var specs := HBoxContainer.new()
	box.add_child(specs)
	for k in (def.get("specs", {}) as Dictionary).keys():
		var b2 := ActionButton.new()
		b2.text = str(def["specs"][k].get("name", k))
		b2.toggle_mode = true
		b2.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b2.pressed.connect(_pick_spec.bind(str(k)))
		specs.add_child(b2)
		_spec_buttons[str(k)] = b2
	var dock := ActionButton.new()
	dock.text = "Показать стыковку"
	dock.pressed.connect(func() -> void:
		_visual.undock()
		_visual.play("Dock"))
	box.add_child(dock)
	var close := ActionButton.new()
	close.text = "Закрыть"
	close.variant = ActionButton.Variant.PRIMARY
	close.pressed.connect(func() -> void: closed.emit())
	box.add_child(close)
	_refresh()


func _caption(t: String) -> Label:
	var l := Label.new()
	l.text = t
	l.theme_type_variation = &"MutedLabel"
	return l


## Данные корабля игрока: type, skin, spec, hull_max, shield_max, armor, energy_max, cargo (если есть).
func set_data(ship: Dictionary) -> void:
	_data = ship
	if str(ship.get("skin", "")) != "":
		skin = str(ship["skin"])
		_visual.set_skin(skin)
	if str(ship.get("spec", "")) != "":
		spec = str(ship["spec"])
	_refresh()


func _pick_skin(k: String) -> void:
	skin = k
	_visual.undock()
	_visual.set_skin(k)
	_refresh()
	skin_selected.emit(k)


func _pick_spec(k: String) -> void:
	spec = k
	_refresh()
	spec_selected.emit(k)


func _refresh() -> void:
	var def := ShipDef.get_def(ship_id)
	var d: Array = def.get("dimensions_m", [0, 0, 0])
	_info.text = "Экипаж %d · двигателей %d · %s\nГабариты %s × %s × %s м" % [int(def.get("crew", 1)), int(def.get("engines", 2)), str(def.get("weapons", "")),
		str(d[0]), str(d[1]), str(d[2])]
	if _data.has("hull_max"):
		_stats.text = "Корпус %d · щит %d · броня %d\nЭнергия %d · груз %d\nХарактеристики определяет сервер, значения балансируются." % [
			int(_data.get("hull_max", 0)), int(_data.get("shield_max", 0)), int(_data.get("armor", 0)), int(_data.get("energy_max", 0)), int(_data.get("cargo", 0))]
	else:
		_stats.text = "Характеристики недоступны: сервер их не прислал."
	for k in _skin_buttons:
		(_skin_buttons[k] as Button).set_pressed_no_signal(k == skin)
	for k in _spec_buttons:
		var b := _spec_buttons[k] as Button
		b.set_pressed_no_signal(k == spec)
		b.tooltip_text = str((def.get("specs", {}) as Dictionary).get(k, {}).get("ability", ""))


func _process(delta: float) -> void:
	_turntable.rotation.y += delta * 0.6
	_visual.update_motion(delta, 0.4, 0.0, false)
