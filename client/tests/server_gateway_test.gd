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
	var combat_got: Array = []
	gw.combat_event.connect(func(e: Dictionary) -> void: combat_got.append(e))
	gw._on_message("combat_events", {"events": [{"type": "fire", "from": "a", "to": "b"}, {"type": "hit_shield", "to": "b", "amount": 44}]}, "")
	_check(combat_got.size() == 2 and combat_got[0]["type"] == "fire", "боевые события сервера передаются эффектам")
	gw._apply_state({"time": 5.0, "ship": {"name": "П", "pos": [0, 0], "dir": [1, 0], "speed": 0.0, "type": "FTR-001", "skin": "b", "spec": "scout",
		"hull": 1200, "hull_max": 1200, "shield": 600, "shield_max": 600, "energy": 100, "energy_max": 100, "armor": 120, "cargo": 8, "pulse_range": 30,
		"boosting": true, "abilities": [{"id": "boost", "name": "Форсаж", "energy_cost": 20, "cooldown": 12, "cooldown_left": 3, "needs_target": false},
			{"id": "scan", "name": "Импульсный сканер", "energy_cost": 25, "cooldown": 20, "cooldown_left": 0, "needs_target": false}]},
		"objects": [{"id": "dr1", "kind": "hostile", "stance": "hostile", "name": "Дрон", "pos": [10, 0], "dir": [0, -1], "hull": 1500, "hull_max": 1500}],
		"target_id": "dr1", "attacking": true})
	var s3: Dictionary = got[got.size() - 1]
	_check(s3["ship"]["hull_max"] == 1200 and s3["ship"]["skin"] == "b" and s3["ship"]["boosting"], "корабль: характеристики, облик и форсаж от сервера")
	_check(s3["skills"].size() == 2 and s3["skills"][0]["cooldown_left"] == 3.0 and s3["combat_available"], "способности сервера показаны кнопками с перезарядкой")
	_check(s3["attacking"] and s3["target_in_range"] and s3["objects"][0]["hull"] == 1500, "атака, дальность и корпус цели")
	gw._apply_state({"time": 4.0, "ship": {"name": "П", "pos": [0, 0], "dir": [1, 0], "speed": 0.0}, "objects": [],
		"credits": 500, "quests": [{"id": "first_flight", "title": "Первый полёт", "text": "…", "progress": 0, "goal": 1}]})
	var s2: Dictionary = got[got.size() - 1]
	_check(s2["credits"] == 500 and s2["quests"][0]["title"] == "Первый полёт" and s2["quests"][0]["goal"] == 1, "кредиты и задания с сервера преобразованы")
	var notes: Array = []
	gw.notification.connect(func(t: String) -> void: notes.append(t))
	gw._on_message("quest_completed", {"id": "first_flight", "title": "Первый полёт", "reward": {"credits": 500}}, "")
	_check(notes.size() == 1 and notes[0].contains("500"), "уведомление о выполнении контракта")
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
	a.character_needed.connect(func(_o: Array) -> void: a.create_character("Tst_A" + tag.right(8), "m", "interceptor"))
	root.add_child(a)
	a.connect_to_server(url)
	_check(await _wait(a, func() -> bool: return not a.snapshot().is_empty()), "вход выполнен, пришло состояние")
	var st: Dictionary = a.snapshot()
	_check(str(st["player"]["name"]).begins_with("Tst_A"), "ник персонажа применён (%s)" % str(st["player"]["name"]))
	_check(st["system"].get("id", "") == "aurora", "система Аврора")
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

	# корабль «Стриж»: паспорт, способности, форсаж и облик от сервера
	var sh: Dictionary = a.snapshot()["ship"]
	_check(sh.get("type") == "FTR-001" and sh.get("hull_max") == 1200.0 and sh.get("shield_max") == 600.0 and sh.get("armor") == 120.0 and sh.get("cargo") == 8.0, "паспорт Стрижа с сервера (корпус 1200, щит 600, броня 120, груз 8)")
	_check(a.snapshot()["skills"].size() >= 4 and a.snapshot()["combat_available"], "умения корабля: атака, модули, ульта и форсаж корпуса")
	var roles: Dictionary = {}
	for sk in a.snapshot()["skills"]:
		roles[str(sk.get("role", ""))] = true
	_check(bool(roles.get("strike", false)) and bool(roles.get("ult", false)) and bool(roles.get("module", false)), "на веере есть атака, ульта и модуль")
	var fx: Array = []
	a.combat_event.connect(func(e: Dictionary) -> void: fx.append(e["type"]))
	a.cast_ability("M01")
	await _wait(a, func() -> bool: return a.snapshot()["ship"]["boosting"])
	_check(a.snapshot()["ship"]["boosting"] and "boost" in fx, "форсаж корпуса включён сервером, событие получено")
	a.cast_ability("M01")
	var errs: Array = []
	a.command_failed.connect(func(_a: String, r: String) -> void: errs.append(r))
	await _wait(a, func() -> bool: return errs.size() > 0, 3.0)
	_check(errs.size() > 0 and "перезаряж" in errs[0], "повтор форсажа отклонён сервером: %s" % str(errs))
	a.set_skin("b")
	await _wait(a, func() -> bool: return a.snapshot()["ship"]["skin"] == "b")
	_check(a.snapshot()["ship"]["skin"] == "b", "облик «b» принят и сохранён сервером")

	# контракт «Первый полёт»: автопилот приводит к станции, сервер выдаёт награду
	_check(a.snapshot().get("quests", []).size() == 1 and a.snapshot()["credits"] == 0, "контракт выдан, кредитов 0")
	var notes_live: Array = []
	a.notification.connect(func(t: String) -> void: notes_live.append(t))
	await _wait(a, func() -> bool: return int(a.snapshot().get("credits", 0)) == 500, 30.0)
	var q_after: Array = a.snapshot()["quests"]
	_check(int(a.snapshot().get("credits", 0)) == 500 and q_after.size() == 1 and str(q_after[0].get("id", "")) == "first_blood", "награда 500 кредитов получена, открыто следующее задание цепочки")
	await _wait(a, func() -> bool: return notes_live.any(func(t: String) -> bool: return t.contains("опыта") or t.contains("Новый уровень")), 5.0)   # опыт за контракт приходит отдельным сообщением (RPG-1); 100 опыта — это уже уровень 2
	_check(notes_live.filter(func(t: String) -> bool: return t.contains("Задание выполнено")).size() == 1, "уведомление о выполнении задания пришло один раз")
	_check(notes_live.any(func(t: String) -> bool: return t.contains("опыта") or t.contains("Новый уровень")) and a.has_pilot(), "за контракт начислен опыт пилота (RPG-1)")

	# второй клиент
	PlayerIdentity.set_override("testb" + tag.pad_zeros(16))
	var b := ServerGateway.new()
	b.character_needed.connect(func(_o: Array) -> void: b.create_character("Tst_B" + tag.right(8), "f", "scout"))
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
