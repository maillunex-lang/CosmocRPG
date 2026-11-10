extends "res://ui/gateway/ui_gateway.gd"
## Серверный шлюз (Этап 01): состояние мира приходит с сервера, команды уходят серверу. Клиент ничего
## не считает. Протокол: docs/design/STAGE01_ONLINE_SPACE.md. Бой и способности сервер пока не знает:
## эти поля в состоянии отсутствуют, интерфейс их не показывает (данных нет, значит не выдумываем).

const PurchaseIntent := preload("res://ui/gateway/purchase_intent.gd")

const WsClient := preload("res://scripts/net/ws_client.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const PlayerIdentity := preload("res://ui/player_identity.gd")
const AutoBattle := preload("res://ui/auto_battle.gd")

const PROTOCOL := 1
const MOVE_INTERVAL := 0.1         # не чаще 10 сообщений движения в секунду (лимит сервера 20/с)
const RETRY_DELAYS: Array[float] = [1.0, 2.0, 4.0, 8.0]
const ALERT_LINK := "link"

## Вход выполнен (получен welcome).
signal joined
## Нужен новый персонаж: сервер прислал стартовые корабли направлений пилота.
signal character_needed(options: Array)
## Создание персонажа отклонено (причина для игрока).
signal character_failed(reason: String)
## Вход не удался (причина для игрока).
signal join_failed(reason: String)
## Сервер применил настройки или сменил версию. reason — коротко, что изменилось.
signal server_updated(reason: String)

var url := ""
var player_name := ""
var player_id := ""
## Имя при создании игрока (по умолчанию сервер даёт «Пилот-NNNN»); нужно тестам, чтобы отличать своих игроков.
var join_name := ""
var system: Dictionary = {}
var systems: Array = []   # Э-1: все системы (звёздная карта)
var links: Array = []     # Э-1: рёбра гиперпереходов (карта галактики)
var gates: Array = []     # Э-1: точки прыжков текущей системы
var layout: Dictionary = {}   # раскладка системы с сервера (зоны, тела, станции, месторождения) и параметры декора
var visual: Dictionary = {}

var _purchase_intent := PurchaseIntent.new()
var _client: Node
var _state: Dictionary = {}
var _target_id := ""
var _autopilot := false
var _joined := false
var _pilot: Dictionary = {}        # состояние пилота от сервера: опыт, уровень, очки характеристик
var _auto_battle := false          # автобой: кнопка «Авто»
var _auto_skip: Dictionary = {}    # id цели -> до какого времени (мс) пропускать (сервер отклонил атаку)
var _auto_sent_id := ""            # цель, на которую автобой отправил атаку и ждёт подтверждения
var _auto_sent_ms := 0
var _auto_next_ms := 0
var _closing := false
var _retry := 0
var _retry_in := -1.0
var _move := Vector2.ZERO
var _sent_move := Vector2.ZERO
var _move_cd := 0.0
var _move_dirty := false
var _token := ""
var _server_version := ""
var _corp_data: Dictionary = {"known": false}
var _corp_request_id := ""
var _corp_action := ""
var _corp_expired_ids: Array[String] = []
var _corp_wait := 0.0
const CORP_TIMEOUT := 10.0

var _ability_ids: Array = []   # слот способности (кнопка HUD) -> идентификатор на сервере
var _locations: Array = []     # журнал прибытий этой сессии и с сервера


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
	_client.connected.connect(func(_s: String, v: String) -> void:
		if _server_version != "" and v != "" and v != _server_version:
			server_updated.emit("новая версия")
		if v != "":
			_server_version = v
		_join())
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
	if _corp_request_id != "":
		_corp_wait += delta
		if _corp_wait >= CORP_TIMEOUT:
			_corp_expired_ids.append(_corp_request_id)
			if _corp_expired_ids.size() > 16:
				_corp_expired_ids.pop_front()
			_corp_data = {"known": false}
			corporation_updated.emit(corporation_snapshot())
			_finish_corporation_request()
			corporation_failed.emit("Ответ не получен. Обновите сведения перед повторной операцией: предыдущая могла выполниться.")
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
	_corp_data = {"known": false}
	corporation_updated.emit(_corp_data)
	if _corp_request_id != "":
		_finish_corporation_request()
		corporation_failed.emit("Связь потеряна. После подключения обновите сведения о корпорации.")
	var was_joined := _joined
	_joined = false
	if not was_joined and _retry == 0 and _state.is_empty():
		join_failed.emit(reason)   # первое подключение не удалось: решает экран подключения
		return
	alert_raised.emit(ALERT_LINK, "Связь потеряна, переподключение…")
	_retry_in = RETRY_DELAYS[mini(_retry, RETRY_DELAYS.size() - 1)]
	_retry += 1


func _on_message(type: String, payload: Dictionary, _id: String) -> void:
	if _id in _corp_expired_ids:
		return
	if _corp_request_id != "" and _id == _corp_request_id and type in ["corp", "corp_left", "error"]:
		_on_corporation_reply(type, payload)
		return
	match type:
		"server_updated":
			server_updated.emit(str(payload.get("reason", "настройки")))
		"welcome":
			if payload.get("needs_character", false):
				player_id = str(payload.get("player_id", ""))
				character_needed.emit(payload.get("pilot_options", []))
				return
			_joined = true
			_retry = 0
			_retry_in = -1.0
			player_name = str(payload.get("name", ""))
			player_id = str(payload.get("player_id", ""))
			_purchase_intent.configure(url, player_id)
			_check_pending_purchase()
			_pilot = payload.get("pilot", {}) if payload.get("pilot") is Dictionary else {}
			system = payload.get("system", {})
			systems = payload.get("systems", []) if payload.get("systems") is Array else []
			gates = payload.get("gates", []) if payload.get("gates") is Array else []
			links = payload.get("links", []) if payload.get("links") is Array else []
			layout = payload.get("layout", {}) if payload.get("layout") is Dictionary else {}
			visual = payload.get("visual", {}) if payload.get("visual") is Dictionary else {}
			alert_cleared.emit(ALERT_LINK)
			_sent_move = Vector2.INF   # после входа заново сообщаем движение
			joined.emit()
			request_chat_history()
			corporation_info()
			set_option("auto_retaliate", UiSettings.get_bool("auto_retaliate", true))   # сохранённая настройка уходит серверу при каждом входе
		"market":
			station_market.emit(payload)
		"ok":
			if payload.get("action", "") == "buy":
				_purchase_intent.acknowledge(str(payload.get("operation_id", "")))
			if payload.get("action", "") != "":
				station_done.emit(payload)
		"purchase_status":
			if payload.get("found", true) == false:
				_purchase_intent.acknowledge(str(payload.get("operation_id", "")))
		"system_changed":
			system = payload.get("system", {}) if payload.get("system") is Dictionary else {}
			if payload.get("layout") is Dictionary:
				layout = payload["layout"]
			if payload.get("visual") is Dictionary:
				visual = payload["visual"]
			systems = payload.get("systems", systems) if payload.get("systems") is Array else systems
			gates = payload.get("gates", []) if payload.get("gates") is Array else []
			links = payload.get("links", links) if payload.get("links") is Array else links
			_target_id = ""
			_sent_move = Vector2.INF
			system_changed.emit(payload)
		"state":
			_apply_state(payload)
		"combat_events":
			for e in payload.get("events", []):
				if e is Dictionary:
					combat_event.emit(e)
		"quest_completed":
			var reward: Dictionary = payload.get("reward", {})
			notification.emit("Задание выполнено: %s. Награда: %d кредитов" % [str(payload.get("title", "")), int(reward.get("credits", 0))])
		"location_arrived":
			_remember_location(payload, true)
		"location_journal":
			for item in payload.get("visits", []):
				if item is Dictionary:
					_remember_location(item, false)
		"dialogue":
			dialogue_opened.emit(payload)
		"reward":
			var credits := int(payload.get("credits", 0))
			var what: String = {"kill": "Награда за победу", "discover": "Открытие", "sell": "Груз продан"}.get(str(payload.get("kind", "")), "Награда")
			notification.emit("%s: +%d кредитов" % [what, credits])
		"xp":
			if payload.get("pilot") is Dictionary:   # сообщение может нести только опыт корабля
				_set_pilot(payload["pilot"])
			if bool(payload.get("ship_level_up", false)) and payload.get("ship") is Dictionary:
				notification.emit("Корабль достиг уровня %d" % int((payload["ship"] as Dictionary).get("level", 1)))
			if bool(payload.get("level_up", false)):
				notification.emit("Новый уровень %d! Свободных очков характеристик: %d" % [int(_pilot.get("level", 1)), int(_pilot.get("attr_free", 0))])
			elif int(payload.get("gained", 0)) > 0:
				notification.emit("+%d опыта" % int(payload.get("gained", 0)))
		"pilot":
			_set_pilot(payload)
		"chat":
			if payload.get("message") is Dictionary:
				chat_received.emit(payload["message"])
		"chat_history":
			chat_history_loaded.emit(payload.get("general", []), payload.get("dms", []), payload.get("settings", {}))
		"ships":
			var fleet: Array = payload.get("ships", [])
			var levels: Dictionary = payload.get("levels", {}) if payload.get("levels") is Dictionary else {}
			for sh in fleet:   # уровень экземпляра корабля: нет записи — первый
				if sh is Dictionary and payload.has("levels"):
					sh["level"] = int((levels.get(str(sh.get("id", "")), {}) as Dictionary).get("level", 1))
			var passport: Dictionary = payload.get("passport", {}) if payload.get("passport") is Dictionary else {}
			for sh in fleet:
				if not (sh is Dictionary):
					continue
				var level_info: Dictionary = levels.get(str(sh.get("id", "")), {})
				var cap := int(payload.get("ship_max_level", level_info.get("max_level", 0)))
				if cap > 0:
					sh["max_level"] = cap
				if not passport.is_empty() and str(passport.get("id", "")) == str(sh.get("id", "")):
					sh["rpg_passport"] = passport
			ships_updated.emit(fleet)
		"shop":
			shop_updated.emit(payload)
		"error":
			_on_error(payload)


func _on_error(payload: Dictionary) -> void:
	var code := str(payload.get("code", ""))
	var text := str(payload.get("message", "ошибка"))
	if not str(payload.get("operation_id", "")).is_empty():
		if code in ["bad_message", "no_credits", "purchase_conflict", "hold_full", "not_at_station", "no_item", "dead", "equipment_busy"]:
			_purchase_intent.acknowledge(str(payload["operation_id"]))
		command_failed.emit("space", text)
		return
	if code in ["bad_nick", "nick_reserved", "nick_taken", "bad_gender", "bad_pilot_spec", "character_exists"]:
		character_failed.emit(text)
		return
	if code.begins_with("chat_"):
		chat_failed.emit(text)
		return
	if code in ["no_item", "already_owned", "vip_required", "not_owned", "bad_slot", "bad_request"]:
		shop_failed.emit(text)
		return
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
		"bad_target", "no_target", "cooldown", "no_energy", "out_of_range", "wrong_spec", "dead", "unknown_ability", "bad_skin", "bad_spec", \
				"equipment_unavailable", "equipment_revision_conflict", "equipment_busy", "equipment_in_combat", "equipment_unsupported_ship", "module_unavailable", "ability_not_learned", "no_module", "hold_full", "deposit_empty", "no_deposit", "not_at_station", "nothing_to_sell", "no_ship", "hold_too_big":
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
	for k in ["cargo_used", "mine_range", "pulse_range"]:
		d[k] = float(ship.get(k, 0.0))
	d["hold"] = ship.get("hold", {}) if ship.get("hold") is Dictionary else {}
	d["mining"] = str(ship.get("mining", ""))
	d["can_mine"] = bool(ship.get("can_mine", false))
	d["in_safe"] = bool(ship.get("in_safe", false))
	if ship.has("level"):
		d["level"] = int(ship["level"])
	if ship.get("equipment") is Dictionary:
		d["equipment"] = ship["equipment"]
	if ship.get("modules") is Array:
		d["modules"] = ship["modules"]
	d["cosmetics"] = ship.get("cosmetics", {}) if ship.get("cosmetics") is Dictionary else {}
	return d


func _vec(a: Variant) -> Vector2:
	if a is Array and a.size() >= 2:
		return Vector2(float(a[0]), float(a[1]))
	return Vector2.ZERO


## Игрок для HUD: уровень и опыт только если сервер их прислал (ничего не выдумываем).
func _player_dict() -> Dictionary:
	var d := {"name": player_name}
	if not _pilot.is_empty():
		d["level"] = int(_pilot.get("level", 1))
		d["xp_into"] = int(_pilot.get("into_level", 0))
		d["xp_to_next"] = int(_pilot.get("to_next", 0))
		d["xp_level"] = int(_pilot.get("level_xp", 0))
		d["attr_free"] = int(_pilot.get("attr_free", 0))
	return d


func _set_pilot(p: Dictionary) -> void:
	_pilot = p
	if not _state.is_empty():
		_state["player"] = _player_dict()
		_state["pilot"] = _pilot
		state_updated.emit(_state)
	pilot_updated.emit(_pilot)


func has_pilot() -> bool:
	return not _pilot.is_empty()


func pilot_open() -> void:
	if _joined:
		_client.send_message("pilot", {})


func pilot_spend(attr: String, points: int) -> void:
	if _joined:
		_client.send_message("pilot_spend", {"attr": attr, "points": points})


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
		for k in ["tier", "resource", "rarity", "zone", "badge", "name_color", "engine"]:
			if o.has(k):
				obj[k] = str(o[k])
		for k in ["amount", "amount_max", "vip"]:
			if o.has(k):
				obj[k] = float(o[k])
		objects.append(obj)
	_target_id = str(p.get("target_id", ""))
	_autopilot = bool(p.get("autopilot", false))
	var abilities: Array = ship.get("abilities", [])
	var skills: Array = []
	_ability_ids.clear()
	for i in abilities.size():
		var ab: Dictionary = abilities[i]
		var catalog_id := str(ab.get("catalog_id", ""))
		_ability_ids.append(catalog_id if catalog_id != "" else str(ab.get("id", "")))
		skills.append({"slot": i, "name": str(ab.get("name", "")), "energy_cost": float(ab.get("energy_cost", 0)), "cooldown": float(ab.get("cooldown", 0)),
			"cooldown_left": float(ab.get("cooldown_left", 0)), "needs_target": bool(ab.get("needs_target", false)),
			"enabled": bool(ab.get("enabled", true)), "unavailable_reason": str(ab.get("unavailable_reason", ""))})
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
		"player": _player_dict(),
		"pilot": _pilot,
		"system": system,
		"ship": _ship_dict(ship),
		"objects": objects,
		"target_id": _target_id,
		"autopilot": _autopilot,
		"combat_available": has_combat,
		"skills": skills,
		"combat": {"in_combat": bool(p.get("attacking", false))},
		"attacking": bool(p.get("attacking", false)), "auto_skills": _auto_battle, "target_in_range": in_range,
		"quests": [], "chat": [],
	}
	# кредиты и задания показываются, только если сервер их прислал (контракты включены флагом на сервере)
	if p.has("credits"):
		_state["credits"] = int(p["credits"])
	var quests: Array = []
	for q in p.get("quests", []):
		quests.append({"id": str(q.get("id", "")), "title": str(q.get("title", "")), "text": str(q.get("text", "")),
			"marker": str(q.get("marker", "")), "system": str(q.get("system", "")), "kind": str(q.get("kind", "")),
			"target": str(q.get("target", "")),
			"progress": int(q.get("progress", 0)), "goal": int(q.get("goal", 1))})
	_state["quests"] = quests
	_state["locations"] = _locations.duplicate(true)
	_auto_battle_tick()
	state_updated.emit(_state)


