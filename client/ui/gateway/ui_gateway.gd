extends Node
## Шлюз данных интерфейса: единственный способ, которым экраны получают состояние мира и
## отправляют команды. Экраны не знают, откуда данные (сервер или демо) и ничего не считают сами:
## урон, дальность, энергию, награды и права определяет источник (в бою и экономике это сервер).
##
## Реализации: DemoGateway (демо без сервера, явно помечен), ServerGateway появится вместе с
## серверным миром (Этап 01). Контракты команд: docs/ui/ACTION_REGISTRY.md.

## Новое состояние мира (словарь, см. docs/ui/ACTION_REGISTRY.md, раздел «Состояние»).
signal state_updated(state: Dictionary)
## Боевое предупреждение поднято/снято (остаётся на экране, пока не снято).
signal alert_raised(id: String, text: String)
signal alert_cleared(id: String)
## Короткое уведомление.
signal notification(text: String)
## Команда отклонена (причина для игрока).
signal command_failed(action: String, reason: String)
## Боевое или визуальное событие (решение уже принято источником данных): {type, from, to, amount, radius}.
signal combat_event(event: Dictionary)


## Данные демонстрационные (не с сервера): интерфейс обязан показывать плашку «Демо».
func is_demo() -> bool:
	return false


func snapshot() -> Dictionary:
	return {}


func set_target(_id: String) -> void:
	_unsupported("space.target")


func attack(_id: String) -> void:
	_unsupported("combat.attack")


func stop_attack() -> void:
	_unsupported("combat.stop")


func use_skill(_slot: int) -> void:
	_unsupported("combat.skill")


func set_auto_skills(_on: bool) -> void:
	_unsupported("combat.auto")


func set_autopilot(_on: bool) -> void:
	_unsupported("space.autopilot")


## Автопилот в произвольную точку текущей системы.
func set_waypoint(_p: Vector2) -> void:
	_unsupported("space.waypoint")


## Ручное управление: вектор 0..1 (джойстик, клавиатура). Нулевой вектор останавливает.
func move_input(_v: Vector2) -> void:
	_unsupported("space.move")


## Добыча, продажа и магазин: есть только у серверного шлюза; в демо магазин и добыча недоступны.
signal chat_received(message: Dictionary)
signal chat_history_loaded(general: Array, dms: Array, settings: Dictionary)
signal friends_updated(list: Array)
signal chat_failed(text: String)
signal ships_updated(ships: Array)
signal shop_updated(data: Dictionary)
signal shop_failed(text: String)
## Пилот: опыт, уровень и очки характеристик (серверный режим): словарь состояния пилота.
signal pilot_updated(pilot: Dictionary)
signal system_changed(info: Dictionary)   # Э-1: гиперпереход завершён {system, layout, spawn...}
signal station_market(data: Dictionary)   # Э-2: цены станции {prices, repair_cost, upgrade_cost}
signal station_done(data: Dictionary)     # Э-2: операция станции выполнена {action, credits, spent...}
signal dialogue_opened(data: Dictionary)  # реплика NPC: говорящий, текст, ответы


func talk(_choice: String) -> void:
	pass


## Игровая настройка (например, автоответ на атаку). В демо без сервера ничего не делает.
func set_option(_name: String, _value: bool) -> void:
	pass


## Звёздная карта (Э-1): системы, точки прыжков текущей системы, команда прыжка.
func star_systems() -> Array:
	return []

func jump_gates() -> Array:
	return []

func gate_links() -> Array:
	return []

func jump(_to: String) -> void:
	pass

## Станция (Э-2): цены, ремонт, покупка, улучшение.
func request_market() -> void:
	pass

func repair() -> void:
	pass

func buy(_resource: String, _amount: float) -> void:
	pass

func ship_upgrade() -> void:
	pass

func has_chat() -> bool:
	return false


func chat_send(_channel: String, _to: String, _text: String) -> void:
	chat_failed.emit("Чат доступен только при игре на сервере")


func request_chat_history() -> void:
	pass


func has_shop() -> bool:
	return false


func has_pilot() -> bool:
	return false


func pilot_open() -> void:
	pass


func pilot_spend(_attr: String, _points: int) -> void:
	_unsupported("pilot.spend")


func mine(_deposit_id: String) -> void:
	_unsupported("space.mine")


func stop_mining() -> void:
	pass


func sell() -> void:
	_unsupported("space.sell")


func request_ships() -> void:
	pass


func set_ship(_ship_id: String) -> void:
	_unsupported("space.ship")


func shop_open() -> void:
	shop_failed.emit("Магазин доступен только при игре на сервере")


func shop_buy(_item: String) -> void:
	shop_open()


func shop_equip(_slot: String, _item: String) -> void:
	shop_open()


func _unsupported(action: String) -> void:
	command_failed.emit(action, "Сервер пока не поддерживает это действие")


## Корпорации: только существующие серверные команды К-1.
signal corporation_updated(data: Dictionary)
signal corporation_failed(text: String)
signal corporation_busy_changed(busy: bool)


func has_corporations() -> bool:
	return false


func corporation_snapshot() -> Dictionary:
	return {"known": false}


func corporation_busy() -> bool:
	return false


func corporation_info() -> void:
	pass


func corporation_create(_name: String, _tag: String) -> void:
	corporation_failed.emit("Корпорации доступны только при игре на сервере")


func corporation_join(_name: String) -> void:
	corporation_failed.emit("Корпорации доступны только при игре на сервере")


func corporation_leave() -> void:
	corporation_failed.emit("Корпорации доступны только при игре на сервере")
