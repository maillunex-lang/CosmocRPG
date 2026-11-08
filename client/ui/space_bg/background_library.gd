extends RefCounted
## Каталог фонов: реестр текстур (ключ -> путь) и профили систем. Данные лежат в JSON
## (res://assets/space/backgrounds.json) и расширяются без выпуска APK: загрузчик контента
## (ресурсные наборы) вызывает register_texture / register_profile.
## Если текстуры нет или она не загрузилась, подставляется заглушка, а ключ попадает в missing_assets().

const Placeholders := preload("res://ui/space_bg/placeholders.gd")
const Profile := preload("res://ui/space_bg/space_background_profile.gd")

const CATALOG_PATH := "res://assets/space/backgrounds.json"
const DEFAULT_PROFILE := "xi"

static var _texture_paths: Dictionary = {}   # ключ -> путь
static var _profiles: Dictionary = {}        # id -> словарь профиля (исходные данные)
static var _cache: Dictionary = {}           # ключ -> Texture2D
static var _missing: Dictionary = {}         # ключ -> причина
static var _loaded := false
static var _catalog_errors: PackedStringArray = PackedStringArray()


## Сброс состояния (тесты).
static func reset() -> void:
	_texture_paths.clear()
	_profiles.clear()
	_cache.clear()
	_missing.clear()
	_catalog_errors = PackedStringArray()
	_loaded = false


## Загрузка каталога. Повторный вызов с тем же путём ничего не меняет. Возвращает true, если каталог корректен.
static func load_catalog(path: String = CATALOG_PATH) -> bool:
	if _loaded and path == CATALOG_PATH:
		return _catalog_errors.is_empty()
	_catalog_errors = PackedStringArray()
	var text := FileAccess.get_file_as_string(path)
	if text == "":
		_catalog_errors.append("каталог не найден или пуст: %s" % path)
		_loaded = true
		return false
	var parsed: Variant = JSON.parse_string(text)
	if typeof(parsed) != TYPE_DICTIONARY:
		_catalog_errors.append("каталог: некорректный JSON")
		_loaded = true
		return false
	var data: Dictionary = parsed
	var tex: Variant = data.get("textures", {})
	if tex is Dictionary:
		for k in tex:
			_texture_paths[str(k)] = str(tex[k])
	else:
		_catalog_errors.append("textures: ожидается объект")
	var profs: Variant = data.get("profiles", [])
	if profs is Array:
		for p in profs:
			if p is Dictionary and str(p.get("id", "")) != "":
				_profiles[str(p["id"])] = p
			else:
				_catalog_errors.append("профиль без id пропущен")
	else:
		_catalog_errors.append("profiles: ожидается массив")
	_loaded = true
	return _catalog_errors.is_empty()


static func catalog_errors() -> PackedStringArray:
	return _catalog_errors


## Добавить/заменить текстуру (например, из скачанного ресурсного набора).
static func register_texture(key: String, path: String) -> void:
	_texture_paths[key] = path
	_cache.erase(key)
	_missing.erase(key)


## Добавить/заменить профиль системы (данные из админки или ресурсного набора).
static func register_profile(d: Dictionary) -> void:
	if str(d.get("id", "")) != "":
		_profiles[str(d["id"])] = d


static func profile_ids() -> Array:
	load_catalog()
	return _profiles.keys()


## Профиль по идентификатору. Неизвестный id даёт профиль по умолчанию, а если и его нет, профиль из заглушек.
static func profile(id: String) -> Resource:
	load_catalog()
	if _profiles.has(id):
		return Profile.from_dict(_profiles[id])
	if id != "":
		push_warning("Фон: профиль «%s» не найден, используется «%s»" % [id, DEFAULT_PROFILE])
	if _profiles.has(DEFAULT_PROFILE):
		return Profile.from_dict(_profiles[DEFAULT_PROFILE])
	return Profile.from_dict(_placeholder_profile())


static func _placeholder_profile() -> Dictionary:
	return {"id": "placeholder", "name": "Техническая заглушка", "layers": [
		{"texture": "missing_stars", "blend": "add", "parallax": 0.1, "zoom": 1.15, "wrap": "mirror"},
		{"texture": "missing_nebula", "blend": "add", "parallax": 0.25, "zoom": 1.2},
		{"texture": "missing_gas", "blend": "mix", "parallax": 0.5, "zoom": 1.3, "intensity": 0.5},
	]}


## Текстура по ключу. role: stars | nebula | gas (для вида заглушки).
static func texture(key: String, role: String = "stars") -> Texture2D:
	load_catalog()
	if _cache.has(key):
		return _cache[key]
	var path: String = _texture_paths.get(key, "")
	var tex: Texture2D = null
	if path == "":
		_missing[key] = "ключ не зарегистрирован в каталоге"
	elif not ResourceLoader.exists(path):
		_missing[key] = "файл не найден: %s" % path
	else:
		tex = load(path) as Texture2D
		if tex == null:
			_missing[key] = "не удалось загрузить: %s" % path
	if tex == null:
		tex = Placeholders.make(role)
		push_warning("Фон: текстура «%s» отсутствует, подставлена заглушка (%s)" % [key, _missing[key]])
	_cache[key] = tex
	return tex


## Ключи, для которых подставлены заглушки (недостающие ассеты).
static func missing_assets() -> Dictionary:
	return _missing.duplicate()
