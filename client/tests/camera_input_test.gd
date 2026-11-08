extends SceneTree
## Интеграционный тест камеры на экране «Космос» с НАСТОЯЩЕЙ маршрутизацией ввода Godot (push_input):
## свайп, щипок, тап, приоритет HUD и джойстика, мультитач, диалог и потеря фокуса, мышь, фон и параллакс,
## смена систем, пресеты графики, интерполяция движения, изменение размера окна.
## Запуск: godot --headless --path client --script res://tests/camera_input_test.gd

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const Library := preload("res://ui/space_bg/background_library.gd")

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


func _make(size: Vector2i = Vector2i(1280, 720)) -> Dictionary:
	var gw = DemoGateway.new()
	gw.auto_tick = false
	Session.gateway = gw
	var holder := SubViewport.new()
	holder.size = size
	holder.disable_3d = true
	root.add_child(holder)
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	holder.add_child(screen)
	for i in 4:
		await process_frame
	return {"holder": holder, "screen": screen, "gw": gw, "hud": screen.get_node("%Hud"), "rig": screen.get_node("%CameraRig")}


func _free(c: Dictionary) -> void:
	c["holder"].queue_free()
	Session.gateway = null


func _touch(c: Dictionary, idx: int, pressed: bool, pos: Vector2) -> void:
	var e := InputEventScreenTouch.new()
	e.index = idx
	e.pressed = pressed
	e.position = pos
	c["holder"].push_input(e, true)


func _drag(c: Dictionary, idx: int, pos: Vector2) -> void:
	var e := InputEventScreenDrag.new()
	e.index = idx
	e.position = pos
	c["holder"].push_input(e, true)


func _mouse(c: Dictionary, button: int, pressed: bool, pos: Vector2, device: int = 0) -> void:
	var e := InputEventMouseButton.new()
	e.button_index = button
	e.pressed = pressed
	e.position = pos
	e.global_position = pos
	e.device = device
	e.button_mask = (MOUSE_BUTTON_MASK_LEFT if button == MOUSE_BUTTON_LEFT else MOUSE_BUTTON_MASK_RIGHT) if pressed else 0
	c["holder"].push_input(e, true)


func _motion(c: Dictionary, mask: int, pos: Vector2, rel: Vector2) -> void:
	var e := InputEventMouseMotion.new()
	e.position = pos
	e.global_position = pos
	e.relative = rel
	e.button_mask = mask
	c["holder"].push_input(e, true)


func _rect(c: Dictionary, path: String) -> Rect2:
	return (c["hud"].get_node(path) as Control).get_global_rect()


func _free_point(c: Dictionary) -> Vector2:
	var center := _rect(c, "%DemoBar").get_center()
	var zone_end: float = c["hud"].get_node("%Joystick").get_global_rect().end.x
	return Vector2(maxf(center.x, zone_end + 100.0), 300.0)  # свободная область правее зоны джойстика


func _state(c: Dictionary) -> Array:
	var r: Node3D = c["rig"]
	return [r.yaw_target_rad(), r.tilt_target_deg(), r.zoom_target()]


func _bg_offsets(c: Dictionary) -> Array:
	var out: Array = []
	var bg: Control = c["screen"].get_node("%SpaceBackground")
	for i in bg.layer_count():
		var info: Dictionary = bg.layer_info(i)
		out.append([info["uv_offset"], info["uv_extent"], info["brightness"], info["intensity"]])
	return out


