extends RefCounted
## Когда на экране космоса показывать разговор. Сервер всё равно не примет ответ вне станции.


static func talk_quest(state: Dictionary) -> Dictionary:
	var quests: Array = state.get("quests", [])
	var ship: Dictionary = state.get("ship", {})
	if not bool(ship.get("in_safe", false)):
		return {}
	var here := ""
	var sys: Variant = state.get("system", {})
	if typeof(sys) == TYPE_DICTIONARY:
		here = str((sys as Dictionary).get("id", ""))
	elif typeof(sys) == TYPE_STRING:
		here = str(sys)
	for raw in quests:
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var q: Dictionary = raw
		if str(q.get("kind", "")) != "talk":
			continue
		var want := str(q.get("system", ""))
		if want != "" and want != here:
			continue
		return q
	return {}
