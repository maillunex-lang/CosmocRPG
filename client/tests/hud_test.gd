extends SceneTree
## Тест экрана «Космос» и HUD на демо-шлюзе: состояние, цель, атака с подтверждением, навыки,
## сворачивание панелей в бою, тревоги, «Скоро», мир, выбор касанием, джойстик, раскладка.
## Запуск: godot --headless --path client --script res://tests/hud_test.gd

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const SpaceScreen := preload("res://scripts/space/space_screen.gd")
const ShipView := preload("res://scripts/space/ship_view.gd")
const Joystick := preload("res://ui/components/virtual_joystick.gd")

var _failures := 0
var _checks := 0


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


## Создаёт экран космоса в SubViewport заданного логического размера.
func _make(size: Vector2i = Vector2i(1280, 720), slow_server: bool = false) -> Dictionary:
	var gw = DemoGateway.new()
	gw.auto_tick = false
	gw.publish_on_command = not slow_server  # slow_server: состояние приходит только по тику (задержка сервера)
	Session.gateway = gw
	var holder := SubViewport.new()
	holder.size = size
	holder.disable_3d = true
	root.add_child(holder)
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	holder.add_child(screen)
	for i in 3:
		await process_frame
	var hud: Control = screen.get_node("%Hud")
	return {"holder": holder, "screen": screen, "hud": hud, "gw": gw}


func _free(ctx: Dictionary) -> void:
	ctx["holder"].queue_free()
	Session.gateway = null


func _n(ctx: Dictionary, path: String) -> Node:
	return (ctx["hud"] as Control).get_node(path)


func _tick(ctx: Dictionary, seconds: float, step: float = 0.1) -> void:
	var t := 0.0
	while t < seconds:
		ctx["gw"].tick(step)
		t += step
	await process_frame


func _select(ctx: Dictionary, id: String) -> void:
	var pts: Dictionary = ctx["screen"].screen_points()
	ctx["screen"].select_at(pts[id])
	await process_frame


