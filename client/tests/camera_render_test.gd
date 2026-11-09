extends SceneTree
## Проверка РЕАЛЬНОЙ отрисовки камеры (нужен рендеринг: xvfb): обязательные виды из ТЗ (стандартный, максимальные
## приближение и отдаление, минимальный и максимальный наклон, повороты 90/180/270/360°, во время движения,
## в бою). Корабль виден в каждом ракурсе, фон не меняется от камеры и соответствует художественному стандарту.
## Переменная SHOT_DIR: куда сохранять кадры для осмотра. Запуск: tools/test_client.sh

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const ArtStandard := preload("res://ui/space_bg/art_standard.gd")

var _failures := 0
var _checks := 0
var _screen: Control
var _gw
var _rig: Node3D


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _grab() -> Image:
	for i in 3:
		await process_frame
	await RenderingServer.frame_post_draw
	var img := root.get_texture().get_image()
	img.convert(Image.FORMAT_RGB8)
	return img


func _save(img: Image, name: String) -> void:
	var dir := OS.get_environment("SHOT_DIR")
	if dir != "":
		img.save_png("%s/%s.png" % [dir, name])


## Число пикселей вокруг точки, отличающихся от кадра без кораблей (корабль виден независимо от его цвета).
func _ship_px(img: Image, p: Vector2, half: int = 30) -> int:
	var ships: Node3D = _screen.get_node("%Ships")
	ships.visible = false
	var bg := await _grab()
	ships.visible = true
	var n := 0
	for y in range(maxi(0, int(p.y) - half), mini(img.get_height(), int(p.y) + half)):
		for x in range(maxi(0, int(p.x) - half), mini(img.get_width(), int(p.x) + half)):
			var a := img.get_pixel(x, y)
			var b := bg.get_pixel(x, y)
			if absf(a.r - b.r) + absf(a.g - b.g) + absf(a.b - b.b) > 0.15:
				n += 1
	return n


func _mean_diff(a: Image, b: Image) -> float:
	var sa := a.duplicate() as Image
	var sb := b.duplicate() as Image
	sa.resize(160, 90)
	sb.resize(160, 90)
	var t := 0.0
	for y in 90:
		for x in 160:
			var ca := sa.get_pixel(x, y)
			var cb := sb.get_pixel(x, y)
			t += (absf(ca.r - cb.r) + absf(ca.g - cb.g) + absf(ca.b - cb.b)) / 3.0
	return t / (160.0 * 90.0)


func _spawn(preset: String = "high") -> void:
	if _screen != null:
		_screen.queue_free()
		await process_frame
	UiSettings.override("graphics_preset", preset)
	_gw = DemoGateway.new()
	_gw.auto_tick = false
	Session.gateway = _gw
	_screen = load("res://scenes/space.tscn").instantiate()
	_screen.auto_exit = false
	root.add_child(_screen)
	_rig = _screen.get_node("%CameraRig")
	for i in 6:
		await process_frame



## SR-007: подтверждаем ожидаемый графический стек (Mobile renderer поверх Vulkan), без ERROR-guard на уровне скрипта.
func _check_renderer() -> bool:
	var method := RenderingServer.get_current_rendering_method()
	var driver := "vulkan" if RenderingServer.get_rendering_device() != null else "opengl3"
	print("RENDERER %s / %s" % [method, driver])
	return method == "mobile" and driver == "vulkan"

