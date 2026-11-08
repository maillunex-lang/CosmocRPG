extends SceneTree
## Тест экрана подключения: ведёт его как пользователь и снимает скриншоты состояний.
## Нужен рендеринг (xvfb): tools/test_client.sh. Переменные: COSMOC_TEST_WS_URL, SHOT_DIR (необязательно).

var _failures := 0
var _screen: Control


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _wait_until(pred: Callable, timeout: float) -> bool:
	var deadline := Time.get_ticks_msec() + int(timeout * 1000.0)
	while not pred.call() and Time.get_ticks_msec() < deadline:
		await process_frame
	return pred.call()


func _shot(name: String) -> void:
	var dir := OS.get_environment("SHOT_DIR")
	if dir == "":
		return
	await create_timer(0.2).timeout
	await RenderingServer.frame_post_draw
	var path := "%s/%s.png" % [dir, name]
	root.get_texture().get_image().save_png(path)
	print("SHOT_SAVED ", path)


func _run() -> void:
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if url == "":
		print("COSMOC_TEST_WS_URL не задана")
		quit(1)
		return
	DirAccess.remove_absolute("user://settings.cfg")

	_screen = load("res://scenes/connect.tscn").instantiate()
	_screen.set("auto_connect", false)
	root.add_child(_screen)
	await process_frame
	var url_edit: LineEdit = _screen.get_node("%UrlEdit")
	var connect_btn: Button = _screen.get_node("%ConnectButton")
	var disconnect_btn: Button = _screen.get_node("%DisconnectButton")
	var ping_btn: Button = _screen.get_node("%PingButton")
	var status: Label = _screen.get_node("%StatusLabel")
	var log: RichTextLabel = _screen.get_node("%Log")

	# Начальное состояние
	_check(status.text == "Не подключено", "начальный статус «Не подключено»")
	_check(not connect_btn.disabled and disconnect_btn.disabled and ping_btn.disabled, "начальное состояние кнопок")
	_check(url_edit.text == "wss://rikagro.ru/ws", "адрес по умолчанию")
	_check(url_edit.virtual_keyboard_type == LineEdit.KEYBOARD_TYPE_URL, "для адреса включена клавиатура URL (не числовая)")
	await _shot("01_idle")

	# Ошибка: некорректный адрес
	url_edit.text = "мусор"
	connect_btn.pressed.emit()
	_check(await _wait_until(func() -> bool: return status.text == "Ошибка подключения", 3.0), "некорректный адрес показывает ошибку")
	_check(not connect_btn.disabled and url_edit.editable, "после ошибки можно подключиться снова")
	_check(log.get_parsed_text().contains("Некорректный адрес"), "причина ошибки в журнале")
	await _shot("02_error_bad_url")

	# Ошибка: сервер недоступен
	url_edit.text = "ws://127.0.0.1:1/ws"
	connect_btn.pressed.emit()
	_check(status.text == "Подключение…" and connect_btn.disabled, "во время подключения кнопка заблокирована")
	_check(await _wait_until(func() -> bool: return status.text == "Ошибка подключения", 8.0), "недоступный сервер показывает ошибку")
	await _shot("03_error_unreachable")

	# Успешное подключение
	url_edit.text = url
	connect_btn.pressed.emit()
	_check(await _wait_until(func() -> bool: return status.text.begins_with("Подключено"), 5.0), "успешное подключение")
	_check(disconnect_btn.disabled == false and ping_btn.disabled == false, "после подключения доступны «Отключиться» и «Пинг»")
	ping_btn.pressed.emit()
	_check(await _wait_until(func() -> bool: return log.get_parsed_text().contains("Задержка:"), 3.0), "кнопка «Пинг» показывает задержку")
	await _shot("04_connected")

	# Отключение
	disconnect_btn.pressed.emit()
	_check(await _wait_until(func() -> bool: return status.text == "Отключено", 3.0), "отключение по кнопке")
	_check(not connect_btn.disabled and disconnect_btn.disabled, "после отключения состояние кнопок сброшено")

	# Адрес сохраняется между запусками
	var cfg := ConfigFile.new()
	_check(cfg.load("user://settings.cfg") == OK and cfg.get_value("server", "url", "") == url, "последний адрес сохранён в user://settings.cfg")

	print("Итого провалов: %d" % _failures)
	quit(1 if _failures > 0 else 0)