func _run() -> void:
	UiSettings.override("reduce_motion", true)  # без анимаций: проверки не зависят от времени

	# ============ начальное состояние и плашка «Демо» ============
	var c = await _make()
	var hud: Control = c["hud"]
	var gw = c["gw"]
	_check(_n(c, "%DemoBar").visible, "демо: плашка «Демо» показана")
	_check(Session.gateway == gw and gw.is_demo(), "шлюз помечен как демо")
	_check((_n(c, "%PlayerName") as Label).text == "Капитан Демо" and (_n(c, "%PlayerLevel") as Label).text == "Ур. 7", "профиль: имя и уровень")
	_check((_n(c, "%ShipName") as Label).text == "Фрегат «Заря»", "название корабля")
	var hull := _n(c, "%HullBar") as ProgressBar
	_check(hull.value == 820.0 and hull.max_value == 1000.0, "полоска корпуса: 820 / 1000")
	_check((_n(c, "%ShieldBar") as ProgressBar).value == 300.0 and (_n(c, "%EnergyBar") as ProgressBar).max_value == 100.0, "полоски щитов и энергии")
	_check(((_n(c, "%Credits") as HBoxContainer).get_child(1) as Label).text == "Кредиты  125 400", "кредиты с разделителями тысяч")
	_check((_n(c, "%TargetName") as Label).text == "Цель не выбрана" and not (_n(c, "%TargetHull") as Control).visible, "цель не выбрана")
	var attack := _n(c, "%AttackButton") as ActionButton
	_check(attack.disabled and attack.text == "Атаковать", "атака без цели недоступна")
	_check((_n(c, "%SpeedLabel") as Label).text.begins_with("0 м/с") and (_n(c, "%SpeedLabel") as Label).text.contains("Ручное"), "скорость: стоит на месте, ручное управление (%s)" % (_n(c, "%SpeedLabel") as Label).text)
	_check(not (_n(c, "%HullBar") as ProgressBar).is_low() and (_n(c, "%HullBar") as ProgressBar).value > 250.0, "корпус в норме не предупреждает")
	(_n(c, "%HullBar") as ProgressBar).set_values(200.0, 1000.0)
	_check((_n(c, "%HullBar") as ProgressBar).is_low(), "критический корпус (20%) переходит в предупреждающий цвет")
	(_n(c, "%HullBar") as ProgressBar).set_values(820.0, 1000.0)
	var skills: Array = (_n(c, "%SkillsGrid") as Container).get_children()
	_check(skills.size() == 4, "четыре способности")
	_check(skills[0].disabled and not skills[1].disabled and not skills[2].disabled and not skills[3].disabled, "залп без цели недоступен, остальные доступны")
	_check((_n(c, "%AutopilotButton") as Button).disabled, "автопилот без цели недоступен")

	# ============ «Скоро»: нереализованное не притворяется рабочим ============
	for nm in ["%MapButton", "%FleetButton", "%MenuButton", "%SettingsButton"]:
		var b := _n(c, nm) as ActionButton
		_check(b.disabled and b.text.ends_with("скоро") and b.tooltip_text != "", "«Скоро»: %s неактивна и помечена" % nm)
	_check((_n(c, "%MapButton") as ActionButton).text.count("скоро") == 1, "«Скоро» не дублируется в тексте кнопки")

	# ============ мир: корабли, цвета, ориентация ============
	var screen: Control = c["screen"]
	_check((screen.get_node("%Ships") as Node3D).get_child_count() == 5, "в мире 5 объектов: свой корабль и четыре чужих")
	_check(screen.ship_view("me") != null and screen.ship_view("pi1") != null, "виды корабля, пирата созданы")
	var sv: Node3D = screen.ship_view("pi1")
	sv.set_world_position(Vector2(7, -3))
	_check(sv.position.is_equal_approx(Vector3(7, 0, -3)), "мировая позиция (x, y) -> 3D (x, 0, y)")
	sv.set_direction(Vector2(1, 0))
	_check((sv.global_transform.basis * Vector3(0, 0, -1)).normalized().is_equal_approx(Vector3(1, 0, 0)), "нос корабля смотрит по направлению движения (+X)")
	sv.set_direction(Vector2(0, -1))
	_check((sv.global_transform.basis * Vector3(0, 0, -1)).normalized().is_equal_approx(Vector3(0, 0, -1)), "нос корабля смотрит по направлению движения (-Y мира)")
	_check(ShipView.color_for("ship", "friendly", true) == Color("#42C6E5") and ShipView.color_for("ship", "hostile", false) == Color("#F16C79"), "цвета: свой акцентный, враг красный")

	# выбор касанием
	var pts: Dictionary = screen.screen_points()
	_check(pts.size() == 4 and not pts.has("me"), "точки выбора: четыре чужих объекта")
	_check(SpaceScreen.nearest_id({"a": Vector2(10, 10), "b": Vector2(100, 100)}, Vector2(14, 12), 50.0) == "a", "nearest_id: ближайший в радиусе")
	_check(SpaceScreen.nearest_id({"a": Vector2(10, 10)}, Vector2(300, 300), 50.0) == "", "nearest_id: вне радиуса ничего")
	_check(SpaceScreen.nearest_id({"a": Vector2(10, 10), "b": Vector2(20, 10)}, Vector2(18, 10), 50.0) == "b", "nearest_id: из двух выбирается ближайший")
	await _select(c, "pi1")
	_check(gw.snapshot()["target_id"] == "pi1", "касание по пирату выбирает цель")
	_check((screen.ship_view("pi1") as Node3D).is_selected() and not (screen.ship_view("pl1") as Node3D).is_selected(), "кольцо выбора только у цели")
	_check((_n(c, "%TargetName") as Label).text == "Пират", "карточка цели: имя")
	var info := (_n(c, "%TargetInfo") as Label).text
	_check(info.contains("Враг") and info.contains("враждебный") and info.contains("км") and info.contains("в зоне огня"), "карточка цели: тип, отношение, расстояние, дальность (%s)" % info)
	_check((_n(c, "%TargetHull") as Control).visible and (_n(c, "%TargetHull") as ProgressBar).value == 400.0, "полоска корпуса цели")
	_check(not attack.disabled and not skills[0].disabled and not (_n(c, "%AutopilotButton") as Button).disabled, "с целью доступны атака, залп и автопилот")
	screen.select_at(Vector2(2, 2))
	await process_frame
	_check(gw.snapshot()["target_id"] == "", "касание пустого места снимает цель")

	# ============ атака враждебной цели: без подтверждения, защита от повторов ============
	_free(c)
	c = await _make(Vector2i(1280, 720), true)  # медленный сервер: ответ придёт только на тике
	hud = c["hud"]
	gw = c["gw"]
	attack = _n(c, "%AttackButton") as ActionButton
	skills = (_n(c, "%SkillsGrid") as Container).get_children()
	await _select(c, "pi1")
	await _tick(c, 0.1)
	gw._objects["pi1"]["next_shot"] = 999.0  # пират пока не отвечает: проверяем только кнопку
	hud._on_attack_pressed()
	_check(attack.is_busy() and attack.disabled, "атака: кнопка блокируется до подтверждения командой")
	hud._on_attack_pressed()  # повторное нажатие игнорируется
	_check(gw.snapshot()["attacking"], "атака началась")
	await _tick(c, 0.1)
	_check(not attack.is_busy() and attack.text == "Прекратить атаку", "после подтверждения кнопка становится «Прекратить атаку»")
	hud._on_attack_pressed()
	await _tick(c, 0.1)
	_check(not gw.snapshot()["attacking"] and attack.text == "Атаковать", "повторное нажатие прекращает атаку")

	# ============ атака нейтрального игрока и AI: требуется подтверждение ============
	_free(c)
	c = await _make()
	hud = c["hud"]
	gw = c["gw"]
	attack = _n(c, "%AttackButton") as ActionButton
	var dlg: Control = _n(c, "%Confirm")
	_check(not dlg.is_open(), "окно подтверждения закрыто")
	await _select(c, "pl1")
	hud._on_attack_pressed()
	await process_frame
	_check(dlg.is_open() and not gw.snapshot()["attacking"], "нейтральный игрок: открыто подтверждение, атака не началась")
	_check(dlg._title.text == "Атаковать нейтрального игрока?", "заголовок подтверждения для игрока")
	_check(dlg._message.text.contains("сервер"), "в тексте подтверждения сказано, что последствия определяет сервер")
	_check(dlg._cancel.has_focus(), "по умолчанию в фокусе «Отмена»")
	await _tick(c, 0.5)
	_check(gw.snapshot()["time"] > 0.0 and dlg.is_open(), "игра идёт, пока открыто окно (PvP не останавливается)")
	dlg._close(false)
	await process_frame
	_check(not dlg.is_open() and not gw.snapshot()["attacking"], "отмена подтверждения: атаки нет")
	hud._on_attack_pressed()
	await process_frame
	dlg._close(true)
	await _tick(c, 0.1)
	_check(gw.snapshot()["attacking"] and gw.snapshot()["target_id"] == "pl1", "подтверждение: атака началась")
	gw.stop_attack()
	await _tick(c, 0.1)
	await _select(c, "ai1")
	hud._on_attack_pressed()
	await process_frame
	_check(dlg.is_open() and dlg._title.text == "Атаковать AI-капитана?", "AI-капитан: подтверждение с другим заголовком")
	gw.set_target("pl1")  # пока окно открыто, цель сменилась
	await process_frame
	dlg._close(true)
	await _tick(c, 0.1)
	_check(not gw.snapshot()["attacking"], "если цель сменилась за время подтверждения, атака отменяется")
	await _select(c, "st1")
	_check(attack.disabled, "союзную станцию атаковать нельзя (кнопка неактивна)")
	_free(c)

	# ============ навыки: энергия, перезарядка, ошибки ============
	c = await _make()
	hud = c["hud"]
	gw = c["gw"]
	skills = (_n(c, "%SkillsGrid") as Container).get_children()
	skills[1].pressed.emit()
	await _tick(c, 0.1)
	_check(skills[1].disabled and skills[1].text.contains("с"), "после применения «Щит» на перезарядке: %s" % skills[1].text)
	_check((_n(c, "%EnergyBar") as ProgressBar).value < 90.0 or gw._ship["energy"] < 90.0, "энергия потрачена")
	gw._ship["energy"] = 10.0
	await _tick(c, 0.1)
	_check(skills[2].disabled, "при нехватке энергии навык недоступен")
	gw.use_skill(0)  # без цели: команда отклоняется, игрок видит причину
	await process_frame
	var alerts: VBoxContainer = _n(c, "%AlertBar")
	_check(alerts.get_children().any(func(n: Node) -> bool: return n.has_meta("toast")), "ошибка команды показана уведомлением")

	# ============ автопилот и автонавыки ============
	await _select(c, "st1")
	var ap := _n(c, "%AutopilotButton") as ActionButton
	ap.button_pressed = true
	await _tick(c, 0.1)
	_check(gw.snapshot()["autopilot"] and ap.text == "Автопилот · вкл" and ap.theme_type_variation == &"PrimaryButton", "автопилот включён, кнопка показывает состояние")
	ap.button_pressed = false
	await _tick(c, 0.1)
	_check(not gw.snapshot()["autopilot"] and ap.text == "Автопилот", "автопилот выключен")
	var au := _n(c, "%AutoSkillsButton") as ActionButton
	au.button_pressed = true
	await _tick(c, 0.1)
	_check(gw.snapshot()["auto_skills"] and au.text == "Авто-навыки · вкл", "автонавыки включены")
	_free(c)

	# ============ бой: панели сворачиваются, тревоги остаются ============
	c = await _make()
	hud = c["hud"]
	gw = c["gw"]
	var quests := _n(c, "%QuestsCard")
	var chat := _n(c, "%ChatCard")
	_check(not quests.collapsed and chat.collapsed, "до боя: задания развёрнуты, чат свёрнут по умолчанию")
	await _select(c, "pi1")
	gw._objects["pi1"]["pos"] = Vector2(20, 0)
	gw.attack("pi1")
	await _tick(c, 1.5)
	_check(gw.in_combat() and quests.collapsed and chat.collapsed, "в бою задания и чат свёрнуты")
	gw._ship["shield"] = 20.0
	await _tick(c, 0.3)
	var alert_bar := _n(c, "%AlertBar")
	_check(alert_bar.has_alert("shield_low"), "боевая тревога видна, когда панели свёрнуты")
	_check((_n(c, "%DemoBar") as Control).visible and (_n(c, "%AttackButton") as Control).visible, "тревога не скрывает управление боем")
	gw.stop_attack()
	gw._objects["pi1"]["pos"] = Vector2(200, 200)
	gw._ship["shield"] = gw._ship["shield_max"]
	await _tick(c, 8.0)
	_check(not gw.in_combat() and not quests.collapsed and chat.collapsed, "после боя панели возвращаются к выбору игрока")
	_check(not alert_bar.has_alert("shield_low"), "тревога снимается, когда причина исчезла")
	quests.set_collapsed(true, false)
	gw._last_combat = gw._t
	await _tick(c, 0.2)
	gw._last_combat = -1000.0
	await _tick(c, 0.2)
	_check(quests.collapsed, "если игрок сам свернул панель, после боя она остаётся свёрнутой")
	_free(c)

	# ============ джойстик ============
	c = await _make()
	hud = c["hud"]
	gw = c["gw"]
	var joy: Control = _n(c, "%Joystick")
	_check(joy.visible and joy.size.x >= joy.radius * 4.0 and joy.size.y >= joy.radius * 6.0, "джойстик показан по умолчанию и занимает всю левую часть (%s)" % str(joy.size))
	var got: Array = []
	joy.vector_changed.connect(func(v: Vector2) -> void: got.append(v))
	var centre := joy.size * 0.5
	var t := InputEventScreenTouch.new()
	t.index = 0
	t.pressed = true
	t.position = centre + Vector2(joy.radius, 0)
	joy._gui_input(t)
	_check(joy.vector == Vector2.ZERO and joy.is_active(), "плавающий джойстик: касание в любом месте зоны задаёт центр, вектор нулевой")
	var d0 := InputEventScreenDrag.new()
	d0.index = 0
	d0.position = t.position + Vector2(joy.radius, 0)
	joy._gui_input(d0)
	_check(joy.vector.is_equal_approx(Vector2(1, 0)), "джойстик: смещение на радиус от точки касания даёт вектор (1, 0)")
	var d := InputEventScreenDrag.new()
	d.index = 0
	d.position = t.position + Vector2(0, -joy.radius * 3.0)
	joy._gui_input(d)
	_check(joy.vector.is_equal_approx(Vector2(0, -1)), "джойстик: вектор ограничен единичной длиной")
	var other := InputEventScreenTouch.new()
	other.index = 1
	other.pressed = true
	other.position = centre
	joy._gui_input(other)
	var d2 := InputEventScreenDrag.new()
	d2.index = 1
	d2.position = centre + Vector2(joy.radius, 0)
	joy._gui_input(d2)
	_check(joy.vector.is_equal_approx(Vector2(0, -1)), "джойстик: второй палец не перехватывает управление")
	var up := InputEventScreenTouch.new()
	up.index = 0
	up.pressed = false
	joy._gui_input(up)
	_check(joy.vector == Vector2.ZERO and not joy.is_active(), "джойстик: отпускание обнуляет вектор")
	t.position = centre + Vector2(3, 0)
	joy._gui_input(t)
	_check(joy.vector == Vector2.ZERO, "джойстик: мёртвая зона")
	var emu := InputEventMouseButton.new()
	emu.button_index = MOUSE_BUTTON_LEFT
	emu.pressed = true
	emu.device = InputEvent.DEVICE_ID_EMULATION
	emu.position = centre + Vector2(joy.radius, 0)
	joy._gui_input(up)
	joy._gui_input(emu)
	_check(not joy.is_active(), "джойстик: эмулированные мышью события касания игнорируются")
	joy._gui_input(up)
	# движение корабля от джойстика через HUD
	hud._joy = Vector2(1, 0)
	hud._process(0.016)
	var x0: float = gw._ship["pos"].x
	await _tick(c, 1.0)
	_check(gw._ship["pos"].x > x0 + 10.0, "вектор джойстика двигает корабль (через шлюз)")
	hud._joy = Vector2.ZERO
	hud._process(0.016)
	var x1: float = gw._ship["pos"].x
	await _tick(c, 1.0)
	_check(absf(gw._ship["pos"].x - x1) < 0.01, "нулевой вектор останавливает корабль")
	_free(c)
	UiSettings.override("joystick", false)
	c = await _make()
	_check(not (_n(c, "%Joystick") as Control).visible, "джойстик можно отключить в настройках")
	_free(c)
	UiSettings.override("joystick", null)

	# ============ выход ============
	c = await _make()
	var exits := []
	c["screen"].exit_requested.connect(func() -> void: exits.append(1))
	(_n(c, "%ExitButton") as Button).pressed.emit()
	_check(exits.size() == 1, "кнопка «Выйти» запрашивает выход")
	var esc := InputEventAction.new()
	esc.action = "ui_cancel"
	esc.pressed = true
	c["screen"]._unhandled_input(esc)
	_check(exits.size() == 2, "Escape / «Назад» запрашивает выход")
	_free(c)

	# ============ смена системы: фон меняется по данным шлюза ============
	c = await _make()
	var bg_node: Control = c["screen"].get_node("%SpaceBackground")
	_check(bg_node.profile.id == "xi" and bg_node.layer_count() == 3, "экран космоса: фон системы Кси (профиль xi)")
	var tex_before = bg_node.layer_info(1)["texture"]
	(_n(c, "%SystemButton") as Button).pressed.emit()
	await process_frame
	_check(bg_node.profile.id == "omega" and (c["screen"]._bg_id == "omega"), "кнопка «Система»: фон сменился на профиль omega")
	_check(c["gw"].snapshot()["system"]["id"] == "omega", "шлюз сообщает новую систему")
	_check(_n(c, "%AlertBar").get_children().any(func(n: Node) -> bool: return n.has_meta("toast")), "смена системы показана уведомлением")
	(_n(c, "%SystemButton") as Button).pressed.emit()
	await process_frame
	_check(bg_node.profile.id == "deep" and bg_node.layer_count() == 2, "следующая система: deep, два слоя фона")
	_check(bg_node.layer_info(0)["texture"] == tex_before or bg_node.layer_info(1)["texture"] == tex_before, "текстуры берутся из каталога по ключам (общие текстуры переиспользуются)")
	_free(c)

	# ============ раскладка: размеры, вырезы, состояния ============
	var sizes := [Vector2i(1280, 720), Vector2i(1572, 720), Vector2i(1600, 720), Vector2i(1920, 720), Vector2i(1280, 853), Vector2i(1280, 960)]
	for sz in sizes:
		await _verify_layout(sz, Vector4i.ZERO, "%dx%d" % [sz.x, sz.y], "обычное")
	await _verify_layout(Vector2i(1572, 720), Vector4i(110, 0, 0, 20), "1572x720, вырез слева", "обычное")
	await _verify_layout(Vector2i(1572, 720), Vector4i(0, 0, 110, 20), "1572x720, вырез справа", "обычное")
	await _verify_layout(Vector2i(1280, 720), Vector4i(0, 0, 0, 0), "1280x720", "всё развёрнуто, цель, тревоги")
	await _verify_layout(Vector2i(1572, 720), Vector4i(0, 0, 0, 0), "1572x720", "всё развёрнуто, цель, тревоги")

	UiSettings.override("reduce_motion", null)
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)


