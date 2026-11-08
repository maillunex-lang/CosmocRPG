extends SceneTree
## Стартовый экран: нет адреса сервера, логотип и кнопки внутри экрана при разных размерах и вырезах,
## состояния связи, секретный служебный экран, адрес по умолчанию, автоподключение с повторами.

const Splash := preload("res://ui/start_splash.gd")

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


func _texts(node: Node, out: Array) -> void:
	for c in node.get_children():
		if c is Control and (c as Control).visible:
			if c is Label:
				out.append((c as Label).text)
			elif c is LineEdit:
				out.append((c as LineEdit).text)
			elif c is Button:
				out.append((c as Button).text)
		_texts(c, out)


func _make(size: Vector2i, insets: Vector4i = Vector4i.ZERO) -> Array:
	var holder := SubViewport.new()
	holder.size = size
	holder.disable_3d = true
	root.add_child(holder)
	var screen: Control = load("res://scenes/connect.tscn").instantiate()
	screen.set("auto_connect", false)
	holder.add_child(screen)
	await process_frame
	var splash: Control = screen.get_node("StartSplash")
	if insets != Vector4i.ZERO:
		splash.safe.set_debug_insets(insets.x, insets.y, insets.z, insets.w)
	await process_frame
	await process_frame
	return [holder, screen, splash]


func _inside(c: Control, rect: Rect2) -> bool:
	return rect.grow(1.0).encloses(c.get_global_rect())


