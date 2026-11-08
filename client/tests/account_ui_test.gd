extends SceneTree
## Аккаунт на экране подключения: создание, вход «с нового телефона», ошибки, выход. Нужен сервер (COSMOC_TEST_WS_URL).

const PlayerIdentity := preload("res://ui/player_identity.gd")
const ServerGateway := preload("res://ui/gateway/server_gateway.gd")

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
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if url == "":
		print("COSMOC_TEST_WS_URL не задана: проверка аккаунта пропущена")
		quit(0)
		return
	PlayerIdentity.PATH = "user://identity_test.cfg"
	DirAccess.remove_absolute(ProjectSettings.globalize_path(PlayerIdentity.PATH))
	var s: Control = load("res://scenes/connect.tscn").instantiate()
	s.set("auto_connect", false)
	root.add_child(s)
	await process_frame
	await process_frame
	s.call("_open_overlay", false)   # игрок видит форму аккаунта на экране «Аккаунт»
	await process_frame
	await process_frame
	var rect := (s.get_node("%AccountStatus") as Control).get_global_rect()
	_c(rect.end.y <= 720.0 and rect.end.x <= 1280.0, "блок аккаунта помещается на экране (низ %.0f)" % rect.end.y)
	_c((s.get_node("%LoginButton") as Button).disabled and (s.get_node("%RegisterButton") as Button).disabled, "без соединения вход и регистрация недоступны")
	_c((s.get_node("%PasswordEdit") as LineEdit).secret, "пароль скрыт при вводе")
	(s.get_node("%UrlEdit") as LineEdit).text = url
	s.get_node("%ConnectButton").pressed.emit()
	await _wait(func() -> bool: return not (s.get_node("%RegisterButton") as Button).disabled)
	_c(not (s.get_node("%LoginButton") as Button).disabled and not (s.get_node("%RegisterButton") as Button).disabled, "после подключения кнопки аккаунта доступны")

	var login := "test_" + str(Time.get_ticks_usec())
	var pw := "пароль-для-теста-42"
	var le := s.get_node("%LoginEdit") as LineEdit
	var pe := s.get_node("%PasswordEdit") as LineEdit
	var st := s.get_node("%AccountStatus") as Label
	le.text = login
	s.get_node("%RegisterButton").pressed.emit()
	await process_frame
	_c(PlayerIdentity.login() == "" and st.text.contains("Введите логин и пароль"), "без пароля запрос не отправляется")
	pe.text = pw
	s.get_node("%RegisterButton").pressed.emit()
	await _wait(func() -> bool: return PlayerIdentity.login() != "")
	_c(PlayerIdentity.login() == login, "аккаунт создан, логин сохранён локально")
	_c(pe.text == "", "пароль очищен из поля и нигде не хранится")
	_c(st.text.contains("Аккаунт: " + login) and (s.get_node("%RegisterButton") as Button).disabled and not (s.get_node("%LogoutButton") as Button).disabled, "статус аккаунта и кнопки обновились")
	_c(not FileAccess.get_file_as_string(PlayerIdentity.PATH).contains(pw), "пароль не записан в файл личности")

	# игрок первого устройства
	var gw1 := ServerGateway.new()
	gw1.character_needed.connect(func(_o: Array) -> void: gw1.create_character("Tst_" + login.right(9), "m", "miner"))
	root.add_child(gw1)
	gw1.connect_to_server(url)
	await _wait(func() -> bool: return gw1.player_id != "")
	var pid1 := gw1.player_id
	_c(pid1 != "", "первое устройство вошло в мир (игрок %s)" % pid1.left(6))
	gw1.close()
	await process_frame

	# «приложение удалено»: выход даёт новый токен
	var tok1 := PlayerIdentity.token()
	s.get_node("%LogoutButton").pressed.emit()
	_c(PlayerIdentity.login() == "" and PlayerIdentity.token() != tok1, "выход: новый токен устройства, логин забыт")
	_c(st.text.contains("Аккаунта нет") and (s.get_node("%LogoutButton") as Button).disabled, "после выхода аккаунта нет")
	le.text = login
	pe.text = "неверный-пароль-123"
	s.get_node("%LoginButton").pressed.emit()
	await _wait(func() -> bool: return st.text.contains("неверный"))
	_c(st.text.contains("неверный логин или пароль") and PlayerIdentity.login() == "", "неверный пароль отклонён сервером")
	pe.text = pw
	s.get_node("%LoginButton").pressed.emit()
	await _wait(func() -> bool: return PlayerIdentity.login() != "")
	_c(PlayerIdentity.login() == login, "вход по логину и паролю выполнен")
	var gw2 := ServerGateway.new()
	root.add_child(gw2)
	gw2.connect_to_server(url)
	await _wait(func() -> bool: return gw2.player_id != "")
	_c(gw2.player_id == pid1, "на новом устройстве тот же игрок и тот же прогресс")
	gw2.close()
	s.queue_free()
	DirAccess.remove_absolute(ProjectSettings.globalize_path(PlayerIdentity.PATH))
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