func _remember_location(payload: Dictionary, announce: bool) -> void:
	var id := str(payload.get("id", ""))
	var system_id := str(payload.get("system", ""))
	if id == "":
		return
	for loc in _locations:
		if str(loc.get("id", "")) == id and str(loc.get("system", "")) == system_id:
			return
	_locations.append({"id": id, "name": str(payload.get("name", id)), "kind": str(payload.get("kind", "")), "system": system_id})
	if announce:
		notification.emit("Прибыли: %s" % str(payload.get("name", id)))
	if not _state.is_empty():
		_state["locations"] = _locations.duplicate(true)
		state_updated.emit(_state)


# --- команды ---

func set_target(id: String) -> void:
	if _joined:
		_client.send_message("target", {"id": id})


func set_autopilot(on: bool) -> void:
	if _joined:
		_client.send_message("autopilot", {"on": on})


func set_waypoint(p: Vector2) -> void:
	if _joined:
		_client.send_message("waypoint", {"x": p.x, "y": p.y})


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
	_auto_battle = false   # ручная остановка атаки выключает автобой: иначе он тут же начал бы снова
	if _joined:
		_client.send_message("attack", {"on": false})


func use_skill(slot: int) -> void:
	if _joined and slot >= 0 and slot < _ability_ids.size():
		_client.send_message("ability", {"id": _ability_ids[slot]})


