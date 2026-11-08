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


## Ручное управление: вектор 0..1 (джойстик, клавиатура). Нулевой вектор останавливает.
func move_input(_v: Vector2) -> void:
	_unsupported("space.move")


## Добыча, продажа и магазин: есть только у серверного шлюза; в демо магазин и добыча недоступны.
signal ships_updated(ships: Array)
signal shop_updated(data: Dictionary)
signal shop_failed(text: String)


## Игровая настройка (например, автоответ на атаку). В демо без сервера ничего не делает.
func set_option(_name: String, _value: bool) -> void:
	pass


func has_shop() -> bool:
	return false


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
