extends SceneTree
## Пилот в интерфейсе (RPG-1): шлюз принимает опыт от сервера, HUD показывает уровень и полосу опыта, профиль — очки характеристик.

const ServerGateway := preload("res://ui/gateway/server_gateway.gd")
const ProfileScreen := preload("res://ui/profile_screen.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const Session := preload("res://ui/session.gd")

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


func _pilot(level := 3, into := 40, need := 120, free := 4, attrs := {}) -> Dictionary:
	var a := {"piloting": 0, "tactics": 0, "engineering": 0, "energetics": 0, "analytics": 0, "logistics": 0}
	a.merge(attrs, true)
	return {"level": level, "max_level": 60, "xp": 500, "into_level": into, "to_next": need - into, "level_xp": need, "attr_free": free, "attr_spent": 0, "talent_points": level - 1, "attributes": a}


func _run() -> void:
	# --- шлюз ---
	var gw := ServerGateway.new()
	root.add_child(gw)
	gw.player_name = "Пилот"
	var got: Array = []
	gw.state_updated.connect(func(s: Dictionary) -> void: got.append(s))
	var pil: Array = []
	gw.pilot_updated.connect(func(p: Dictionary) -> void: pil.append(p))
	var notes: Array = []
	gw.notification.connect(func(t: String) -> void: notes.append(t))
	gw._apply_state({"time": 1.0, "ship": {"name": "П", "pos": [0, 0], "dir": [1, 0], "speed": 0.0}, "objects": []})
	var s0: Dictionary = got[got.size() - 1]
	_c(not s0["player"].has("level") and not gw.has_pilot(), "без данных пилота уровень не выдумывается")
	gw._on_message("xp", {"gained": 12, "source": "sell", "level_up": false, "pilot": _pilot()}, "")
	var s1: Dictionary = got[got.size() - 1]
	_c(gw.has_pilot() and s1["player"]["level"] == 3 and s1["player"]["xp_into"] == 40 and s1["player"]["xp_level"] == 120 and s1["player"]["attr_free"] == 4, "опыт от сервера попал в данные игрока")
	_c(pil.size() == 1 and notes.size() == 1 and notes[0] == "+12 опыта", "уведомление о полученном опыте")
	gw._on_message("xp", {"gained": 50, "source": "kill", "level_up": true, "pilot": _pilot(4, 0, 150, 6)}, "")
	_c(notes.size() == 2 and notes[1].contains("Новый уровень 4") and notes[1].contains("6"), "уведомление о новом уровне и очках")
	var cnt := notes.size()
	gw._on_message("xp", {"source": "kill", "ship": {"ship": "FTR-001", "level": 7}, "ship_level_up": true}, "")
	_c(gw.has_pilot() and notes.size() == cnt + 1 and notes[cnt] == "Корабль достиг уровня 7", "опыт только корабля: пилот не затёрт, уведомление об уровне корабля")
	gw._apply_state({"time": 2.0, "ship": {"name": "К", "pos": [0, 0], "dir": [1, 0], "speed": 0.0, "level": 4}, "objects": []})
	_c(got[got.size() - 1]["ship"]["level"] == 4, "уровень корабля из состояния сервера")
	var fl: Array = [{"id": "FTR-001", "name": "Стриж"}, {"id": "TMP-MINE", "name": "Шахтёр"}]
	var seen: Array = []
	gw.ships_updated.connect(func(f: Array) -> void: seen.assign(f))
	gw._on_message("ships", {"ships": fl, "levels": {"FTR-001": {"level": 9}}}, "")
	_c(seen[0]["level"] == 9 and seen[1]["level"] == 1, "ангар: уровень корабля от сервера, без записи — первый")
	gw._on_message("pilot", _pilot(4, 0, 150, 5, {"piloting": 1}), "")
	_c(got[got.size() - 1]["pilot"]["attributes"]["piloting"] == 1 and got[got.size() - 1]["player"]["attr_free"] == 5, "ответ pilot обновляет состояние")
	gw.queue_free()

	# --- HUD ---
	var dg := DemoGateway.new()
	dg.auto_tick = false
	Session.gateway = dg
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	holder.disable_3d = true
	root.add_child(holder)
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	holder.add_child(screen)
	for i in 3:
		await process_frame
	var hud: Control = screen.get_node("%Hud")
	var bar := hud.find_child("XpBar", true, false) as ProgressBar
	var base: Dictionary = dg.snapshot().duplicate(true)
	hud.apply_state(base)
	_c(bar != null and not bar.visible, "в демо без опыта полоса скрыта")
	var st := base.duplicate(true)
	st["player"] = {"name": "Пилот", "level": 3, "xp_into": 40, "xp_to_next": 80, "xp_level": 120, "attr_free": 4}
	hud.apply_state(st)
	_c(bar.visible and absf(bar.value - 40.0 / 120.0) < 0.001, "полоса опыта показывает долю уровня")
	_c((hud.get_node("%PlayerLevel") as Label).text == "Ур. 3 · +4", "уровень и свободные очки в карточке пилота")
	st["player"] = {"name": "Пилот", "level": 60, "xp_into": 0, "xp_to_next": 0, "xp_level": 0, "attr_free": 0}
	st["ship"]["level"] = 6
	hud.apply_state(st)
	_c((hud.get_node("%ShipName") as Label).text.ends_with(" · ур. 6"), "уровень корабля в карточке пилота")
	st["player"] = {"name": "Пилот", "level": 60, "xp_into": 0, "xp_to_next": 0, "xp_level": 0, "attr_free": 0}
	hud.apply_state(st)
	_c(bar.value == 1.0 and (hud.get_node("%PlayerLevel") as Label).text == "Ур. 60", "максимальный уровень: полная полоса, без очков")
	holder.queue_free()
	Session.gateway = null

	# --- профиль ---
	var p: Control = ProfileScreen.new()
	root.add_child(p)
	await process_frame
	var dev := p.find_child("Development", true, false) as Control
	_c(dev != null and not dev.visible, "без данных пилота раздел развития скрыт")
	var req: Array = []
	p.attribute_requested.connect(func(a: String) -> void: req.append(a))
	p.set_pilot(_pilot(3, 40, 120, 2, {"tactics": 5}))
	await process_frame
	_c(dev.visible and (p.find_child("DevLevel", true, false) as Label).text == "Уровень 3 из 60", "раздел развития показан")
	_c((p.find_child("DevFree", true, false) as Label).text == "Свободно очков: 2", "свободные очки показаны")
	var plus := p.find_child("Plus_tactics", true, false) as Button
	_c(not plus.disabled, "кнопка «+» доступна при свободных очках")
	plus.pressed.emit()
	_c(req == ["tactics"], "нажатие «+» запрашивает характеристику у сервера")
	p.set_pilot(_pilot(3, 40, 120, 0))
	await process_frame
	_c((p.find_child("Plus_tactics", true, false) as Button).disabled and (p.find_child("DevFree", true, false) as Label).text == "", "без очков кнопки отключены")
	p.set_data({"player": {"name": "Пилот"}, "credits": 0, "ship": {"name": "К", "cosmetics": {}}, "pilot": _pilot(2, 10, 100, 1)})
	await process_frame
	_c(dev.visible and (p.find_child("DevFree", true, false) as Label).text == "Свободно очков: 1", "профиль берёт пилота из снимка состояния")

	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
