extends RefCounted
## Токен устройства: случайный, создаётся при первом запуске и хранится локально (user://identity.cfg).
## Сервер хранит только SHA-256 токена. Токен = игрок: по нему возвращаются координаты корабля.
## Не логировать и не показывать токен.

const PATH := "user://identity.cfg"

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
