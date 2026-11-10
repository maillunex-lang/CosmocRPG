extends SceneTree
## Новый капитан видит вступление только пока «Первый полёт» не начат.

const Briefing := preload("res://ui/briefing.gd")

var _failures := 0


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _run() -> void:
	var fresh: Array = [{"id": "first_flight", "progress": 0}]
	var begun: Array = [{"id": "first_flight", "progress": 1}]
	var other: Array = [{"id": "first_flight", "progress": 0}, {"id": "first_ore", "progress": 2}]
	_check(Briefing.should_offer(fresh, false), "новый капитан получает вступление")
	_check(not Briefing.should_offer(fresh, true), "пропущенное вступление не возвращается")
	_check(not Briefing.should_offer(begun, false), "уже летевший не слушает вступление заново")
	_check(not Briefing.should_offer(other, false), "продвижение по другим контрактам закрывает вступление")
	_check(not Briefing.should_offer([], false), "без контракта молчим, пока сервер не пришлёт задание")
	print("briefing: ", "FAIL" if _failures else "OK")
	quit(_failures)