## Создание персонажа: ник, пол ("m"/"f"), направление пилота (interceptor, scout, trader, miner).
func create_character(nick: String, gender: String, pilot: String) -> void:
	_client.send_message("create_character", {"nick": nick, "gender": gender, "pilot_spec": pilot})


## Выбор облика и специализации корабля (сохраняется сервером).
func set_skin(skin_id: String) -> void:
	if _joined:
		_client.send_message("set_skin", {"value": skin_id})


func set_spec(spec_id: String) -> void:
	if _joined:
		_client.send_message("set_spec", {"value": spec_id})


func star_systems() -> Array:
	return systems

func jump_gates() -> Array:
	return gates

func gate_links() -> Array:
	return links

func jump(to: String) -> void:
	_client.send_message("jump", {"to": to})

func talk(choice: String) -> void:
	_client.send_message("talk", {"choice": choice})

func request_market() -> void:
	_client.send_message("market", {})

func repair() -> void:
	_client.send_message("repair", {})

func buy(resource: String, amount: float) -> void:
	if not _joined or _client == null or not _purchase_intent.configure(url, player_id):
		command_failed.emit("space", "Не удалось подготовить покупку. Проверьте соединение.")
		return
	var request: Dictionary = _purchase_intent.prepare(resource, amount)
	if request.is_empty():
		_check_pending_purchase()
		command_failed.emit("space", "Сначала дождитесь результата предыдущей покупки.")
		return
	_client.send_message("buy", request)

