extends "res://ui/gateway/ui_gateway.gd"
## ДЕМО-шлюз: небольшая локальная симуляция для разработки интерфейса без сервера.
## Данные НЕ с сервера. Интерфейс обязан показывать плашку «Демо» (is_demo() == true).
## В рабочем режиме этот класс не используется: его заменит серверный шлюз.
## Детерминирован (фиксированный seed) и тикается вручную в тестах: tick(dt).

const STEP := 0.1               # шаг симуляции, с
const SPEED := 14.0             # скорость корабля, ед/с
const WEAPON_RANGE := 30.0      # дальность оружия
const ATTACK_DAMAGE := 40.0
const ATTACK_PERIOD := 1.0
const ENERGY_REGEN := 5.0
const COMBAT_TIMEOUT := 5.0     # сколько секунд после последнего боевого действия длится бой
const AUTOPILOT_STOP := 10.0
const PIRATE_AGGRO := 14.0       # без повода пират нападает только вблизи; атакованный — всегда

var auto_tick := true           # в тестах выключается, тогда время идёт только через tick()
var publish_on_command := true  # сразу публиковать состояние после команды (в тестах выключается для имитации задержки сервера)

var _rng := RandomNumberGenerator.new()
var _acc := 0.0
var _t := 0.0
var _ship := {}
var _objects: Dictionary = {}
var _target_id := ""
var _attacking := false
var _auto_skills := false
var _autopilot := false
var _waypoint := Vector2.ZERO
var _has_waypoint := false
var _move := Vector2.ZERO
var _boost_left := 0.0
var _attack_timer := 0.0
var _last_combat := -1000.0
var _credits := 125400
var _skills: Array = []
var _alerts: Dictionary = {}
var _respawn_at := -1.0
var _chat: Array = []
var _quests: Array = []
var _daily_claimed: Dictionary = {}
var _systems: Array = [
	{"id": "xi", "name": "Система Кси", "background": "xi"},
	{"id": "omega", "name": "Пояс Омега", "background": "omega"},
	{"id": "deep", "name": "Глубокий космос", "background": "deep"},
]
var _system_idx := 0


func _init() -> void:
	_rng.seed = 42
	_ship = {
		"name": "Фрегат «Заря»", "hull": 820.0, "hull_max": 1000.0, "shield": 300.0, "shield_max": 400.0,
		"energy": 90.0, "energy_max": 100.0, "pos": Vector2.ZERO, "dir": Vector2(1, 0),
	}
	_objects = {
		"pl1": {"id": "pl1", "name": "Орион-7", "kind": "player", "stance": "neutral", "is_ai": false,
			"pos": Vector2(-11, -10), "dir": Vector2(1, 0.3).normalized(), "hull": 700.0, "hull_max": 700.0},
		"ai1": {"id": "ai1", "name": "Капитан Вега", "kind": "ai", "stance": "neutral", "is_ai": true,
			"pos": Vector2(17, -9), "dir": Vector2(-1, 0), "hull": 900.0, "hull_max": 900.0},
		"pi1": _make_pirate(Vector2(13, 14)),
		"st1": {"id": "st1", "name": "Станция «Рубеж»", "kind": "station", "stance": "friendly", "is_ai": false,
			"pos": Vector2(-12, 14), "dir": Vector2(0, -1), "hull": 5000.0, "hull_max": 5000.0},
	}
	_skills = [
		{"slot": 0, "name": "Залп", "energy_cost": 25.0, "cooldown": 6.0, "cooldown_left": 0.0, "needs_target": true},
		{"slot": 1, "name": "Щит", "energy_cost": 20.0, "cooldown": 12.0, "cooldown_left": 0.0, "needs_target": false},
		{"slot": 2, "name": "Ремонт", "energy_cost": 30.0, "cooldown": 15.0, "cooldown_left": 0.0, "needs_target": false},
		{"slot": 3, "name": "Рывок", "energy_cost": 15.0, "cooldown": 10.0, "cooldown_left": 0.0, "needs_target": false},
	]
	_quests = [
		{"id": "demo_ore", "title": "Добыть руду в поясе Кси", "text": "Добудьте железную руду из астероидов.", "kind": "mine", "target": "iron_ore", "progress": 3, "goal": 10},
		{"id": "demo_station", "title": "Доставить груз на станцию", "text": "Долетите до станции «Рубеж».", "kind": "reach", "target": "st1", "progress": 0, "goal": 1},
	]
	_chat = [
		{"channel": "Общий", "author": "Орион-7", "text": "Кто идёт в пояс?"},
		{"channel": "Система", "author": "", "text": "Станция «Рубеж»: ремонт доступен."},
		{"channel": "Общий", "author": "Капитан Вега", "text": "Осторожно, рядом пираты."},
	]


