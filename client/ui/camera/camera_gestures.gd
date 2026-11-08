extends RefCounted
## Распознавание жестов камеры. Получает ТОЛЬКО события, не перехваченные интерфейсом (джойстик, кнопки,
## панели забирают свои касания раньше), поэтому UI всегда в приоритете. Один палец на свободной области
## вращает камеру (или это тап, если палец почти не двигался); два пальца масштабируют (щипок).
## Дополнительные пальцы игнорируются; после щипка оставшийся палец не вращает камеру, пока его не поднимут,
## это исключает скачки положения. Мышь (проверка на компьютере): левая кнопка (тап и вращение),
## правая кнопка (вращение), колесо (масштаб).

signal orbited(delta_px: Vector2)
signal pinched(factor: float)           # > 1 приближает
signal tapped(position: Vector2)
signal gesture_started(kind: String)    # "orbit" | "pinch"
signal gesture_ended()

const SLOP := 12.0              # px: меньше этого палец считается неподвижным (тап)
const TAP_MAX_MS := 350
const MOUSE_LEFT := -100
const MOUSE_RIGHT := -101

enum State { IDLE, PENDING, ORBIT, PINCH, IGNORING }

var state := State.IDLE
var pinch_sensitivity := 1.0

var _touches: Dictionary = {}   # index -> {start, pos, t0}
var _orbit_index := -1000
var _pinch_a := -1000
var _pinch_b := -1000
var _pinch_dist := 0.0


func _init(sensitivity: float = 1.0) -> void:
	pinch_sensitivity = sensitivity


func is_active() -> bool:
	return state == State.ORBIT or state == State.PINCH


func touch_count() -> int:
	return _touches.size()


func has_touch(index: int) -> bool:
	return _touches.has(index)


## Касание (index >= 0: палец; MOUSE_LEFT / MOUSE_RIGHT: кнопки мыши). t_ms: время события.
func touch(index: int, pressed: bool, pos: Vector2, t_ms: int) -> void:
	if pressed:
		if _touches.has(index):
			return
		_touches[index] = {"start": pos, "pos": pos, "t0": t_ms}
		match state:
			State.IDLE:
				state = State.PENDING
				_orbit_index = index
			State.PENDING, State.ORBIT:
				# второй палец: начинаем щипок, вращение прекращается
				if index >= 0 and _orbit_index >= 0 and index != _orbit_index:
					_pinch_a = _orbit_index
					_pinch_b = index
					_pinch_dist = maxf(1.0, (_touches[_pinch_a]["pos"] as Vector2).distance_to(pos))
					state = State.PINCH
					gesture_started.emit("pinch")
			_:
				pass  # третий и далее: игнорируются
	else:
		if not _touches.has(index):
			return
		var t: Dictionary = _touches[index]
		_touches.erase(index)
		match state:
			State.PENDING:
				if index == _orbit_index:
					var moved: float = (t["start"] as Vector2).distance_to(pos)
					if moved < SLOP and t_ms - int(t["t0"]) <= TAP_MAX_MS and index != MOUSE_RIGHT:
						tapped.emit(pos)
					_finish()
			State.ORBIT:
				if index == _orbit_index:
					_finish()
			State.PINCH:
				if index == _pinch_a or index == _pinch_b:
					# один палец поднят: оставшийся не вращает камеру, пока его не отпустят
					state = State.IGNORING
					gesture_ended.emit()
					if _touches.is_empty():
						_finish_silent()
			State.IGNORING:
				if _touches.is_empty():
					_finish_silent()


## Перемещение пальца или мыши.
func drag(index: int, pos: Vector2) -> void:
	if not _touches.has(index):
		return
	var t: Dictionary = _touches[index]
	var prev: Vector2 = t["pos"]
	t["pos"] = pos
	match state:
		State.PENDING:
			if index == _orbit_index and (t["start"] as Vector2).distance_to(pos) >= SLOP:
				state = State.ORBIT
				gesture_started.emit("orbit")
				# движение до порога не теряем, но и не даём скачка: считаем от точки выхода за порог
		State.ORBIT:
			if index == _orbit_index:
				orbited.emit(pos - prev)
		State.PINCH:
			if index == _pinch_a or index == _pinch_b:
				var d := (_touches[_pinch_a]["pos"] as Vector2).distance_to(_touches[_pinch_b]["pos"])
				if d > 1.0 and _pinch_dist > 1.0:
					var ratio := pow(d / _pinch_dist, pinch_sensitivity)
					pinched.emit(ratio)
					_pinch_dist = d
		_:
			pass


## Колесо мыши: direction > 0 приближает.
func wheel(direction: int, step: float) -> void:
	if direction == 0:
		return
	pinched.emit(pow(1.0 + step, float(signi(direction))))


## Принудительное завершение (потеря фокуса, открытие меню или диалога): без тапа и без скачков.
func cancel() -> void:
	var was_active := is_active()
	_touches.clear()
	_finish_silent()
	if was_active:
		gesture_ended.emit()


func _finish() -> void:
	var was_active := state == State.ORBIT
	_finish_silent()
	if was_active:
		gesture_ended.emit()


func _finish_silent() -> void:
	state = State.IDLE
	_orbit_index = -1000
	_pinch_a = -1000
	_pinch_b = -1000
	_pinch_dist = 0.0
	if not _touches.is_empty():
		_touches.clear()
