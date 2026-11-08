extends Resource
## Конфигурация камеры космоса: углы, масштаб, чувствительность, сглаживание, смещение фокуса,
## автоцентрирование. Хранится в данных (assets/space/camera.json), все значения проходят проверку.
## Углы наклона отсчитываются от мировой вертикали: 0° смотрит строго сверху, 90° вдоль плоскости движения.
## Масштаб (zoom) — размер обзора по вертикали в мировых единицах (проекция ортографическая).

var tilt_initial := 45.0
var tilt_min := 25.0
var tilt_max := 65.0
var zoom_initial := 50.0     # при длине корабля около 3,5 ед. это 14 px на единицу на экране 720 px
var zoom_min := 18.0         # корабль примерно 150 px: максимальное приближение
var zoom_max := 100.0        # виден весь район с запасом: максимальное отдаление
var follow_rate := 14.0      # скорость сглаживания следования, 1/с
var follow_lead := true      # компенсация запаздывания: без постоянного отставания при движении
var yaw_rate := 12.0
var yaw_max_speed := 540.0   # предел угловой скорости, градусов/с: сброс и автоцентрирование не стартуют рывком
var tilt_rate := 12.0
var zoom_rate := 10.0
var orbit_sensitivity := 0.25    # градусов поворота на пиксель
var tilt_sensitivity := 0.18     # градусов наклона на пиксель
var pinch_sensitivity := 1.0     # степень отношения расстояний между пальцами
var wheel_zoom_step := 0.12      # относительное изменение масштаба за щелчок колеса
var focus_offset := Vector2(0.12, 0.0)   # смещение корабля на экране (доли размера обзора): освобождает центр между колонками HUD
var auto_center_enabled := false
var auto_center_delay := 2.5     # секунд без ручного управления до выравнивания
var auto_center_rate := 1.8
var auto_center_min_speed := 1.0 # выравнивать только при движении корабля, ед/с
# режимы с закреплённой камерой (кнопка «Вид»): сверху и сзади корабля; углы от вертикали
var top_tilt := 0.0              # 0°: строго сверху
var top_zoom := 32.0
var chase_tilt := 70.0           # 70° от вертикали: камера на 20° выше горизонта, позади корабля
var chase_zoom := 24.0
var chase_offset_y := 0.18       # корабль ниже центра: впереди больше обзора (доли размера обзора)
var locked_yaw_rate := 5.0       # скорость разворота камеры за курсом корабля, 1/с
var snapshot_delay := 0.12       # задержка интерполяции снимков состояния, с

var warnings: PackedStringArray = PackedStringArray()


static func _num(d: Dictionary, key: String, default: float, lo: float, hi: float, w: PackedStringArray) -> float:
	var v := float(d.get(key, default))
	if is_nan(v) or is_inf(v):
		w.append("%s: некорректное число, взято %s" % [key, default])
		return default
	var c := clampf(v, lo, hi)
	if c != v:
		w.append("%s: %s вне диапазона [%s, %s], ограничено до %s" % [key, v, lo, hi, c])
	return c