func _make_pirate(pos: Vector2) -> Dictionary:
	return {"id": "pi1", "name": "Пират", "kind": "hostile", "stance": "hostile", "is_ai": true,
		"pos": pos, "dir": Vector2(-1, 0), "hull": 400.0, "hull_max": 400.0, "next_shot": 2.0}


func is_demo() -> bool:
	return true


func _process(delta: float) -> void:
	if auto_tick:
		tick(delta)


## Продвигает время на dt секунд (шагами по STEP) и публикует новое состояние.
func tick(dt: float) -> void:
	_acc += dt
	while _acc >= STEP:
		_acc -= STEP
		_step(STEP)
	state_updated.emit(snapshot())


## Корабль игрока для интерфейса: облик «Стрижа» и признак форсажа (демо-данные).
func _ship_view() -> Dictionary:
	var d: Dictionary = _ship.duplicate()
	d["type"] = "FTR-001"
	d["skin"] = "a"
	d["boosting"] = _boost_left > 0.0
	return d


func in_combat() -> bool:
	return _t - _last_combat < COMBAT_TIMEOUT


func snapshot() -> Dictionary:
	var objects: Array = []
	for id in _objects:
		objects.append(_objects[id].duplicate())
	var target: Dictionary = _objects.get(_target_id, {})
	var in_range := not target.is_empty() and _dist(target) <= WEAPON_RANGE
	return {
		"mode": "demo",
		"time": _t,
		"player": {"name": "Капитан Демо", "level": 7},
		"system": _systems[_system_idx].duplicate(),
		"ship": _ship_view(),
		"credits": _credits,
		"objects": objects,
		"target_id": _target_id,
		"target_in_range": in_range,
		"attacking": _attacking,
		"auto_skills": _auto_skills,
		"autopilot": _autopilot,
		"skills": _skills.duplicate(true),
		"combat": {"in_combat": in_combat()},
		"quests": _quests.duplicate(true),
		"chat": _chat.duplicate(true),
	}


# --- команды (в демо выполняются сразу; на сервере их проверяет сервер) ---

func set_target(id: String) -> void:
	if id != "" and not _objects.has(id):
		command_failed.emit("space.target", "Цель недоступна")
		return
	if id == _target_id:
		return
	_target_id = id
	_has_waypoint = false
	if _attacking:
		_attacking = false  # смена цели прекращает атаку прежней
	if _autopilot and id == "":
		_autopilot = false
	_publish()


func attack(id: String) -> void:
	if not _objects.has(id):
		command_failed.emit("combat.attack", "Цель недоступна")
		return
	if _objects[id]["stance"] == "friendly":
		command_failed.emit("combat.attack", "Нельзя атаковать союзную цель")
		return
	_target_id = id
	_attacking = true
	_attack_timer = 0.0
	_publish()


func stop_attack() -> void:
	_attacking = false
	_publish()


func use_skill(slot: int) -> void:
	if _try_skill(slot, true):
		_publish()


func set_auto_skills(on: bool) -> void:
	_auto_skills = on
	_publish()


func set_autopilot(on: bool) -> void:
	if on and not _has_waypoint and not _objects.has(_target_id):
		command_failed.emit("space.autopilot", "Выберите цель для автопилота")
		return
	if not on:
		_has_waypoint = false
	_autopilot = on
	_publish()


func set_waypoint(p: Vector2) -> void:
	_waypoint = p
	_has_waypoint = true
	_target_id = ""
	_move = Vector2.ZERO
	_autopilot = true
	_publish()


func move_input(v: Vector2) -> void:
	_move = v.limit_length(1.0)
	if _move.length() > 0.05:
		_autopilot = false
		_has_waypoint = false


