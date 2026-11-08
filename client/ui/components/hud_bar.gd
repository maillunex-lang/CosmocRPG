extends ProgressBar
## Полоска состояния (корпус, щиты, энергия): значения приходят от сервера.
## Цвет задаётся токеном, текст «текущее / максимум» показывается по центру.

const Tokens := preload("res://ui/theme/tokens.gd")

@export var fill_color: Color = Tokens.ACCENT:
	set(v):
		fill_color = v
		_apply_fill()

var _text: Label


func _ready() -> void:
	custom_minimum_size.y = maxf(custom_minimum_size.y, 22.0)
	show_percentage = false
	_text = Label.new()
	_text.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_text.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_text.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_text.add_theme_font_size_override("font_size", Tokens.FONT_SMALL - 2)
	_text.add_theme_color_override("font_color", Tokens.TEXT)
	_text.add_theme_color_override("font_outline_color", Tokens.BG)
	_text.add_theme_constant_override("outline_size", 4)
	add_child(_text)
	_apply_fill()
	_refresh_text()


## Установить значения (максимум не может быть меньше 1).
func set_values(current: float, maximum: float) -> void:
	max_value = maxf(1.0, maximum)
	value = clampf(current, 0.0, max_value)
	tooltip_text = "%d / %d" % [int(round(value)), int(round(max_value))]
	_refresh_text()


func _refresh_text() -> void:
	if _text != null:
		_text.text = "%d / %d" % [int(round(value)), int(round(max_value))]


func _apply_fill() -> void:
	var s := StyleBoxFlat.new()
	s.bg_color = fill_color
	s.set_corner_radius_all(6)
	add_theme_stylebox_override("fill", s)
