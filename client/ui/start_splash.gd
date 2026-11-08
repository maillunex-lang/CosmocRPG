extends Control
## Стартовый экран: логотип, «ИГРАТЬ», статус связи, «Аккаунт». Адрес сервера и служебные сведения не показываются.
## Только оформление и сигналы: сетевую логику ведёт экран подключения (connect_screen.gd).

const UiTheme := preload("res://ui/theme/ui_theme.gd")
const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const SafeAreaRoot := preload("res://ui/safe_area_root.gd")
const BgShader := preload("res://ui/start_bg.gdshader")

signal play_pressed
signal retry_pressed
signal account_pressed
signal secret_opened   # пять быстрых нажатий на логотип: служебный экран

enum State { CONNECTING, READY, ERROR, ENTERING }

const SECRET_TAPS := 5
const SECRET_WINDOW_MS := 2500

var state := State.CONNECTING
var play_button: ActionButton
var account_button: ActionButton
var retry_button: ActionButton
var status_label: Label

var safe: SafeAreaRoot
var _bg: ColorRect
var _mat: ShaderMaterial
var _spinner: Control
var _glow: Panel
var _content: Control
var _taps: Array[int] = []
var _t := 0.0
var _animate := true


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_animate = DisplayServer.get_name() != "headless"
	_bg = ColorRect.new()
	_bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_mat = ShaderMaterial.new()
	_mat.shader = BgShader
	_bg.material = _mat
	add_child(_bg)

	safe = SafeAreaRoot.new()
	safe.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	safe.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(safe)
	_content = VBoxContainer.new()
	_content.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_content.add_theme_constant_override("separation", 0)
	safe.add_child(_content)

	_content.add_child(_spacer())
	_content.add_child(_logo())
	_content.add_child(_spacer())
	_content.add_child(_play_block())
	_content.add_child(_bottom_bar())
	set_state(State.CONNECTING, "Подключение к серверу…")
	if _animate:
		_intro()


func _spacer() -> Control:
	var c := Control.new()
	c.size_flags_vertical = Control.SIZE_EXPAND_FILL
	c.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return c


func _label(text: String, size: int, weight: int, color: Color, spacing: float = 0.0) -> Label:
	var l := Label.new()
	l.text = text
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var f := UiTheme.make_font(weight).duplicate() as FontVariation
	f.spacing_glyph = int(spacing)
	l.add_theme_font_override("font", f)
	l.add_theme_font_size_override("font_size", size)
	l.add_theme_color_override("font_color", color)
	return l


func _logo() -> Control:
	var box := VBoxContainer.new()
	box.name = "Logo"
	box.add_theme_constant_override("separation", 2)
	box.mouse_filter = Control.MOUSE_FILTER_STOP   # нажатия на логотип считаются для служебного экрана
	box.gui_input.connect(_on_logo_input)
	var cap := _label("КОСМИЧЕСКАЯ  MMORPG", 17, 600, Color(Tokens.ACCENT, 0.9), 6)
	box.add_child(cap)
	var l1 := _label("ЗВЁЗДНЫЕ", 58, 700, Tokens.TEXT, 14)
	l1.add_theme_color_override("font_outline_color", Color(Tokens.ACCENT, 0.35))
	l1.add_theme_constant_override("outline_size", 2)
	l1.add_theme_color_override("font_shadow_color", Color(Tokens.ACCENT, 0.45))
	l1.add_theme_constant_override("shadow_offset_y", 0)
	l1.add_theme_constant_override("shadow_outline_size", 14)
	box.add_child(l1)
	var l2 := _label("РУБЕЖИ", 104, 800, Color.WHITE, 22)
	l2.add_theme_color_override("font_outline_color", Color(Tokens.ACCENT, 0.5))
	l2.add_theme_constant_override("outline_size", 3)
	l2.add_theme_color_override("font_shadow_color", Color(Tokens.ACCENT_2, 0.55))
	l2.add_theme_constant_override("shadow_outline_size", 22)
	box.add_child(l2)
	var line := ColorRect.new()
	line.custom_minimum_size = Vector2(0, 2)
	line.color = Color(Tokens.ACCENT, 0.75)
	line.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var lw := CenterContainer.new()
	lw.mouse_filter = Control.MOUSE_FILTER_IGNORE
	line.custom_minimum_size = Vector2(320, 2)
	lw.add_child(line)
	box.add_child(lw)
	return box