## Демо: перейти в следующую систему (другой профиль фона). На сервере переход между системами — отдельная команда.
func demo_next_system() -> void:
	_system_idx = (_system_idx + 1) % _systems.size()
	notification.emit("Система: %s" % _systems[_system_idx]["name"])
	_publish()


func _publish() -> void:
	if publish_on_command:
		state_updated.emit(snapshot())


# --- симуляция ---

func _dist(obj: Dictionary) -> float:
	return (_ship["pos"] as Vector2).distance_to(obj["pos"])


func _step(dt: float) -> void:
	_t += dt
	_ship["energy"] = minf(_ship["energy_max"], _ship["energy"] + ENERGY_REGEN * dt)
	_boost_left = maxf(0.0, _boost_left - dt)
	for s in _skills:
		s["cooldown_left"] = maxf(0.0, s["cooldown_left"] - dt)

	_move_ship(dt)
	_attack_step(dt)
	_pirates_step(dt)
	if _auto_skills and in_combat():
		_auto_skill_step()
	if _respawn_at > 0.0 and _t >= _respawn_at:
		_respawn_at = -1.0
		_objects["pi1"] = _make_pirate(Vector2(13, 14) + Vector2(_rng.randf_range(-3, 3), _rng.randf_range(-3, 3)))
		notification.emit("Появился новый пират")
	_alerts_step()


func _move_ship(dt: float) -> void:
	var speed := SPEED * (1.8 if _boost_left > 0.0 else 1.0)
	var pos: Vector2 = _ship["pos"]
	if _move.length() > 0.05:
		pos += _move * speed * dt
		_ship["dir"] = _move.normalized()
	elif _autopilot and _has_waypoint:
		var to_wp: Vector2 = _waypoint - pos
		if to_wp.length() > AUTOPILOT_STOP:
			var d2 := to_wp.normalized()
			pos += d2 * minf(speed * dt, to_wp.length() - AUTOPILOT_STOP)
			_ship["dir"] = d2
		else:
			_autopilot = false
			_has_waypoint = false
			notification.emit("Автопилот: прибыли к точке")
	elif _autopilot and _objects.has(_target_id):
		var to: Vector2 = (_objects[_target_id]["pos"] as Vector2) - pos
		if to.length() > AUTOPILOT_STOP:
			var d := to.normalized()
			pos += d * minf(speed * dt, to.length() - AUTOPILOT_STOP)
			_ship["dir"] = d
		else:
			_autopilot = false
			notification.emit("Автопилот: прибыли к цели")
	_ship["pos"] = pos


func _attack_step(dt: float) -> void:
	if not _attacking:
		return
	if not _objects.has(_target_id):
		_attacking = false
		return
	_attack_timer -= dt
	if _attack_timer > 0.0:
		return
	_attack_timer = ATTACK_PERIOD
	var target: Dictionary = _objects[_target_id]
	if _dist(target) > WEAPON_RANGE:
		return  # вне дальности: выстрела нет (на сервере это проверяет сервер)
	_last_combat = _t
	_damage_object(target, ATTACK_DAMAGE)


func _damage_object(target: Dictionary, amount: float) -> void:
	combat_event.emit({"type": "fire", "from": "me", "to": target["id"]})
	combat_event.emit({"type": "hit_hull", "from": "me", "to": target["id"], "amount": amount})
	target["hull"] = maxf(0.0, target["hull"] - amount)
	target["provoked"] = true
	if target["hull"] > 0.0:
		return
	var id: String = target["id"]
	combat_event.emit({"type": "death", "to": id})
	_objects.erase(id)
	if id == _target_id:
		_target_id = ""
		_attacking = false
		_autopilot = false
	if target["kind"] == "hostile":
		_credits += 250
		notification.emit("Цель уничтожена: +250 кредитов (демо)")
		_respawn_at = _t + 6.0


