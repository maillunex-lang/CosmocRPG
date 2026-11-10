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
