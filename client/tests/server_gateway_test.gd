extends SceneTree
## Серверный шлюз: преобразование снимка в формат HUD (офлайн) и сквозная проверка с настоящим сервером
## (если задана COSMOC_TEST_WS_URL): вход, движение, цель и автопилот, два клиента видят друг друга.

const ServerGateway := preload("res://ui/gateway/server_gateway.gd")
const PlayerIdentity := preload("res://ui/player_identity.gd")

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


func _wait(gw: Node, cond: Callable, seconds: float = 6.0) -> bool:
	var t0 := Time.get_ticks_msec()
	while Time.get_ticks_msec() - t0 < int(seconds * 1000.0):
		if cond.call():
			return true
		await process_frame
	return false


func _run() -> void:
	# --- офлайн: преобразование ---
	var gw := ServerGateway.new()
	root.add_child(gw)
	gw.player_name = "Пилот-1"
	var got: Array = []
	gw.state_updated.connect(func(s: Dictionary) -> void: got.append(s))
	gw._apply_state({"time": 3.5, "ship": {"name": "Пилот-1", "pos": [1.0, 2.0], "dir": [0.0, 1.0], "speed": 14.0},
		"objects": [{"id": "st1", "kind": "station", "stance": "friendly", "name": "Станция", "pos": [-12, 14], "dir": [0, -1]}],
		"target_id": "st1", "autopilot": true})
	var s: Dictionary = got[0]
	_check(s["ship"]["pos"] == Vector2(1, 2) and s["ship"]["speed"] == 14.0, "позиция и скорость корабля преобразованы")
	_check(s["objects"][0]["pos"] == Vector2(-12, 14) and s["objects"][0]["kind"] == "station", "объекты преобразованы")
	_check(s["target_id"] == "st1" and s["autopilot"] and s["time"] == 3.5, "цель, автопилот, время")
	_check(not s["ship"].has("hull") and not s.has("credits") and not s["player"].has("level"), "данных, которых сервер не присылает, нет (ничего не выдумано)")
	_check(s["skills"].is_empty() and not s["combat_available"], "способностей и боя пока нет")
	_check(not gw.is_demo(), "серверный шлюз не демо")
	gw.queue_free()

	# --- онлайн ---
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if url == "":
		print("COSMOC_TEST_WS_URL не задана: сквозная проверка пропущена")
		_finish()
		return
	var tag := str(Time.get_ticks_usec())
	PlayerIdentity.set_override("testa" + tag.pad_zeros(16))
	var a := ServerGateway.new()
	root.add_child(a)
	a.connect_to_server(url)
	_check(await _wait(a, func() -> bool: return not a.snapshot().is_empty()), "вход выполнен, пришло состояние")
	var st: Dictionary = a.snapshot()
	_check(str(st["player"]["name"]).begins_with("Пилот-"), "имя по умолчанию «Пилот-NNNN» (%s)" % str(st["player"]["name"]))
	_check(st["system"].get("id", "") == "xi", "система Кси")
	var has_station := false
	for o in st["objects"]:
		if o["kind"] == "station":
			has_station = true
	_check(has_station, "станция в мире")

	var p0: Vector2 = st["ship"]["pos"]
	a.move_input(Vector2(1, 0))
	await _wait(a, func() -> bool: return (a.snapshot()["ship"]["pos"] as Vector2).x > p0.x + 5.0)
	_check((a.snapshot()["ship"]["pos"] as Vector2).x > p0.x + 5.0, "джойстик вправо двигает корабль по серверу")
	_check(a.snapshot()["ship"]["speed"] > 5.0, "скорость приходит с сервера (%.1f)" % a.snapshot()["ship"]["speed"])
	a.move_input(Vector2.ZERO)
	await _wait(a, func() -> bool: return a.snapshot()["ship"]["speed"] < 0.5)
	_check(a.snapshot()["ship"]["speed"] < 0.5, "нулевой вектор останавливает корабль")

	a.set_target("st1")
	await _wait(a, func() -> bool: return a.snapshot()["target_id"] == "st1")
	_check(a.snapshot()["target_id"] == "st1", "цель выбрана сервером")
	a.set_autopilot(true)
	await _wait(a, func() -> bool: return a.snapshot()["autopilot"])
	_check(a.snapshot()["autopilot"], "автопилот включён")

	# второй клиент
	PlayerIdentity.set_override("testb" + tag.pad_zeros(16))
	var b := ServerGateway.new()
	root.add_child(b)
	b.connect_to_server(url)
	await _wait(b, func() -> bool: return not b.snapshot().is_empty())
	var a_name: String = str(a.snapshot()["player"]["name"])
	var seen := await _wait(b, func() -> bool:
		for o in b.snapshot().get("objects", []):
			if o["name"] == a_name:
				return true
		return false)
	_check(seen, "второй клиент видит первого")
	var b_name: String = str(b.snapshot()["player"]["name"])
	var seen2 := await _wait(a, func() -> bool:
		for o in a.snapshot().get("objects", []):
			if o["name"] == b_name:
				return true
		return false)
	_check(seen2, "первый клиент видит второго")

	a.close()
	b.close()
	await _wait(a, func() -> bool: return false, 0.3)
	PlayerIdentity.set_override("")
	_finish()


func _finish() -> void:
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
