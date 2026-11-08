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


static func get_string(key: String, default: String) -> String:
	if _overrides.has(key):
		return str(_overrides[key])
	var cfg := ConfigFile.new()
	if cfg.load(PATH) == OK:
		return str(cfg.get_value(SECTION, key, default))
	return default


static func set_string(key: String, value: String) -> void:
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


static func get_float(key: String, default: float) -> float:
	if _overrides.has(key):
		return float(_overrides[key])
	var cfg := ConfigFile.new()
	if cfg.load(PATH) == OK:
		return float(cfg.get_value(SECTION, key, default))
	return default


static func set_float(key: String, value: float) -> void:
	var cfg := ConfigFile.new()
	cfg.load(PATH)
	cfg.set_value(SECTION, key, value)
	cfg.save(PATH)


## Пресет графики: low | medium | high (по умолчанию medium). Экран настроек появится в UI-11.
static func graphics_preset() -> String:
	var v := "medium"
	if _overrides.has("graphics_preset"):
		v = str(_overrides["graphics_preset"])
	else:
		var cfg := ConfigFile.new()
		if cfg.load(PATH) == OK:
			v = str(cfg.get_value(SECTION, "graphics_preset", "medium"))
	return v if v in ["low", "medium", "high"] else "medium"


## Уменьшение анимаций (доступность).
static func reduce_motion() -> bool:
	return get_bool("reduce_motion", false)


## Показывать ли виртуальный джойстик (по умолчанию да, опционален).
static func joystick_enabled() -> bool:
	return get_bool("joystick", true)
