extends SceneTree
## Тест раскладки экрана подключения в горизонтальных разрешениях (и с вырезом камеры):
## ничего не выходит за экран и за безопасную зону, никакой элемент не сжат меньше минимума,
## кнопки не меньше 48 px. Запуск: godot --headless --path client --script res://tests/layout_test.gd

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

# ЛОГИЧЕСКИЕ размеры области интерфейса (после stretch=canvas_items, aspect=expand):
# высота никогда не меньше 720, а на более широких экранах растёт ширина, на более узких (4:3) высота.
# 16:9, 20:9 (2400x1080), IQOO 15R (2000x916), ультраширокий 24:9, 3:2, 4:3.
const SIZES := [Vector2i(1280, 720), Vector2i(1600, 720), Vector2i(1572, 720), Vector2i(1920, 720),
	Vector2i(1280, 853), Vector2i(1280, 960)]

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


func _controls(node: Node, out: Array) -> void:
	for c in node.get_children():
		if c is Control and c.visible:
			out.append(c)
		_controls(c, out)


func _verify(size: Vector2i, insets: Vector4i, label: String) -> void:
	var holder := SubViewport.new()
	holder.size = size
	holder.disable_3d = true
	holder.transparent_bg = false
	root.add_child(holder)
	var screen: Control = load("res://scenes/connect.tscn").instantiate()
	holder.add_child(screen)
	await process_frame
	if insets != Vector4i.ZERO:
		screen.get_node("%SafeArea").set_debug_insets(insets.x, insets.y, insets.z, insets.w)
	await process_frame
	await process_frame

	var screen_rect := Rect2(Vector2.ZERO, Vector2(size))
	var safe := Rect2(Vector2(insets.x, insets.y), Vector2(size.x - insets.x - insets.z, size.y - insets.y - insets.w))
	var all: Array = []
	_controls(screen, all)

	var outside := []
	var outside_safe := []
	var squeezed := []
	var small_buttons := []
	for c: Control in all:
		if c.name == &"Background":
			continue
		var is_container := c.name == &"SafeArea"  # сам контейнер безопасной зоны занимает весь экран
		var r := c.get_global_rect()
		if not screen_rect.grow(1.0).encloses(r):
			outside.append("%s %s" % [c.name, r])
		if insets != Vector4i.ZERO and not is_container and not safe.grow(1.0).encloses(r):
			outside_safe.append("%s %s" % [c.name, r])
		var mn := c.get_combined_minimum_size()
		if c.size.x + 1.0 < mn.x or c.size.y + 1.0 < mn.y:
			squeezed.append("%s размер %s < минимум %s" % [c.name, c.size, mn])
		if c is ActionButton and c.size.y < 48.0:
			small_buttons.append(str(c.name))
	_check(outside.is_empty(), "%s: ничего не выходит за экран %s" % [label, str(outside)])
	_check(outside_safe.is_empty(), "%s: всё внутри безопасной зоны %s" % [label, str(outside_safe)])
	_check(squeezed.is_empty(), "%s: ни один элемент не сжат меньше минимума %s" % [label, str(squeezed)])
	_check(small_buttons.is_empty(), "%s: кнопки не ниже 48 px %s" % [label, str(small_buttons)])
	_check(size.x > size.y, "%s: ориентация горизонтальная" % label)

	# основные элементы видны и имеют ненулевой размер
	for n in ["%UrlEdit", "%ConnectButton", "%DisconnectButton", "%PingButton", "%StatusLabel", "%Log"]:
		var node: Control = screen.get_node(n)
		_check(node.is_visible_in_tree() and node.size.x > 0.0 and node.size.y > 0.0, "%s: %s виден" % [label, n])
	# журнал получает остаток высоты
	var log_h: float = screen.get_node("%Log").size.y
	_check(log_h >= 40.0, "%s: журнал не схлопнулся (%.0f px)" % [label, log_h])

	holder.queue_free()
	await process_frame


func _run() -> void:
	for s in SIZES:
		await _verify(s, Vector4i.ZERO, "%dx%d" % [s.x, s.y])
	# вырез камеры с одной и с другой стороны (поворот на 180°) и жест-полоса
	await _verify(Vector2i(1572, 720), Vector4i(110, 0, 0, 20), "1572x720 (IQOO 15R), вырез слева")
	await _verify(Vector2i(1572, 720), Vector4i(0, 0, 110, 20), "1572x720 (IQOO 15R), вырез справа")
	await _verify(Vector2i(1280, 720), Vector4i(90, 20, 90, 20), "1280x720, вырезы по краям")

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