func _visible_controls(node: Node, out: Array) -> void:
	for ch in node.get_children():
		if ch is Control and not ch.is_visible_in_tree():
			continue  # скрытые поддеревья (например, закрытое окно подтверждения) не проверяем
		if ch is Control:
			out.append(ch)
		_visible_controls(ch, out)


func _verify_layout(size: Vector2i, insets: Vector4i, label: String, scenario: String) -> void:
	var c = await _make(size)
	var screen: Control = c["screen"]
	var gw = c["gw"]
	if insets != Vector4i.ZERO:
		screen.get_node("%Hud/SafeArea").set_debug_insets(insets.x, insets.y, insets.z, insets.w)
	if scenario != "обычное":
		_n(c, "%QuestsCard").set_collapsed(false, false)
		_n(c, "%ChatCard").set_collapsed(false, false)
		await _select(c, "pl1")
		gw._last_combat = -1000.0
		gw._alerts.clear()
		gw.alert_raised.emit("shield_low", "Щиты на исходе!")
		gw.alert_raised.emit("hull_critical", "Корпус в критическом состоянии!")
		for i in 3:
			gw.notification.emit("Уведомление %d" % i)
		await _tick(c, 0.1)
	for i in 3:
		await process_frame

	var hud: Control = c["hud"]
	var screen_rect := Rect2(Vector2.ZERO, Vector2(size))
	var safe := Rect2(Vector2(insets.x, insets.y), Vector2(size.x - insets.x - insets.z, size.y - insets.y - insets.w))
	var all: Array = []
	_visible_controls(hud, all)
	var outside := []
	var squeezed := []
	var small := []
	for ctl: Control in all:
		if ctl.name in [&"Hud", &"SafeArea", &"Confirm", &"Columns", &"Joystick"] or ctl.get_parent().name == &"SafeArea":
			continue
		var r := ctl.get_global_rect()
		if r.size.x <= 0.0 or r.size.y <= 0.0:
			continue
		if ctl.has_meta("toast") or ctl is Label and ctl.get_parent() is PanelContainer and ctl.get_parent().has_meta("toast"):
			pass
		if not safe.grow(1.0).encloses(r):
			outside.append("%s %s" % [ctl.name, r])
		var mn := ctl.get_combined_minimum_size()
		if ctl.size.x + 1.0 < mn.x or ctl.size.y + 1.0 < mn.y:
			squeezed.append("%s размер %s < минимум %s" % [ctl.name, ctl.size, mn])
		if ctl is ActionButton and ctl.size.y < 48.0:
			small.append(str(ctl.name))
	_check(outside.is_empty(), "%s (%s): всё внутри экрана и безопасной зоны %s" % [label, scenario, str(outside).left(300)])
	_check(squeezed.is_empty(), "%s (%s): ничто не сжато меньше минимума %s" % [label, scenario, str(squeezed).left(300)])
	_check(small.is_empty(), "%s (%s): кнопки не ниже 48 px" % [label, scenario])

	# колонки не шире экрана (иначе корневой контейнер раздвинется и обрежет края)
	var columns: Control = hud.get_node("SafeArea/Columns")
	var col_min := columns.get_combined_minimum_size()
	_check(col_min.x <= safe.size.x - 32.0 + 1.0, "%s (%s): колонки HUD влезают по ширине (%.0f из %.0f)" % [label, scenario, col_min.x, safe.size.x - 32.0])
	_check(col_min.y <= safe.size.y - 32.0 + 1.0, "%s (%s): колонки HUD влезают по высоте (%.0f из %.0f)" % [label, scenario, col_min.y, safe.size.y - 32.0])

	# блоки не пересекаются между собой
	var blocks := []
	for nm in ["ProfileCard", "QuestsCard", "ChatCard", "DemoBar", "AlertBar", "DebugSlot", "BottomNav", "TopRight", "TargetCard", "AttackButton", "CombatBlock", "ToggleRow"]:
		var node := hud.find_child(nm, true, false) as Control
		if node != null and node.visible and node.size.x > 0.0 and node.size.y > 0.0:
			blocks.append(node)
	var overlaps := []
	for i in blocks.size():
		for j in range(i + 1, blocks.size()):
			var a: Rect2 = blocks[i].get_global_rect()
			var b: Rect2 = blocks[j].get_global_rect()
			if a.intersects(b):
				overlaps.append("%s x %s" % [blocks[i].name, blocks[j].name])
	_check(overlaps.is_empty(), "%s (%s): блоки HUD не пересекаются %s" % [label, scenario, str(overlaps)])

	# мир виден: центральная область свободна и достаточно велика
	var left_edge := 0.0
	var right_edge := float(size.x)
	for nm in ["LeftStack", "RightStack"]:
		var st := hud.find_child(nm, true, false) as Control
		if nm == "LeftStack":
			left_edge = st.get_global_rect().end.x
		else:
			right_edge = st.get_global_rect().position.x
	var free_w := right_edge - left_edge
	_check(free_w >= float(size.x) * 0.30, "%s (%s): космос занимает центр (%.0f px из %d)" % [label, scenario, free_w, size.x])
	c["holder"].queue_free()
	Session.gateway = null
	await process_frame
