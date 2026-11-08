extends ProgressBar
## Полоска состояния (корпус, щиты, энергия): значения приходят от сервера.
## Цвет задаётся токеном, текст «текущее / максимум» показывается по центру.

const Tokens := preload("res://ui/theme/tokens.gd")

@export var fill_color: Color = Tokens.ACCENT:
	set(v):
		fill_color = v
		_apply_fill()


func _ready() -> void:
	custom_minimum_size.y = maxf(custom_minimum_size.y, 22.0)
	show_percentage = false
	_apply_fill()


## Установить значения (максимум не может быть меньше 1).
func set_values(current: float, maximum: float) -> void:
	max_value = maxf(1.0, maximum)
	value = clampf(current, 0.0, max_value)
	tooltip_text = "%d / %d" % [int(round(value)), int(round(max_value))]


func _apply_fill() -> void:
	var s := StyleBoxFlat.new()
	s.bg_color = fill_color
	s.set_corner_radius_all(6)
	add_theme_stylebox_override("fill", s)
