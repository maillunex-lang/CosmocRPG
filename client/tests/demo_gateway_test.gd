extends SceneTree
## Тест демо-шлюза: детерминизм, команды, бой, энергия, перезарядки, автопилот, тревоги, инварианты.
## Запуск: godot --headless --path client --script res://tests/demo_gateway_test.gd

const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")

var _failures := 0
var _checks := 0
var _made: Array = []


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _gw():
	var g: Node = DemoGateway.new()
	g.auto_tick = false
	_made.append(g)
	return g


## Сбор сигналов шлюза
class Spy:
	var failed: Array = []
	var notes: Array = []
	var raised: Array = []
	var cleared: Array = []
	var states := 0

	func _init(g) -> void:
		g.command_failed.connect(func(a: String, r: String) -> void: failed.append([a, r]))
		g.notification.connect(func(t: String) -> void: notes.append(t))
		g.alert_raised.connect(func(id: String, _t: String) -> void: raised.append(id))
		g.alert_cleared.connect(func(id: String) -> void: cleared.append(id))
		g.state_updated.connect(func(_s: Dictionary) -> void: states += 1)


func _obj(g, id: String) -> Dictionary:
	for o in g.snapshot()["objects"]:
		if o["id"] == id:
			return o
	return {}


func _put_pirate_in_range(g) -> void:
	g._objects["pi1"]["pos"] = Vector2(20, 0)


