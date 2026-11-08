extends Control
## Экран подключения к серверу (Этап 00). Только отображение и ввод;
## сетевая логика живёт в WsClient. Оформление: общая тема UiTheme, компоненты ui/components.

const WsClientScript := preload("res://scripts/net/ws_client.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const ServerGateway := preload("res://ui/gateway/server_gateway.gd")
const PlayerIdentity := preload("res://ui/player_identity.gd")
const CharacterScreen := preload("res://ui/character_screen.gd")
const StartSplash := preload("res://ui/start_splash.gd")
const Updater := preload("res://ui/updater.gd")
const ContentLoader := preload("res://ui/content_loader.gd")
const ApkInstaller := preload("res://ui/apk_installer.gd")

const SETTINGS_PATH := "user://settings.cfg"
const DEFAULT_URL := "wss://rikagro.ru/ws"  # домен с сертификатом Let's Encrypt (запасной адрес: ws://78.17.214.25:8080/ws)

## Автоподключение при запуске (тесты выключают его до добавления на сцену).
var auto_connect := true

var _client: Node
var _account_pending := false
var _splash: Control
var _updater: Node
var _content: Node
var _installer: Node
var _update_info: Dictionary = {}
var _update_manual := false
var _overlay_bg: ColorRect
var _overlay_back: ActionButton
var _retry_timer: Timer
var _retry_attempt := 0
var _retry_left := 0
var _manual_close := false

@onready var _url_edit: LineEdit = %UrlEdit
@onready var _connect_button: ActionButton = %ConnectButton
@onready var _disconnect_button: ActionButton = %DisconnectButton
@onready var _ping_button: ActionButton = %PingButton
@onready var _demo_space_button: ActionButton = %DemoSpaceButton
@onready var _world_button: ActionButton = %WorldButton
@onready var _login_edit: LineEdit = %LoginEdit
@onready var _password_edit: LineEdit = %PasswordEdit
@onready var _login_button: ActionButton = %LoginButton
@onready var _register_button: ActionButton = %RegisterButton
@onready var _logout_button: ActionButton = %LogoutButton
@onready var _account_status: Label = %AccountStatus
@onready var _status_label: Label = %StatusLabel
@onready var _log: RichTextLabel = %Log


func _ready() -> void:
	theme = UiTheme.build()

	_client = WsClientScript.new()
	add_child(_client)
	_client.connected.connect(_on_connected)
	_client.connection_failed.connect(_on_failed)
	_client.disconnected.connect(_on_disconnected)
	_client.latency_updated.connect(_on_latency)
	_client.message_received.connect(_on_message)

	_connect_button.pressed.connect(_on_connect_pressed)
	_disconnect_button.pressed.connect(func() -> void:
		_manual_close = true
		_client.close())
	_ping_button.pressed.connect(func() -> void: _client.ping())
	_url_edit.text_submitted.connect(func(_t: String) -> void: _on_connect_pressed())
	_demo_space_button.pressed.connect(_open_demo_space)
	_world_button.pressed.connect(_enter_world)
	_login_button.pressed.connect(func() -> void: _account_action("login"))
	_register_button.pressed.connect(func() -> void: _account_action("register"))
	_logout_button.pressed.connect(_logout)
	_refresh_account()

	_url_edit.text = _load_url()
	_set_idle("Не подключено")
	_build_start_screen()
	if auto_connect:
		_on_connect_pressed.call_deferred()
		_updater.check.call_deferred()   # тихая проверка обновлений при запуске: окно только если обновление есть


## Стартовый экран поверх служебного: адрес сервера игроку не показывается. Служебный экран открывается пятью нажатиями на логотип.
func _build_start_screen() -> void:
	var safe := $SafeArea as Control
	safe.visible = false
	_overlay_bg = ColorRect.new()
	_overlay_bg.name = "OverlayBackdrop"
	_overlay_bg.color = Color(Tokens.BG, 0.97)
	_overlay_bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_overlay_bg.visible = false
	add_child(_overlay_bg)
	move_child(_overlay_bg, safe.get_index())
	_overlay_back = ActionButton.new()
	_overlay_back.text = "Назад"
	_overlay_back.custom_minimum_size = Vector2(0, 56)
	_overlay_back.pressed.connect(_close_overlay)
	$SafeArea/Column.add_child(_overlay_back)

	_retry_timer = Timer.new()
	_retry_timer.wait_time = 1.0
	_retry_timer.timeout.connect(_on_retry_tick)
	add_child(_retry_timer)

	_splash = StartSplash.new()
	_splash.name = "StartSplash"
	_splash.theme = theme
	add_child(_splash)
	# экраны-наложения (служебный, аккаунт) должны быть выше стартового
	move_child(_overlay_bg, get_child_count() - 1)
	move_child(safe, get_child_count() - 1)
	_splash.play_pressed.connect(_enter_world)
	_splash.retry_pressed.connect(_retry_now)
	_splash.account_pressed.connect(func() -> void: _open_overlay(false))
	_updater = Updater.new()
	add_child(_updater)
	_updater.finished.connect(func(res: Dictionary) -> void:
		_splash.show_update_result(res, _update_manual)
		_update_manual = false)
	_content = ContentLoader.new()
	add_child(_content)
	_content.progress.connect(func(_id: String, f: float) -> void: _splash.set_content_note("Загрузка контента… %d%%" % int(f * 100.0)))
	_content.finished.connect(func(res: Dictionary) -> void: _splash.set_content_note(_content_note(res)))
	_content.check.call_deferred()   # докачка файлов игры в фоне, без переустановки APK
	_splash.update_check_requested.connect(func() -> void:
		_update_manual = true
		_splash.begin_update_check()
		_updater.check()
		_content.check(true))
	_installer = ApkInstaller.new()
	add_child(_installer)
	_installer.progress.connect(func(f: float) -> void: _splash.show_update_progress("Скачивание обновления… %d%%" % int(f * 100.0), true))
	_installer.finished.connect(func(status: String, msg: String) -> void:
		if status == "fallback":
			if not _updater.open_download(_update_info):
				_append(_danger("Ссылка на обновление отклонена как небезопасная"))
			_splash.show_update_progress("", false)
		else:
			_splash.show_update_progress(msg, false))
	_splash.update_confirmed.connect(func(info: Dictionary) -> void:
		_update_info = info
		_splash.show_update_progress("Подготовка…", true)
		_installer.start(info))
	_splash.secret_opened.connect(func() -> void: _open_overlay(true))
	_sync_splash()


## Короткая строка о докачке контента для стартового экрана.
func _content_note(res: Dictionary) -> String:
	match str(res.get("status", "")):
		"downloaded":
			return "Контент обновлён — применится при следующем запуске"
		"error":
			return ""   # фоновая ошибка не мешает игре; подробности — в журнале сервера раздачи
		_:
			return ""


## Показ служебного экрана (dev=true: сервер, журнал) или только аккаунта.
func _open_overlay(dev: bool) -> void:
	$SafeArea/Column/TopRow/ConnectionCard.visible = dev
	$SafeArea/Column/LogCard.visible = dev
	_overlay_bg.visible = true
	($SafeArea as Control).visible = true


func _close_overlay() -> void:
	_overlay_bg.visible = false
	($SafeArea as Control).visible = false


func _retry_now() -> void:
	_retry_timer.stop()
	if _client.state == WsClientScript.State.IDLE:
		_on_connect_pressed()


func _schedule_retry() -> void:
	_retry_attempt += 1
	_retry_left = mini(2 + _retry_attempt * 2, 12)
	_retry_timer.start()
	_sync_splash()


func _on_retry_tick() -> void:
	_retry_left -= 1
	if _retry_left <= 0:
		_retry_timer.stop()
		if _client.state == WsClientScript.State.IDLE:
			_on_connect_pressed()
	_sync_splash()


## Подпись и состояние стартового экрана по состоянию соединения.
func _sync_splash() -> void:
	if _splash == null:
		return
	if _world_button != null and _world_button.is_busy():
		_splash.set_state(StartSplash.State.ENTERING, "Вход в космос…")
		return
	match _client.state:
		WsClientScript.State.OPEN:
			_splash.set_state(StartSplash.State.READY, "Сервер на связи")
		WsClientScript.State.IDLE:
			if _retry_timer != null and not _retry_timer.is_stopped():
				_splash.set_state(StartSplash.State.ERROR, "Нет связи с сервером. Повтор через %d с" % _retry_left)
			else:
				_splash.set_state(StartSplash.State.ERROR, "Нет связи с сервером")
		_:
			_splash.set_state(StartSplash.State.CONNECTING, "Подключение к серверу…")


## Регистрация или вход по логину и паролю (соединение уже открыто). Пароль нигде не сохраняется и не пишется в журнал.
func _account_action(kind: String) -> void:
	if _client.state != WsClientScript.State.OPEN or _account_pending:
		return
	var login := _login_edit.text.strip_edges()
	var password := _password_edit.text
	if login == "" or password == "":
		_account_status.text = "Введите логин и пароль."
		return
	_account_pending = true
	_acct_busy(true)
	_account_status.text = "Проверка…"
	_client.send_message(kind, {"token": PlayerIdentity.token(), "login": login, "password": password})
	_append("Запрос: %s (логин %s)" % ["создание аккаунта" if kind == "register" else "вход по аккаунту", login])


func _acct_busy(on: bool) -> void:
	_login_button.disabled = on or _client.state != WsClientScript.State.OPEN
	_register_button.disabled = _login_button.disabled
	if not on:
		_account_pending = false


func _logout() -> void:
	PlayerIdentity.reset_device()
	_append("Вы вышли из аккаунта: этот телефон теперь новый игрок. Прогресс вернётся входом по логину и паролю.")
	_refresh_account()


func _refresh_account() -> void:
	var login := PlayerIdentity.login()
	var open: bool = _client != null and _client.state == WsClientScript.State.OPEN
	_login_button.disabled = not open or _account_pending
	_register_button.disabled = _login_button.disabled or login != ""
	_logout_button.disabled = login == ""
	_account_status.text = ("Аккаунт: %s. Прогресс можно вернуть на новом телефоне входом по логину и паролю." % login) if login != "" else "Аккаунта нет: прогресс хранится только на этом телефоне. Создайте аккаунт, чтобы не потерять его при удалении игры. Восстановления пароля нет."


## Вход в общий космос: соединение передаётся серверному шлюзу, который выполняет join.
func _enter_world() -> void:
	if _client.state != WsClientScript.State.OPEN or _world_button.is_busy():
		return
	_world_button.begin_busy("Вход…")
	_sync_splash()
	var gw := ServerGateway.new()
	add_child(gw)
	gw.joined.connect(func() -> void:
		remove_child(gw)   # дальше шлюз живёт в сцене космоса
		Session.gateway = gw
		Session.opened_msec = Time.get_ticks_msec()
		get_tree().change_scene_to_file("res://scenes/space.tscn"))
	gw.character_needed.connect(func(options: Array) -> void:
		# новый игрок: создаём персонажа (ник, пол, направление пилота), затем сервер впускает в мир
		var cs: Control = CharacterScreen.new()
		cs.options = options
		cs.theme = theme
		# отдельный верхний слой: экран подключения под ним не просвечивает и не получает ввод
		var layer := CanvasLayer.new()
		layer.layer = 50
		add_child(layer)
		layer.add_child(cs)
		cs.submitted.connect(func(nick: String, g: String, p: String) -> void: gw.create_character(nick, g, p))
		gw.character_failed.connect(func(reason: String) -> void:
			if is_instance_valid(cs):
				cs.show_error(reason))
		gw.joined.connect(func() -> void:
			if is_instance_valid(cs):
				cs.get_parent().queue_free()))
	gw.join_failed.connect(func(reason: String) -> void:
		_append(_danger("Вход не удался: %s" % reason))
		_world_button.end_busy()
		_sync_splash()
		if _client.get_parent() == gw:
			gw.remove_child(_client)   # соединение возвращается экрану подключения
			add_child(_client)
		gw.queue_free())
	var url := _url_edit.text.strip_edges()
	gw.adopt(_client, url)


## Демонстрационный космос: данные не с сервера (интерфейс сам покажет плашку «Демо»).
func _open_demo_space() -> void:
	Session.gateway = DemoGateway.new()
	Session.opened_msec = Time.get_ticks_msec()
	get_tree().change_scene_to_file("res://scenes/space.tscn")


func _on_connect_pressed() -> void:
	if _connect_button.is_busy() or _client.state != WsClientScript.State.IDLE:
		return  # защита от повторного подключения
	var url := _url_edit.text.strip_edges()
	_save_url(url)
	_status("Подключение…", &"MutedLabel")
	_connect_button.begin_busy("Подключение…")
	_url_edit.editable = false
	_append("Подключение к %s" % url)
	_client.connect_to(url)
	_sync_splash()


func _on_connected(session_id: String, server_version: String) -> void:
	_status("Подключено (сервер %s)" % server_version, &"SuccessLabel")
	_append("Сессия %s" % session_id)
	_connect_button.end_busy()
	_connect_button.disabled = true  # уже подключены
	_disconnect_button.disabled = false
	_ping_button.disabled = false
	_world_button.disabled = false
	_retry_attempt = 0
	_retry_timer.stop() if _retry_timer != null else null
	_refresh_account()
	_sync_splash()


func _on_failed(reason: String) -> void:
	_append(_danger("Ошибка: %s" % reason))
	_set_idle("Ошибка подключения", &"DangerLabel")
	if auto_connect and _retry_timer != null:
		_schedule_retry()


func _on_disconnected(code: int, reason: String) -> void:
	_append("Соединение закрыто (код %d %s)" % [code, reason])
	_set_idle("Отключено")
	if auto_connect and _retry_timer != null and not _manual_close:
		_schedule_retry()


func _on_latency(rtt_ms: int) -> void:
	_append("Задержка: %d мс" % rtt_ms)


func _on_message(type: String, payload: Dictionary, _id: String) -> void:
	if type == "account":
		PlayerIdentity.set_login(str(payload.get("login", "")))
		_password_edit.text = ""   # пароль не хранится и не остаётся в поле
		_append("Аккаунт «%s»: %s" % [str(payload.get("login", "")), "создан, прогресс сохранён" if payload.get("created", false) else "вход выполнен"])
		_acct_busy(false)
		_refresh_account()
	elif type == "error":
		var text := str(payload.get("message", "ошибка"))
		if _account_pending:
			_account_pending = false
			_acct_busy(false)
			_account_status.text = text
		_append(_danger("Сервер: %s" % text))
	else:
		_append("Сообщение: %s" % type)


func _set_idle(text: String, variation: StringName = &"Label") -> void:
	_status(text, variation)
	_connect_button.end_busy()
	_connect_button.disabled = false
	_disconnect_button.disabled = true
	_ping_button.disabled = true
	_world_button.disabled = true
	_url_edit.editable = true
	_refresh_account()
	_sync_splash()


func _status(text: String, variation: StringName) -> void:
	_status_label.text = text
	_status_label.theme_type_variation = variation


func _danger(text: String) -> String:
	return "[color=%s]%s[/color]" % [Tokens.hex(Tokens.DANGER), text]


func _append(bbcode: String) -> void:
	_log.append_text(bbcode + "\n")


func _load_url() -> String:
	var cfg := ConfigFile.new()
	# сохранённый адрес действует только если его задали вручную на служебном экране; иначе всегда адрес по умолчанию
	if cfg.load(SETTINGS_PATH) == OK and bool(cfg.get_value("server", "custom", false)):
		return str(cfg.get_value("server", "url", DEFAULT_URL))
	return DEFAULT_URL


func _save_url(url: String) -> void:
	var cfg := ConfigFile.new()
	cfg.load(SETTINGS_PATH)
	cfg.set_value("server", "url", url)
	cfg.set_value("server", "custom", url != DEFAULT_URL)
	cfg.save(SETTINGS_PATH)