func _check_pending_purchase() -> void:
	var pending: Dictionary = _purchase_intent.pending()
	if not pending.is_empty() and _client != null:
		_client.send_message("purchase_status", pending)

func ship_upgrade() -> void:
	_client.send_message("ship_upgrade", {})

func has_chat() -> bool:
	return true


func chat_send(channel: String, to: String, text: String) -> void:
	if _joined and _client != null:
		_client.send_message("chat_send", {"channel": channel, "to": to, "text": text})
	else:
		chat_failed.emit("Нет соединения с сервером")


func request_chat_history() -> void:
	if _joined:
		_client.send_message("chat_history", {})


func has_shop() -> bool:
	return true


## Игровая настройка на сервере (сейчас: auto_retaliate).
func set_option(opt: String, value: bool) -> void:
	if _joined:
		_client.send_message("set_option", {"name": opt, "value": value})


func request_ships() -> void:
	if _joined:
		_client.send_message("ships", {})


## Смена корабля у станции (сервер проверяет место, груз и наличие корабля).
func set_ship(ship_id: String) -> void:
	if _joined:
		_client.send_message("set_ship", {"ship": ship_id})


## Добыча из месторождения: цель выбирается и отправляется в одном действии.
func mine(deposit_id: String) -> void:
	if _joined:
		_client.send_message("mine", {"id": deposit_id})


