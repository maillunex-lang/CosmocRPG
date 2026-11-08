extends RefCounted
## Токен устройства: случайный, создаётся при первом запуске и хранится локально (user://identity.cfg).
## Сервер хранит только SHA-256 токена. Токен = игрок: по нему возвращаются координаты корабля.
## Не логировать и не показывать токен.

static var PATH := "user://identity.cfg"   # в тестах подменяется, чтобы не трогать настоящую личность

static var _override := ""   # тесты


static func token() -> String:
	if _override != "":
		return _override
	var cfg := ConfigFile.new()
	if cfg.load(PATH) == OK:
		var t := str(cfg.get_value("player", "token", ""))
		if t.length() >= 16:
			return t
	var t := Crypto.new().generate_random_bytes(16).hex_encode()
	cfg.set_value("player", "token", t)
	cfg.save(PATH)
	return t


static func set_override(t: String) -> void:
	_override = t


## Логин сохранённого аккаунта (пароль не хранится нигде). "" — аккаунта нет.
static func login() -> String:
	var cfg := ConfigFile.new()
	if cfg.load(PATH) == OK:
		return str(cfg.get_value("player", "login", ""))
	return ""


static func set_login(l: String) -> void:
	var cfg := ConfigFile.new()
	cfg.load(PATH)
	cfg.set_value("player", "login", l)
	cfg.save(PATH)


## Выход из аккаунта: устройство получает новый случайный токен (новый игрок), логин забывается.
## Прогресс остаётся на сервере и возвращается входом по логину и паролю.
static func reset_device() -> void:
	_override = ""
	var cfg := ConfigFile.new()
	cfg.set_value("player", "token", Crypto.new().generate_random_bytes(16).hex_encode())
	cfg.set_value("player", "login", "")
	cfg.save(PATH)
