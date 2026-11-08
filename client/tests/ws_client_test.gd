extends SceneTree
## Интеграционный тест сетевого слоя против запущенного сервера.
## Запуск: COSMOC_TEST_WS_URL=ws://127.0.0.1:8080/ws \
##   godot --headless --path client --script res://tests/ws_client_test.gd
## Код выхода 0 — все проверки прошли, 1 — есть провалы.

const WsClientScript := preload("res://scripts/net/ws_client.gd")

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


func _new_client() -> Node:
	var c: Node = WsClientScript.new()
	root.add_child(c)
	return c


## Ждём, пока флаг в словаре не станет true, но не дольше timeout секунд.
func _wait_for(flags: Dictionary, key: String, timeout: float) -> bool:
	var deadline := Time.get_ticks_msec() + int(timeout * 1000.0)
	while not flags.get(key, false) and Time.get_ticks_msec() < deadline:
		await process_frame
	return flags.get(key, false)


func _run() -> void:
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if url == "":
		print("COSMOC_TEST_WS_URL не задана")
		quit(1)
		return

	# 1. Подключение и hello
	var f1 := {}
	var c1 := _new_client()
	c1.connected.connect(func(sid: String, ver: String) -> void:
		f1["sid"] = sid
		f1["ver"] = ver
		f1["ok"] = true)
	c1.connect_to(url)
	_check(await _wait_for(f1, "ok", 5.0), "подключение и получение hello")
	_check(String(f1.get("sid", "")).length() == 32, "идентификатор сессии из 32 символов")
	_check(c1.state == WsClientScript.State.OPEN, "состояние OPEN после hello")

	# 2. ping -> pong с замером задержки
	c1.latency_updated.connect(func(rtt: int) -> void:
		f1["rtt"] = rtt
		f1["pong"] = true)
	c1.ping()
	_check(await _wait_for(f1, "pong", 3.0), "ping получает pong")
	_check(int(f1.get("rtt", -1)) >= 0, "задержка вычислена")

	# 3. Неизвестный тип -> error от сервера
	c1.message_received.connect(func(type: String, payload: Dictionary, _id: String) -> void:
		if type == "error":
			f1["err_code"] = str(payload.get("code", ""))
			f1["err"] = true)
	c1.send_message("teleport", {})
	_check(await _wait_for(f1, "err", 3.0), "сервер отвечает error на неизвестный тип")
	_check(f1.get("err_code", "") == "unknown_type", "код ошибки unknown_type")

	# 4. Сообщение больше лимита закрывает сессию, клиент это замечает
	c1.disconnected.connect(func(code: int, _r: String) -> void:
		f1["close_code"] = code
		f1["closed"] = true)
	# клиент сам не отправит >4096 байт, поэтому шлём в обход через внутренний peer
	c1._peer.send_text(JSON.stringify({"type": "ping", "payload": "x".repeat(10000)}))
	_check(await _wait_for(f1, "closed", 3.0), "клиент получает disconnected при закрытии сервером")
	_check(f1.get("close_code", 0) == 1009, "код закрытия 1009 (слишком большое сообщение)")
	_check(c1.state == WsClientScript.State.IDLE, "состояние IDLE после закрытия")

	# 5. Клиентский лимит исходящих сообщений
	var f5 := {}
	var c5 := _new_client()
	c5.connected.connect(func(_s: String, _v: String) -> void: f5["ok"] = true)
	c5.connect_to(url)
	await _wait_for(f5, "ok", 5.0)
	_check(c5.send_message("ping", {"x": "y".repeat(5000)}) == "", "слишком большое исходящее сообщение отклоняется")
	_check(c5.send_message("ping") != "", "обычное сообщение отправляется")
	c5.close()
	f5["closed"] = false
	c5.disconnected.connect(func(_c: int, _r: String) -> void: f5["closed"] = true)
	_check(await _wait_for(f5, "closed", 3.0), "close() по инициативе клиента завершается disconnected")

	# 6. Некорректный адрес
	var f6 := {}
	var c6 := _new_client()
	c6.connection_failed.connect(func(reason: String) -> void:
		f6["reason"] = reason
		f6["failed"] = true)
	c6.connect_to("http://example.com")
	_check(await _wait_for(f6, "failed", 2.0), "некорректный адрес даёт connection_failed")
	_check(c6.state == WsClientScript.State.IDLE, "после ошибки адреса состояние IDLE")

	# 7. Порт, на котором никто не слушает
	var f7 := {}
	var c7 := _new_client()
	c7.connection_failed.connect(func(reason: String) -> void:
		f7["reason"] = reason
		f7["failed"] = true)
	c7.connect_to("ws://127.0.0.1:1/ws")
	_check(await _wait_for(f7, "failed", 8.0), "недоступный сервер даёт connection_failed")
	_check(c7.state == WsClientScript.State.IDLE, "после отказа состояние IDLE")

	# 8. Повторное подключение после ошибки работает
	var f8 := {}
	c7.connected.connect(func(_s: String, _v: String) -> void: f8["ok"] = true)
	c7.connect_to(url)
	_check(await _wait_for(f8, "ok", 5.0), "повторное подключение после ошибки")
	c7.close()

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
