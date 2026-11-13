extends MarginContainer
## Корневой контейнер интерфейса: отступы от выреза камеры и скруглений экрана.
## Пересчитывается при каждом изменении размера окна (в том числе при повороте на 180°:
## вырез оказывается то слева, то справа). Всё содержимое экрана кладётся внутрь.

const Tokens := preload("res://ui/theme/tokens.gd")

signal insets_changed(insets: Vector4i)

## Минимальный отступ со всех сторон (логические px), даже на экранах без выреза.
@export var min_margin: int = Tokens.SPACE_LG

var _use_debug := false
var _debug_insets := Vector4i.ZERO  # левый, верхний, правый, нижний отступ в px окна
var insets := Vector4i.ZERO         # текущие отступы: left, top, right, bottom (логические px)


## Чистая функция расчёта отступов (удобна для тестов).
## safe — безопасная зона в px окна; window_size — размер окна в px;
## viewport_size — размер видимой области в логических px (после stretch).
static func compute_margins(safe: Rect2i, window_size: Vector2i, viewport_size: Vector2, min_margin: int) -> Vector4i:
	var floor_m := Vector4i(min_margin, min_margin, min_margin, min_margin)
	if window_size.x <= 0 or window_size.y <= 0 or viewport_size.x <= 0.0 or viewport_size.y <= 0.0:
		return floor_m
	if safe.size.x <= 0 or safe.size.y <= 0:
		safe = Rect2i(Vector2i.ZERO, window_size)  # платформа не сообщила зону: считаем весь экран безопасным
	var sx := viewport_size.x / float(window_size.x)
	var sy := viewport_size.y / float(window_size.y)
	var left := maxf(0.0, float(safe.position.x)) * sx
	var top := maxf(0.0, float(safe.position.y)) * sy
	var right := maxf(0.0, float(window_size.x - safe.end.x)) * sx
	var bottom := maxf(0.0, float(window_size.y - safe.end.y)) * sy
	return Vector4i(
		maxi(min_margin, int(ceil(left))),
		maxi(min_margin, int(ceil(top))),
		maxi(min_margin, int(ceil(right))),
		maxi(min_margin, int(ceil(bottom))))


## Тестовый режим: имитация выреза без устройства. Отступы в px окна.
func set_debug_insets(left: int, top: int, right: int, bottom: int) -> void:
	_use_debug = true
	_debug_insets = Vector4i(left, top, right, bottom)
	refresh()


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	get_viewport().size_changed.connect(refresh)
	refresh()


## Пересчитать отступы по текущему состоянию экрана.
func refresh() -> void:
	if not is_inside_tree():
		return  # до входа в дерево: значения применит _ready
	var vp := get_viewport()
	var viewport_size := vp.get_visible_rect().size
	var window_size: Vector2i
	var safe: Rect2i
	if vp == get_tree().root:
		window_size = DisplayServer.window_get_size()
		# get_display_safe_area$6>76@4M05B >>>D48=4BK$4<E??5O,$0$=5$>:=0.
		#$0 landscape Android$>>=>$<>65B$5KFL E<5M5=>$87-70$2OD570/?4=5?8$65EF>2.
		var display_safe := DisplayServer.get_display_safe_area()
		var window_pos := DisplayServer.window_get_position()
		safe = Rect2i(display_safe.position - window_pos, display_safe.size)
		var window_rect := Rect2i(Vector2i.ZERO, window_size)
		safe = safe.intersection(window_rect)
	else:
		# вложенный viewport (например, в тестах): масштаба и выреза нет
		window_size = Vector2i(viewport_size)
		safe = Rect2i(Vector2i.ZERO, window_size)
	if _use_debug:
		safe = Rect2i(_debug_insets.x, _debug_insets.y,
			window_size.x - _debug_insets.x - _debug_insets.z,
			window_size.y - _debug_insets.y - _debug_insets.w)
	var m := compute_margins(safe, window_size, viewport_size, min_margin)
	add_theme_constant_override("margin_left", m.x)
	add_theme_constant_override("margin_top", m.y)
	add_theme_constant_override("margin_right", m.z)
	add_theme_constant_override("margin_bottom", m.w)
	if m != insets:
		insets = m
		insets_changed.emit(m)
