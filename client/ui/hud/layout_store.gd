extends RefCounted
## Раскладка HUD: профили, нормализованные координаты безопасной зоны, импорт и экспорт.
## Игровые команды сюда не входят. Файл только на устройстве.

const VERSION := 1
const PATH := "user://hud_profiles.json"

## id, заголовок, категория. hideable=false нельзя спрятать (иначе пропадёт вход в настройки).
const CATALOG: Array = [
	{"id": "ship.joystick", "title": "Джойстик", "category": "ship", "hideable": true},
	{"id": "ship.bars", "title": "Щит, корпус и энергия", "category": "ship", "hideable": true},
	{"id": "ship.cargo", "title": "Груз", "category": "ship", "hideable": true},
	{"id": "ship.speed", "title": "Скорость", "category": "ship", "hideable": true},
	{"id": "player.card", "title": "Карточка игрока", "category": "player", "hideable": true},
	{"id": "player.quests", "title": "Задания", "category": "nav", "hideable": true},
	{"id": "player.credits", "title": "Кредиты", "category": "player", "hideable": true},
	{"id": "social.chat", "title": "Чат", "category": "social", "hideable": true},
	{"id": "social.party", "title": "Группа", "category": "social", "hideable": true},
	{"id": "nav.map", "title": "Карта", "category": "nav", "hideable": true},
	{"id": "nav.menu", "title": "Меню", "category": "nav", "hideable": false},
	{"id": "nav.camera", "title": "Вид камеры", "category": "nav", "hideable": true},
	{"id": "nav.demo", "title": "Плашка демо", "category": "info", "hideable": true},
	{"id": "combat.target", "title": "Выбранная цель", "category": "combat", "hideable": true},
	{"id": "combat.cycle", "title": "Выбор цели", "category": "combat", "hideable": true},
	{"id": "combat.attack", "title": "Основная атака", "category": "combat", "hideable": true},
	{"id": "combat.skill.0", "title": "Способность 1", "category": "combat", "hideable": true},
	{"id": "combat.skill.1", "title": "Способность 2", "category": "combat", "hideable": true},
	{"id": "combat.skill.2", "title": "Способность 3", "category": "combat", "hideable": true},
	{"id": "combat.skill.3", "title": "Способность 4", "category": "combat", "hideable": true},
	{"id": "combat.skill.4", "title": "Способность 5", "category": "combat", "hideable": true},
	{"id": "combat.skill.5", "title": "Способность 6", "category": "combat", "hideable": true},
	{"id": "combat.auto", "title": "Автобой", "category": "combat", "hideable": true},
	{"id": "combat.autopilot", "title": "Автопилот", "category": "combat", "hideable": true},
	{"id": "combat.mine", "title": "Добыча", "category": "combat", "hideable": true},
	{"id": "combat.sell", "title": "Продажа груза", "category": "combat", "hideable": true},
]

static var path: String = PATH


static func ids() -> PackedStringArray:
	var out := PackedStringArray()
	for row in CATALOG:
		out.append(str(row["id"]))
	return out


static func meta(id: String) -> Dictionary:
	for row in CATALOG:
		if str(row["id"]) == id:
			return row
	return {}


static func hideable(id: String) -> bool:
	var row := meta(id)
	return not row.is_empty() and bool(row.get("hideable", true))


static func default_doc() -> Dictionary:
	return {
		"version": VERSION,
		"active": "standard",
		"profiles": {
			"standard": _profile("Стандартный", true, {}),
			"minimal": _profile("Минималистичный", true, _minimal_items()),
			"combat": _profile("Боевой", true, _combat_items()),
		},
	}


static func _profile(title: String, builtin: bool, items: Dictionary) -> Dictionary:
	return {"name": title, "builtin": builtin, "items": items}


static func _hide(ids: Array) -> Dictionary:
	var items := {}
	for id in ids:
		items[str(id)] = {"visible": false}
	return items


static func _minimal_items() -> Dictionary:
	return _hide(["player.quests", "social.chat", "player.credits", "ship.speed", "ship.cargo", "social.party", "nav.demo"])


static func _combat_items() -> Dictionary:
	var items := _hide(["player.quests", "social.chat", "player.credits", "nav.demo"])
	items["combat.attack"] = {"scale": 1.12, "opacity": 1.0}
	items["combat.auto"] = {"scale": 1.05}
	return items


