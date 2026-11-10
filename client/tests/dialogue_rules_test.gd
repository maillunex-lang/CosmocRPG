extends SceneTree
## Кнопка разговора только у станции своей системы и только для задания «поговорить».

const Rules := preload("res://ui/dialogue_rules.gd")

var _f := 0
var _n := 0


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _run() -> void:
	var state := {
		"system": {"id": "meridian", "name": "Меридиан"},
		"ship": {"in_safe": true},
		"quests": [{"id": "meridian_voice", "kind": "talk", "system": "meridian", "title": "Голос диспетчера"}],
	}
	_c(Rules.talk_quest(state).get("id") == "meridian_voice", "у станции Меридиана можно говорить")
	state["ship"] = {"in_safe": false}
	_c(Rules.talk_quest(state).is_empty(), "вне станции кнопка скрыта")
	state["ship"] = {"in_safe": true}
	state["system"] = {"id": "aurora"}
	_c(Rules.talk_quest(state).is_empty(), "чужая система не открывает разговор")
	state["system"] = {"id": "meridian"}
	state["quests"] = [{"id": "storm_wreck", "kind": "visit", "system": "redstorm"}]
	_c(Rules.talk_quest(state).is_empty(), "осмотр обломков — не разговор")
	state["system"] = {"id": "redstorm"}
	state["quests"] = [{"id": "storm_dispatch", "kind": "talk", "system": "redstorm", "title": "Голос рубежа"}]
	_c(Rules.talk_quest(state).get("id") == "storm_dispatch", "у форпоста Шторма можно говорить")
	state["system"] = {"id": "meridian"}
	_c(Rules.talk_quest(state).is_empty(), "Кира не отвечает из Меридиана")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