static func from_dict(d: Dictionary) -> Resource:
	var c = load("res://ui/camera/space_camera_config.gd").new()
	var w := PackedStringArray()
	c.tilt_min = _num(d, "tilt_min", 25.0, 5.0, 85.0, w)
	c.tilt_max = _num(d, "tilt_max", 65.0, 5.0, 85.0, w)
	if c.tilt_min > c.tilt_max:
		w.append("tilt_min больше tilt_max: значения поменяны местами")
		var t: float = c.tilt_min
		c.tilt_min = c.tilt_max
		c.tilt_max = t
	c.tilt_initial = clampf(_num(d, "tilt_initial", 45.0, 5.0, 85.0, w), c.tilt_min, c.tilt_max)
	c.zoom_min = _num(d, "zoom_min", 18.0, 5.0, 500.0, w)
	c.zoom_max = _num(d, "zoom_max", 100.0, 5.0, 500.0, w)
	if c.zoom_min > c.zoom_max:
		w.append("zoom_min больше zoom_max: значения поменяны местами")
		var z: float = c.zoom_min
		c.zoom_min = c.zoom_max
		c.zoom_max = z
	c.zoom_initial = clampf(_num(d, "zoom_initial", 50.0, 5.0, 500.0, w), c.zoom_min, c.zoom_max)
	c.follow_rate = _num(d, "follow_rate", 14.0, 1.0, 60.0, w)
	c.follow_lead = bool(d.get("follow_lead", true))
	c.yaw_rate = _num(d, "yaw_rate", 12.0, 1.0, 60.0, w)
	c.yaw_max_speed = _num(d, "yaw_max_speed", 540.0, 90.0, 2000.0, w)
	c.tilt_rate = _num(d, "tilt_rate", 12.0, 1.0, 60.0, w)
	c.zoom_rate = _num(d, "zoom_rate", 10.0, 1.0, 60.0, w)
	c.orbit_sensitivity = _num(d, "orbit_sensitivity", 0.25, 0.02, 2.0, w)
	c.tilt_sensitivity = _num(d, "tilt_sensitivity", 0.18, 0.02, 2.0, w)
	c.pinch_sensitivity = _num(d, "pinch_sensitivity", 1.0, 0.2, 3.0, w)
	c.wheel_zoom_step = _num(d, "wheel_zoom_step", 0.12, 0.01, 0.5, w)
	var fo: Variant = d.get("focus_offset", [0.12, 0.0])
	if fo is Array and fo.size() >= 2:
		c.focus_offset = Vector2(clampf(float(fo[0]), -0.4, 0.4), clampf(float(fo[1]), -0.4, 0.4))
	else:
		w.append("focus_offset: ожидается массив [x, y]")
	c.auto_center_enabled = bool(d.get("auto_center_enabled", false))
	c.auto_center_delay = _num(d, "auto_center_delay", 2.5, 0.2, 30.0, w)
	c.auto_center_rate = _num(d, "auto_center_rate", 1.8, 0.2, 20.0, w)
	c.auto_center_min_speed = _num(d, "auto_center_min_speed", 1.0, 0.0, 50.0, w)
	c.top_tilt = _num(d, "top_tilt", 0.0, 0.0, 30.0, w)
	c.top_zoom = clampf(_num(d, "top_zoom", 32.0, 5.0, 500.0, w), c.zoom_min, c.zoom_max)
	c.chase_tilt = _num(d, "chase_tilt", 70.0, 45.0, 85.0, w)
	c.chase_zoom = clampf(_num(d, "chase_zoom", 24.0, 5.0, 500.0, w), c.zoom_min, c.zoom_max)
	c.chase_offset_y = _num(d, "chase_offset_y", 0.18, -0.4, 0.4, w)
	c.locked_yaw_rate = _num(d, "locked_yaw_rate", 5.0, 0.5, 30.0, w)
	c.snapshot_delay = _num(d, "snapshot_delay", 0.12, 0.0, 0.5, w)
	c.warnings = w
	return c


func to_dict() -> Dictionary:
	return {
		"tilt_initial": tilt_initial, "tilt_min": tilt_min, "tilt_max": tilt_max,
		"zoom_initial": zoom_initial, "zoom_min": zoom_min, "zoom_max": zoom_max,
		"follow_rate": follow_rate, "follow_lead": follow_lead, "yaw_rate": yaw_rate, "yaw_max_speed": yaw_max_speed, "tilt_rate": tilt_rate, "zoom_rate": zoom_rate,
		"orbit_sensitivity": orbit_sensitivity, "tilt_sensitivity": tilt_sensitivity, "pinch_sensitivity": pinch_sensitivity,
		"wheel_zoom_step": wheel_zoom_step, "focus_offset": [focus_offset.x, focus_offset.y],
		"auto_center_enabled": auto_center_enabled, "auto_center_delay": auto_center_delay,
		"auto_center_rate": auto_center_rate, "auto_center_min_speed": auto_center_min_speed,
		"top_tilt": top_tilt, "top_zoom": top_zoom, "chase_tilt": chase_tilt, "chase_zoom": chase_zoom,
		"chase_offset_y": chase_offset_y, "locked_yaw_rate": locked_yaw_rate,
		"snapshot_delay": snapshot_delay,
	}


const DEFAULT_PATH := "res://assets/space/camera.json"


## Конфигурация по умолчанию: из файла данных, а при его отсутствии или порче значения по умолчанию.
static func load_default(path: String = DEFAULT_PATH) -> Resource:
	var text := FileAccess.get_file_as_string(path)
	if text != "":
		var parsed: Variant = JSON.parse_string(text)
		if typeof(parsed) == TYPE_DICTIONARY:
			return from_dict(parsed)
	var c = from_dict({})
	c.warnings.append("файл камеры не найден или повреждён: %s, взяты значения по умолчанию" % path)
	return c
