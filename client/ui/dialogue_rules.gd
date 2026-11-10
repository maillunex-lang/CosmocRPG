extends RefCounted
## Когда на экране космоса показывать разговор. Сервер всё равно не примет ответ вне станции.


static func talk_quest(state: Dictionary) -> Dictionary:
	var quests: Array = state.get("quests", [])
	if quests.is_empty() or typeof(quests[0]) != TYPE_DICTIONARY:
		return {}
	var q: Dictionary = quests[0]
	if str(q.get("kind", "")) != "talk":
		return {}
	var ship: Dictionary = state.get("ship", {})
	if not bool(ship.get("in_safe", false)):
		return {}
	var want := str(q.get("system", ""))
	var sys: Variant = state.get("system", {})
	var here := ""
	if typeof(sys) == TYPE_DICTIONARY:
		here = str((sys as Dictionary).get("id", ""))
	elif typeof(sys) == TYPE_STRING:
		here = str(sys)
	if want != "" and want != here:
		return {}
	return q
