extends RefCounted
## Описание корабля для клиента (assets/ships/<id>/ship.json): облики, точки привязки, габариты, подписи.
## Игровые характеристики и способности принадлежат серверу; здесь только то, что нужно для отображения.

const PATHS := {"FTR-001": "res://assets/ships/ftr001/ship.json"}
const DEFAULT_ID := "FTR-001"

static var _cache: Dictionary = {}


static func get_def(ship_id: String = DEFAULT_ID) -> Dictionary:
	var id := ship_id if PATHS.has(ship_id) else DEFAULT_ID
	if _cache.has(id):
		return _cache[id]
	var parsed: Variant = JSON.parse_string(FileAccess.get_file_as_string(PATHS[id]))
	var d: Dictionary = parsed if typeof(parsed) == TYPE_DICTIONARY else {}
	_cache[id] = d
	return d


static func skin_ids(ship_id: String = DEFAULT_ID) -> Array:
	return (get_def(ship_id).get("skins", {}) as Dictionary).keys()


static func skin(ship_id: String, skin_id: String) -> Dictionary:
	var skins: Dictionary = get_def(ship_id).get("skins", {})
	return skins.get(skin_id, skins.get(skin_ids(ship_id)[0], {}))


static func is_ship_kind(kind: String) -> bool:
	return kind in ["ship", "player", "ai", "hostile"]
