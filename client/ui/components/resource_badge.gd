extends HBoxContainer
## Индикатор ресурса или валюты: цветной значок и число с разделителями тысяч.
## Значение приходит от сервера; компонент только отображает.

const Tokens := preload("res://ui/theme/tokens.gd")

@export var color: Color = Tokens.ACCENT:
	set(v):
		color = v
		_refresh()
@export var caption: String = "":
	set(v):
		caption = v
		_refresh()

var value: int = 0

var _dot: Panel
var _label: Label


static func format_number(n: int) -> String:
	var s := str(absi(n))
	var out := ""
	var count := 0
	for i in range(s.length() - 1, -1, -1):
		out = s[i] + out
		count += 1
		if count % 3 == 0 and i > 0:
			out = " " + out  # неразрывный пробел: число не переносится
	return ("-" if n < 0 else "") + out


func _ready() -> void:
	add_theme_constant_override("separation", Tokens.SPACE_SM)
	_dot = Panel.new()
	_dot.custom_minimum_size = Vector2(14, 14)
	_dot.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_dot.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_dot)
	_label = Label.new()
	add_child(_label)
	_refresh()


func set_value(v: int) -> void:
	value = v
	_refresh()


func _refresh() -> void:
	if _label == null:
		return
	var s := StyleBoxFlat.new()
	s.bg_color = color
	s.set_corner_radius_all(7)
	_dot.add_theme_stylebox_override("panel", s)
	_label.text = ("%s  %s" % [caption, format_number(value)]) if not caption.is_empty() else format_number(value)