func _run() -> void:
	UiSettings.override("reduce_motion", true)
	UiSettings.override("graphics_preset", "high")
	UiSettings.override("debug_overlay", true)

	var c = await _make()
	var rig: Node3D = c["rig"]
	var gw = c["gw"]
	var screen: Control = c["screen"]
	var free := _free_point(c)

	# ============ свободная область: свайп вращает и наклоняет ============
	var s0 := _state(c)
	_touch(c, 0, true, free)
	_drag(c, 0, free + Vector2(5, 0))
	_check(_state(c) == s0, "свайп меньше порога камеру не двигает")
	_drag(c, 0, free + Vector2(30, 0))
	_check(_state(c) == s0, "переход через порог не даёт скачка")
	_drag(c, 0, free + Vector2(130, 0))
	var s1 := _state(c)
	_check(absf(rad_to_deg(s1[0] - s0[0]) - 25.0) < 0.01, "горизонтальный свайп на 100 px поворачивает на 25° (%.2f°)" % rad_to_deg(s1[0] - s0[0]))
	_drag(c, 0, free + Vector2(130, 40))
	var s2 := _state(c)
	_check(absf((s0[1] - s2[1]) - 7.2) < 0.01 and absf(s2[0] - s1[0]) < 1e-9, "вертикальный свайп на 40 px меняет наклон на 7,2° (%.2f)" % (s0[1] - s2[1]))
	_touch(c, 0, false, free + Vector2(130, 40))
	_check(gw.snapshot()["target_id"] == "", "после свайпа цель не выбрана (тап не сработал)")
	_check(screen.gestures.state == screen.gestures.State.IDLE, "после свайпа жест завершён")

	# ============ тап по объекту выбирает цель ============
	var pts: Dictionary = screen.screen_points()
	var pirate_pt: Vector2 = pts["pi1"]
	var before := _state(c)
	_touch(c, 0, true, pirate_pt)
	_touch(c, 0, false, pirate_pt)
	_check(gw.snapshot()["target_id"] == "pi1" and _state(c) == before, "тап по объекту выбирает цель и не двигает камеру")
	_touch(c, 0, true, free + Vector2(0, 40))
	_touch(c, 0, false, free + Vector2(0, 40))
	_check(gw.snapshot()["target_id"] == "", "тап по пустому месту снимает цель")

	# ============ приоритет HUD ============
	var card := _rect(c, "SafeArea/Columns/LeftStack/ProfileCard").get_center()
	var s_before := _state(c)
	_touch(c, 0, true, card)
	_drag(c, 0, card + Vector2(40, 0))
	_drag(c, 0, card + Vector2(200, 60))
	_touch(c, 0, false, card + Vector2(200, 60))
	_check(_state(c) == s_before, "свайп, начатый на панели HUD, камеру не вращает")
	_check(gw.snapshot()["target_id"] == "", "касание панели HUD не выбирает цель")
	var reset_btn := _rect(c, "%ResetButton").get_center()
	_mouse(c, MOUSE_BUTTON_LEFT, true, reset_btn)
	_check(screen.gestures.touch_count() == 0, "нажатие на кнопку HUD камера не получает")
	_mouse(c, MOUSE_BUTTON_LEFT, false, reset_btn)
	rig.set_mode(0)   # настоящее нажатие переключило режим камеры: возвращаем свободный

	# ============ джойстик ============
	var joy: Control = c["hud"].get_node("%Joystick")
	var jc := Vector2(joy.get_global_rect().size.x * 0.3, 560.0)   # свободное место в зоне джойстика
	var s_j := _state(c)
	_touch(c, 0, true, jc)
	_drag(c, 0, jc + Vector2(30, 0))
	_drag(c, 0, jc + Vector2(30, -60))
	_check(joy.vector.length() > 0.5 and _state(c) == s_j, "джойстик управляет кораблём и не вращает камеру")
	_check(screen.gestures.touch_count() == 0, "касание джойстика не попадает в распознавание жестов камеры")
	# палец джойстика закреплён за ним, второй палец вращает камеру независимо
	var jv: Vector2 = joy.vector
	_touch(c, 1, true, free)
	_drag(c, 1, free + Vector2(30, 0))
	_drag(c, 1, free + Vector2(90, 0))
	_check(_state(c)[0] > s_j[0] and joy.vector == jv and joy.is_active(), "джойстик и свайп работают одновременно, не мешая друг другу")
	_touch(c, 1, false, free + Vector2(90, 0))
	_check(joy.is_active() and joy.vector == jv, "отпускание пальца камеры джойстик не сбрасывает")
	_touch(c, 0, false, jc)
	_check(not joy.is_active() and joy.vector == Vector2.ZERO, "джойстик отпускается своим пальцем")

	# ============ щипок ============
	rig.set_view(0, 45, 50)
	var z0: float = rig.zoom_target()
	var p1 := free + Vector2(-60, 0)
	var p2 := free + Vector2(60, 0)
	_touch(c, 1, true, p1)
	_touch(c, 2, true, p2)
	_drag(c, 2, free + Vector2(120, 0))
	_check(rig.zoom_target() < z0, "щипок на раздвигание приближает (%.1f → %.1f)" % [z0, rig.zoom_target()])
	_check(absf(rig.zoom_target() - z0 / (180.0 / 120.0)) < 0.5, "коэффициент щипка соответствует расстоянию между пальцами: 120 → 180 px даёт 1,5× (%.2f)" % rig.zoom_target())
	var yaw_p: float = rig.yaw_target_rad()
	_drag(c, 1, p1 + Vector2(0, -10))
	_check(rig.yaw_target_rad() == yaw_p, "во время щипка камера не вращается")
	# палец уходит на кнопку HUD: кнопка не срабатывает, так как нажатие начиналось не на ней
	var shield_btn := _rect(c, "%SkillsGrid").get_center()
	var energy0: float = gw._ship["energy"]
	_drag(c, 2, shield_btn)
	_touch(c, 2, false, shield_btn)
	_touch(c, 1, false, p1)
	_check(gw._ship["energy"] >= energy0 and gw.snapshot()["skills"][1]["cooldown_left"] == 0.0, "щипок с отпусканием над кнопкой способности её не активирует")
	# щипок вместе с удерживаемым джойстиком: панель FPS не переключается
	var vis := bool(screen.stats.visible)
	rig.set_view(0, 45, 50)  # вид сброшен: приближение ещё возможно
	_touch(c, 0, true, jc + Vector2(50, 0))
	_touch(c, 1, true, p1)
	_touch(c, 2, true, p2)
	var z1: float = rig.zoom_target()
	_drag(c, 2, free + Vector2(160, 0))
	await create_timer(0.7).timeout
	_touch(c, 2, false, free + Vector2(160, 0))
	_touch(c, 1, false, p1)
	_touch(c, 0, false, jc)
	_check(rig.zoom_target() < z1 and bool(screen.stats.visible) == vis, "щипок вместе с джойстиком работает и не переключает панель FPS (масштаб %.1f → %.1f, панель %s → %s, состояние жеста %d, касаний %d)" % [z1, rig.zoom_target(), str(vis), str(bool(screen.stats.visible)), screen.gestures.state, screen.gestures.touch_count()])
	# отдаление до границы
	_touch(c, 1, true, free + Vector2(-200, 0))
	_touch(c, 2, true, free + Vector2(200, 0))
	for i in 20:
		_drag(c, 2, free + Vector2(200 - i * 9, 0))
	_touch(c, 2, false, free)
	_touch(c, 1, false, free)
	_check(rig.zoom_target() >= 18.0 and rig.zoom_target() <= 100.0, "масштаб жестами не выходит за границы (%.1f)" % rig.zoom_target())

	# ============ диалог и потеря фокуса прерывают жест ============
	rig.set_view(0, 45, 50)
	_touch(c, 0, true, free)
	_drag(c, 0, free + Vector2(40, 0))
	_drag(c, 0, free + Vector2(80, 0))
	_check(screen.gestures.state == screen.gestures.State.ORBIT, "жест вращения идёт")
	c["hud"].get_node("%Confirm").ask("Тест", "Окно открылось во время жеста", "ОК")
	_check(screen.gestures.state == screen.gestures.State.IDLE, "открытие окна подтверждения завершает жест камеры")
	var st_dialog := _state(c)
	_drag(c, 0, free + Vector2(300, 0))
	_touch(c, 0, false, free + Vector2(300, 0))
	_check(_state(c) == st_dialog, "после открытия окна движение того же пальца камеру не вращает")
	c["hud"].get_node("%Confirm")._close(false)
	await process_frame
	_touch(c, 0, true, free)
	_drag(c, 0, free + Vector2(40, 0))
	_drag(c, 0, free + Vector2(80, 0))
	_check(screen.gestures.state == screen.gestures.State.ORBIT, "после закрытия окна жесты работают снова")
	screen._notification(Node.NOTIFICATION_APPLICATION_FOCUS_OUT)
	_check(screen.gestures.state == screen.gestures.State.IDLE, "потеря фокуса приложения завершает жест")
	_touch(c, 0, false, free + Vector2(80, 0))

	# ============ мышь (проверка на компьютере) ============
	rig.set_view(0, 45, 50)
	var m0 := _state(c)
	_mouse(c, MOUSE_BUTTON_RIGHT, true, free)
	_motion(c, MOUSE_BUTTON_MASK_RIGHT, free + Vector2(40, 0), Vector2(40, 0))
	_motion(c, MOUSE_BUTTON_MASK_RIGHT, free + Vector2(80, 10), Vector2(40, 10))
	_mouse(c, MOUSE_BUTTON_RIGHT, false, free + Vector2(80, 10))
	_check(_state(c)[0] > m0[0], "мышь: правая кнопка вращает камеру")
	_check(gw.snapshot()["target_id"] == "", "мышь: правая кнопка цель не выбирает")
	var zz: float = rig.zoom_target()
	_mouse(c, MOUSE_BUTTON_WHEEL_UP, true, free)
	_check(rig.zoom_target() < zz, "мышь: колесо вверх приближает")
	zz = rig.zoom_target()
	_mouse(c, MOUSE_BUTTON_WHEEL_DOWN, true, free)
	_check(rig.zoom_target() > zz, "мышь: колесо вниз отдаляет")
	var me := _state(c)
	_mouse(c, MOUSE_BUTTON_RIGHT, true, free, InputEvent.DEVICE_ID_EMULATION)
	_motion(c, MOUSE_BUTTON_MASK_RIGHT, free + Vector2(100, 0), Vector2(100, 0))
	_mouse(c, MOUSE_BUTTON_RIGHT, false, free, InputEvent.DEVICE_ID_EMULATION)
	_check(_state(c) == me, "мышь: события, эмулированные из касаний, игнорируются (нет двойной обработки)")
	pts = screen.screen_points()
	_mouse(c, MOUSE_BUTTON_LEFT, true, pts["pi1"])
	_mouse(c, MOUSE_BUTTON_LEFT, false, pts["pi1"])
	_check(gw.snapshot()["target_id"] == "pi1", "мышь: щелчок левой кнопкой по объекту выбирает цель")
	gw.set_target("")
	# отпускание над панелью HUD не оставляет жест «залипшим»
	_mouse(c, MOUSE_BUTTON_LEFT, true, free)
	_motion(c, MOUSE_BUTTON_MASK_LEFT, free + Vector2(40, 0), Vector2(40, 0))
	_motion(c, MOUSE_BUTTON_MASK_LEFT, card, Vector2(-100, 0))
	_mouse(c, MOUSE_BUTTON_LEFT, false, card)
	_check(screen.gestures.touch_count() == 0 and screen.gestures.state == screen.gestures.State.IDLE, "мышь: отпускание над панелью HUD завершает жест (нет «залипания»)")

	# ============ кнопка «Вид» ============
	rig.set_view(0, 45, 50)
	rig.orbit(800.0, 100.0)
	rig.zoom_by(2.5)
	c["hud"].camera_mode_requested.emit()
	c["hud"].camera_mode_requested.emit()
	c["hud"].camera_mode_requested.emit()   # круг: сверху, сзади, снова свободный (со сбросом)
	_check(rig.tilt_target_deg() == 45.0 and rig.zoom_target() == 50.0, "кнопка «Вид»: целевые наклон и масштаб стандартные")
	var gtarget_before: String = gw.snapshot()["target_id"]
	var ship_before: Vector2 = gw.snapshot()["ship"]["pos"]
	for i in 200:
		await process_frame
	_check(absf(rig.tilt_deg() - 45.0) < 0.1 and absf(rig.zoom_size() - 50.0) < 0.2, "сброс доводит камеру до стандартного вида")
	_check(gw.snapshot()["ship"]["pos"] == ship_before and gw.snapshot()["target_id"] == gtarget_before, "камера не меняет положение корабля и игровое состояние")
	# настоящее нажатие на кнопку мышью
	rig.orbit(400.0, 0.0)
	rig.zoom_by(2.0)
	_mouse(c, MOUSE_BUTTON_LEFT, true, reset_btn)
	_mouse(c, MOUSE_BUTTON_LEFT, false, reset_btn)
	_check(rig.mode() == 1 and rig.tilt_target_deg() == 0.0 and rig.zoom_target() == 32.0, "настоящее нажатие на кнопку «Вид» включает вид сверху")
	_check(c["hud"].get_node("%ResetButton").text == "Вид: Сверху", "подпись кнопки показывает текущий режим")
	rig.set_mode(0)
	_free(c)

	# ============ фон и параллакс ============
	c = await _make()
	rig = c["rig"]
	gw = c["gw"]
	screen = c["screen"]
	var bg0 := _bg_offsets(c)
	for yaw in [90, 180, 270, 360]:
		rig.set_view(yaw, 45, 50)
		for i in 3:
			await process_frame
	_check(_same_bg(bg0, _bg_offsets(c), 1e-5), "вращение камеры на 90°, 180°, 270°, 360°: смещение, масштаб и яркость фона не меняются")
	for tilt in [25.0, 65.0]:
		rig.set_view(0, tilt, 50)
		await process_frame
	for zoom in [18.0, 100.0]:
		rig.set_view(0, 45, zoom)
		await process_frame
	_check(_same_bg(bg0, _bg_offsets(c), 1e-5), "наклон и масштаб камеры: фон не растягивается и не меняет яркость")
	# движение корабля двигает фон; края не показываются при любом вращении
	rig.set_view(0, 45, 50)
	gw.move_input(Vector2(1, 0))
	for i in 240:
		gw.tick(1.0 / 60.0)
		await process_frame
	var bg_moved := _bg_offsets(c)
	var moved := false
	for i in bg_moved.size():
		if not (bg_moved[i][0] as Vector2).is_equal_approx(bg0[i][0]):
			moved = true
	_check(moved, "перемещение корабля сдвигает слои фона (параллакс работает)")
	var bg: Control = screen.get_node("%SpaceBackground")
	var edges_ok := true
	for yaw in [0, 60, 120, 180, 240, 300]:
		rig.set_view(yaw, 45 + (yaw % 40) - 15, 30 + yaw / 6.0)
		gw.move_input(Vector2.from_angle(deg_to_rad(yaw)))
		for i in 90:
			gw.tick(1.0 / 60.0)
			await process_frame
		for li in bg.layer_count():
			var info: Dictionary = bg.layer_info(li)
			if info["mirror"]:
				continue
			var o: Vector2 = info["uv_offset"]
			var e: Vector2 = info["uv_extent"]
			if o.x < -1e-4 or o.y < -1e-4 or o.x + e.x > 1.0 + 1e-4 or o.y + e.y > 1.0 + 1e-4:
				edges_ok = false
	_check(edges_ok, "после длительного перелёта с поворотами края текстур фона не показываются")
	gw.move_input(Vector2.ZERO)

	# ============ смена звёздных систем сохраняет вид камеры ============
	rig.set_view(135, 55, 30)
	var sys_state := [rig._yaw, rig.tilt_deg(), rig.zoom_size()]
	var ids := []
	for k in 3:
		c["hud"].get_node("%SystemButton").pressed.emit()
		await process_frame
		await process_frame
		ids.append(bg.profile.id)
	_check(ids == ["omega", "deep", "xi"], "кнопка «Система» переключает профили по кругу (%s)" % str(ids))
	_check(absf(rig._yaw - sys_state[0]) < 1e-5 and absf(rig.tilt_deg() - sys_state[1]) < 1e-3 and absf(rig.zoom_size() - sys_state[2]) < 1e-3, "переключение системы не меняет вид камеры")
	_check(bg.layer_count() == 3 and Library.profile("xi").layers.size() == 3, "профили фона сохранены: у системы Кси три слоя")
	_free(c)

	# ============ пресеты графики ============
	for preset in ["low", "medium", "high"]:
		UiSettings.override("graphics_preset", preset)
		c = await _make()
		rig = c["rig"]
		screen = c["screen"]
		bg = screen.get_node("%SpaceBackground")
		var enabled := 0
		for li in bg.layer_count():
			if bg.layer_info(li)["enabled"]:
				enabled += 1
		_check(enabled == (2 if preset == "low" else 3), "пресет «%s»: активных слоёв фона %d" % [preset, enabled])
		var free2 := _free_point(c)
		_touch(c, 0, true, free2)
		_drag(c, 0, free2 + Vector2(40, 0))
		_drag(c, 0, free2 + Vector2(140, -50))
		_touch(c, 0, false, free2 + Vector2(140, -50))
		rig.zoom_by(1.5)
		for i in 30:
			await process_frame
		_check(rig.tilt_target_deg() >= 25.0 and rig.tilt_target_deg() <= 65.0 and rig.zoom_target() < 50.0 and screen.screen_points().size() == 4, "пресет «%s»: камера и выбор объектов работают" % preset)
		_free(c)
	UiSettings.override("graphics_preset", "high")

	# ============ интерполяция движения (устранение подёргивания 10 Гц) ============
	c = await _make()
	rig = c["rig"]
	gw = c["gw"]
	screen = c["screen"]
	screen.set_process(false)
	var clock := [100.0]
	screen.time_source = func() -> float: return clock[0]
	gw.move_input(Vector2(1, 0))
	var xs: Array[float] = []
	var cams: Array[float] = []
	var proj: Array[float] = []
	for frame in 360:
		clock[0] += 1.0 / 60.0
		if frame % 6 == 0:
			gw.tick(0.1)  # шаг симуляции раз в 0,1 с
		screen._process(1.0 / 60.0)
		xs.append((screen.ship_view("me") as Node3D).world_pos.x)
		cams.append(rig.position.x)
		proj.append((screen._camera as Camera3D).unproject_position((screen.ship_view("me") as Node3D).global_position).x)
	var steps := _steps(xs, 90)
	var cam_steps := _steps(cams, 90)
	var proj_steps := _steps(proj, 90)
	_check(_variation(steps) < 0.12, "корабль движется ровно: разброс шага %.0f%% (без интерполяции был бы рывками)" % (100.0 * _variation(steps)))
	_check(_variation(cam_steps) < 0.12, "камера следует ровно: разброс шага %.0f%%" % (100.0 * _variation(cam_steps)))
	_check(_max_dev(proj, 90) < 1.5, "корабль на экране стоит на месте при равномерном движении (отклонение %.2f px)" % _max_dev(proj, 90))
	_free(c)

	# ============ изменение размера окна ============
	c = await _make()
	rig = c["rig"]
	screen = c["screen"]
	var ok_resize := true
	for sz in [Vector2i(1280, 720), Vector2i(1572, 720), Vector2i(1920, 720), Vector2i(1280, 960), Vector2i(1024, 768)]:
		c["holder"].size = sz
		for i in 4:
			await process_frame
		var sp: Vector2 = screen._camera.unproject_position(screen.ship_view("me").global_position)
		if sp.x < 0 or sp.x > sz.x or sp.y < 0 or sp.y > sz.y or absf(rig.viewport_height - sz.y) > 1.0:
			ok_resize = false
	_check(ok_resize, "изменение размера окна: корабль в кадре, высота экрана для параллакса обновляется")
	_free(c)

	UiSettings.override("reduce_motion", null)
	UiSettings.override("graphics_preset", null)
	UiSettings.override("debug_overlay", null)
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)


func _same_bg(a: Array, b: Array, eps: float) -> bool:
	if a.size() != b.size():
		return false
	for i in a.size():
		if not (a[i][0] as Vector2).is_equal_approx(b[i][0]) and (a[i][0] as Vector2).distance_to(b[i][0]) > eps:
			return false
		if (a[i][1] as Vector2).distance_to(b[i][1]) > eps:
			return false
		if absf(a[i][2] - b[i][2]) > eps or absf(a[i][3] - b[i][3]) > eps:
			return false
	return true


func _steps(arr: Array[float], skip: int) -> Array[float]:
	var out: Array[float] = []
	for i in range(skip + 1, arr.size()):
		out.append(arr[i] - arr[i - 1])
	return out


func _variation(steps: Array[float]) -> float:
	var mean := 0.0
	for v in steps:
		mean += v
	mean /= steps.size()
	var dev := 0.0
	for v in steps:
		dev = maxf(dev, absf(v - mean))
	return dev / maxf(absf(mean), 1e-9)


func _max_dev(arr: Array[float], skip: int) -> float:
	var base := arr[skip]
	var d := 0.0
	for i in range(skip, arr.size()):
		d = maxf(d, absf(arr[i] - base))
	return d
