extends "res://ui/gateway/ui_gateway.gd"
## Серверный шлюз (Этап 01): состояние мира приходит с сервера, команды уходят серверу. Клиент ничего
## не считает. Протокол: docs/design/STAGE01_ONLINE_SPACE.md. Бой и способности сервер пока не знает:
## эти поля в состоянии отсутствуют, интерфейс их не показывает (данных нет, значит не выдумываем).

const WsClient := preload("res://scripts/net/ws_client.gd")
const PlayerIdentity := preload("res://ui/player_identity.gd")

const PROTOCOL := 1
const MOVE_INTERVAL := 0.1         # не чаще 10 сообщений движения в секунду (лимит сервера 20/с)
const RETRY_DELAYS: Array[float] = [1.0, 2.0, 4.0, 8.0]
const ALERT_LINK := "link"

## Вход выполнен (получен welcome).
signal joined
## Вход не удался (причина для игрока).
signal join_failed(reason: String)

var url := ""
var player_name := ""
var player_id := ""
## Имя при создании игрока (по умолчанию сервер даёт «Пилот-NNNN»); нужно тестам, чтобы отличать своих игроков.
var join_name := ""
var system: Dictionary = {}

var _client: Node
var _state: Dictionary = {}
var _target_id := ""
var _autopilot := false
var _joined := false
var _closing := false
var _retry := 0
var _retry_in := -1.0
var _move := Vector2.ZERO
var _sent_move := Vector2.ZERO
var _move_cd := 0.0
var _move_dirty := false
var _token := ""
var _ability_ids: Array = []   # слот способности (кнопка HUD) -> идентификатор на сервере


func is_demo() -> bool:
	return false


## Публичный идентификатор своего игрока (в событиях боя).
func own_id() -> String:
	return player_id


func snapshot() -> Dictionary:
	return _state


## Принять уже открытое соединение (после hello). Сразу отправляет join.
func adopt(client: Node, server_url: String) -> void:
	url = server_url
	_client = client
	if _client.get_parent() != self:
		if _client.get_parent() != null:
			_client.get_parent().remove_child(_client)
		add_child(_client)
	_wire()
	_join()


## Подключиться самостоятельно (повтор подключения и тесты).
func connect_to_server(server_url: String) -> void:
	url = server_url
	_client = WsClient.new()
	add_child(_client)
	_wire()
	_client.connect_to(url)


func _wire() -> void:
	_client.connected.connect(func(_s: String, _v: String) -> void: _join())
	_client.connection_failed.connect(_on_link_lost)
	_client.disconnected.connect(func(_c: int, _r: String) -> void: _on_link_lost("Соединение закрыто"))
	_client.message_received.connect(_on_message)


func close() -> void:
	_closing = true
	if _client != null:
		_client.close()


func _join() -> void:
	_token = PlayerIdentity.token()
	var payload := {"token": _token, "protocol": PROTOCOL}
	if join_name != "":
		payload["name"] = join_name
	_client.send_message("join", payload)


func _process(delta: float) -> void:
	_move_cd -= delta
	if _joined and _move_dirty and _move_cd <= 0.0:
		_send_move()
	if _retry_in >= 0.0:
		_retry_in -= delta
		if _retry_in < 0.0:
			_client.connect_to(url)


func _on_link_lost(reason: String) -> void:
	if _closing:
		return
	var was_joined := _joined
	_joined = false
	if not was_joined and _retry == 0 and _state.is_empty():
		join_failed.emit(reason)   # первое подключение не удалось: решает экран подключения
		return
	alert_raised.emit(ALERT_LINK, "Связь потеряна, переподключение…")
	_retry_in = RETRY_DELAYS[mini(_retry, RETRY_DELAYS.size() - 1)]
	_retry += 1


func _on_message(type: String, payload: Dictionary, _id: String) -> void:
	match type:
		"welcome":
			_joined = true
			_retry = 0
			_retry_in = -1.0
			player_name = str(payload.get("name", ""))
			player_id = str(payload.get("player_id", ""))
			system = payload.get("system", {})
			alert_cleared.emit(ALERT_LINK)
			_sent_move = Vector2.INF   # после входа заново сообщаем движение
			joined.emit()
		"state":
			_apply_state(payload)
		"combat_events":
			for e in payload.get("events", []):
				if e is Dictionary:
					combat_event.emit(e)
		"quest_completed":
			var reward: Dictionary = payload.get("reward", {})
			notification.emit("Задание выполнено: %s. Награда: %d кредитов" % [str(payload.get("title", "")), int(reward.get("credits", 0))])
		"error":
			_on_error(payload)


func _on_error(payload: Dictionary) -> void:
	var code := str(payload.get("code", ""))
	var text := str(payload.get("message", "ошибка"))
	match code:
		"replaced":
			_closing = true   # вошли с другого устройства: переподключаться не нужно
			alert_raised.emit(ALERT_LINK, "Вход выполнен с другого устройства")
		"bad_protocol", "bad_token", "server_full":
			_closing = true
			if _joined:
				alert_raised.emit(ALERT_LINK, text)
			else:
				join_failed.emit(text)
		"bad_target", "no_target", "cooldown", "no_energy", "out_of_range", "wrong_spec", "dead", "unknown_ability", "bad_skin", "bad_spec":
			command_failed.emit("space", text)
		_:
			command_failed.emit("server", text)