func _run() -> void:
	if not _check_renderer():
		print("FAIL  неожиданный графический стек (ожидался mobile/vulkan)")
		_failures += 1
	UiSettings.override("reduce_motion", true)
	UiSettings.override("debug_overlay2", false)
	await _spawn()

	var views := [
		["01_standard", 0.0, 45.0, 50.0],
		["02_zoom_in_max", 0.0, 45.0, 18.0],
		["03_zoom_out_max", 0.0, 45.0, 100.0],
		["04_tilt_min", 0.0, 25.0, 50.0],
		["05_tilt_max", 0.0, 65.0, 50.0],
		["06_yaw_90", 90.0, 45.0, 50.0],
		["07_yaw_180", 180.0, 45.0, 50.0],
		["08_yaw_270", 270.0, 45.0, 50.0],
		["09_yaw_360", 360.0, 45.0, 50.0],
	]
	var hud: Control = _screen.get_node("%Hud")
	var ships: Node3D = _screen.get_node("%Ships")
	var bg_lums: Array[float] = []
	var bg_imgs: Array[Image] = []
	for v in views:
		_rig.set_view(v[1], v[2], v[3])
		var img := await _grab()
		_save(img, v[0])
		var sp: Vector2 = _screen._camera.unproject_position(_screen.ship_view("me").global_position)
		var cyan := await _ship_px(img, sp)
		_check(cyan >= 150, "вид «%s»: корабль виден на экране в точке (%.0f, %.0f), пикселей корабля %d" % [v[0], sp.x, sp.y, cyan])
		_check(sp.x > 0 and sp.x < 1280 and sp.y > 0 and sp.y < 720, "вид «%s»: корабль внутри кадра" % v[0])
		# только фон
		hud.visible = false
		ships.visible = false
		var bg_img := await _grab()
		hud.visible = true
		ships.visible = true
		var m := ArtStandard.measure(bg_img)
		var viol := ArtStandard.violations(m)
		_check(viol.is_empty(), "вид «%s»: фон соответствует художественному стандарту (яркость %.3f, центр %.3f) %s" % [v[0], m["mean_lum"], m["center_lum"], str(viol)])
		bg_lums.append(m["mean_lum"])
		bg_imgs.append(bg_img)
	var lmin := 1.0
	var lmax := 0.0
	for l in bg_lums:
		lmin = minf(lmin, l)
		lmax = maxf(lmax, l)
	_check(lmax - lmin < 0.004, "яркость фона одинакова во всех видах камеры (разброс %.4f)" % (lmax - lmin))
	var max_diff := 0.0
	for i in range(1, bg_imgs.size()):
		max_diff = maxf(max_diff, _mean_diff(bg_imgs[0], bg_imgs[i]))
	_check(max_diff < 0.01, "фон не вращается, не растягивается и не смещается от вращения, наклона и зума (разница между видами %.4f)" % max_diff)

	# камера во время движения корабля
	_rig.set_view(0, 45, 50)
	_gw.move_input(Vector2(1, 0.4))
	for i in 150:
		_gw.tick(1.0 / 60.0)
		await process_frame
	var moving := await _grab()
	_save(moving, "10_moving")
	var sp2: Vector2 = _screen._camera.unproject_position(_screen.ship_view("me").global_position)
	_check(await _ship_px(moving, sp2) >= 150 and sp2.x > 200 and sp2.x < 1100, "во время движения корабль виден и остаётся в свободной области экрана (x = %.0f)" % sp2.x)
	_gw.move_input(Vector2.ZERO)

	# камера в бою
	_rig.set_view(60, 50, 40)
	var pts: Dictionary = _screen.screen_points()
	_screen.select_at(pts["pi1"])
	_gw.attack("pi1")
	for i in 40:
		_gw.tick(0.1)
	_gw._ship["shield"] = 60.0
	_gw.tick(0.1)
	var combat := await _grab()
	_save(combat, "11_combat")
	var sp3: Vector2 = _screen._camera.unproject_position(_screen.ship_view("me").global_position)
	_check(await _ship_px(combat, sp3) >= 150, "в бою с повёрнутой камерой корабль виден")
	_check(_screen.ship_view("pi1").is_selected(), "в бою выбранная цель подсвечена при повёрнутой камере")

	# пресеты графики с повёрнутой камерой
	for preset in ["low", "medium", "high"]:
		await _spawn(preset)
		_rig.set_view(135, 40, 35)
		var pimg := await _grab()
		_save(pimg, "12_preset_" + preset)
		var sp4: Vector2 = _screen._camera.unproject_position(_screen.ship_view("me").global_position)
		_check(await _ship_px(pimg, sp4) >= 150, "пресет «%s»: корабль виден при повёрнутой камере" % preset)
		var hud2: Control = _screen.get_node("%Hud")
		var ships2: Node3D = _screen.get_node("%Ships")
		hud2.visible = false
		ships2.visible = false
		var pm := ArtStandard.measure(await _grab())
		_check(ArtStandard.violations(pm).is_empty(), "пресет «%s» с камерой: фон по стандарту (%.3f)" % [preset, pm["mean_lum"]])

	# смена системы при повёрнутой камере
	await _spawn()
	_rig.set_view(210, 55, 30)
	var shots := []
	for k in 3:
		_screen.get_node("%Hud").get_node("%SystemButton").pressed.emit()
		var s_img := await _grab()
		_save(s_img, "13_system_%d" % k)
		shots.append(s_img)
		var sp5: Vector2 = _screen._camera.unproject_position(_screen.ship_view("me").global_position)
		_check(await _ship_px(s_img, sp5) >= 150, "система %d: корабль виден при повёрнутой камере" % k)
	_check(_mean_diff(shots[0], shots[1]) > 0.01, "смена системы реально меняет фон при повёрнутой камере")

	# режимы кнопки «Вид»: сверху и сзади (в движении, камера разворачивается за курсом)
	await _spawn()
	for mode in [1, 2]:
		_screen.get_node("%Hud").camera_mode_requested.emit()
		_gw.move_input(Vector2(-0.7, 0.7))
		for i in 60:
			_gw.tick(0.1)   # 6 с игрового времени за 60 кадров: на медленных машинах (CI) тест не растягивается
			await process_frame
		var m_img := await _grab()
		_save(m_img, "14_mode_%d" % mode)
		var sp6: Vector2 = _screen._camera.unproject_position(_screen.ship_view("me").global_position)
		_check(await _ship_px(m_img, sp6) >= 150 and sp6.x > 200 and sp6.x < 1100 and sp6.y > 100 and sp6.y < 620, "режим камеры %d: корабль виден в свободной области экрана (%.0f, %.0f)" % [mode, sp6.x, sp6.y])
	_gw.move_input(Vector2.ZERO)

	UiSettings.override("reduce_motion", null)
	UiSettings.override("graphics_preset", null)
	UiSettings.override("debug_overlay2", null)
	Session.gateway = null
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
