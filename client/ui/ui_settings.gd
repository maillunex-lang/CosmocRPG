extends RefCounted
## Пользовательские настройки интерфейса (только удобство, не игровые данные):
## хранятся локально в user://settings.cfg, раздел [ui]. Экран настроек появится в UI-11.

const PATH := "user://settings.cfg"
const SECTION := "ui"

static var _overrides: Dictionary = {}  # для тестов: ключ -> значение


static func get_bool(key: String, default: bool) -> bool:
	if _overrides.has(key):
		return bool(_overrides[key])
	var cfg := ConfigFile.new()
	if cfg.load(PATH) == OK:
		return bool(cfg.get_value(SECTION, key, default))
	return default


static func set_bool(key: String, value: bool) -> void:
	var cfg := ConfigFile.new()
	cfg.load(PATH)
	cfg.set_value(SECTION, key, value)
	cfg.save(PATH)


## Тестовое переопределение без записи на диск (null снимает его).
static func override(key: String, value: Variant) -> void:
	if value == null:
		_overrides.erase(key)
	else:
		_overrides[key] = value


## Уменьшение анимаций (доступность).
static func reduce_motion() -> bool:
	return get_bool("reduce_motion", false)


## Показывать ли виртуальный джойстик (по умолчанию да, опционален).
static func joystick_enabled() -> bool:
	return get_bool("joystick", true)
