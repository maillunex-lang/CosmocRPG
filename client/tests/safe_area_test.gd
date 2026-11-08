extends SceneTree
## Тест безопасной зоны: расчёт отступов, поворот выреза слева/справа, масштаб, граничные случаи.
## Запуск: godot --headless --path client --script res://tests/safe_area_test.gd

const SafeArea := preload("res://ui/safe_area_root.gd")

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


func _run() -> void:
	var win := Vector2i(2000, 916)       # как экран iQOO 15R в альбомной ориентации
	var vp := Vector2(1280.0 * 2000.0 / 916.0 * 720.0 / 1280.0, 720.0)  # логический размер после stretch expand
	var scale_x := vp.x / win.x
	var m: Vector4i

	# без выреза: только минимальный отступ
	m = SafeArea.compute_margins(Rect2i(Vector2i.ZERO, win), win, vp, 16)
	_check(m == Vector4i(16, 16, 16, 16), "нет выреза: минимальные отступы 16")

	# вырез камеры слева (телефон повёрнут одним боком)
	m = SafeArea.compute_margins(Rect2i(120, 0, win.x - 120, win.y), win, vp, 16)
	_check(m.x == int(ceil(120.0 * scale_x)) and m.z == 16, "вырез слева: левый отступ %d, правый 16" % m.x)

	# вырез справа (повернули на 180°)
	m = SafeArea.compute_margins(Rect2i(0, 0, win.x - 120, win.y), win, vp, 16)
	_check(m.z == int(ceil(120.0 * scale_x)) and m.x == 16, "вырез справа: правый отступ %d, левый 16" % m.z)

	# поворот: отступы меняются местами
	var left_notch := SafeArea.compute_margins(Rect2i(120, 0, win.x - 120, win.y), win, vp, 16)
	var right_notch := SafeArea.compute_margins(Rect2i(0, 0, win.x - 120, win.y), win, vp, 16)
	_check(left_notch.x == right_notch.z and left_notch.z == right_notch.x, "при повороте на 180° левый и правый отступы меняются местами")

	# жест-полоса снизу и скругления сверху
	m = SafeArea.compute_margins(Rect2i(0, 40, win.x, win.y - 40 - 60), win, vp, 16)
	_check(m.y > 16 and m.w > 16 and m.x == 16, "верхний и нижний отступы учитываются")

	# масштаб: больше физических px — больше логических отступов не бывает, расчёт линеен
	var m1 := SafeArea.compute_margins(Rect2i(100, 0, win.x - 100, win.y), win, vp, 0)
	var m2 := SafeArea.compute_margins(Rect2i(200, 0, win.x - 200, win.y), win, vp, 0)
	_check(m2.x == m1.x * 2 or absi(m2.x - m1.x * 2) <= 1, "отступ пропорционален размеру выреза")

	# граничные случаи
	_check(SafeArea.compute_margins(Rect2i(), win, vp, 16) == Vector4i(16, 16, 16, 16), "пустая безопасная зона = весь экран")
	_check(SafeArea.compute_margins(Rect2i(0, 0, 10, 10), Vector2i.ZERO, vp, 16) == Vector4i(16, 16, 16, 16), "нулевое окно: минимальные отступы")
	_check(SafeArea.compute_margins(Rect2i(0, 0, 10, 10), win, Vector2.ZERO, 16) == Vector4i(16, 16, 16, 16), "нулевой viewport: минимальные отступы")
	m = SafeArea.compute_margins(Rect2i(-50, -50, win.x + 100, win.y + 100), win, vp, 16)
	_check(m == Vector4i(16, 16, 16, 16), "зона больше окна: отрицательные отступы не возникают")

	# узел: поворот через debug-вставки, сигнал и значения margin
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	holder.disable_3d = true
	root.add_child(holder)
	var node: MarginContainer = SafeArea.new()
	holder.add_child(node)
	await process_frame
	_check(node.get_theme_constant("margin_left") == 16 and node.get_theme_constant("margin_right") == 16, "узел: начальные отступы 16")
	var emitted := []
	node.insets_changed.connect(func(i: Vector4i) -> void: emitted.append(i))
	node.set_debug_insets(100, 0, 0, 0)
	_check(node.get_theme_constant("margin_left") == 100 and node.get_theme_constant("margin_right") == 16, "узел: вырез слева")
	node.set_debug_insets(0, 0, 100, 0)
	_check(node.get_theme_constant("margin_left") == 16 and node.get_theme_constant("margin_right") == 100, "узел: после поворота вырез справа")
	_check(emitted.size() == 2, "узел: сигнал insets_changed при каждом изменении (%d)" % emitted.size())
	node.set_debug_insets(0, 0, 100, 0)
	_check(emitted.size() == 2, "узел: без изменений сигнал не повторяется")
	holder.size = Vector2i(1600, 720)
	await process_frame
	_check(node.size.x == 1600.0, "узел: следует за изменением размера")

	# вызов set_debug_insets до входа в дерево не падает, значения применяются при входе
	var early: MarginContainer = SafeArea.new()
	early.set_debug_insets(70, 0, 0, 0)
	holder.add_child(early)
	await process_frame
	_check(early.get_theme_constant("margin_left") == 70, "узел: вставки, заданные до входа в дерево, применяются в _ready")

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
