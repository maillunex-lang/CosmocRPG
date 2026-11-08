extends SceneTree
## Тест камеры космоса (без рендеринга): конфигурация, ядро (наклон, 360°, масштаб, сброс, автоцентрирование,
## следование, смещение корабля в экранных осях), жесты, буфер интерполяции, математика параллакса.
## Запуск: godot --headless --path client --script res://tests/camera_test.gd

const Config := preload("res://ui/camera/space_camera_config.gd")
const RigScript := preload("res://ui/camera/space_camera_rig.gd")
const Gestures := preload("res://ui/camera/camera_gestures.gd")
const SnapshotBuffer := preload("res://scripts/space/snapshot_buffer.gd")

var _failures := 0
var _checks := 0


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _holder() -> SubViewport:
	var h := SubViewport.new()
	h.size = Vector2i(1280, 720)
	h.disable_3d = false
	root.add_child(h)
	return h


func _make_rig(h: SubViewport, cfg: Resource = null) -> Node3D:
	var rig := Node3D.new()
	rig.set_script(RigScript)
	var yaw := Node3D.new()
	yaw.name = "Yaw"
	rig.add_child(yaw)
	var pitch := Node3D.new()
	pitch.name = "Pitch"
	yaw.add_child(pitch)
	var cam := Camera3D.new()
	cam.name = "Camera3D"
	pitch.add_child(cam)
	if cfg != null:
		rig.config = cfg
	h.add_child(rig)
	cam.current = true
	return rig


## Положение точки мира (12, 0, 0) на экране: смещение этой точки за кадр показывает, как «видит» вращение игрок.
func _probe(rig: Node3D) -> Vector2:
	return rig.camera.unproject_position(rig.position + Vector3(12, 0, 0))


func _settle(rig: Node3D, seconds: float, target: Vector3 = Vector3.ZERO, heading: Vector2 = Vector2(0, -1), speed: float = 0.0) -> void:
	var t := 0.0
	while t < seconds:
		rig.update(1.0 / 60.0, target, heading, speed)
		t += 1.0 / 60.0


