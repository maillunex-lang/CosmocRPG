extends SceneTree
## Автобой (R-48): подходящие цели по режимам, выбор ближайшей, удержание текущей цели, пропуск отклонённых, радиус поиска.

const AutoBattle := preload("res://ui/auto_battle.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

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


func _o(id: String, kind: String, stance: String, x: float, dead := false) -> Dictionary:
	return {"id": id, "kind": kind, "stance": stance, "pos": Vector2(x, 0), "dead": dead}


func _state(objs: Array, target := "") -> Dictionary:
	return {"ship": {"pos": Vector2.ZERO, "pulse_range": 100.0}, "objects": objs, "target_id": target}


func _run() -> void:
	var E := AutoBattle.MODE_ENEMIES
	var P := AutoBattle.MODE_ENEMIES_PLAYERS
	var A := AutoBattle.MODE_ALL
	var enemy := _o("e", "hostile", "hostile", 50)
	var hostile_player := _o("hp", "player", "hostile", 50)
	var neutral_player := _o("np", "player", "neutral", 50)
	var friend := _o("fr", "player", "friendly", 50)
	var station := _o("st", "station", "friendly", 50)
	var deposit := _o("dp", "deposit", "neutral", 50)
	_c(AutoBattle.eligible(enemy, E) and AutoBattle.eligible(enemy, P) and AutoBattle.eligible(enemy, A), "враг подходит во всех режимах")
	_c(not AutoBattle.eligible(hostile_player, E) and AutoBattle.eligible(hostile_player, P) and AutoBattle.eligible(hostile_player, A), "вражеский игрок: не в «Враги», да в остальных")
	_c(not AutoBattle.eligible(neutral_player, E) and not AutoBattle.eligible(neutral_player, P) and AutoBattle.eligible(neutral_player, A), "нейтральный игрок: только «Все подряд»")
	_c(not AutoBattle.eligible(friend, A) and not AutoBattle.eligible(station, A) and not AutoBattle.eligible(deposit, A), "союзники, станции и месторождения не атакуются никогда")
	_c(not AutoBattle.eligible(_o("x", "hostile", "hostile", 1, true), A), "уничтоженная цель не подходит")
	_c(AutoBattle.eligible(_o("ai1", "ai", "hostile", 1), P), "AI-капитан во вражеском отношении подходит в «Враги и игроки»")

	_c(AutoBattle.pick(_state([enemy]), E, 0) == "e", "ближайший враг выбирается")
	_c(AutoBattle.pick(_state([_o("far", "hostile", "hostile", 90), _o("near", "hostile", "hostile", 30)]), E, 0) == "near", "из двух выбирается ближайший")
	_c(AutoBattle.pick(_state([_o("far", "hostile", "hostile", 400)]), E, 0) == "", "вне радиуса поиска цели нет")
	_c(AutoBattle.pick(_state([_o("edge", "hostile", "hostile", 249)]), E, 0) == "edge" and AutoBattle.pick(_state([_o("out", "hostile", "hostile", 251)]), E, 0) == "", "радиус поиска — 250 единиц")
	var big := _state([_o("edge", "hostile", "hostile", 390), _o("out", "hostile", "hostile", 410)])
	big["ship"]["pulse_range"] = 320.0
	_c(AutoBattle.pick(big, E, 0) == "edge", "у дальнобойного корабля радиус поиска — 1,25 дальности залпа")
	_c(AutoBattle.pick(_state([_o("near", "hostile", "hostile", 10), _o("cur", "hostile", "hostile", 80)], "cur"), E, 0) == "cur", "подходящая текущая цель сохраняется")
	_c(AutoBattle.pick(_state([_o("near", "hostile", "hostile", 10), station], "st"), E, 0) == "near", "неподходящая текущая цель заменяется")
	_c(AutoBattle.pick(_state([_o("a", "hostile", "hostile", 10), _o("b", "hostile", "hostile", 20)]), E, 1000, {"a": 5000}) == "b", "пропущенная цель обходится")
	_c(AutoBattle.pick(_state([_o("a", "hostile", "hostile", 10)]), E, 6000, {"a": 5000}) == "a", "пропуск истекает")
	_c(AutoBattle.pick({"ship": {"pos": Vector2.ZERO}, "objects": [enemy]}, E, 0) == "e", "без данных о залпе действует минимальный радиус поиска")
	_c(AutoBattle.pick({"objects": [enemy]}, E, 0) == "", "без корабля цель не выбирается")
	_c(AutoBattle.pick(_state([neutral_player]), E, 0) == "" and AutoBattle.pick(_state([neutral_player]), A, 0) == "np", "режим меняет набор целей")

	UiSettings.override(AutoBattle.SETTING_KEY, null)
	_c(AutoBattle.mode() == E or AutoBattle.mode() in AutoBattle.MODES, "режим по умолчанию допустим")
	UiSettings.override(AutoBattle.SETTING_KEY, "мусор")
	_c(AutoBattle.mode() == E, "неизвестный режим заменяется на «Враги»")
	UiSettings.override(AutoBattle.SETTING_KEY, A)
	_c(AutoBattle.mode() == A, "режим читается из настроек")
	UiSettings.override(AutoBattle.SETTING_KEY, null)
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