func _play_block() -> Control:
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var holder := CenterContainer.new()
	holder.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(holder)
	var stack := Control.new()
	stack.custom_minimum_size = Vector2(440, 92)
	stack.mouse_filter = Control.MOUSE_FILTER_IGNORE
	holder.add_child(stack)
	_glow = Panel.new()
	_glow.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_glow.offset_left = -10
	_glow.offset_right = 10
	_glow.offset_top = -8
	_glow.offset_bottom = 8
	_glow.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var gs := StyleBoxFlat.new()
	gs.bg_color = Color(Tokens.ACCENT, 0.0)
	gs.set_corner_radius_all(18)
	gs.shadow_color = Color(Tokens.ACCENT, 0.55)
	gs.shadow_size = 26
	_glow.add_theme_stylebox_override("panel", gs)
	stack.add_child(_glow)
	play_button = ActionButton.new()
	play_button.text = "ИГРАТЬ"
	play_button.variant = ActionButton.Variant.PRIMARY
	play_button.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	play_button.add_theme_font_size_override("font_size", 34)
	play_button.add_theme_font_override("font", UiTheme.make_font(800))
	play_button.pressed.connect(func() -> void: play_pressed.emit())
	stack.add_child(play_button)
	# статус связи
	var row := HBoxContainer.new()
	row.alignment = BoxContainer.ALIGNMENT_CENTER
	row.add_theme_constant_override("separation", 10)
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_spinner = Control.new()
	_spinner.custom_minimum_size = Vector2(22, 22)
	_spinner.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_spinner.draw.connect(_draw_spinner)
	row.add_child(_spinner)
	status_label = Label.new()
	status_label.name = "Status"
	status_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	row.add_child(status_label)
	retry_button = ActionButton.new()
	retry_button.text = "Повторить"
	retry_button.custom_minimum_size = Vector2(150, 44)
	retry_button.pressed.connect(func() -> void: retry_pressed.emit())
	row.add_child(retry_button)
	box.add_child(row)
	return box


func _bottom_bar() -> Control:
	var bar := HBoxContainer.new()
	bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
	bar.custom_minimum_size = Vector2(0, 56)
	account_button = ActionButton.new()
	account_button.text = "Аккаунт"
	account_button.custom_minimum_size = Vector2(190, 52)
	account_button.pressed.connect(func() -> void: account_pressed.emit())
	bar.add_child(account_button)
	var gap := Control.new()
	gap.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	gap.mouse_filter = Control.MOUSE_FILTER_IGNORE
	bar.add_child(gap)
	var ver := Label.new()
	ver.text = "Альфа-тест"
	ver.theme_type_variation = &"MutedLabel"
	ver.size_flags_vertical = Control.SIZE_SHRINK_END
	ver.mouse_filter = Control.MOUSE_FILTER_IGNORE
	bar.add_child(ver)
	return bar


## Состояние связи: подпись, доступность кнопок, спиннер.
func set_state(s: State, text: String) -> void:
	state = s
	status_label.text = text
	var variation := &"MutedLabel"
	match s:
		State.READY:
			variation = &"SuccessLabel"
		State.ERROR:
			variation = &"DangerLabel"
	status_label.theme_type_variation = variation
	play_button.disabled = s != State.READY
	retry_button.visible = s == State.ERROR
	_spinner.visible = s == State.CONNECTING or s == State.ENTERING
	account_button.disabled = s == State.ENTERING
	if s == State.ENTERING:
		play_button.text = "ВХОД…"
	else:
		play_button.text = "ИГРАТЬ"
	_spinner.queue_redraw()


func _draw_spinner() -> void:
	var c := _spinner.size / 2.0
	var r := minf(c.x, c.y) - 2.0
	_spinner.draw_arc(c, r, 0, TAU, 32, Color(Tokens.ACCENT, 0.2), 3.0, true)
	_spinner.draw_arc(c, r, _t * 5.0, _t * 5.0 + PI * 0.9, 20, Tokens.ACCENT, 3.0, true)


func _process(delta: float) -> void:
	_t += delta
	_mat.set_shader_parameter("time_s", _t)
	_mat.set_shader_parameter("aspect", Vector2(size.x / maxf(size.y, 1.0), 1.0))
	if _spinner.visible:
		_spinner.queue_redraw()
	if _animate and play_button != null and not play_button.disabled:
		# мягкая пульсация свечения кнопки «ИГРАТЬ»
		var k := 0.55 + 0.45 * sin(_t * 2.2)
		(_glow.get_theme_stylebox("panel") as StyleBoxFlat).shadow_color = Color(Tokens.ACCENT, 0.25 + 0.4 * k)
	elif _glow != null:
		(_glow.get_theme_stylebox("panel") as StyleBoxFlat).shadow_color = Color(Tokens.ACCENT, 0.0)


func _intro() -> void:
	_content.modulate.a = 0.0
	var tw := create_tween()
	tw.tween_property(_content, "modulate:a", 1.0, 0.9).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)


func _on_logo_input(ev: InputEvent) -> void:
	var pressed := false
	if ev is InputEventMouseButton:
		pressed = (ev as InputEventMouseButton).pressed
	elif ev is InputEventScreenTouch:
		pressed = (ev as InputEventScreenTouch).pressed
	if pressed:
		register_logo_tap(Time.get_ticks_msec())


## Учёт нажатий на логотип: SECRET_TAPS нажатий за SECRET_WINDOW_MS открывают служебный экран.
func register_logo_tap(now_msec: int) -> void:
	_taps.append(now_msec)
	while _taps.size() > 0 and now_msec - _taps[0] > SECRET_WINDOW_MS:
		_taps.pop_front()
	if _taps.size() >= SECRET_TAPS:
		_taps.clear()
		secret_opened.emit()