func _run() -> void:
	# ============ конфигурация ============
	var c = Config.from_dict({})
	_check(c.warnings.is_empty() and c.tilt_initial == 45.0 and c.tilt_min == 25.0 and c.tilt_max == 65.0, "конфигурация по умолчанию: наклон 45° (25–65°)")
	c = Config.from_dict({"tilt_min": 70, "tilt_max": 30, "tilt_initial": 5, "zoom_min": 90, "zoom_max": 10, "zoom_initial": 1000, "orbit_sensitivity": -3, "focus_offset": [9, -9]})
	_check(c.tilt_min == 30.0 and c.tilt_max == 70.0 and c.tilt_initial == 30.0, "конфигурация: границы наклона меняются местами, начальный внутри диапазона")
	_check(c.zoom_min == 10.0 and c.zoom_max == 90.0 and c.zoom_initial == 90.0, "конфигурация: границы масштаба меняются местами, начальный внутри диапазона")
	_check(c.orbit_sensitivity == 0.02 and c.focus_offset == Vector2(0.4, -0.4) and c.warnings.size() >= 3, "конфигурация: чувствительность и смещение ограничены, есть замечания (%d)" % c.warnings.size())
	c = Config.from_dict({"tilt_min": 85, "tilt_max": 99})
	_check(c.tilt_max == 85.0, "конфигурация: наклон не выше 85° (иначе камера смотрит вдоль плоскости)")
	_check(Config.from_dict({"zoom_min": NAN}).zoom_min == 18.0, "конфигурация: NaN заменяется значением по умолчанию")
	var rt = Config.from_dict(Config.from_dict({"tilt_max": 60.0, "wheel_zoom_step": 0.2, "auto_center_enabled": true}).to_dict())
	_check(rt.tilt_max == 60.0 and is_equal_approx(rt.wheel_zoom_step, 0.2) and rt.auto_center_enabled, "конфигурация: словарь -> объект -> словарь без потерь")
	var df = Config.load_default()
	_check(df.warnings.is_empty() and df.zoom_min < df.zoom_initial and df.zoom_initial < df.zoom_max, "файл camera.json читается без замечаний")
	var broken = Config.load_default("res://нет/файла.json")
	_check(not broken.warnings.is_empty() and broken.tilt_initial == 45.0, "отсутствующий файл камеры: значения по умолчанию и замечание")

	# ============ ядро камеры ============
	var h := _holder()
	var rig := _make_rig(h)
	await process_frame
	_check(rig.camera.projection == Camera3D.PROJECTION_ORTHOGONAL, "проекция остаётся ортографической")
	_check(rig.tilt_deg() == 45.0 and rig.zoom_size() == 50.0 and rig.azimuth_deg() == 0.0, "начальный вид: наклон 45°, масштаб 50, азимут 0°")
	_check(is_equal_approx(rig.camera.size, 50.0), "размер обзора камеры соответствует масштабу")

	# наклон: границы и направление
	rig.orbit(0, 100000)     # палец ушёл вниз: камера поднимается (наклон от вертикали уменьшается)
	_check(rig.tilt_target_deg() == 25.0, "наклон: нижняя граница 25°")
	rig.orbit(0, -100000)
	_check(rig.tilt_target_deg() == 65.0, "наклон: верхняя граница 65°")
	rig.orbit(0, 100)
	_check(rig.tilt_target_deg() < 65.0, "свайп вниз уменьшает наклон (камера выше)")
	var tilt_min_seen := 999.0
	var tilt_max_seen := -999.0
	for i in 400:
		rig.orbit(0, sin(i * 0.3) * 3000.0)
		rig.update(1.0 / 60.0, Vector3.ZERO, Vector2(0, -1))
		tilt_min_seen = minf(tilt_min_seen, rig.tilt_deg())
		tilt_max_seen = maxf(tilt_max_seen, rig.tilt_deg())
	_check(tilt_min_seen >= 25.0 - 1e-4 and tilt_max_seen <= 65.0 + 1e-4, "наклон при любых жестах остаётся в [25°, 65°] (%.2f…%.2f)" % [tilt_min_seen, tilt_max_seen])
	_check(rig.camera.global_position.y > 0.0, "камера всегда над плоскостью движения")

	# горизонтальное вращение непрерывно, без ограничений
	rig.set_view(0, 45, 50)
	var prev_yaw: float = rig.yaw_target_rad()
	var max_step := 0.0
	var last_raw := 0.0
	var monotonic := true
	rig.orbit(-2880.0, 0.0)   # 720° (свайп влево) при чувствительности 0,25°/px
	_check(is_equal_approx(rad_to_deg(rig.yaw_target_rad() - prev_yaw), 720.0), "вращение: 720° накапливаются без ограничения 360° (%.1f)" % rad_to_deg(rig.yaw_target_rad() - prev_yaw))
	for i in 600:
		rig.update(1.0 / 60.0, Vector3.ZERO, Vector2(0, -1))
		var raw := rad_to_deg(rig._yaw)
		max_step = maxf(max_step, absf(raw - last_raw))
		if raw < last_raw - 1e-6:
			monotonic = false
		last_raw = raw
	_check(monotonic and max_step <= 540.0 / 60.0 + 0.01, "вращение на 720°: непрерывно, монотонно, угловая скорость ограничена (макс. %.1f° за кадр)" % max_step)
	_check(absf(rig.azimuth_deg()) < 0.5 or absf(rig.azimuth_deg() - 360.0) < 0.5, "после двух оборотов азимут снова около 0° (%.2f°)" % rig.azimuth_deg())
	rig.orbit(1500.0, 0.0)  # обратное вращение: -375°
	_settle(rig, 4.0)
	_check(absf(rig.azimuth_deg() - fposmod(-375.0, 360.0)) < 0.5, "обратное вращение за границу 0°/360°: азимут %.2f°" % rig.azimuth_deg())
	# при пересечении границы положение камеры меняется плавно (нет скачков)
	rig.set_view(350, 45, 50)
	var prev_pt := _probe(rig)
	var max_jump := 0.0
	var ship_moved := 0.0
	var ship_pt0: Vector2 = rig.camera.unproject_position(rig.position)
	rig.orbit(160.0, 0.0)    # +40°: пересекаем 360°
	for i in 120:
		rig.update(1.0 / 60.0, Vector3.ZERO, Vector2(0, -1))
		var pt := _probe(rig)
		max_jump = maxf(max_jump, pt.distance_to(prev_pt))
		prev_pt = pt
		ship_moved = maxf(ship_moved, rig.camera.unproject_position(rig.position).distance_to(ship_pt0))
	_check(max_jump < 60.0, "переход через 360°: точка мира смещается на экране не больше %.1f px за кадр (плавно)" % max_jump)
	_check(ship_moved < 1.0, "при вращении сам корабль остаётся на месте экрана (%.2f px)" % ship_moved)

	# масштаб
	rig.set_view(0, 45, 50)
	rig.zoom_by(1000.0)
	_check(rig.zoom_target() == 18.0, "масштаб: максимальное приближение 18")
	rig.zoom_by(0.0001)
	_check(rig.zoom_target() == 100.0, "масштаб: максимальное отдаление 100")
	var z0: float = rig.zoom_target()
	for bad in [0.0, -2.0, NAN, INF]:
		rig.zoom_by(bad)
	_check(rig.zoom_target() == z0, "масштаб: некорректные множители игнорируются")
	rig.set_view(0, 45, 50)
	rig.zoom_by(2.0)
	_check(is_equal_approx(rig.zoom_target(), 25.0), "щипок в 2 раза приближает вдвое (25)")
	var s_prev: float = rig.zoom_size()
	var overshoot := false
	for i in 200:
		rig.update(1.0 / 60.0, Vector3.ZERO, Vector2(0, -1))
		if rig.zoom_size() < 25.0 - 1e-3 or rig.zoom_size() > s_prev + 1e-6:
			overshoot = true
		s_prev = rig.zoom_size()
	_check(not overshoot and absf(rig.zoom_size() - 25.0) < 0.05, "масштаб сглаживается без перелёта (%.3f)" % rig.zoom_size())
	_check(is_equal_approx(rig.camera.size, rig.zoom_size()), "размер обзора камеры следует за масштабом")

	# сброс
	rig.set_view(200, 60, 20)
	rig.orbit(1000.0, 0.0)
	rig.zoom_by(0.3)
	var ship_dir := Vector2(1, 0)
	_settle(rig, 0.2, Vector3.ZERO, ship_dir)
	var yaw_before: float = rig._yaw
	rig.reset()
	_check(absf(angle_difference(rig.yaw_target_rad(), RigScript.behind_yaw(ship_dir))) < 1e-4 and absf(rig.yaw_target_rad() - yaw_before) <= PI + 1e-4, "сброс: цель по азимуту за кораблём по кратчайшему пути")
	var jump := 0.0
	prev_pt = _probe(rig)
	for i in 300:
		rig.update(1.0 / 60.0, Vector3.ZERO, ship_dir)
		var pt2 := _probe(rig)
		jump = maxf(jump, pt2.distance_to(prev_pt))
		prev_pt = pt2
	_check(jump < 60.0, "сброс плавный: точка мира смещается на экране не больше %.1f px за кадр" % jump)
	_check(absf(rig.tilt_deg() - 45.0) < 0.05 and absf(rig.zoom_size() - 50.0) < 0.1, "сброс: стандартные наклон 45° и масштаб 50")
	var from_cam: Vector3 = rig.camera.global_position - rig.global_position
	var horiz := Vector2(from_cam.x, from_cam.z).normalized()
	_check(horiz.dot(-ship_dir) > 0.999, "сброс: камера позади корабля, смотрящего вдоль (1, 0) (cos = %.4f)" % horiz.dot(-ship_dir))
	# позади корабля при любом направлении
	var all_behind := true
	for ang in range(0, 360, 30):
		var d := Vector2.from_angle(deg_to_rad(ang))
		rig.set_view(rad_to_deg(RigScript.behind_yaw(d)), 45, 50)
		var cp: Vector3 = rig.camera.global_position - rig.global_position
		if Vector2(cp.x, cp.z).normalized().dot(-d) < 0.999:
			all_behind = false
	_check(all_behind, "behind_yaw: камера позади корабля при любом курсе")
	# кратчайший путь через границу
	rig.set_view(350, 45, 50)
	rig._heading = Vector2.from_angle(deg_to_rad(0))
	var target_behind := RigScript.behind_yaw(rig._heading)
	rig.reset()
	_check(absf(rig.yaw_target_rad() - rig._yaw) <= PI + 1e-4, "сброс идёт по кратчайшему пути, не разворачиваясь в обход (%.0f°)" % rad_to_deg(rig.yaw_target_rad() - rig._yaw))
	_check(absf(angle_difference(rig.yaw_target_rad(), target_behind)) < 1e-4, "сброс: итоговый угол эквивалентен нужному")

	# ============ автоцентрирование ============
	var cfg_off = Config.from_dict({})
	var rig_off := _make_rig(h, cfg_off)
	await process_frame
	rig_off.set_view(120, 45, 50)
	_settle(rig_off, 12.0, Vector3.ZERO, Vector2(0, -1), 14.0)
	_check(absf(rig_off.azimuth_deg() - 120.0) < 0.01, "автоцентрирование выключено по умолчанию: ручной вид сохраняется")
	var cfg_on = Config.from_dict({"auto_center_enabled": true, "auto_center_delay": 2.0})
	var rig_on := _make_rig(h, cfg_on)
	await process_frame
	rig_on.set_view(120, 45, 50)
	_settle(rig_on, 1.5, Vector3.ZERO, Vector2(0, -1), 14.0)
	_check(absf(rig_on.azimuth_deg() - 120.0) < 0.01, "автоцентрирование ждёт паузу после ручного действия")
	_settle(rig_on, 10.0, Vector3.ZERO, Vector2(0, -1), 14.0)
	var want := fposmod(rad_to_deg(RigScript.behind_yaw(Vector2(0, -1))), 360.0)
	_check(absf(angle_difference(deg_to_rad(rig_on.azimuth_deg()), deg_to_rad(want))) < deg_to_rad(2.0), "автоцентрирование: камера выровнялась за кораблём (%.1f° из %.1f°)" % [rig_on.azimuth_deg(), want])
	rig_on.orbit(-200.0, 0.0)
	var az1: float = rig_on.azimuth_deg()
	_settle(rig_on, 1.0, Vector3.ZERO, Vector2(0, -1), 14.0)
	_check(absf(angle_difference(deg_to_rad(rig_on.azimuth_deg()), deg_to_rad(az1 + 50.0))) < deg_to_rad(1.0), "ручное действие в приоритете: автоцентрирование не перебивает свайп сразу")
	rig_on.set_view(120, 45, 50)
	_settle(rig_on, 8.0, Vector3.ZERO, Vector2(0, -1), 0.2)
	_check(absf(rig_on.azimuth_deg() - 120.0) < 0.01, "автоцентрирование не работает, пока корабль стоит")
	# через границу 0/360 — кратчайший путь
	rig_on.set_view(350, 45, 50)
	rig_on._idle = 100.0
	var max_a_step := 0.0
	var last_a: float = rig_on._yaw
	for i in 600:
		rig_on.update(1.0 / 60.0, Vector3.ZERO, Vector2.from_angle(deg_to_rad(20)), 14.0)
		max_a_step = maxf(max_a_step, absf(rig_on._yaw - last_a))
		last_a = rig_on._yaw
	_check(max_a_step < 0.2, "автоцентрирование без скачков через границу 360°")

	# ============ следование за кораблём ============
	var cfg_f = Config.from_dict({"follow_lead": true})
	var rig_f := _make_rig(h, cfg_f)
	await process_frame
	var pos := Vector3.ZERO
	rig_f.snap_to(pos)
	for i in 300:
		pos += Vector3(14.0 / 60.0, 0, 0)
		rig_f.update(1.0 / 60.0, pos, Vector2(1, 0), 14.0)
	_check(rig_f.position.distance_to(pos) < 0.02, "следование: при равномерном движении ни отставания, ни опережения (%.4f ед.)" % rig_f.position.distance_to(pos))
	var cfg_nl = Config.from_dict({"follow_lead": false})
	var rig_nl := _make_rig(h, cfg_nl)
	await process_frame
	rig_nl.snap_to(Vector3.ZERO)
	pos = Vector3.ZERO
	for i in 300:
		pos += Vector3(14.0 / 60.0, 0, 0)
		rig_nl.update(1.0 / 60.0, pos, Vector2(1, 0), 14.0)
	_check(rig_nl.position.distance_to(pos) > 0.5, "без компенсации отставание действительно было бы (%.2f ед.): компенсация нужна" % rig_nl.position.distance_to(pos))
	rig_f.snap_to(Vector3(50, 0, 50))
	_check(rig_f.position == Vector3(50, 0, 50), "snap_to: мгновенный перенос фокуса")
	for i in 120:
		rig_f.update(1.0 / 60.0, Vector3(80, 0, 20), Vector2(1, 0), 0.0)
	_check(rig_f.position.distance_to(Vector3(80, 0, 20)) < 0.05, "следование: за остановившимся кораблём камера приходит ровно")
	# сглаживание: ступенчатые данные 10 Гц превращаются в более плавное движение камеры
	var rig_s := _make_rig(h, Config.from_dict({}))
	await process_frame
	rig_s.snap_to(Vector3.ZERO)
	var stepped := Vector3.ZERO
	var in_var := 0.0
	var out_var := 0.0
	var prev_in := Vector3.ZERO
	var prev_out := Vector3.ZERO
	for i in 300:
		if i % 6 == 0:
			stepped += Vector3(1.4, 0, 0)  # шаг симуляции каждые 0,1 с
		rig_s.update(1.0 / 60.0, stepped, Vector2(1, 0), 14.0)
		if i > 30:
			in_var = maxf(in_var, absf((stepped - prev_in).x))
			out_var = maxf(out_var, absf((rig_s.position - prev_out).x))
		prev_in = stepped
		prev_out = rig_s.position
	_check(out_var < in_var * 0.5, "ступенчатые данные сглаживаются камерой: макс. шаг %.3f вместо %.3f" % [out_var, in_var])

	# ============ смещение корабля в экранных осях ============
	var rig_o := _make_rig(h, Config.from_dict({}))
	await process_frame
	rig_o.snap_to(Vector3.ZERO)
	var xs: Array[float] = []
	var ok_visible := true
	for yaw in range(0, 360, 45):
		for tilt in [25.0, 45.0, 65.0]:
			for zoom in [18.0, 50.0, 100.0]:
				rig_o.set_view(yaw, tilt, zoom)
				var sp: Vector2 = rig_o.camera.unproject_position(Vector3.ZERO)
				if yaw == 0 and tilt == 45.0 and zoom == 50.0:
					xs.append(sp.x)
				if rig_o.camera.is_position_behind(Vector3.ZERO) or sp.x < 0 or sp.x > 1280 or sp.y < 0 or sp.y > 720:
					ok_visible = false
	_check(ok_visible, "корабль виден на экране при любых азимуте, наклоне и масштабе (всего 72 вида)")
	rig_o.set_view(0, 45, 50)
	var base_x: float = rig_o.camera.unproject_position(Vector3.ZERO).x
	_check(base_x < 640.0 - 60.0, "корабль смещён влево от центра, освобождая центр между колонками HUD (x = %.0f)" % base_x)
	var same := true
	for yaw in [0, 90, 180, 270, 360]:
		rig_o.set_view(yaw, 45, 50)
		if absf(rig_o.camera.unproject_position(Vector3.ZERO).x - base_x) > 1.0:
			same = false
	_check(same, "смещение корабля на экране не зависит от азимута (не вращается вместе с камерой)")
	var frac := []
	for zoom in [20.0, 50.0, 90.0]:
		rig_o.set_view(0, 45, zoom)
		frac.append((640.0 - rig_o.camera.unproject_position(Vector3.ZERO).x) / 720.0 * zoom / zoom)
	_check(absf(frac[0] - frac[2]) < 0.02 * 720.0 / 720.0 + 0.1, "доля смещения от размера обзора сохраняется при зуме")

	# ============ параллакс: адаптер между камерой и фоном ============
	var rig_p := _make_rig(h, Config.from_dict({"follow_rate": 60.0, "follow_lead": false}))
	await process_frame
	rig_p.viewport_height = 720.0
	rig_p.snap_to(Vector3(10, 0, 10))
	_settle(rig_p, 0.5, Vector3(10, 0, 10))
	var base: Vector2 = rig_p.parallax_px()
	for yaw in [30, 90, 180, 270, 360]:
		rig_p.set_view(yaw, 45, 50)
		_settle(rig_p, 0.2, Vector3(10, 0, 10))
	_check(rig_p.parallax_px().distance_to(base) < 0.01, "параллакс: вращение камеры вокруг неподвижного корабля фон не двигает")
	for tilt in [25.0, 65.0]:
		rig_p.set_view(0, tilt, 50)
		_settle(rig_p, 0.2, Vector3(10, 0, 10))
	for zoom in [18.0, 100.0]:
		rig_p.set_view(0, 45, zoom)
		_settle(rig_p, 0.2, Vector3(10, 0, 10))
	_check(rig_p.parallax_px().distance_to(base) < 0.01, "параллакс: наклон и масштаб фон не двигают")
	# движение корабля вправо по экрану сдвигает фон вправо (область видимости вправо) при любом азимуте
	var right_ok := true
	var fwd_ok := true
	for yaw in [0, 60, 135, 200, 300]:
		rig_p.set_view(yaw, 45, 50)
		rig_p._parallax_px = Vector2.ZERO
		var axes: Dictionary = rig_p.screen_axes()
		var start: Vector3 = rig_p.position
		rig_p.snap_to(start)
		var p2: Vector3 = start
		for i in 60:
			p2 += (axes["right"] as Vector3) * (5.0 / 60.0)
			rig_p.update(1.0 / 60.0, p2, Vector2(1, 0), 5.0)
		var dpx: Vector2 = rig_p.parallax_px()
		if not (dpx.x > 20.0 and absf(dpx.y) < 2.0):
			right_ok = false
		rig_p._parallax_px = Vector2.ZERO
		rig_p.snap_to(p2)
		var p3: Vector3 = p2
		for i in 60:
			p3 += (axes["forward"] as Vector3) * (5.0 / 60.0)
			rig_p.update(1.0 / 60.0, p3, Vector2(1, 0), 5.0)
		var dpf: Vector2 = rig_p.parallax_px()
		if not (dpf.y < -10.0 and absf(dpf.x) < 2.0):
			fwd_ok = false
	_check(right_ok, "параллакс: движение вправо по экрану даёт сдвиг по X при любом азимуте")
	_check(fwd_ok, "параллакс: движение вперёд (вверх по экрану) даёт сдвиг по Y при любом азимуте")
	# укорочение плоскости: вперёд на экране меньше при большем наклоне (камера ниже)
	var dy_by_tilt := {}
	for tilt in [25.0, 65.0]:
		rig_p.set_view(0, tilt, 50)
		rig_p._parallax_px = Vector2.ZERO
		var q: Vector3 = rig_p.position
		rig_p.snap_to(q)
		for i in 60:
			q += Vector3(0, 0, -5.0 / 60.0)
			rig_p.update(1.0 / 60.0, q, Vector2(0, -1), 5.0)
		dy_by_tilt[tilt] = absf(rig_p.parallax_px().y)
	_check(dy_by_tilt[25.0] > dy_by_tilt[65.0] * 1.5, "параллакс учитывает укорочение плоскости при наклоне (%.1f против %.1f px)" % [dy_by_tilt[25.0], dy_by_tilt[65.0]])
	# масштаб экранного движения: при большем приближении тот же путь даёт больший сдвиг
	var dx_by_zoom := {}
	for zoom in [25.0, 100.0]:
		rig_p.set_view(0, 45, zoom)
		rig_p._parallax_px = Vector2.ZERO
		var q2: Vector3 = rig_p.position
		rig_p.snap_to(q2)
		for i in 60:
			q2 += Vector3(5.0 / 60.0, 0, 0)
			rig_p.update(1.0 / 60.0, q2, Vector2(1, 0), 5.0)
		dx_by_zoom[zoom] = rig_p.parallax_px().x
	_check(dx_by_zoom[25.0] > dx_by_zoom[100.0] * 3.0, "параллакс масштабируется вместе с зумом (%.1f против %.1f px)" % [dx_by_zoom[25.0], dx_by_zoom[100.0]])
	h.queue_free()

	# ============ режимы камеры: свободный, сверху, сзади ============
	var hm := _holder()
	var rm := _make_rig(hm)
	await process_frame
	var changes: Array = []
	rm.mode_changed.connect(func(m: int) -> void: changes.append(m))
	_check(rm.mode() == 0 and rm.mode_name() == "Свободный", "по умолчанию свободный режим")
	rm.set_view(0, 45, 50)
	rm.cycle_mode()
	_check(rm.mode() == 1 and rm.mode_name() == "Сверху", "первое нажатие: вид сверху")
	var hd := Vector2(1, 0)   # корабль летит вдоль +X
	_settle(rm, 3.0, Vector3.ZERO, hd, 5.0)
	_check(absf(rm.tilt_deg()) < 0.2, "вид сверху: камера строго сверху (наклон %.2f°)" % rm.tilt_deg())
	var fwd: Vector3 = rm.screen_axes()["forward"]
	_check(fwd.dot(Vector3(1, 0, 0)) > 0.99, "вид сверху: курс корабля направлен вверх по экрану")
	var hd2 := Vector2(0, 1)   # корабль повернул к +Z
	_settle(rm, 3.0, Vector3.ZERO, hd2, 5.0)
	fwd = rm.screen_axes()["forward"]
	_check(fwd.dot(Vector3(0, 0, 1)) > 0.99, "вид сверху: камера развернулась вслед за кораблём")
	var tl0: float = rm.tilt_target_deg()
	var yt0: float = rm.yaw_target_rad()
	rm.orbit(300.0, 200.0)
	_check(rm.tilt_target_deg() == tl0 and rm.yaw_target_rad() == yt0, "вид сверху: вращение и наклон жестом отключены")
	var zm0: float = rm.zoom_target()
	rm.zoom_by(1.5)
	_check(rm.zoom_target() < zm0, "вид сверху: масштаб работает")
	rm.cycle_mode()
	_check(rm.mode() == 2 and rm.mode_name() == "Сзади", "второе нажатие: вид сзади")
	_settle(rm, 3.0, Vector3.ZERO, hd, 5.0)
	_check(absf(rm.tilt_deg() - rm.config.chase_tilt) < 0.3 and rm.tilt_deg() > 45.0, "вид сзади: камера низко, наклон %.1f°" % rm.tilt_deg())
	fwd = rm.screen_axes()["forward"]
	_check(fwd.dot(Vector3(1, 0, 0)) > 0.99, "вид сзади: камера позади корабля")
	_settle(rm, 2.0, Vector3.ZERO, Vector2(-1, 0), 5.0)
	fwd = rm.screen_axes()["forward"]
	_check(fwd.dot(Vector3(-1, 0, 0)) > 0.99, "вид сзади: камера разворачивается при повороте корабля на 180°")
	tl0 = rm.tilt_target_deg()
	rm.orbit(300.0, 200.0)
	_check(rm.tilt_target_deg() == tl0, "вид сзади: вращение жестом отключено")
	rm.cycle_mode()
	_check(rm.mode() == 0, "третье нажатие: свободный 3D")
	_check(rm.tilt_target_deg() == rm.config.tilt_initial and rm.zoom_target() == rm.config.zoom_initial, "возврат в свободный режим: стандартный наклон и масштаб")
	var yt1: float = rm.yaw_target_rad()
	rm.orbit(100.0, 0.0)
	_check(rm.yaw_target_rad() != yt1, "в свободном режиме вращение снова работает")
	_check(changes == [1, 2, 0], "сигнал mode_changed: 1, 2, 0 (%s)" % str(changes))
	hm.queue_free()

	# ============ жесты ============
	await _gestures_tests()

	# ============ буфер интерполяции ============
	_snapshot_tests()

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)


