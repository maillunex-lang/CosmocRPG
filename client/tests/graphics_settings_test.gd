extends SceneTree
const Settings := preload("res://ui/ui_settings.gd")
const Session := preload("res://ui/session.gd")
const Demo := preload("res://ui/gateway/demo_gateway.gd")
var failures := 0
var checks := 0
func _initialize() -> void:
	run.call_deferred()
func check(ok: bool, title: String) -> void:
	checks += 1
	if not ok:
		failures += 1
	print("PASS  " if ok else "FAIL  ", title)
func run() -> void:
	var old_cap := Engine.max_fps
	var gateway := Demo.new()
	gateway.auto_tick = false
	gateway._objects.clear()
	for i in 60:
		gateway._objects[str(i)] = {"id": str(i), "kind": "player", "name": "Fixture", "stance": "neutral", "ship": "FTR-001", "skin": "a", "pos": Vector2(i % 10, i / 10), "dir": Vector2.RIGHT}
	Session.gateway = gateway
	Settings.override("ship_limit", 5.0)
	Settings.override("fps_limit", 30.0)
	Settings.override("render_scale", 0.5)
	Settings.override("graphics_preset", "low")
	var screen = load("res://scenes/space.tscn").instantiate()
	root.add_child(screen)
	await process_frame
	check(Engine.max_fps == 30, "30 FPS применяется")
	check(screen.get_node("%Ships").get_viewport().scaling_3d_scale == 0.5, "масштаб применяется к WorldViewport")
	check(root.scaling_3d_scale == 1.0, "UI viewport сохраняет разрешение")
	for limit in range(5, 51, 5):
		Settings.override("ship_limit", float(limit))
		screen._apply_graphics()
		var full := 0
		for view in screen._ships.values():
			if not view.me and view.visual != null and view.visual.visible:
				full += 1
		check(full == limit, "лимит %d соблюдается" % limit)
	check(gateway.snapshot().objects.size() == 60 and screen._ships.size() == 61, "snapshot и все сущности сохранены")
	check(screen.ship_view("me").visual.visible, "свой корабль вне лимита")
	Settings.override("ship_limit", 5.0)
	Settings.override("fps_limit", 60.0)
	screen._apply_graphics()
	var hidden_id := ""
	for id in screen._ships:
		var view = screen._ships[id]
		if id != "me" and view.visual != null and not view.visual.visible:
			hidden_id = id
			break
	var hidden = screen.ship_view(hidden_id)
	hidden.play("Fire", Vector3.ONE)
	check(hidden.visual._bolts.is_empty() and hidden.visual.active_effect_count() == 0, "скрытая модель не запускает декоративный выстрел")
	hidden.play("Death")
	check(not hidden.visual.alive and hidden.visual.active_effect_count() == 0, "смерть обрабатывается у скрытой модели")
	hidden.play("Respawn")
	check(hidden.visual.alive, "respawn обрабатывается у скрытой модели")
	var visual_id: int = screen.ship_view(hidden_id).visual.get_instance_id()
	gateway.set_target(hidden_id)
	check(screen.ship_view(hidden_id).visual.visible, "скрытая выбранная цель получает 3D")
	check(screen.ship_view(hidden_id).visual.get_instance_id() == visual_id, "переключение не создаёт GLB")
	check(Engine.max_fps == 60, "60 FPS применяется")
	var settings_ui = screen.open_settings()
	check(settings_ui.find_child("GraphicsSettings", true, false) != null, "раздел графики встроен в настройки")
	check(settings_ui.find_child("ShipLimit", true, false).step == 5, "шаг лимита 5")
	Settings.override("ship_limit", -100.0)
	check(Settings.ship_limit() == 5, "некорректный нижний лимит зажат")
	Settings.override("ship_limit", 500.0)
	check(Settings.ship_limit() == 50, "некорректный верхний лимит зажат")
	screen.queue_free()
	await process_frame
	Session.gateway = null
	for key in ["ship_limit", "fps_limit", "render_scale", "graphics_preset"]:
		Settings.override(key, null)
	Engine.max_fps = old_cap
	print("Итого: проверок %d, провалов %d" % [checks, failures])
	quit(1 if failures else 0)