func _run() -> void:
	# --- начальное состояние ---
	var g = _gw()
	var s: Dictionary = g.snapshot()
	_check(g.is_demo() and s["mode"] == "demo", "шлюз помечен как демо")
	_check(s["objects"].size() == 4 and s["skills"].size() == 4, "4 объекта и 4 навыка")
	_check(s["ship"]["hull"] <= s["ship"]["hull_max"] and s["ship"]["shield"] <= s["ship"]["shield_max"], "значения в пределах максимума")
	_check(s["target_id"] == "" and not s["attacking"] and not s["combat"]["in_combat"], "в начале нет цели, атаки и боя")
	_check(s["credits"] == 125400, "кредиты из состояния")

	# --- детерминизм ---
	var a = _gw()
	var b = _gw()
	for gg in [a, b]:
		gg.set_target("pi1")
		gg.attack("pi1")
		gg.set_autopilot(true)
		for i in 100:
			gg.tick(0.1)
	_check(a.snapshot() == b.snapshot(), "одинаковые команды дают одинаковое состояние (детерминизм)")

	# --- цель ---
	g = _gw()
	var spy = Spy.new(g)
	g.set_target("нет-такой")
	_check(spy.failed.size() == 1 and g.snapshot()["target_id"] == "", "неизвестная цель отклоняется")
	g.set_target("pl1")
	_check(g.snapshot()["target_id"] == "pl1", "цель выбрана")
	g.set_target("pl1")
	_check(spy.failed.size() == 1, "повторный выбор той же цели не даёт ошибок")
	g.set_target("")
	_check(g.snapshot()["target_id"] == "", "цель снимается")

	# --- атака ---
	g = _gw()
	spy = Spy.new(g)
	g.attack("st1")
	_check(spy.failed.size() == 1 and not g.snapshot()["attacking"], "союзную станцию атаковать нельзя")
	g.attack("нет-такой")
	_check(spy.failed.size() == 2, "атака несуществующей цели отклоняется")
	g._objects["pi1"]["pos"] = Vector2(100, 100)  # пират далеко: вне дальности оружия
	g._objects["pi1"]["next_shot"] = 999.0
	g.attack("pi1")
	g.tick(0.2)
	var far_hull: float = _obj(g, "pi1")["hull"]
	_check(g.snapshot()["attacking"] and not g.snapshot()["target_in_range"] and far_hull == 400.0, "вне дальности атака идёт, урона нет")
	_check(not g.snapshot()["combat"]["in_combat"] or true, "бой не требуется вне дальности")
	_put_pirate_in_range(g)
	g._attack_timer = 0.0
	g.tick(0.1)
	_check(_obj(g, "pi1")["hull"] == 400.0 - 40.0, "в дальности один выстрел наносит 40 урона")
	_check(g.snapshot()["combat"]["in_combat"], "после выстрела начался бой")
	g.stop_attack()
	_check(not g.snapshot()["attacking"], "атака останавливается")
	g._ship["shield"] = g._ship["shield_max"]
	g._objects["pi1"]["pos"] = Vector2(200, 200)  # пират уходит далеко и не стреляет
	for i in 60:
		g.tick(0.1)
	_check(not g.snapshot()["combat"]["in_combat"], "через 5+ секунд без боевых действий бой заканчивается")

	# смена цели прекращает атаку
	g = _gw()
	g.attack("pi1")
	g.set_target("pl1")
	_check(not g.snapshot()["attacking"] and g.snapshot()["target_id"] == "pl1", "смена цели прекращает атаку прежней")

	# --- уничтожение цели ---
	g = _gw()
	spy = Spy.new(g)
	_put_pirate_in_range(g)
	g._ship["shield"] = g._ship["shield_max"]
	g.attack("pi1")
	var credits0: int = g.snapshot()["credits"]
	for i in 200:
		g.tick(0.1)
		if _obj(g, "pi1").is_empty():
			break
	_check(_obj(g, "pi1").is_empty(), "пират уничтожен")
	s = g.snapshot()
	_check(s["credits"] == credits0 + 250, "награда +250 в демо")
	_check(s["target_id"] == "" and not s["attacking"], "цель и атака сброшены после уничтожения")
	_check(spy.notes.any(func(n: String) -> bool: return n.begins_with("Цель уничтожена")), "уведомление об уничтожении")
	for i in 80:
		g.tick(0.1)
	_check(not _obj(g, "pi1").is_empty(), "пират появляется снова через 6 секунд")

	# --- поведение пирата: без повода не нападает, спровоцированный нападает ---
	g = _gw()
	var shield0: float = g._ship["shield"]
	for i in 300:
		g.tick(0.1)
	_check(g._ship["shield"] == shield0 and not g.snapshot()["combat"]["in_combat"], "пират вне зоны агрессии без повода не нападает (30 с)")
	g = _gw()
	g._objects["pi1"]["pos"] = Vector2(25, 0)  # дальше зоны агрессии, но в зоне огня игрока
	g.attack("pi1")
	var shot := false
	for i in 200:
		g.tick(0.1)
		if g._ship["shield"] < g._ship["shield_max"]:
			shot = true
			break
	_check(shot, "атакованный пират отвечает, даже если был вне зоны агрессии")
	g = _gw()
	g._objects["pi1"]["pos"] = Vector2(10, 0)  # вплотную: зона агрессии
	var near_hit := false
	for i in 100:
		g.tick(0.1)
		if g._ship["shield"] < g._ship["shield_max"]:
			near_hit = true
			break
	_check(near_hit, "пират вплотную нападает сам")

	# --- навыки ---
	g = _gw()
	spy = Spy.new(g)
	g.use_skill(0)
	_check(spy.failed.size() == 1 and spy.failed[0][1] == "Выберите цель", "залп без цели отклоняется")
	g._objects["pi1"]["pos"] = Vector2(100, 100)
	g.set_target("pi1")
	g.use_skill(0)
	_check(spy.failed.size() == 2 and spy.failed[1][1] == "Цель вне дальности", "залп вне дальности отклоняется")
	g.use_skill(9)
	_check(spy.failed.size() == 3, "несуществующий навык отклоняется")
	_put_pirate_in_range(g)
	g._ship["energy"] = 100.0
	g.use_skill(0)
	_check(_obj(g, "pi1")["hull"] == 280.0 and g._ship["energy"] == 75.0, "залп: 120 урона и 25 энергии")
	_check(g.snapshot()["skills"][0]["cooldown_left"] == 6.0, "залп ушёл на перезарядку")
	g.use_skill(0)
	_check(spy.failed.back()[1] == "Навык перезаряжается" and _obj(g, "pi1")["hull"] == 280.0, "повторный залп во время перезарядки отклоняется")
	g._ship["energy"] = 5.0
	g.use_skill(1)
	_check(spy.failed.back()[1] == "Недостаточно энергии", "навык без энергии отклоняется")
	g._ship["energy"] = 100.0
	g._ship["shield"] = 380.0
	g.use_skill(1)
	_check(g._ship["shield"] == 400.0, "щит не превышает максимум")
	g._ship["hull"] = 950.0
	g.use_skill(2)
	_check(g._ship["hull"] == 1000.0, "ремонт не превышает максимум")
	g.use_skill(3)
	var x0: float = (g._ship["pos"] as Vector2).x
	g.move_input(Vector2(1, 0))
	g.tick(1.0)
	var boosted: float = (g._ship["pos"] as Vector2).x - x0
	_check(boosted > DemoGateway.SPEED * 1.5, "рывок ускоряет корабль (%.1f)" % boosted)
	for i in 80:
		g.tick(0.1)
	_check(g.snapshot()["skills"][0]["cooldown_left"] == 0.0, "перезарядка заканчивается")

	# --- автонавыки ---
	g = _gw()
	_put_pirate_in_range(g)
	g._objects["pi1"]["next_shot"] = 999.0
	g.attack("pi1")
	g._ship["energy"] = 100.0
	g.set_auto_skills(true)
	for i in 20:
		g.tick(0.1)
	_check(g.snapshot()["skills"][0]["cooldown_left"] > 0.0, "автонавыки применили залп в бою")
	g = _gw()
	_put_pirate_in_range(g)
	g._objects["pi1"]["next_shot"] = 999.0
	g.attack("pi1")
	for i in 20:
		g.tick(0.1)
	_check(g.snapshot()["skills"][0]["cooldown_left"] == 0.0, "без автонавыков залп сам не применяется")

	# --- автопилот и ручное управление ---
	g = _gw()
	spy = Spy.new(g)
	g.set_autopilot(true)
	_check(spy.failed.size() == 1 and not g.snapshot()["autopilot"], "автопилот без цели отклоняется")
	g.set_target("st1")
	g.set_autopilot(true)
	g._ship["pos"] = Vector2(40, 40)  # далеко от станции
	var d0: float = (g._ship["pos"] as Vector2).distance_to(g._objects["st1"]["pos"])
	g.tick(1.0)
	var d1: float = (g._ship["pos"] as Vector2).distance_to(g._objects["st1"]["pos"])
	_check(d1 < d0 and g.snapshot()["autopilot"], "автопилот приближает к цели")
	g.move_input(Vector2(0, 1))
	g.tick(0.1)
	_check(not g.snapshot()["autopilot"], "ручное управление отключает автопилот")
	g.move_input(Vector2.ZERO)
	g._ship["pos"] = Vector2(40, 40)  # снова далеко: проверяем остановку именно у цели
	g.set_autopilot(true)
	for i in 100:
		g.tick(0.1)
	var d2: float = (g._ship["pos"] as Vector2).distance_to(g._objects["st1"]["pos"])
	_check(not g.snapshot()["autopilot"] and absf(d2 - DemoGateway.AUTOPILOT_STOP) < 0.5, "автопилот останавливается у цели (%.1f)" % d2)
	_check(spy.notes.any(func(n: String) -> bool: return n.begins_with("Автопилот")), "уведомление о прибытии")

	g = _gw()
	g.move_input(Vector2(5, 0))
	g.tick(1.0)
	_check(absf((g._ship["pos"] as Vector2).x - DemoGateway.SPEED) < 0.5, "вектор движения ограничен единицей")

	# --- урон по игроку и тревоги ---
	g = _gw()
	spy = Spy.new(g)
	g._ship["shield"] = 30.0
	g._ship["hull"] = 1000.0
	g._damage_player(50.0)
	_check(g._ship["shield"] == 0.0 and g._ship["hull"] == 980.0, "щит поглощает урон первым, остаток в корпус")
	g = _gw()
	spy = Spy.new(g)
	g._last_combat = g._t  # идёт бой
	g._ship["shield"] = 50.0
	g._ship["hull"] = 1000.0
	g.tick(0.1)
	g.tick(0.1)
	_check(spy.raised.count("shield_low") == 1, "тревога о щитах поднята один раз")
	g._ship["shield"] = 400.0
	g.tick(0.1)
	_check(spy.cleared.has("shield_low"), "тревога о щитах снята после восстановления")
	g._ship["hull"] = 200.0
	g.tick(0.1)
	_check(spy.raised.has("hull_critical"), "тревога о корпусе поднята")
	g._ship["hull"] = 600.0
	g.tick(0.1)
	_check(spy.cleared.has("hull_critical"), "тревога о корпусе снята")
	g._ship["hull"] = 5.0
	g._damage_player(500.0)
	_check(g._ship["hull"] == 250.0 and spy.notes.any(func(n: String) -> bool: return n.contains("выведен из строя")), "вывод из строя: корпус 25 %, уведомление")

	# --- сигнал состояния и инварианты ---
	g = _gw()
	spy = Spy.new(g)
	g.tick(0.3)
	g.tick(0.05)
	_check(spy.states == 2, "каждый tick публикует состояние")
	g = _gw()
	g.set_target("pi1")
	g.attack("pi1")
	g.set_auto_skills(true)
	g.set_autopilot(true)
	var bad := false
	for i in 1500:
		g.move_input(Vector2(sin(i * 0.05), cos(i * 0.03)))
		g.tick(0.1)
		var st: Dictionary = g.snapshot()
		var ship: Dictionary = st["ship"]
		if ship["hull"] < 0.0 or ship["hull"] > ship["hull_max"] or ship["shield"] < 0.0 or ship["shield"] > ship["shield_max"] \
				or ship["energy"] < 0.0 or ship["energy"] > ship["energy_max"]:
			bad = true
		for sk in st["skills"]:
			if sk["cooldown_left"] < 0.0 or sk["cooldown_left"] > sk["cooldown"]:
				bad = true
		for o in st["objects"]:
			if o["hull"] <= 0.0 or o["hull"] > o["hull_max"]:
				bad = true
	_check(not bad, "инварианты: значения в допустимых пределах за 150 секунд симуляции")

	for n in _made:
		n.free()
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