func stop_mining() -> void:
	if _joined:
		_client.send_message("mine_stop", {})


func sell() -> void:
	if _joined:
		_client.send_message("sell", {})


func shop_open() -> void:
	if _joined:
		_client.send_message("shop", {})


## Покупка идемпотентна по request_id: повторная отправка того же запроса безопасна.
func shop_buy(item: String) -> void:
	if _joined:
		_client.send_message("shop_buy", {"item": item, "request_id": "%d-%d" % [Time.get_ticks_usec(), randi()]})


func shop_equip(slot: String, item: String) -> void:
	if _joined:
		_client.send_message("shop_equip", {"slot": slot, "item": item})


## Кнопка «Авто»: включает автобой (выбор целей и атака по настройке «Кого атаковать»).
func set_auto_skills(on: bool) -> void:
	_auto_battle = on
	_auto_sent_id = ""
	_auto_skip.clear()
	_auto_next_ms = 0
	if not _state.is_empty():
		_state["auto_skills"] = on
		state_updated.emit(_state)


## Шаг автобоя на каждом снимке (не чаще двух раз в секунду): атаковать подходящую цель, пока не идёт атака или добыча.
func _auto_battle_tick() -> void:
	if not _auto_battle or not _joined:
		return
	var now := Time.get_ticks_msec()
	var ship: Dictionary = _state.get("ship", {})
	if bool(_state.get("attacking", false)):
		_auto_sent_id = ""
		return
	if bool(ship.get("dead", false)) or str(ship.get("mining", "")) != "" or now < _auto_next_ms:
		return
	if _auto_sent_id != "" and now - _auto_sent_ms > 1500:
		_auto_skip[_auto_sent_id] = now + 15000   # атака не началась (например, бой с игроками выключен): не настаиваем
		_auto_sent_id = ""
	var id := AutoBattle.pick(_state, AutoBattle.mode(), now, _auto_skip)
	_auto_next_ms = now + 500
	if id == "":
		return
	if id != _auto_sent_id:
		_auto_sent_id = id
		_auto_sent_ms = now
	attack(id)