func _run() -> void:
	DirAccess.remove_absolute("user://settings.cfg")
	# --- размеры и вырезы ---
	for case in [[Vector2i(1280, 720), Vector4i.ZERO, "1280x720"], [Vector2i(2000, 916), Vector4i.ZERO, "2000x916"],
			[Vector2i(960, 540), Vector4i.ZERO, "960x540"], [Vector2i(1572, 720), Vector4i(110, 0, 0, 0), "1572x720, вырез слева"],
			[Vector2i(1280, 720), Vector4i(60, 30, 60, 30), "вырезы по краям"]]:
		var size: Vector2i = case[0]
		var ins: Vector4i = case[1]
		var made: Array = await _make(size, ins)
		var sp: Splash = made[2]
		var safe := Rect2(Vector2(ins.x, ins.y), Vector2(size.x - ins.x - ins.z, size.y - ins.y - ins.w))
		_c(_inside(sp.play_button, safe), "%s: «ИГРАТЬ» внутри безопасной зоны" % case[2])
		_c(_inside(sp.account_button, safe), "%s: «Аккаунт» внутри безопасной зоны" % case[2])
		_c(_inside(sp.status_label, safe), "%s: статус внутри безопасной зоны" % case[2])
		_c(_inside(sp.get_node("%s" % "SafeAreaRoot" if false else ".").get_child(1).get_child(0).get_node("Logo") as Control, safe), "%s: логотип внутри безопасной зоны" % case[2])
		_c(sp.play_button.size.y >= 56.0 and sp.account_button.size.y >= 48.0, "%s: кнопки не меньше 48 dp" % case[2])
		(made[0] as Node).queue_free()
		await process_frame

	var made: Array = await _make(Vector2i(1280, 720))
	var screen: Control = made[1]
	var sp: Splash = made[2]
	# --- нет служебных сведений ---
	var texts: Array = []
	_texts(sp, texts)
	var joined := " ".join(texts)
	_c(not joined.contains("ws://") and not joined.contains("wss://") and not joined.contains("rikagro") and not joined.contains("78.17"), "адрес сервера на экране не показан")
	_c(not (screen.get_node("SafeArea") as Control).visible, "служебный экран скрыт по умолчанию")
	# --- состояния ---
	sp.set_state(Splash.State.CONNECTING, "Подключение к серверу…")
	_c(sp.play_button.disabled and not sp.retry_button.visible, "при подключении «ИГРАТЬ» недоступна")
	sp.set_state(Splash.State.READY, "Сервер на связи")
	_c(not sp.play_button.disabled and not sp.retry_button.visible, "при готовности «ИГРАТЬ» доступна")
	sp.set_state(Splash.State.ERROR, "Нет связи с сервером")
	_c(sp.play_button.disabled and sp.retry_button.visible, "при ошибке «Повторить» видна, «ИГРАТЬ» недоступна")
	sp.set_state(Splash.State.ENTERING, "Вход")
	_c(sp.play_button.disabled and sp.account_button.disabled and sp.play_button.text == "ВХОД…", "при входе кнопки заблокированы")
	# --- секретный экран: пять нажатий за 2,5 с ---
	var opened := [0]
	sp.secret_opened.connect(func() -> void: opened[0] += 1)
	for i in 4:
		sp.register_logo_tap(1000 + i * 100)
	_c(opened[0] == 0, "четырёх нажатий недостаточно")
	sp.register_logo_tap(1500)
	_c(opened[0] == 1, "пять быстрых нажатий открывают служебный экран")
	for i in 5:
		sp.register_logo_tap(10000 + i * 900)   # слишком медленно
	_c(opened[0] == 1, "медленные нажатия не открывают служебный экран")
	# --- экран «Аккаунт»: только аккаунт, без адреса ---
	screen.call("_open_overlay", false)
	await process_frame
	_c((screen.get_node("SafeArea") as Control).visible and not (screen.get_node("%UrlEdit") as Control).is_visible_in_tree(), "«Аккаунт»: форма видна, адрес сервера скрыт")
	screen.call("_close_overlay")
	_c(not (screen.get_node("SafeArea") as Control).visible, "служебный экран закрывается")
	# --- адрес по умолчанию и сохранённый старый адрес ---
	_c((screen.get_node("%UrlEdit") as LineEdit).text == "wss://rikagro.ru/ws", "по умолчанию адрес домена")
	var cfg := ConfigFile.new()
	cfg.set_value("server", "url", "ws://78.17.214.25:8080/ws")   # так сохраняли старые версии
	cfg.save("user://settings.cfg")
	var made2: Array = await _make(Vector2i(1280, 720))
	_c(((made2[1] as Control).get_node("%UrlEdit") as LineEdit).text == "wss://rikagro.ru/ws", "старый сохранённый адрес не мешает переходу на домен")
	DirAccess.remove_absolute("user://settings.cfg")

	# --- автоподключение: недоступный сервер -> повтор по таймеру ---
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	root.add_child(holder)
	var auto: Control = load("res://scenes/connect.tscn").instantiate()
	holder.add_child(auto)   # auto_connect включён
	(auto.get_node("%UrlEdit") as LineEdit).text = "ws://127.0.0.1:1/ws"
	await process_frame
	var asp: Splash = auto.get_node("StartSplash")
	var t0 := Time.get_ticks_msec()
	while Time.get_ticks_msec() - t0 < 8000 and asp.state != Splash.State.ERROR:
		await process_frame
	_c(asp.state == Splash.State.ERROR and asp.status_label.text.begins_with("Нет связи"), "недоступный сервер: ошибка и «Нет связи»")
	t0 = Time.get_ticks_msec()
	while Time.get_ticks_msec() - t0 < 4000 and not asp.status_label.text.contains("Повтор через"):
		await process_frame
	_c(asp.status_label.text.contains("Повтор через"), "показан отсчёт до повтора подключения")

	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if url != "":
		var ok_holder := SubViewport.new()
		ok_holder.size = Vector2i(1280, 720)
		root.add_child(ok_holder)
		var ok: Control = load("res://scenes/connect.tscn").instantiate()
		ok_holder.add_child(ok)
		(ok.get_node("%UrlEdit") as LineEdit).text = url   # подмена до первого кадра
		var osp: Splash = ok.get_node("StartSplash")
		t0 = Time.get_ticks_msec()
		while Time.get_ticks_msec() - t0 < 8000 and osp.state != Splash.State.READY:
			await process_frame
		_c(osp.state == Splash.State.READY and not osp.play_button.disabled, "автоподключение к серверу: «ИГРАТЬ» доступна без нажатий")
	else:
		print("COSMOC_TEST_WS_URL не задана: проверка автоподключения к серверу пропущена")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
