extends Control
## Плавающий виртуальный джойстик: занимает всю левую часть экрана (кнопки и панели HUD поверх него
## в приоритете). Палец, коснувшийся свободного места зоны, задаёт центр джойстика, туда не нужно
## попадать. Мультитач: палец на джойстике не мешает кнопкам способностей и жестам камеры. Короткое
## касание без движения не двигает корабль, а сообщается сигналом tapped (выбор объекта под пальцем).
## Мышь работает для проверки на компьютере. Выдаёт вектор длиной 0..1; сервер сам проверяет и ограничивает движение.

const Tokens := preload("res://ui/theme/tokens.gd")

signal vector_changed(vector: Vector2)
signal tapped(pos: Vector2)

const DEAD_ZONE := 0.12
const MOUSE_INDEX := -100
const TAP_MAX_MSEC := 250
const TAP_MAX_MOVE := 14.0

@export var radius: float = 84.0

var vector := Vector2.ZERO
var _index := -1000  # индекс пальца, который держит джойстик (нет = -1000)
var _knob := Vector2.ZERO
var _origin := Vector2.ZERO   # центр джойстика: точка первого касания
var _press_msec := 0
var _moved := 0.0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_STOP


func is_active() -> bool:
	return _index != -1000


## Центр: у активного джойстика точка касания, иначе место подсказки.
func _centre() -> Vector2:
	return _origin if is_active() else rest_position()


## Где рисуется бледная подсказка, пока палец не касается.
func rest_position() -> Vector2:
	return Vector2(size.x * 0.5, size.y * 0.66)


func _gui_input(event: InputEvent) -> void:
	if event is InputEventScreenTouch:
		if event.pressed and not is_active():
			_begin(event.index, event.position)
		elif not event.pressed and event.index == _index:
			_end(event.position + global_position)
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
			_end(event.position + global_position)
	elif event is InputEventMouseMotion and _index == MOUSE_INDEX:
		_move(event.position)


func _begin(index: int, pos: Vector2) -> void:
	_index = index
	_origin = pos
	_press_msec = Time.get_ticks_msec()
	_moved = 0.0
	_move(pos)


func _move(pos: Vector2) -> void:
	_moved = maxf(_moved, pos.distance_to(_origin))
	var offset := pos - _centre()
	var length := offset.length()
	_knob = offset.limit_length(radius)
	var v := _knob / radius
	if length / radius < DEAD_ZONE:
		v = Vector2.ZERO
	_set_vector(v)
	queue_redraw()


func _end(release_pos: Vector2 = Vector2.INF) -> void:
	var was_tap := release_pos != Vector2.INF and _moved <= TAP_MAX_MOVE and Time.get_ticks_msec() - _press_msec <= TAP_MAX_MSEC
	_index = -1000
	_knob = Vector2.ZERO
	_set_vector(Vector2.ZERO)
	queue_redraw()
	if was_tap:
		tapped.emit(release_pos)


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
	if not is_active():
		# подсказка: бледное кольцо в зоне; джойстик появится там, где коснётся палец
		draw_arc(c, radius, 0.0, TAU, 48, Color(Tokens.ACCENT, 0.18), 2.0, true)
		return
	draw_circle(c, radius, Color(Tokens.SURFACE, 0.45))
	draw_arc(c, radius, 0.0, TAU, 48, Color(Tokens.ACCENT, 0.55), 2.0, true)
	draw_circle(c + _knob, radius * 0.42, Color(Tokens.ACCENT, 0.7))
