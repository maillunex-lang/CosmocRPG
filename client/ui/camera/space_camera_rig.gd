extends Node3D
## Камера космоса: привязка к кораблю (этот узел), горизонтальный поворот (Yaw), наклон (Pitch)
## и ортографическая Camera3D с управляемым масштабом. Плавное сглаживание следования, вращения,
## наклона и масштаба; сброс; опциональное автоцентрирование; смещение корабля на экране в экранных
## координатах (не вращается вместе с камерой).
##
## Камера ничего не меняет в игровом мире: она только смотрит на переданную позицию корабля.
## Структура сцены: CameraRig (этот скрипт) > Yaw > Pitch > Camera3D.

const Config := preload("res://ui/camera/space_camera_config.gd")

## Режимы камеры (кнопка «Вид»): свободный 3D, фиксированный вид сверху (разворот за курсом),
## вид сзади чуть сверху (разворот за курсом). В закреплённых режимах вращение и наклон жестом отключены, масштаб работает.
enum Mode { FREE, TOP, CHASE }
const MODE_NAMES := ["Свободный", "Сверху", "Сзади"]

signal mode_changed(mode: int)

const CAMERA_DISTANCE := 150.0   # у ортографической камеры расстояние влияет только на отсечение

var config: Resource = Config.from_dict({})
var camera: Camera3D

var _yaw := 0.0          # текущий горизонтальный угол, рад; непрерывный, без обрезки на 360°
var _yaw_target := 0.0
var _tilt := 45.0        # текущий наклон от вертикали, градусы
var _tilt_target := 45.0
var _size := 50.0        # текущий размер обзора по вертикали, ед. мира
var _size_target := 50.0
var _focus := Vector3.ZERO
var _prev_target := Vector3.ZERO
var _target_vel := Vector3.ZERO
var _have_target := false
var _idle := 0.0         # секунд без ручного управления
var _parallax_px := Vector2.ZERO   # накопленный экранный сдвиг для параллакса фона, px
var _heading := Vector2(0, -1)
var _mode: int = Mode.FREE
var _offset_y := 0.0     # текущее вертикальное смещение корабля на экране (доли обзора)
var viewport_height := 720.0       # высота экрана в px (для перевода сдвига в пиксели)

@onready var _yaw_node: Node3D = $Yaw
@onready var _pitch_node: Node3D = $Yaw/Pitch


func _ready() -> void:
	camera = $Yaw/Pitch/Camera3D
	camera.projection = Camera3D.PROJECTION_ORTHOGONAL
	camera.position = Vector3(0, 0, CAMERA_DISTANCE)
	camera.near = 0.1
	camera.far = CAMERA_DISTANCE * 4.0
	apply_config(config, true)


## Применить конфигурацию. snap: сразу выставить начальные значения.
func apply_config(c: Resource, snap: bool = false) -> void:
	config = c
	if snap:
		_tilt = config.tilt_initial
		_tilt_target = _tilt
		_size = config.zoom_initial
		_size_target = _size
		_yaw = 0.0
		_yaw_target = 0.0
	else:
		if _mode == Mode.FREE:
			_tilt_target = clampf(_tilt_target, config.tilt_min, config.tilt_max)
		_size_target = clampf(_size_target, config.zoom_min, config.zoom_max)
	_apply_transform()


# --- управление (все значения ограничиваются здесь) ---

## Поворот и наклон жестом: dx, dy в пикселях. Свайп вправо вращает мир по часовой стрелке, свайп вниз поднимает камеру.
func orbit(dx_px: float, dy_px: float) -> void:
	if _mode != Mode.FREE:
		return   # в закреплённых режимах вид задаёт курс корабля
	_yaw_target += deg_to_rad(dx_px * config.orbit_sensitivity)
	_tilt_target = clampf(_tilt_target - dy_px * config.tilt_sensitivity, config.tilt_min, config.tilt_max)
	_idle = 0.0


## Масштаб во столько раз: factor > 1 приближает, < 1 отдаляет.
func zoom_by(factor: float) -> void:
	if factor <= 0.0 or is_nan(factor) or is_inf(factor):
		return
	_size_target = clampf(_size_target / factor, config.zoom_min, config.zoom_max)
	_idle = 0.0


func mode() -> int:
	return _mode


func mode_name() -> String:
	return MODE_NAMES[_mode]


## Следующий режим по кругу: свободный → сверху → сзади → свободный.
func cycle_mode() -> void:
	set_mode((_mode + 1) % MODE_NAMES.size())


func set_mode(m: int) -> void:
	m = clampi(m, 0, MODE_NAMES.size() - 1)
	var changed := m != _mode
	_mode = m
	match _mode:
		Mode.FREE:
			reset()   # возврат в свободный режим: стандартный наклон и масштаб, камера за кораблём
		Mode.TOP:
			_tilt_target = config.top_tilt
			_size_target = config.top_zoom
		Mode.CHASE:
			_tilt_target = config.chase_tilt
			_size_target = config.chase_zoom
	_idle = 0.0
	if changed:
		mode_changed.emit(_mode)


## Сброс: стандартный наклон и масштаб, плавное выравнивание за кораблём по кратчайшему пути.
func reset() -> void:
	_tilt_target = config.tilt_initial
	_size_target = config.zoom_initial
	_yaw_target = _yaw + angle_difference(_yaw, behind_yaw(_heading))
	_idle = 0.0


## Угол камеры, при котором она находится позади корабля, смотрящего по направлению dir (мировые x, z).
static func behind_yaw(dir: Vector2) -> float:
	if dir.length() < 0.001:
		return 0.0
	return atan2(-dir.x, -dir.y)