func _ship_dict(ship: Dictionary) -> Dictionary:
	var d := {"name": str(ship.get("name", "")), "pos": _vec(ship.get("pos")), "dir": _vec(ship.get("dir")), "speed": float(ship.get("speed", 0.0)),
		"type": str(ship.get("type", "")), "skin": str(ship.get("skin", "")), "spec": str(ship.get("spec", "")),
		"boosting": bool(ship.get("boosting", false)), "dead": bool(ship.get("dead", false))}
	if ship.has("hull"):   # шкалы показываются только если сервер прислал значения
		for k in ["hull", "hull_max", "shield", "shield_max", "energy", "energy_max", "armor", "cargo"]:
			d[k] = float(ship.get(k, 0.0))
	return d


func _vec(a: Variant) -> Vector2:
	if a is Array and a.size() >= 2:
		return Vector2(float(a[0]), float(a[1]))
	return Vector2.ZERO


## Снимок сервера в формат HUD. Поля боя и экономики не добавляются: сервер их не присылает.
func _apply_state(p: Dictionary) -> void:
	var ship: Dictionary = p.get("ship", {})
	var objects: Array = []
	for o in p.get("objects", []):
		var obj := {
			"id": str(o.get("id", "")), "kind": str(o.get("kind", "")), "stance": str(o.get("stance", "neutral")),
			"name": str(o.get("name", "")), "pos": _vec(o.get("pos")), "dir": _vec(o.get("dir")),
			"ship": str(o.get("ship", "")), "skin": str(o.get("skin", "")), "dead": bool(o.get("dead", false)),
		}
		if o.has("hull"):
			obj["hull"] = float(o["hull"])
			obj["hull_max"] = float(o.get("hull_max", o["hull"]))
		objects.append(obj)
	_target_id = str(p.get("target_id", ""))
	_autopilot = bool(p.get("autopilot", false))
	var abilities: Array = ship.get("abilities", [])
	var skills: Array = []
	_ability_ids.clear()
	for i in abilities.size():
		var ab: Dictionary = abilities[i]
		_ability_ids.append(str(ab.get("id", "")))
		skills.append({"slot": i, "name": str(ab.get("name", "")), "energy_cost": float(ab.get("energy_cost", 0)), "cooldown": float(ab.get("cooldown", 0)),
			"cooldown_left": float(ab.get("cooldown_left", 0)), "needs_target": bool(ab.get("needs_target", false))})
	var has_combat := ship.has("hull")
	var tpos: Variant = null
	for o in objects:
		if o["id"] == _target_id:
			tpos = o["pos"]
	var in_range := false
	if tpos != null and has_combat:
		in_range = (_vec(ship.get("pos")) as Vector2).distance_to(tpos) <= float(ship.get("pulse_range", 0.0))
	_state = {
		"mode": "server",
		"time": float(p.get("time", 0.0)),
		"player": {"name": player_name},
		"system": system,
		"ship": _ship_dict(ship),
		"objects": objects,
		"target_id": _target_id,
		"autopilot": _autopilot,
		"combat_available": has_combat,
		"skills": skills,
		"combat": {"in_combat": bool(p.get("attacking", false))},
		"attacking": bool(p.get("attacking", false)), "auto_skills": false, "target_in_range": in_range,
		"quests": [], "chat": [],
	}
	# кредиты и задания показываются, только если сервер их прислал (контракты включены флагом на сервере)
	if p.has("credits"):
		_state["credits"] = int(p["credits"])
	var quests: Array = []
	for q in p.get("quests", []):
		quests.append({"title": str(q.get("title", "")), "progress": int(q.get("progress", 0)), "goal": int(q.get("goal", 1))})
	_state["quests"] = quests
	state_updated.emit(_state)


# --- команды ---

func set_target(id: String) -> void:
	if _joined:
		_client.send_message("target", {"id": id})


func set_autopilot(on: bool) -> void:
	if _joined:
		_client.send_message("autopilot", {"on": on})


func move_input(v: Vector2) -> void:
	_move = v
	_move_dirty = not v.is_equal_approx(_sent_move)


func _send_move() -> void:
	_move_dirty = false
	_move_cd = MOVE_INTERVAL
	if _move.is_equal_approx(_sent_move):
		return
	_sent_move = _move
	_client.send_message("move", {"x": _move.x, "y": _move.y})


func attack(id: String) -> void:
	if not _joined:
		return
	if id != "" and id != _target_id:
		_client.send_message("target", {"id": id})
	_client.send_message("attack", {"on": true})


func stop_attack() -> void:
	if _joined:
		_client.send_message("attack", {"on": false})


func use_skill(slot: int) -> void:
	if _joined and slot >= 0 and slot < _ability_ids.size():
		_client.send_message("ability", {"id": _ability_ids[slot]})


## Выбор облика и специализации корабля (сохраняется сервером).
func set_skin(skin_id: String) -> void:
	if _joined:
		_client.send_message("set_skin", {"value": skin_id})


func set_spec(spec_id: String) -> void:
	if _joined:
		_client.send_message("set_spec", {"value": spec_id})


func set_auto_skills(_on: bool) -> void:
	command_failed.emit("combat.auto", "Авто-навыки появятся на следующем этапе")