class Rec:
	var orbits: Array = []
	var pinches: Array = []
	var taps: Array = []
	var started: Array = []
	var ended := 0

	func _init(g: RefCounted) -> void:
		g.orbited.connect(func(d: Vector2) -> void: orbits.append(d))
		g.pinched.connect(func(f: float) -> void: pinches.append(f))
		g.tapped.connect(func(p: Vector2) -> void: taps.append(p))
		g.gesture_started.connect(func(k: String) -> void: started.append(k))
		g.gesture_ended.connect(func() -> void: ended += 1)

	func total_orbit() -> Vector2:
		var s := Vector2.ZERO
		for o in orbits:
			s += o
		return s

	func total_pinch() -> float:
		var p := 1.0
		for f in pinches:
			p *= f
		return p


func _gestures_tests() -> void:
	# тап
	var g := Gestures.new()
	var r := Rec.new(g)
	g.touch(0, true, Vector2(100, 100), 0)
	g.touch(0, false, Vector2(103, 102), 120)
	_check(r.taps.size() == 1 and r.taps[0] == Vector2(103, 102) and r.orbits.is_empty(), "жест: короткое касание — тап")
	_check(g.state == Gestures.State.IDLE and g.touch_count() == 0, "жест: после тапа состояние чистое")
	g.touch(0, true, Vector2(100, 100), 0)
	g.touch(0, false, Vector2(100, 100), 800)
	_check(r.taps.size() == 1, "жест: долгое удержание без движения — не тап")
	# вращение
	g = Gestures.new()
	r = Rec.new(g)
	g.touch(0, true, Vector2(100, 100), 0)
	g.drag(0, Vector2(105, 100))
	_check(r.orbits.is_empty() and g.state == Gestures.State.PENDING, "жест: движение меньше порога вращением не считается")
	g.drag(0, Vector2(130, 100))
	_check(r.started == ["orbit"] and r.orbits.is_empty(), "жест: переход через порог начинает вращение без скачка (первое смещение не применяется)")
	g.drag(0, Vector2(150, 110))
	_check(r.orbits.size() == 1 and r.orbits[0] == Vector2(20, 10), "жест: далее приходят приращения пальца (%s)" % str(r.orbits))
	g.drag(0, Vector2(150, 90))
	_check(r.total_orbit() == Vector2(20, -10), "жест: сумма приращений равна пути пальца после порога")
	g.touch(0, false, Vector2(150, 90), 400)
	_check(r.taps.is_empty() and r.ended == 1 and g.state == Gestures.State.IDLE, "жест: после вращения тапа нет, жест завершён")
	# щипок
	g = Gestures.new()
	r = Rec.new(g)
	g.touch(3, true, Vector2(300, 300), 0)
	g.touch(7, true, Vector2(400, 300), 20)
	_check(r.started == ["pinch"] and g.state == Gestures.State.PINCH, "жест: второй палец начинает щипок")
	g.drag(7, Vector2(500, 300))
	g.drag(7, Vector2(600, 300))
	_check(absf(r.total_pinch() - 3.0) < 1e-4, "жест: щипок 100 -> 300 px даёт коэффициент 3 (%.3f)" % r.total_pinch())
	g.drag(7, Vector2(450, 300))
	_check(absf(r.total_pinch() - 1.5) < 1e-4, "жест: обратное сближение уменьшает коэффициент (%.3f)" % r.total_pinch())
	g.drag(3, Vector2(300, 320))  # палец ушёл вбок: расстояние изменилось чуть-чуть
	_check(r.orbits.is_empty(), "жест: во время щипка камера не вращается")
	# второй палец не вызывает скачок: начальное расстояние запоминается
	_check(r.pinches.size() >= 3 and absf(r.pinches[0] - 2.0) < 1e-4, "жест: первое движение после касания второго пальца считается от его исходного положения")
	# один палец поднят: оставшийся не вращает
	g.touch(7, false, Vector2(450, 300), 600)
	g.drag(3, Vector2(100, 100))
	_check(r.orbits.is_empty() and g.state == Gestures.State.IGNORING, "жест: после щипка оставшийся палец не вращает камеру (нет скачка)")
	g.touch(3, false, Vector2(100, 100), 700)
	_check(g.state == Gestures.State.IDLE and r.taps.is_empty(), "жест: после отпускания всех пальцев состояние чистое, тапа нет")
	g.touch(1, true, Vector2(100, 100), 800)
	g.drag(1, Vector2(160, 100))
	g.drag(1, Vector2(190, 100))
	_check(not r.orbits.is_empty(), "жест: новое касание после щипка снова вращает камеру")
	# третий палец игнорируется
	g = Gestures.new()
	r = Rec.new(g)
	g.touch(0, true, Vector2(300, 300), 0)
	g.touch(1, true, Vector2(400, 300), 5)
	g.touch(2, true, Vector2(100, 100), 10)
	g.drag(2, Vector2(500, 500))
	g.touch(2, false, Vector2(500, 500), 30)
	_check(r.orbits.is_empty() and r.taps.is_empty() and g.state == Gestures.State.PINCH, "жест: третий палец ничего не меняет, щипок продолжается")
	g.drag(1, Vector2(500, 300))
	_check(absf(r.total_pinch() - 2.0) < 1e-4, "жест: щипок работает после третьего пальца")
	# повторные и неизвестные события
	g = Gestures.new()
	r = Rec.new(g)
	g.touch(0, true, Vector2(10, 10), 0)
	g.touch(0, true, Vector2(500, 500), 5)   # дубликат
	g.drag(9, Vector2(1, 1))                  # неизвестный палец
	g.touch(9, false, Vector2(1, 1), 10)
	_check(g.touch_count() == 1 and r.orbits.is_empty(), "жест: дубликаты и неизвестные события игнорируются")
	g.touch(0, false, Vector2(10, 10), 20)
	# мышь
	g = Gestures.new()
	r = Rec.new(g)
	g.touch(Gestures.MOUSE_LEFT, true, Vector2(50, 50), 0)
	g.touch(Gestures.MOUSE_LEFT, false, Vector2(50, 50), 80)
	_check(r.taps.size() == 1, "мышь: щелчок левой кнопкой — тап (выбор цели)")
	g.touch(Gestures.MOUSE_RIGHT, true, Vector2(200, 200), 100)
	g.drag(Gestures.MOUSE_RIGHT, Vector2(240, 200))
	g.drag(Gestures.MOUSE_RIGHT, Vector2(260, 210))
	g.touch(Gestures.MOUSE_RIGHT, false, Vector2(260, 210), 300)
	_check(r.orbits.size() == 1 and r.taps.size() == 1, "мышь: правая кнопка вращает камеру и не выбирает цель")
	var rw := Rec.new(g)
	g.wheel(1, 0.12)
	g.wheel(-1, 0.12)
	g.wheel(0, 0.12)
	_check(rw.pinches.size() == 2 and absf(rw.pinches[0] - 1.12) < 1e-6 and absf(rw.pinches[1] - 1.0 / 1.12 * 1.0) < 0.1 and rw.pinches[0] > 1.0 and rw.pinches[1] < 1.0, "колесо: вверх приближает, вниз отдаляет, ноль игнорируется")
	# отмена
	g = Gestures.new()
	r = Rec.new(g)
	g.touch(0, true, Vector2(100, 100), 0)
	g.drag(0, Vector2(150, 100))
	g.drag(0, Vector2(170, 100))
	g.cancel()
	_check(r.ended == 1 and g.state == Gestures.State.IDLE and g.touch_count() == 0, "отмена во время вращения завершает жест один раз")
	var n_orbits := r.orbits.size()
	g.drag(0, Vector2(300, 100))
	g.touch(0, false, Vector2(300, 100), 400)
	_check(r.orbits.size() == n_orbits and r.taps.is_empty(), "после отмены продолжение движения пальца ничего не делает, тапа нет")
	g.touch(0, true, Vector2(10, 10), 0)
	g.touch(1, true, Vector2(110, 10), 0)
	g.cancel()
	_check(g.state == Gestures.State.IDLE and r.ended == 2, "отмена во время щипка")
	var ended_before := r.ended
	g.cancel()
	_check(r.ended == ended_before, "отмена без активного жеста сигналов не даёт")
	# случайные потоки событий: состояние всегда согласовано, значения конечные
	var rng := RandomNumberGenerator.new()
	rng.seed = 7
	g = Gestures.new()
	r = Rec.new(g)
	var down := {}
	var t_ms := 0
	var consistent := true
	for step in 4000:
		t_ms += rng.randi_range(1, 40)
		var idx := rng.randi_range(0, 4)
		match rng.randi_range(0, 2):
			0:
				g.touch(idx, true, Vector2(rng.randf_range(0, 1280), rng.randf_range(0, 720)), t_ms)
				down[idx] = true
			1:
				g.drag(idx, Vector2(rng.randf_range(0, 1280), rng.randf_range(0, 720)))
			2:
				g.touch(idx, false, Vector2(rng.randf_range(0, 1280), rng.randf_range(0, 720)), t_ms)
				down.erase(idx)
		if rng.randi_range(0, 400) == 0:
			g.cancel()
			down.clear()
	for idx in down.keys():
		g.touch(idx, false, Vector2.ZERO, t_ms + 10)
	for o in r.orbits:
		if is_nan(o.x) or is_inf(o.x) or is_nan(o.y) or is_inf(o.y):
			consistent = false
	for f in r.pinches:
		if is_nan(f) or is_inf(f) or f <= 0.0:
			consistent = false
	_check(consistent and g.state == Gestures.State.IDLE and g.touch_count() == 0, "жесты: случайный поток из 4000 событий оставляет чистое состояние и конечные значения")


