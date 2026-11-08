extends ProgressBar
## Полоска состояния (корпус, щиты, энергия): значения приходят от сервера.
## Цвет задаётся токеном, текст «текущее / максимум» показывается по центру.

const Tokens := preload("res://ui/theme/tokens.gd")

@export var fill_color: Color = Tokens.ACCENT:
	set(v):
		fill_color = v
		_apply_fill()

## Подпись слева от чисел (например «Щит»).
@export var caption := ""
## Ниже этой доли шкала становится предупреждающей (0 — не меняется).
@export var warn_below := 0.0

var _low := false
var _text: Label


func _ready() -> void:
	custom_minimum_size.y = maxf(custom_minimum_size.y, 22.0)
	show_percentage = false
	_text = Label.new()
	_text.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_text.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_text.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_text.add_theme_font_size_override("font_size", Tokens.FONT_SMALL - 4)
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
	tooltip_text = "%s %d / %d" % [caption, int(round(value)), int(round(max_value))]
	_refresh_text()
	_refresh_warning()


func _refresh_text() -> void:
	if _text != null:
		var nums := "%d / %d" % [int(round(value)), int(round(max_value))]
		_text.text = (caption + " " + nums) if caption != "" else nums


## Предупреждение о низком значении: цвет заливки меняется на красный (только цвет, без эффектов).
func _refresh_warning() -> void:
	var low := warn_below > 0.0 and value / max_value < warn_below
	if low != _low:
		_low = low
		_apply_fill()


func is_low() -> bool:
	return _low


func _apply_fill() -> void:
	var s := StyleBoxFlat.new()
	s.bg_color = Tokens.DANGER if _low else fill_color
	s.set_corner_radius_all(6)
	add_theme_stylebox_override("fill", s)
