extends SceneTree
## Скриншоты экрана «Космос» в характерных состояниях (нужен рендеринг: xvfb).
## Переменные: SHOT_DIR (каталог), INSETS="л,в,п,н" (имитация выреза, px окна),
## SYSTEM=N (сколько раз переключить демо-систему: 0 Кси, 1 Омега, 2 Глубокий космос),
## NOHUD=1 (скрыть HUD и корабли: только фон), ONLY_IDLE=1 (только первый кадр).

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiSettings := preload("res://ui/ui_settings.gd")


func _shot(name: String) -> void:
	await create_timer(0.3).timeout
	await RenderingServer.frame_post_draw
	var path := "%s/%s.png" % [OS.get_environment("SHOT_DIR"), name]
	root.get_texture().get_image().save_png(path)
	print("SHOT_SAVED ", path)


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	UiSettings.override("reduce_motion", true)
	var gw = DemoGateway.new()
	gw.auto_tick = false
	Session.gateway = gw
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	root.add_child(screen)
	var ins := OS.get_environment("INSETS")
	if ins != "":
		var p := ins.split(",")
		screen.get_node("%Hud/SafeArea").set_debug_insets(int(p[0]), int(p[1]), int(p[2]), int(p[3]))
	for i in int(OS.get_environment("SYSTEM") if OS.get_environment("SYSTEM") != "" else "0"):
		gw.demo_next_system()
	if OS.get_environment("NOHUD") == "1":
		screen.get_node("%Hud").visible = false
		screen.get_node("%Ships").visible = false
	for i in 5:
		await process_frame
	await _shot("1_idle")
	if OS.get_environment("ONLY_IDLE") == "1":
		quit()
		return

	# цель выбрана
	var pts: Dictionary = screen.screen_points()
	screen.select_at(pts["pi1"])
	await process_frame
	await _shot("2_target")

	# бой: атака, панели свёрнуты, тревога
	gw.attack("pi1")
	for i in 30:
		gw.tick(0.1)
	gw._ship["shield"] = 40.0
	gw._ship["hull"] = 250.0
	gw.tick(0.1)
	gw.use_skill(1)
	await process_frame
	await _shot("3_combat_alerts")

	# подтверждение атаки нейтрального игрока
	gw.stop_attack()
	gw._objects["pi1"]["pos"] = Vector2(200, 200)
	gw._ship["shield"] = gw._ship["shield_max"]
	gw._ship["hull"] = gw._ship["hull_max"]
	for i in 80:
		gw.tick(0.1)
	pts = screen.screen_points()
	screen.select_at(pts["pl1"])
	await process_frame
	screen.get_node("%Hud")._on_attack_pressed()
	await process_frame
	await _shot("4_confirm_attack")
	quit()
