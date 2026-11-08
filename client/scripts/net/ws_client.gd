extends Node
## Сетевой слой: WebSocket-соединение с игровым сервером.
## Ничего не знает про интерфейс; общается сигналами. Формат сообщений: docs/API.md.

signal connected(session_id: String, server_version: String)
signal message_received(type: String, payload: Dictionary, id: String)
signal connection_failed(reason: String)
signal disconnected(code: int, reason: String)
signal latency_updated(rtt_ms: int)

enum State { IDLE, CONNECTING, OPEN }

const CONNECT_TIMEOUT_SEC := 5.0
const PING_INTERVAL_SEC := 10.0
const MAX_OUTGOING_BYTES := 4096

var state: State = State.IDLE
var session_id: String = ""

var _peer: WebSocketPeer
var _started_ms: int = 0
var _got_hello: bool = false
var _next_ping_ms: int = 0
var _pending_pings: Dictionary = {}  # id -> время отправки, мс
var _msg_counter: int = 0


func _ready() -> void:
	set_process(false)


## Начать подключение. Результат придёт сигналами connected / connection_failed.
func connect_to(url: String) -> void:
	if state != State.IDLE:
		push_warning("WsClient: подключение уже выполняется или открыто")
		return
	if not (url.begins_with("ws://") or url.begins_with("wss://")) or url.length() < 8:
		# сигнал отправляем отложенно, чтобы вызывающий успел подписаться
		connection_failed.emit.call_deferred("Некорректный адрес: ожидается ws://хост:порт/путь")
		return
	_peer = WebSocketPeer.new()
	_peer.handshake_headers = PackedStringArray()
	var err := _peer.connect_to_url(url)
	if err != OK:
		_peer = null
		connection_failed.emit.call_deferred("Не удалось начать подключение (код %d)" % err)
		return
	state = State.CONNECTING
	_got_hello = false
	_started_ms = Time.get_ticks_msec()
	_pending_pings.clear()
	set_process(true)


## Закрыть соединение по инициативе клиента.
func close() -> void:
	if _peer != null and state != State.IDLE:
		_peer.close(1000, "client close")


## Отправить команду серверу. Возвращает id сообщения или "" при ошибке.
func send_message(type: String, payload: Dictionary = {}) -> String:
	if state != State.OPEN or _peer == null:
		return ""
	_msg_counter += 1
	var id := str(_msg_counter)
	var text := JSON.stringify({"type": type, "id": id, "payload": payload})
	if text.to_utf8_buffer().size() > MAX_OUTGOING_BYTES:
		push_warning("WsClient: сообщение слишком большое, не отправлено")
		return ""
	if _peer.send_text(text) != OK:
		return ""
	return id


func _process(_delta: float) -> void:
	if _peer == null:
		return
	_peer.poll()
	var ready_state := _peer.get_ready_state()
	var now := Time.get_ticks_msec()

	match state:
		State.CONNECTING:
			if ready_state == WebSocketPeer.STATE_OPEN:
				state = State.OPEN
				_next_ping_ms = now + int(PING_INTERVAL_SEC * 1000.0)
			elif ready_state == WebSocketPeer.STATE_CLOSED:
				_fail("Сервер недоступен или отклонил подключение")
				return
			elif now - _started_ms > int(CONNECT_TIMEOUT_SEC * 1000.0):
				_peer.close()
				_fail("Превышено время ожидания подключения")
				return
		State.OPEN:
			if ready_state == WebSocketPeer.STATE_OPEN:
				_read_packets()
				if state != State.OPEN:
					return
				if not _got_hello and now - _started_ms > int(CONNECT_TIMEOUT_SEC * 1000.0):
					_peer.close(1002, "no hello")
					_fail("Сервер не прислал приветствие (hello)")
					return
				if _got_hello and now >= _next_ping_ms:
					_next_ping_ms = now + int(PING_INTERVAL_SEC * 1000.0)
					_send_ping()
			elif ready_state == WebSocketPeer.STATE_CLOSED:
				_read_packets()  # забрать то, что успело прийти перед закрытием
				var code := _peer.get_close_code()
				var reason := _peer.get_close_reason()
				if not _got_hello:
					_fail("Соединение закрыто до приветствия (код %d)" % code)
				else:
					_reset()
					disconnected.emit(code, reason)


## Отправить ping и запомнить время для расчёта задержки.
func ping() -> void:
	_send_ping()


func _send_ping() -> void:
	var id := send_message("ping")
	if id != "":
		_pending_pings[id] = Time.get_ticks_msec()


func _read_packets() -> void:
	while _peer.get_available_packet_count() > 0:
		var text := _peer.get_packet().get_string_from_utf8()
		var parsed: Variant = JSON.parse_string(text)
		if typeof(parsed) != TYPE_DICTIONARY or typeof(parsed.get("type")) != TYPE_STRING:
			push_warning("WsClient: получено некорректное сообщение: %s" % text.left(100))
			continue
		var msg: Dictionary = parsed
		var type: String = msg["type"]
		var payload: Dictionary = msg["payload"] if typeof(msg.get("payload")) == TYPE_DICTIONARY else {}
		var id: String = str(msg.get("id", ""))
		if type == "hello" and not _got_hello:
			_got_hello = true
			session_id = str(payload.get("session", ""))
			connected.emit(session_id, str(payload.get("server_version", "")))
			continue
		if type == "pong" and _pending_pings.has(id):
			var rtt := Time.get_ticks_msec() - int(_pending_pings[id])
			_pending_pings.erase(id)
			latency_updated.emit(rtt)
		message_received.emit(type, payload, id)


func _fail(reason: String) -> void:
	_reset()
	connection_failed.emit(reason)


func _reset() -> void:
	state = State.IDLE
	session_id = ""
	_peer = null
	_pending_pings.clear()
	set_process(false)