func set_starter_engine(equipped: bool, revision: int) -> void:
	if _joined and _client != null:
		_client.send_message("equipment_set", {"equipped": equipped, "revision": revision})


func refresh_equipment() -> void:
	if _joined and _client != null:
		_client.send_message("equipment_get", {})


func has_corporations() -> bool:
	return true


func corporation_snapshot() -> Dictionary:
	return _corp_data.duplicate(true)


func corporation_busy() -> bool:
	return _corp_request_id != ""


func corporation_info() -> void:
	_send_corporation("corp_info", {})


func corporation_create(corp_name: String, tag: String) -> void:
	_send_corporation("corp_create", {"name": corp_name.strip_edges(), "tag": tag.strip_edges().to_upper()})


func corporation_join(corp_name: String) -> void:
	_send_corporation("corp_join", {"name": corp_name.strip_edges()})


func corporation_leave() -> void:
	_send_corporation("corp_leave", {})


func _send_corporation(action: String, payload: Dictionary) -> void:
	if corporation_busy():
		return
	if not _joined or _client == null:
		corporation_failed.emit("Нет соединения с сервером")
		return
	_corp_action = action
	_corp_wait = 0.0
	_corp_request_id = _client.send_message(action, payload)
	if _corp_request_id == "":
		corporation_failed.emit("Не удалось отправить запрос. Проверьте соединение")
		return
	corporation_busy_changed.emit(true)


func _finish_corporation_request() -> void:
	_corp_request_id = ""
	_corp_action = ""
	_corp_wait = 0.0
	corporation_busy_changed.emit(false)


func _on_corporation_reply(type: String, payload: Dictionary) -> void:
	var action := _corp_action
	_finish_corporation_request()
	if type == "error":
		if action == "corp_info" and str(payload.get("code", "")) == "no_corp":
			_corp_data = {"known": true}
			corporation_updated.emit(corporation_snapshot())
		else:
			var text := str(payload.get("message", "Операция отклонена"))
			if action == "corp_join" and str(payload.get("code", "")) == "no_corp":
				text = "Корпорация не найдена. Проверьте имя или тег"
			corporation_failed.emit(text)
		return
	if type == "corp_left":
		_corp_data = {"known": true}
	else:
		_corp_data = payload.duplicate(true)
		_corp_data["known"] = true
	corporation_updated.emit(corporation_snapshot())
	# Ответ вступления не содержит состав; после создания тоже читаем актуальные имена.
	if action in ["corp_join", "corp_create"]:
		corporation_info()
