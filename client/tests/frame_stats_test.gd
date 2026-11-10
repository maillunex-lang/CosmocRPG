extends SceneTree
## Тест отладочной панели производительности: расчёт процентилей, зависаний, окна 10 с, кольцевого
## буфера, анализа старта, меток загрузки, жестов, раскладки и интеграции в экран «Космос».
## Запуск: godot --headless --path client --script res://tests/frame_stats_test.gd

const FrameStats := preload("res://ui/debug/frame_stats.gd")
const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

var _fake_ms := 1000
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


func _stats_node() -> CanvasLayer:
	UiSettings.override(FrameStats.SETTING_KEY, true)
	var n: CanvasLayer = FrameStats.new()
	n.clock = func() -> int: return _fake_ms
	root.add_child(n)
	return n


func _feed(n: CanvasLayer, count: int, ms: float, t0: float = 10.0) -> void:
	for i in count:
		n.record(ms, t0 + i * ms / 1000.0)


func _run() -> void:
	var fast := _stats_node()
	for i in 1441:
		fast.record(1000.0 / 120.0, 10.0 + i / 120.0)
	var fast_stats: Dictionary = fast.stats()
	_check(fast_stats.n >= 1200 and not fast_stats.window_truncated, "120 FPS: полное десятисекундное окно")
	_check(absf(fast_stats.mean_ms - 1000.0 / 120.0) < 0.01, "среднее время в миллисекундах")
	_check(fast.metric(null) == "N/A", "недоступное значение N/A")
	_check(fast.export_json("user://bad.json", "token", 0) == ERR_INVALID_PARAMETER, "экспорт отвергает произвольные метаданные")
	fast.queue_free()

	# ============ процентили ============
	var arr := PackedFloat32Array()
	for i in range(1, 101):
		arr.append(float(i))
	_check(FrameStats.percentile(arr, 0.50) == 50.0 and FrameStats.percentile(arr, 0.95) == 95.0 and FrameStats.percentile(arr, 0.99) == 99.0, "процентили p50/p95/p99 на ряду 1..100")
	_check(FrameStats.percentile(arr, 1.0) == 100.0 and FrameStats.percentile(arr, 0.0) == 1.0, "процентили на границах")
	_check(FrameStats.percentile(PackedFloat32Array(), 0.5) == 0.0, "процентиль пустого ряда равен нулю")
	_check(FrameStats.percentile(PackedFloat32Array([7.0]), 0.99) == 7.0, "процентиль одного значения")

	# ============ ровная работа ============
	var n := _stats_node()
	_check(n.stats()["n"] == 0 and n.stats()["fps"] == 0.0, "без кадров: нули без деления на ноль")
	_feed(n, 600, 16.6)
	var s: Dictionary = n.stats()
	_check(absf(s["fps"] - 60.24) < 0.1 and absf(s["p50"] - 16.6) < 0.01 and s["hitches25"] == 0 and s["hitches50"] == 0, "ровные 16,6 мс: 60,2 FPS, зависаний нет")

	# ============ зависания ============
	for i in 5:
		n.record(30.0, 20.0 + i * 0.03)
	for i in 2:
		n.record(60.0, 21.0 + i * 0.06)
	s = n.stats()
	_check(s["hitches25"] == 7 and s["hitches50"] == 2 and s["max"] == 60.0, "зависания считаются: >25 мс — 7, >50 мс — 2, макс 60")
	_check(s["p99"] >= 30.0, "p99 отражает редкие длинные кадры (%.1f)" % s["p99"])
	n.record(25.0, 22.0)
	_check(n.stats()["hitches25"] == 7, "ровно 25 мс не считается зависанием (строго больше)")
	n.queue_free()

	# ============ окно 10 секунд ============
	n = _stats_node()
	n.record(200.0, 1.0)                 # давний фриз
	_feed(n, 100, 16.6, 5.0)
	_check(n.stats()["hitches50"] == 1, "фриз внутри окна виден")
	_feed(n, 100, 16.6, 30.0)            # 25 секунд спустя
	_check(n.stats()["hitches50"] == 0 and n.stats()["max"] < 20.0, "старые кадры (старше 10 с) в показатели не входят")
	n.queue_free()

	# ============ кольцевой буфер ============
	n = _stats_node()
	_feed(n, FrameStats.RING * 3, 16.6)
	_check(n._count == FrameStats.RING, "кольцо не растёт: хранится не больше %d кадров" % FrameStats.RING)
	n.record(80.0, 500.0)
	_check(n.stats()["max"] == 80.0 and n.stats()["n"] <= FrameStats.RING, "новые кадры вытесняют старые")
	n.queue_free()

	# ============ анализ старта ============
	n = _stats_node()
	n.record(349.0, 0.15)
	n.record(100.0, 0.66)
	n.record(116.0, 1.31)
	n.record(40.0, 2.0)
	n.record(500.0, 5.0)                 # после первых 3 секунд в старт не входит
	n.record(16.6, 1.0)
	_check(n.startup_hitches == 4 and n.startup_max_ms == 349.0, "старт: 4 зависания, макс 349 мс; кадр на 5-й секунде не учтён")
	for i in 10:
		n.record(30.0 + i, 0.5 + i * 0.01)
	_check(n.startup_list.size() == FrameStats.MAX_STARTUP_LIST, "список заметных кадров старта ограничен (%d)" % n.startup_list.size())
	var top := 0.0
	for e in n.startup_list:
		top = maxf(top, e["ms"])
	_check(top == 349.0, "в списке остаются самые длинные кадры")
	n.queue_free()

	# ============ метки загрузки и текст ============
	n = _stats_node()
	_feed(n, 60, 16.6)
	n.mark("сцена", 412.0)
	n.mark("HUD", 55.0)
	var t: String = n.text()
	_check("FPS 60" in t and "p99" in t and "сцена 412" in t and "HUD 55" in t, "текст панели содержит FPS, процентили и метки загрузки")
	_check(t.split("\n").size() >= 4, "текст разбит на строки (%d)" % t.split("\n").size())
	_check(n._label.text == t, "панель показывает актуальный текст")
	n.queue_free()

	# ============ жесты и состояние ============
	UiSettings.override(FrameStats.SETTING_KEY, null)
	UiSettings.set_bool(FrameStats.SETTING_KEY, true)
	n = FrameStats.new()
	n.clock = func() -> int: return _fake_ms
	root.add_child(n)
	_check(n.visible, "состояние берётся из настроек (включено)")
	var f3 := InputEventKey.new()
	f3.keycode = KEY_F3
	f3.pressed = true
	n._input(f3)
	_check(not n.visible and not n._panel.visible and not UiSettings.get_bool(FrameStats.SETTING_KEY, true), "F3 выключает панель и запоминает выбор")
	n._input(f3)
	_check(n.visible and n._panel.visible and UiSettings.get_bool(FrameStats.SETTING_KEY, false), "F3 включает панель снова")
	for idx in [0, 1]:
		var tt := InputEventScreenTouch.new()
		tt.index = idx
		tt.pressed = true
		n._input(tt)
	_check(n.visible, "одно и два касания панель не переключают")
	# быстрое касание тремя пальцами переключает панель
	n._touch_start.clear()
	n._tap3_armed = false
	var before_vis: bool = n.visible
	for idx in [0, 1, 2]:
		var tt3 := InputEventScreenTouch.new()
		tt3.index = idx
		tt3.pressed = true
		tt3.position = Vector2(100 + idx * 80, 300)
		n._input(tt3)
	for idx in [0, 1, 2]:
		var up3 := InputEventScreenTouch.new()
		up3.index = idx
		up3.pressed = false
		n._input(up3)
	_check(n.visible != before_vis, "быстрое касание тремя пальцами переключает панель")
	var after_vis: bool = n.visible
	# три касания, но первое удерживается давно (например, джойстик): не жест
	var held := InputEventScreenTouch.new()
	held.index = 0
	held.pressed = true
	n._input(held)
	_fake_ms += 400
	for idx in [1, 2]:
		var late := InputEventScreenTouch.new()
		late.index = idx
		late.pressed = true
		n._input(late)
	for idx in [2, 1, 0]:
		var up_l := InputEventScreenTouch.new()
		up_l.index = idx
		up_l.pressed = false
		n._input(up_l)
	_check(n.visible == after_vis, "удерживаемый палец плюс два быстрых (джойстик и щипок) панель не переключают")
	# три пальца удерживаются дольше 600 мс
	for idx in [0, 1, 2]:
		var tl := InputEventScreenTouch.new()
		tl.index = idx
		tl.pressed = true
		n._input(tl)
	_fake_ms += 700
	for idx in [0, 1, 2]:
		var ul := InputEventScreenTouch.new()
		ul.index = idx
		ul.pressed = false
		n._input(ul)
	_check(n.visible == after_vis, "долгое удержание трёх пальцев (больше 600 мс) панель не переключает")
	# три быстрых касания, но с заметным сдвигом (свайп)
	for idx in [0, 1, 2]:
		var tm := InputEventScreenTouch.new()
		tm.index = idx
		tm.pressed = true
		tm.position = Vector2(100, 100)
		n._input(tm)
	var drag := InputEventScreenDrag.new()
	drag.index = 1
	drag.position = Vector2(300, 100)
	n._input(drag)
	for idx in [0, 1, 2]:
		var um := InputEventScreenTouch.new()
		um.index = idx
		um.pressed = false
		n._input(um)
	_check(n.visible == after_vis, "три пальца со сдвигом (свайп) панель не переключают")
	# после жеста счётчики сбрасываются: следующее касание одним пальцем ничего не делает
	var single := InputEventScreenTouch.new()
	single.index = 0
	single.pressed = true
	n._input(single)
	single = InputEventScreenTouch.new()
	single.index = 0
	single.pressed = false
	n._input(single)
	_check(n.visible == after_vis and n._touch_start.is_empty(), "после жеста состояние касаний очищено")
	if n.visible:
		n.toggle()
	var echo := InputEventKey.new()
	echo.keycode = KEY_F3
	echo.pressed = true
	echo.echo = true
	n._input(echo)
	_check(not n.visible, "автоповтор клавиши не переключает панель")
	_check(n._panel.mouse_filter == Control.MOUSE_FILTER_IGNORE and n._label.mouse_filter == Control.MOUSE_FILTER_IGNORE, "панель не перехватывает ввод")
	n.queue_free()
	UiSettings.set_bool(FrameStats.SETTING_KEY, FrameStats.DEFAULT_VISIBLE)

	# ============ значение по умолчанию и список «перед публикацией» ============
	var roadmap := FileAccess.get_file_as_string(ProjectSettings.globalize_path("res://").path_join("../docs/ROADMAP.md"))
	_check(roadmap.contains("Перед публикацией") and roadmap.contains("DEFAULT_VISIBLE"), "в ROADMAP есть пункт «Перед публикацией» про отключение панели FPS")
	UiSettings.override(FrameStats.SETTING_KEY, null)

	# ============ интеграция в экран «Космос» ============
	UiSettings.override("reduce_motion", true)
	for with_open in [false, true]:
		var gw = DemoGateway.new()
		gw.auto_tick = false
		Session.gateway = gw
		Session.opened_msec = Time.get_ticks_msec() if with_open else -1
		if with_open:
			await create_timer(0.26).timeout  # «загрузка» длится не меньше 260 мс
		var holder := SubViewport.new()
		holder.size = Vector2i(1280, 720)
		holder.disable_3d = true
		root.add_child(holder)
		var screen: Control = load("res://scenes/space.tscn").instantiate()
		screen.auto_exit = false
		holder.add_child(screen)
		for i in 5:
			await process_frame
		var st: CanvasLayer = screen.stats
		_check(st != null and st.marks.has("HUD") and st.marks.has("мир+фон"), "экран космоса: замеры HUD и мира с фоном записаны (%s)" % str(st.marks.keys()))
		_check(st.marks.has("сцена") == with_open, "экран космоса: метка «сцена» %s" % ("есть, когда задан момент открытия" if with_open else "отсутствует при прямом запуске"))
		if with_open:
			_check(st.marks["сцена"] >= 250.0 and st.marks["сцена"] < 5000.0, "метка «сцена» разумна (%.0f мс)" % st.marks["сцена"])
			_check(Session.opened_msec == -1, "момент открытия сбрасывается после замера")
		_check(st.stats()["n"] > 0, "кадры реально записываются (%d)" % st.stats()["n"])
		holder.queue_free()
		Session.gateway = null
	# раскладка: панель не налезает на нижнюю панель и колонки HUD
	for size in [Vector2i(1280, 720), Vector2i(1572, 720), Vector2i(1280, 960)]:
		UiSettings.override(FrameStats.SETTING_KEY, true)
		var gw2 = DemoGateway.new()
		gw2.auto_tick = false
		Session.gateway = gw2
		var h2 := SubViewport.new()
		h2.size = size
		h2.disable_3d = true
		root.add_child(h2)
		var sc: Control = load("res://scenes/space.tscn").instantiate()
		sc.auto_exit = false
		h2.add_child(sc)
		sc.stats.mark("сцена", 1234.0)
		sc.stats.mark("HUD", 88.0)
		sc.stats.mark("мир+фон", 40.0)
		for i in 4:
			await process_frame
		var panel: Control = sc.stats._panel
		var rect := panel.get_global_rect()
		var vp := Rect2(Vector2.ZERO, Vector2(size))
		var hud: Control = sc.get_node("%Hud")
		var others := []
		for nm in ["BottomNav", "RightStack", "LeftStack", "DemoBar"]:
			var node := hud.find_child(nm, true, false) as Control
			if node != null and rect.intersects(node.get_global_rect()):
				others.append(nm)
		_check(panel.get_parent() == hud.get_node("%DebugSlot"), "панель FPS %dx%d: размещена в слоте HUD" % [size.x, size.y])
		_check(vp.encloses(rect), "панель FPS %dx%d: внутри экрана %s" % [size.x, size.y, str(rect)])
		_check(others.is_empty(), "панель FPS %dx%d: не налезает на HUD %s" % [size.x, size.y, str(others)])
		h2.queue_free()
		Session.gateway = null
		await process_frame
	UiSettings.override(FrameStats.SETTING_KEY, null)
	UiSettings.override("reduce_motion", null)

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
