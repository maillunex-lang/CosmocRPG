extends RefCounted
## Подсказки карты по активному заданию. Карта только рисует то, что уже прислал сервер.


static func text(state: Dictionary) -> String:
	var quests: Array = state.get("quests", [])
	if quests.is_empty() or typeof(quests[0]) != TYPE_DICTIONARY:
		return ""
	return str((quests[0] as Dictionary).get("text", ""))


static func marker(state: Dictionary) -> String:
	var quests: Array = state.get("quests", [])
	if quests.is_empty() or typeof(quests[0]) != TYPE_DICTIONARY:
		return ""
	return str((quests[0] as Dictionary).get("marker", ""))


## Жёлтая метка задания: объект или ворота, куда сейчас имеет смысл лететь.
static func marked(state: Dictionary, id: String, kind: String, extra: String = "") -> bool:
	var cur := ""
	var system: Variant = state.get("system", {})
	if system is Dictionary:
		cur = str((system as Dictionary).get("id", ""))
	for raw in state.get("quests", []):
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var q: Dictionary = raw
		var sys := str(q.get("system", ""))
		if sys != "" and cur != "" and sys != cur:
			continue
		var target := str(q.get("target", ""))
		var qkind := str(q.get("kind", ""))
		var mark := str(q.get("marker", ""))
		if id != "" and target != "" and target == id:
			return true
		if kind == "gate" and target != "" and target == extra:
			return true
		if mark != "" and mark == kind and (target == "" or target == kind or target == extra or target == id):
			return true
		match qkind:
			"kill":
				if kind == "hostile" and (target == "" or target == id):
					return true
			"mine":
				if kind == "deposit" and (target == "" or target == extra):
					return true
			"sell":
				if kind == "station":
					return true
			"reach":
				if kind == "station" and (target == "" or target == id or target == "station"):
					return true
			"deliver", "talk", "report":
				if kind == "station" and (target == "" or target == id or target == "station" or mark == "station" or mark == ""):
					return true
			"visit":
				if kind == target or (mark != "" and kind == mark):
					return true
			"jump":
				if kind == "gate" and (target == "" or target == extra):
					return true
	return false


## Строка над обычной подсказкой карты. Пустая, если задания и журнала нет.
static func prefix(state: Dictionary) -> String:
	var lines: PackedStringArray = []
	var quest := text(state)
	if quest != "":
		lines.append(quest)
	var places: Array = state.get("locations", [])
	if places.size() > 0:
		lines.append("Журнал локаций: %d" % places.size())
	return "\n".join(lines)
