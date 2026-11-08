extends Control
## Виртуальный джойстик (опциональный). Поддерживает мультитач: палец на джойстике не мешает
## кнопкам способностей. Мышь работает для проверки на компьютере. Выдаёт вектор длиной 0..1;
## сервер сам проверяет и ограничивает движение.

const Tokens := preload("res://ui/theme/tokens.gd")

signal vector_changed(vector: Vector2)

const DEAD_ZONE := 0.12
const MOUSE_INDEX := -100

@export var radius: float = 84.0

var vector := Vector2.ZERO
var _index := -1000  # индекс пальца, который держит джойстик (нет = -1000)
var _knob := Vector2.ZERO


func _ready() -> void:
	custom_minimum_size = Vector2(radius * 2.0 + 20.0, radius * 2.0 + 20.0)
	mouse_filter = Control.MOUSE_FILTER_STOP


func is_active() -> bool:
	return _index != -1000


func _centre() -> Vector2:
	return size * 0.5


func _gui_input(event: InputEvent) -> void:
	if event is InputEventScreenTouch:
		if event.pressed and not is_active():
			_begin(event.index, event.position)
		elif not event.pressed and event.index == _index:
			_end()
	elif event is InputEventScreenDrag:
		if event.index == _index:
			_move(event.position)
	elif event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT:
		# касание уже обработано как ScreenTouch; эмулированные мышью события пропускаем
		if event.device == InputEvent.DEVICE_ID_EMULATION:
			return
		if event.pressed and not is_active():
			_begin(MOUSE_INDEX, event.position)
		elif not event.pressed and _index == MOUSE_INDEX:
			_end()
	elif event is InputEventMouseMotion and _index == MOUSE_INDEX:
		_move(event.position)


func _begin(index: int, pos: Vector2) -> void:
	_index = index
	_move(pos)


func _move(pos: Vector2) -> void:
	var offset := pos - _centre()
	var length := offset.length()
	_knob = offset.limit_length(radius)
	var v := _knob / radius
	if length / radius < DEAD_ZONE:
		v = Vector2.ZERO
	_set_vector(v)
	queue_redraw()


func _end() -> void:
	_index = -1000
	_knob = Vector2.ZERO
	_set_vector(Vector2.ZERO)
	queue_redraw()


func _set_vector(v: Vector2) -> void:
	if v.is_equal_approx(vector):
		return
	vector = v
	vector_changed.emit(v)


func _notification(what: int) -> void:
	if what == NOTIFICATION_VISIBILITY_CHANGED and not is_visible_in_tree() and is_active():
		_end()  # скрыли во время удержания: отпускаем


func _draw() -> void:
	var c := _centre()
	draw_circle(c, radius, Color(Tokens.SURFACE, 0.45))
	draw_arc(c, radius, 0.0, TAU, 48, Color(Tokens.ACCENT, 0.55), 2.0, true)
	draw_circle(c + _knob, radius * 0.42, Color(Tokens.ACCENT, 0.7 if is_active() else 0.4))