static func load_doc() -> Dictionary:
	var doc := _read(path)
	if doc.is_empty():
		doc = _read(path + ".bak")
	if doc.is_empty():
		doc = default_doc()
	return sanitize(doc)


static func save_doc(doc: Dictionary) -> bool:
	var clean := sanitize(doc)
	var text := JSON.stringify(clean)
	if text == "":
		return false
	if FileAccess.file_exists(path):
		var old := FileAccess.get_file_as_string(path)
		if old != "":
			var bak := FileAccess.open(path + ".bak", FileAccess.WRITE)
			if bak != null:
				bak.store_string(old)
				bak.close()
	var f := FileAccess.open(path, FileAccess.WRITE)
	if f == null:
		return false
	f.store_string(text)
	f.close()
	return true


static func export_text(doc: Dictionary) -> String:
	return JSON.stringify(sanitize(doc), "\t")


static func import_text(text: String) -> Dictionary:
	var parsed: Variant = JSON.parse_string(text)
	if not (parsed is Dictionary):
		return {"ok": false, "error": "Файл не является JSON-объектом"}
	var doc: Dictionary = parsed
	if int(doc.get("version", 0)) > VERSION:
		return {"ok": false, "error": "Версия раскладки новее игры"}
	if int(doc.get("version", 0)) <= 0:
		return {"ok": false, "error": "В файле нет версии раскладки"}
	return {"ok": true, "doc": sanitize(doc)}


static func sanitize(raw: Dictionary) -> Dictionary:
	var base := default_doc()
	var profiles_in: Dictionary = raw.get("profiles", {}) if raw.get("profiles") is Dictionary else {}
	var profiles: Dictionary = base["profiles"]
	for pid in profiles_in.keys():
		var src: Variant = profiles_in[pid]
		if not (src is Dictionary):
			continue
		var id := str(pid)
		if id == "":
			continue
		var builtin := id in ["standard", "minimal", "combat"]
		var items_in: Dictionary = src.get("items", {}) if src.get("items") is Dictionary else {}
		var items := {}
		for eid in items_in.keys():
			var sid := str(eid)
			if meta(sid).is_empty():
				continue
			var one: Variant = items_in[eid]
			if one is Dictionary:
				items[sid] = clamp_item(sid, one)
		var title := str(src.get("name", profiles.get(id, {}).get("name", "Свой")))
		if title == "":
			title = "Свой"
		profiles[id] = {"name": title, "builtin": builtin, "items": items}
	for fixed in ["standard", "minimal", "combat"]:
		if not profiles.has(fixed):
			profiles[fixed] = base["profiles"][fixed]
		profiles[fixed]["builtin"] = true
	var active := str(raw.get("active", "standard"))
	if not profiles.has(active):
		active = "standard"
	return {"version": VERSION, "active": active, "profiles": profiles}


static func clamp_item(id: String, item: Dictionary) -> Dictionary:
	var out := {}
	if item.has("nx"):
		out["nx"] = clampf(float(item["nx"]), -0.25, 1.25)
	if item.has("ny"):
		out["ny"] = clampf(float(item["ny"]), -0.25, 1.25)
	if item.has("scale"):
		out["scale"] = clampf(float(item["scale"]), 0.5, 2.0)
	if item.has("opacity"):
		out["opacity"] = clampf(float(item["opacity"]), 0.2, 1.0)
	if item.has("visible"):
		out["visible"] = bool(item["visible"])
	if item.has("z"):
		out["z"] = clampi(int(item["z"]), 0, 30)
	if item.has("locked"):
		out["locked"] = bool(item["locked"])
	if not hideable(id):
		out["visible"] = true
	return out


static func active_items(doc: Dictionary) -> Dictionary:
	var profiles: Dictionary = doc.get("profiles", {})
	var pid := str(doc.get("active", "standard"))
	var profile: Dictionary = profiles.get(pid, {})
	var items: Variant = profile.get("items", {})
	return items if items is Dictionary else {}


static func _read(p: String) -> Dictionary:
	if not FileAccess.file_exists(p):
		return {}
	var parsed: Variant = JSON.parse_string(FileAccess.get_file_as_string(p))
	if parsed is Dictionary and int(parsed.get("version", 0)) >= 1 and int(parsed.get("version", 0)) <= VERSION:
		return parsed
	return {}