## Мгновенно выставить вид (тесты, снимки экрана, загрузка сохранённого ракурса). Углы в градусах.
func set_view(yaw_deg: float, tilt_deg: float, size: float) -> void:
	_yaw = deg_to_rad(yaw_deg)
	_yaw_target = _yaw
	_tilt = clampf(tilt_deg, config.tilt_min, config.tilt_max)
	_tilt_target = _tilt
	_size = clampf(size, config.zoom_min, config.zoom_max)
	_size_target = _size
	_apply_transform()


## Мгновенно перенести фокус к цели (старт, телепорт): без плавного «подъезда».
func snap_to(target: Vector3) -> void:
	_focus = target
	_prev_target = target
	_target_vel = Vector3.ZERO
	_have_target = true
	position = target


# --- чтение состояния ---

func azimuth_deg() -> float:
	return fposmod(rad_to_deg(_yaw), 360.0)


func tilt_deg() -> float:
	return _tilt


func zoom_size() -> float:
	return _size


func tilt_target_deg() -> float:
	return _tilt_target


func zoom_target() -> float:
	return _size_target


func yaw_target_rad() -> float:
	return _yaw_target


func is_settled(eps: float = 0.05) -> bool:
	return absf(_yaw - _yaw_target) < 0.001 and absf(_tilt - _tilt_target) < eps and absf(_size - _size_target) < eps


## Накопленный экранный сдвиг (px) для параллакса фона. Вращение, наклон и масштаб фон не двигают:
## фон не повторяет вращение камеры, смещается только при перемещении корабля, в направлении экрана.
func parallax_px() -> Vector2:
	return _parallax_px


## Экранные оси камеры в мировой плоскости: направление «вправо на экране» и «вперёд (вверх по экрану)».
func screen_axes() -> Dictionary:
	var s := sin(_yaw)
	var c := cos(_yaw)
	return {"right": Vector3(c, 0.0, -s), "forward": Vector3(-s, 0.0, -c)}


## Шаг камеры. target: позиция корабля в мире (уже интерполированная), heading: его направление (x, z),
## speed: скорость, ед/с (для автоцентрирования).
func update(delta: float, target: Vector3, heading: Vector2, speed: float = 0.0) -> void:
	if delta <= 0.0:
		return
	if heading.length() > 0.001:
		_heading = heading.normalized()
	if not _have_target:
		snap_to(target)
	# скорость цели для компенсации запаздывания
	var vel := (target - _prev_target) / delta
	_target_vel = _target_vel.lerp(vel, clampf(delta * 20.0, 0.0, 1.0))
	_prev_target = target
	var follow_a := _alpha(config.follow_rate, delta)
	var goal := target
	if config.follow_lead:
		# точная для дискретного шага компенсация: при равномерном движении камера не отстаёт и не опережает
		goal += _target_vel * delta * (1.0 - follow_a) / follow_a
	var prev_focus := _focus
	_focus = _focus.lerp(goal, follow_a)
	position = _focus

	# сдвиг фона: перемещение фокуса в экранных осях, с учётом масштаба и укорочения плоскости при наклоне
	var d := _focus - prev_focus
	var axes := screen_axes()
	var px_per_unit := viewport_height / maxf(_size, 0.001)
	_parallax_px += Vector2(d.dot(axes["right"]), -d.dot(axes["forward"]) * cos(deg_to_rad(_tilt))) * px_per_unit

	# автоцентрирование: только без ручного управления и при движении; ручное действие всегда в приоритете
	_idle += delta
	if config.auto_center_enabled and _idle >= config.auto_center_delay and speed >= config.auto_center_min_speed:
		var want := _yaw + angle_difference(_yaw, behind_yaw(_heading))
		_yaw_target = lerpf(_yaw_target, want, _alpha(config.auto_center_rate, delta))

	# закреплённые режимы: камера разворачивается за курсом корабля
	var yaw_rate: float = config.yaw_rate
	if _mode != Mode.FREE:
		_yaw_target = _yaw + angle_difference(_yaw, behind_yaw(_heading))
		yaw_rate = config.locked_yaw_rate
	# поворот сглаживается, а угловая скорость ограничена: на больших углах (сброс, 720°) нет рывка при старте
	var yaw_limit := deg_to_rad(config.yaw_max_speed) * delta
	_yaw += clampf((_yaw_target - _yaw) * _alpha(yaw_rate, delta), -yaw_limit, yaw_limit)
	_tilt = lerpf(_tilt, _tilt_target, _alpha(config.tilt_rate, delta))
	# масштаб сглаживается в логарифмическом пространстве: ощущение одинаково на любом уровне
	_size = _size * pow(_size_target / _size, _alpha(config.zoom_rate, delta))
	_offset_y = lerpf(_offset_y, config.chase_offset_y if _mode == Mode.CHASE else 0.0, _alpha(config.tilt_rate, delta))
	_apply_transform()


static func _alpha(rate: float, delta: float) -> float:
	return 1.0 - exp(-rate * delta)


func _apply_transform() -> void:
	if _yaw_node == null:
		return
	_yaw_node.rotation = Vector3(0.0, _yaw, 0.0)
	_pitch_node.rotation = Vector3(-deg_to_rad(90.0 - _tilt), 0.0, 0.0)   # возвышение над горизонтом = 90° − наклон
	camera.size = _size
	# смещение корабля на экране задаётся в экранных координатах и не зависит от поворота
	camera.h_offset = config.focus_offset.x * _size
	camera.v_offset = (config.focus_offset.y + _offset_y) * _size
