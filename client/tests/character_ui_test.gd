extends SceneTree
## Создание персонажа: форма (проверка ника, выбор пола и направления) и сквозной вход нового игрока (нужен сервер).

const CharacterScreen := preload("res://ui/character_screen.gd")
const ServerGateway := preload("res://ui/gateway/server_gateway.gd")
const PlayerIdentity := preload("res://ui/player_identity.gd")
const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVisual := preload("res://ui/ships/ship_visual.gd")

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


func _wait(cond: Callable, sec: float = 6.0) -> bool:
	var t0 := Time.get_ticks_msec()
	while Time.get_ticks_msec() - t0 < int(sec * 1000.0):
		if cond.call():
			return true
		await process_frame
	return false


func _run() -> void:
	# форма
	for bad in ["", "ab", "a b", "_ab", "ab_", "123456", "Привет!", "очень_длинный_ник_17"]:
		_c(CharacterScreen.nick_problem(bad) != "", "ник «%s» отклоняется формой" % bad)
	for ok in ["Vasya", "Вася_77", "Ace-9", "Ёжик"]:
		_c(CharacterScreen.nick_problem(ok) == "", "ник «%s» подходит" % ok)
	var options := [
		{"pilot": "interceptor", "ship_name": "Стриж", "temporary": false}, {"pilot": "scout", "ship_name": "Стриж", "temporary": false},
		{"pilot": "trader", "ship_name": "Грузовик «Тягач»", "temporary": true}, {"pilot": "miner", "ship_name": "Буровик «Крот»", "temporary": true}]
	var cs: Control = CharacterScreen.new()
	cs.options = options
	root.add_child(cs)
	await process_frame
	_c(cs._pilot_buttons.size() == 4, "четыре стартовых направления")
	_c((cs._pilot_buttons["trader"] as Button).text.contains("временная модель") and not (cs._pilot_buttons["interceptor"] as Button).text.contains("временная"), "временные модели помечены, «Стриж» нет")
	_c((cs._pilot_buttons["scout"] as Button).text.contains("Стриж"), "разведчик и перехватчик показывают один корабль (без копий)")
	_c(cs._start.disabled, "без выбора «Начать» недоступна")
	cs.set_nick("Vasya_Pro")
	cs._pick_gender("m")
	_c(cs._start.disabled, "без направления «Начать» недоступна")
	cs._pick_pilot("trader")
	_c(not cs._start.disabled and cs.ready_to_submit(), "после выбора всего «Начать» доступна")
	var got: Array = []
	cs.submitted.connect(func(n: String, g: String, p: String) -> void: got.append([n, g, p]))
	cs._start.pressed.emit()
	_c(got == [["Vasya_Pro", "m", "trader"]], "форма отправляет ник, пол и направление")
	cs.show_error("такой ник уже занят")
	_c(cs._error.text.contains("занят") and not cs._start.disabled, "ошибка сервера показана, кнопка снова доступна")
	cs.queue_free()

	# корабли каталога
	_c(ShipDef.is_temporary("TMP-HAUL") and ShipDef.is_temporary("TMP-MINE") and not ShipDef.is_temporary("FTR-001"), "временные модели помечены в описаниях")
	for id in ["TMP-HAUL", "TMP-MINE"]:
		var v: Node3D = ShipVisual.new()
		v.ship_id = id
		root.add_child(v)
		await process_frame
		_c(v.model != null and v.point("Engine_L") != null and v.skin_id == "a", "модель %s строится по своему описанию (GLB заменяется в ship.json)" % id)
		v.queue_free()
	var def_haul := ShipDef.get_def("TMP-HAUL")
	_c(str(def_haul["skins"]["a"]["glb"]) != "" and def_haul["visual_scale"] != ShipDef.get_def("FTR-001")["visual_scale"], "у временной модели свой путь GLB и масштаб")

	# сквозной вход нового игрока
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if url == "":
		print("COSMOC_TEST_WS_URL не задана: сквозная часть пропущена")
		print("Итого: проверок %d, провалов %d" % [_n, _f])
		quit(1 if _f > 0 else 0)
		return
	PlayerIdentity.PATH = "user://identity_test2.cfg"
	DirAccess.remove_absolute(ProjectSettings.globalize_path(PlayerIdentity.PATH))
	var tag := str(Time.get_ticks_usec()).right(8)
	var gw := ServerGateway.new()
	var opts: Array = []
	var fails: Array = []
	gw.character_needed.connect(func(o: Array) -> void: opts.append_array(o))
	gw.character_failed.connect(func(r: String) -> void: fails.append(r))
	root.add_child(gw)
	gw.connect_to_server(url)
	await _wait(func() -> bool: return opts.size() > 0)
	_c(opts.size() == 4, "сервер прислал 4 стартовых направления")
	gw.create_character("ab", "m", "scout")
	await _wait(func() -> bool: return fails.size() >= 1)
	_c(fails.size() == 1 and fails[0].contains("ник"), "сервер отклонил короткий ник: %s" % str(fails))
	gw.create_character("Tst_" + tag, "f", "trader")
	await _wait(func() -> bool: return not gw.snapshot().is_empty())
	var ship: Dictionary = gw.snapshot()["ship"]
	_c(gw.snapshot()["player"]["name"] == "Tst_" + tag, "ник применён: %s" % gw.snapshot()["player"]["name"])
	_c(ship.get("type") == "TMP-HAUL" and ship.get("cargo") == 60.0 and ship.get("hull_max") == 1800.0, "торговец получил временный грузовик (груз 60, корпус 1800)")
	_c(gw.snapshot()["skills"].size() == 1, "у грузовика только общий «Форсаж»: способности специализаций «Стрижа» не выданы")
	gw.close()
	await process_frame
	# повторный вход тем же устройством: персонаж уже есть
	var gw2 := ServerGateway.new()
	var again := [0]
	gw2.character_needed.connect(func(_o: Array) -> void: again[0] += 1)
	root.add_child(gw2)
	gw2.connect_to_server(url)
	await _wait(func() -> bool: return not gw2.snapshot().is_empty())
	_c(again[0] == 0 and gw2.snapshot()["player"]["name"] == "Tst_" + tag, "повторный вход без создания персонажа")
	gw2.close()
	DirAccess.remove_absolute(ProjectSettings.globalize_path(PlayerIdentity.PATH))
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