func _snapshot_tests() -> void:
	var b := SnapshotBuffer.new(0.12)
	_check(not b.sample(0.0)["valid"], "буфер: пустой не выдаёт позиций")
	b.push(1.0, 10.0, Vector2(0, 0), Vector2(1, 0))
	_check(b.sample(10.0)["valid"] and b.sample(10.0)["pos"] == Vector2.ZERO, "буфер: один снимок отдаёт свою позицию")
	_check(b.push(1.0, 10.1, Vector2(5, 5), Vector2(1, 0)) == false and b.count() == 1, "буфер: повтор того же времени игнорируется")
	b.push(1.1, 10.1, Vector2(1.4, 0), Vector2(1, 0))
	_check(b.push(1.05, 10.12, Vector2(9, 9), Vector2(1, 0)) == false and b.count() == 2, "буфер: снимок с более старым временем игнорируется")
	var mid: Dictionary = b.sample(10.0 + 0.05 + 0.12)  # середина между двумя снимками
	_check(absf(mid["pos"].x - 0.7) < 0.01, "буфер: между снимками позиция линейно интерполируется (%.3f)" % mid["pos"].x)
	_check(b.sample(0.0)["pos"] == Vector2.ZERO, "буфер: время раньше первого снимка даёт первый снимок")
	var far: Dictionary = b.sample(10.0 + 100.0)
	_check(far["pos"].x <= 1.4 + 14.0 * SnapshotBuffer.MAX_EXTRAPOLATION + 0.01 and far["pos"].x > 1.4, "буфер: экстраполяция ограничена по времени (%.2f)" % far["pos"].x)
	# плавность: снимки 10 Гц, кадры 60 Гц
	var smooth := SnapshotBuffer.new(0.12)
	var rng := RandomNumberGenerator.new()
	rng.seed = 3
	var raw_steps: Array[float] = []
	var out_steps: Array[float] = []
	var prev_out := -1.0
	var prev_raw := 0.0
	var sim_pos := 0.0
	for frame in 600:
		var now := frame / 60.0
		if frame % 6 == 0:
			sim_pos = frame / 60.0 * 14.0  # шаг симуляции раз в 0,1 с, скорость 14 ед/с
			var arrival := now + rng.randf_range(-0.012, 0.012)  # дрожание времени получения
			smooth.push(frame / 60.0, arrival, Vector2(sim_pos, 0), Vector2(1, 0))
		var s: Dictionary = smooth.sample(now)
		if frame > 60:
			out_steps.append(s["pos"].x - prev_out)
			raw_steps.append(sim_pos - prev_raw)
		prev_out = s["pos"].x
		prev_raw = sim_pos
	var mean_out := 0.0
	var max_dev_out := 0.0
	for v in out_steps:
		mean_out += v
	mean_out /= out_steps.size()
	for v in out_steps:
		max_dev_out = maxf(max_dev_out, absf(v - mean_out))
	var max_raw := 0.0
	for v in raw_steps:
		max_raw = maxf(max_raw, v)
	_check(absf(mean_out - 14.0 / 60.0) < 0.01 and max_dev_out < mean_out * 0.35, "интерполяция: ровное движение при 10 снимках в секунду и дрожании времени (отклонение шага %.0f%%)" % (100.0 * max_dev_out / mean_out))
	_check(max_raw > mean_out * 4.0, "без интерполяции шаги были бы рывками (макс. шаг %.2f против среднего %.2f)" % [max_raw, mean_out])
	# телепорт
	var tp := SnapshotBuffer.new(0.12)
	tp.push(1.0, 1.0, Vector2(0, 0), Vector2(1, 0))
	tp.push(1.1, 1.1, Vector2(1, 0), Vector2(1, 0))
	tp.push(1.2, 1.2, Vector2(500, 500), Vector2(1, 0))
	_check(tp.count() == 1 and tp.sample(1.3)["pos"] == Vector2(500, 500), "телепорт: буфер очищается, корабль не пролетает через весь мир")
	# направление через границу ±180°
	var dr := SnapshotBuffer.new(0.0)
	dr.push(0.0, 0.0, Vector2.ZERO, Vector2.from_angle(deg_to_rad(170)))
	dr.push(1.0, 1.0, Vector2.ZERO, Vector2.from_angle(deg_to_rad(-170)))
	var dmid: Dictionary = dr.sample(0.5)
	_check(absf(absf(rad_to_deg(dmid["dir"].angle())) - 180.0) < 1.0, "направление интерполируется по кратчайшему пути через ±180° (%.1f°)" % rad_to_deg(dmid["dir"].angle()))
	# размер буфера ограничен
	var big := SnapshotBuffer.new(0.1)
	for i in 100:
		big.push(i * 0.1, i * 0.1, Vector2(i, 0), Vector2(1, 0))
	_check(big.count() == SnapshotBuffer.SIZE, "буфер: хранится не больше %d снимков" % SnapshotBuffer.SIZE)
	# сдвиг часов: большой скачок локального времени пересчитывает смещение
	var cl := SnapshotBuffer.new(0.1)
	cl.push(1.0, 1.0, Vector2(0, 0), Vector2(1, 0))
	cl.push(1.1, 1.1, Vector2(1, 0), Vector2(1, 0))
	cl.push(1.2, 51.2, Vector2(2, 0), Vector2(1, 0))  # локальное время скакнуло на 50 с
	_check(cl.sample(51.2 + 0.0)["valid"], "буфер переживает скачок локальных часов")
