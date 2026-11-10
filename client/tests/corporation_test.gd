extends SceneTree
## Корпорации: корреляция запросов, ошибки, timeout, подтверждения, профиль и меню.
const Gateway := preload("res://ui/gateway/server_gateway.gd")
const Screen := preload("res://ui/corporation_screen.gd")
const Profile := preload("res://ui/profile_screen.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")

class Client extends Node:
	var sent: Array = []
	func send_message(type: String, payload: Dictionary = {}) -> String:
		sent.append({"type": type, "payload": payload})
		return str(sent.size())

var failures := 0
var checks := 0

func _initialize() -> void:
	_run.call_deferred()

func check(ok: bool, text: String) -> void:
	checks += 1
	if not ok:
		failures += 1
	print("PASS  " if ok else "FAIL  ", text)

func _run() -> void:
	var client := Client.new()
	var gw := Gateway.new()
	root.add_child(gw)
	gw.add_child(client)
	gw.set_process(false)
	gw._client = client
	gw._joined = true
	var errors: Array = []
	var unrelated: Array = []
	gw.corporation_failed.connect(func(t: String) -> void: errors.append(t))
	gw.command_failed.connect(func(_a: String, t: String) -> void: unrelated.append(t))
	gw.corporation_info()
	gw.corporation_info()
	check(client.sent.size() == 1 and gw.corporation_busy(), "повторный запрос блокируется")
	gw._on_message("error", {"code": "no_corp", "message": "нет корпорации"}, "other")
	check(gw.corporation_busy() and not gw.corporation_snapshot().known, "чужой ответ не завершает запрос")
	gw._on_message("error", {"code": "no_corp"}, "1")
	check(gw.corporation_snapshot().known and errors.is_empty(), "no_corp при чтении — отсутствие членства")

	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	holder.disable_3d = true
	root.add_child(holder)
	var screen := Screen.new()
	screen.theme = UiTheme.build()
	holder.add_child(screen)
	screen.bind(gw)
	gw._on_message("error", {"code": "no_corp"}, "2")
	await process_frame
	check(screen._empty.visible and not screen._details.visible, "показаны формы создания и вступления")
	screen._join_input.text = "SL"
	screen._update_actions()
	screen._join.pressed.emit()
	check(client.sent[-1] == {"type": "corp_join", "payload": {"name": "SL"}} and screen._join.disabled, "вступление отправлено, кнопка блокируется")
	gw._on_message("error", {"code": "no_corp"}, "3")
	check(errors[-1].contains("не найдена") and not screen._join.disabled, "неизвестная корпорация — ошибка, форма остаётся доступной")

	screen._name_input.text = "Стальная Лиса"
	screen._tag_input.text = "sl"
	screen._update_actions()
	var before := client.sent.size()
	screen._create.pressed.emit()
	check(screen._confirm.is_open() and client.sent.size() == before, "создание требует подтверждения стоимости")
	screen._confirm._close(false)
	check(client.sent.size() == before, "отмена создания ничего не отправляет")
	screen._create.pressed.emit()
	screen._confirm._close(true)
	check(client.sent[-1] == {"type": "corp_create", "payload": {"name": "Стальная Лиса", "tag": "SL"}}, "подтверждённое создание: нормализация тега")
	gw._on_message("error", {"code": "no_credits", "message": "Недостаточно кредитов"}, "4")
	check(screen._notice.text == "Недостаточно кредитов" and unrelated.size() == 1, "отказ создания показан внутри экрана, не попадает в другие команды")

	var corp := {"id": "co-sl", "name": "Стальная Лиса", "tag": "SL", "treasury": 12345}
	screen._join.pressed.emit()
	gw._on_message("corp", {"corp": corp}, "5")
	check(client.sent[-1].type == "corp_info" and gw.corporation_busy(), "после вступления запрашивается полный состав")
	gw._on_message("corp", {"corp": corp, "members": [{"name": "Лиса", "role": "owner"}, {"name": "Волк", "role": "member"}]}, "6")
	check(screen._details.visible and not screen._empty.visible and screen._treasury.text.contains("12345"), "сведения и казна берутся из ответа сервера")
	check(screen._members.get_child_count() == 2 and screen._members.get_child(0).text.contains("Основатель"), "показаны состав и роли")
	var profile := Profile.new()
	holder.add_child(profile)
	profile.set_corporation(gw.corporation_snapshot())
	check(profile.find_child("CorporationValue", true, false).text == "[SL] Стальная Лиса", "тег и название в профиле")
	profile.queue_free()
	screen._leave.pressed.emit()
	screen._confirm._close(false)
	check(client.sent.size() == 6, "отмена выхода сохраняет членство")
	screen._leave.pressed.emit()
	screen._confirm._close(true)
	check(client.sent[-1].type == "corp_leave", "подтверждённый выход отправляется серверу")
	gw._on_message("corp_left", {"ok": true}, "7")
	check(screen._empty.visible and gw.corporation_snapshot().get("corp", {}).is_empty(), "членство очищается только после ответа")

	screen._join.pressed.emit()
	gw._process(11.0)
	check(not gw.corporation_busy() and not gw.corporation_snapshot().known and screen._join.disabled and not screen._refresh.disabled, "timeout скрывает недостоверное членство; доступно обновление")
	gw._on_message("corp", {"corp": corp}, "8")
	check(not gw.corporation_snapshot().known, "поздний ответ после timeout не восстанавливает устаревший кэш")
	screen._refresh.pressed.emit()
	gw._on_message("corp", {"corp": corp, "members": []}, "9")
	check(screen._details.visible, "обновление после timeout восстанавливает сведения")
	gw._on_link_lost("test")
	check(not gw.corporation_snapshot().known and not screen._empty.visible and not screen._details.visible, "потеря связи не выдаётся за отсутствие членства")
	gw.set_process(false)

	# Обрезка прокруткой при длинном составе на разных логических размерах.
	var members: Array = []
	for i in range(60):
		members.append({"name": "Длинное имя участника %d" % i, "role": "member"})
	for sz in [Vector2i(1280, 720), Vector2i(1600, 720), Vector2i(640, 360)]:
		holder.size = sz
		screen._on_data({"known": true, "corp": corp, "members": members})
		await process_frame
		await process_frame
		var scroll := screen.find_child("CorporationScroll", true, false) as ScrollContainer
		check(Rect2(Vector2.ZERO, Vector2(sz)).grow(1).encloses(scroll.get_global_rect()), "прокрутка состава помещается в %s" % sz)
		check(scroll.get_v_scroll_bar().max_value > scroll.size.y, "длинный состав прокручивается в %s" % sz)
	for sz in [Vector2i(1280, 720), Vector2i(1600, 720), Vector2i(640, 360)]:
		holder.size = sz
		screen._on_data({"known": true})
		screen._on_busy(false)
		await process_frame
		await process_frame
		check(screen._create.size.x <= sz.x - 40 and screen._create.size.y >= 48, "форма создания: ширина и зона касания в %s" % sz)
		screen._create.pressed.emit()
		await process_frame
		await process_frame
		var confirm_panel := screen._confirm._title.get_parent().get_parent() as Control
		check(Rect2(Vector2.ZERO, Vector2(sz)).grow(1).encloses(confirm_panel.get_global_rect()), "подтверждение помещается в %s" % sz)
		screen._confirm._close(false)
	# Меню: новый пункт связан с сигналом, длинное меню прокручивается.
	holder.size = Vector2i(1280, 720)
	var hud: Control = load("res://scenes/hud.tscn").instantiate()
	holder.add_child(hud)
	hud.theme = UiTheme.build()
	await process_frame
	var asked := [0]
	hud.corporation_requested.connect(func() -> void: asked[0] += 1)
	hud._set_menu_open(true)
	hud.get_node("%CorporationMenuButton").pressed.emit()
	check(asked[0] == 1 and not hud.is_menu_open(), "пункт меню открывает корпорацию и закрывает меню")
	hud._set_menu_open(true)
	await process_frame
	await process_frame
	var menu_scroll := hud.get_node("MenuPopup/Panel/Scroll") as ScrollContainer
	check(Rect2(Vector2.ZERO, Vector2(holder.size)).grow(1).encloses(menu_scroll.get_global_rect()), "добавленный пункт не вытесняет меню за экран")
	holder.queue_free()
	await process_frame
	check(not gw.corporation_updated.has_connections(), "закрытие экрана отключает обработчики")
	gw.queue_free()
	await process_frame
	print("Итого: проверок %d, провалов %d" % [checks, failures])
	quit(1 if failures else 0)