func _pirates_step(dt: float) -> void:
	for id in _objects:
		var o: Dictionary = _objects[id]
		if o["kind"] != "hostile":
			continue
		var d := _dist(o)
		if not (d < PIRATE_AGGRO or o.get("provoked", false)):
			continue
		if d > 12.0:
			var dir: Vector2 = ((_ship["pos"] as Vector2) - o["pos"]).normalized()
			o["pos"] += dir * 6.0 * dt
			o["dir"] = dir
		if d < 35.0:
			o["next_shot"] = (o["next_shot"] as float) - dt
			if o["next_shot"] <= 0.0:
				o["next_shot"] = 2.0
				_last_combat = _t
				_damage_player(22.0)


func _damage_player(amount: float) -> void:
	var absorbed := minf(_ship["shield"], amount)
	combat_event.emit({"type": "hit_shield" if absorbed > 0.0 else "hit_hull", "to": "me", "amount": amount})
	_ship["shield"] -= absorbed
	_ship["hull"] = maxf(0.0, _ship["hull"] - (amount - absorbed))
	if _ship["hull"] <= 0.0:
		_ship["hull"] = _ship["hull_max"] * 0.25
		_attacking = false
		_target_id = ""
		_autopilot = false
		notification.emit("Корабль выведен из строя (демо): корпус частично восстановлен")


func _try_skill(slot: int, announce: bool) -> bool:
	if slot < 0 or slot >= _skills.size():
		if announce:
			command_failed.emit("combat.skill", "Нет такого навыка")
		return false
	var s: Dictionary = _skills[slot]
	var fail := ""
	if s["cooldown_left"] > 0.0:
		fail = "Навык перезаряжается"
	elif _ship["energy"] < s["energy_cost"]:
		fail = "Недостаточно энергии"
	elif s["needs_target"]:
		if not _objects.has(_target_id):
			fail = "Выберите цель"
		elif _dist(_objects[_target_id]) > WEAPON_RANGE:
			fail = "Цель вне дальности"
		elif _objects[_target_id]["stance"] == "friendly":
			fail = "Нельзя атаковать союзную цель"
	if fail != "":
		if announce:
			command_failed.emit("combat.skill", fail)
		return false
	_ship["energy"] -= s["energy_cost"]
	s["cooldown_left"] = s["cooldown"]
	match slot:
		0:
			_last_combat = _t
			_damage_object(_objects[_target_id], 120.0)
		1:
			_ship["shield"] = minf(_ship["shield_max"], _ship["shield"] + 150.0)
		2:
			_ship["hull"] = minf(_ship["hull_max"], _ship["hull"] + 120.0)
		3:
			_boost_left = 3.0
			combat_event.emit({"type": "boost", "from": "me"})
	return true


func _auto_skill_step() -> void:
	if _target_id != "" and _try_skill(0, false):
		return
	if _ship["shield"] < _ship["shield_max"] * 0.5 and _try_skill(1, false):
		return
	if _ship["hull"] < _ship["hull_max"] * 0.5:
		_try_skill(2, false)


func _alerts_step() -> void:
	var low_shield: bool = in_combat() and _ship["shield"] < _ship["shield_max"] * 0.25
	var low_hull: bool = _ship["hull"] < _ship["hull_max"] * 0.30
	_set_alert("shield_low", "Щиты на исходе!", low_shield)
	_set_alert("hull_critical", "Корпус в критическом состоянии!", low_hull)


func _set_alert(id: String, text: String, on: bool) -> void:
	if on and not _alerts.has(id):
		_alerts[id] = true
		alert_raised.emit(id, text)
	elif not on and _alerts.has(id):
		_alerts.erase(id)
		alert_cleared.emit(id)


func request_daily() -> void:
	var day := Time.get_date_string_from_system()
	daily_updated.emit({
		"day": day,
		"streak": 1,
		"can_claim_return": not _daily_claimed.has("return"),
		"return_reward": 200,
		"claimed": _daily_claimed.duplicate(),
		"activities": [
			{"id": "daily_mine", "title": "Сбор ресурсов", "goal": 3, "reward": 180},
			{"id": "daily_trade", "title": "Торговый маршрут", "goal": 1, "reward": 260},
			{"id": "daily_jump", "title": "Дальний прыжок", "goal": 1, "reward": 220}
		]
	})

func claim_daily(id: String) -> void:
	if id == "" or _daily_claimed.has(id):
		return
	_daily_claimed[id] = true
	daily_rewarded.emit({"id": id, "granted": true, "reward": 200, "credits": _credits})
