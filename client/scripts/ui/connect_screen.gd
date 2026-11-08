extends Control
## Экран подключения к серверу (Этап 00). Только отображение и ввод;
## сетевая логика живёт в WsClient.

const WsClientScript := preload("res://scripts/net/ws_client.gd")
const SETTINGS_PATH := "user://settings.cfg"
const DEFAULT_URL := "ws://78.17.214.25:8080/ws"  # тестовый сервер; позже будет настраиваться при сборке

var _client: Node

@onready var _url_edit: LineEdit = %UrlEdit
@onready var _connect_button: Button = %ConnectButton
@onready var _disconnect_button: Button = %DisconnectButton
@onready var _ping_button: Button = %PingButton
@onready var _status_label: Label = %StatusLabel
@onready var _log: RichTextLabel = %Log


func _ready() -> void:
	_client = WsClientScript.new()
	add_child(_client)
	_client.connected.connect(_on_connected)
	_client.connection_failed.connect(_on_failed)
	_client.disconnected.connect(_on_disconnected)
	_client.latency_updated.connect(_on_latency)
	_client.message_received.connect(_on_message)

	_connect_button.pressed.connect(_on_connect_pressed)
	_disconnect_button.pressed.connect(func() -> void: _client.close())
	_ping_button.pressed.connect(func() -> void: _client.ping())
	_url_edit.text_submitted.connect(func(_t: String) -> void: _on_connect_pressed())

	_url_edit.text = _load_url()
	_set_idle("Не подключено")


func _on_connect_pressed() -> void:
	var url := _url_edit.text.strip_edges()
	_save_url(url)
	_status("Подключение…", Color.YELLOW)
	_connect_button.disabled = true
	_url_edit.editable = false
	_append("Подключение к %s" % url)
	_client.connect_to(url)


func _on_connected(session_id: String, server_version: String) -> void:
	_status("Подключено (сервер %s)" % server_version, Color.LIGHT_GREEN)
	_append("Сессия %s" % session_id)
	_disconnect_button.disabled = false
	_ping_button.disabled = false


func _on_failed(reason: String) -> void:
	_append("[color=tomato]Ошибка: %s[/color]" % reason)
	_set_idle("Ошибка подключения")


func _on_disconnected(code: int, reason: String) -> void:
	_append("Соединение закрыто (код %d %s)" % [code, reason])
	_set_idle("Отключено")


func _on_latency(rtt_ms: int) -> void:
	_append("Задержка: %d мс" % rtt_ms)


func _on_message(type: String, payload: Dictionary, _id: String) -> void:
	if type == "error":
		_append("[color=tomato]Сервер: %s[/color]" % str(payload.get("message", "ошибка")))
	else:
		_append("Сообщение: %s" % type)


func _set_idle(text: String) -> void:
	_status(text, Color.WHITE)
	_connect_button.disabled = false
	_disconnect_button.disabled = true
	_ping_button.disabled = true
	_url_edit.editable = true


func _status(text: String, color: Color) -> void:
	_status_label.text = text
	_status_label.add_theme_color_override("font_color", color)


func _append(bbcode: String) -> void:
	_log.append_text(bbcode + "\n")


func _load_url() -> String:
	var cfg := ConfigFile.new()
	if cfg.load(SETTINGS_PATH) == OK:
		return str(cfg.get_value("server", "url", DEFAULT_URL))
	return DEFAULT_URL


func _save_url(url: String) -> void:
	var cfg := ConfigFile.new()
	cfg.load(SETTINGS_PATH)
	cfg.set_value("server", "url", url)
	cfg.save(SETTINGS_PATH)
