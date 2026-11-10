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
				if kind == target:
					return true
				if mark != "" and kind == mark and (target == "" or target == kind or target == id):
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


## Сюжетное задание, к которому сейчас вести автопилот. Побочные не перебивают главу.
static func guide_quest(state: Dictionary) -> Dictionary:
	var cur := _system_id(state)
	var side := {}
	for raw in state.get("quests", []):
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var q: Dictionary = raw
		var sys := str(q.get("system", ""))
		var kind := str(q.get("kind", ""))
		if sys != "" and cur != "" and sys != cur and kind != "jump":
			continue
		if str(q.get("id", "")).begins_with("aur_side_"):
			if side.is_empty():
				side = q
			continue
		return q
	return side


## Есть ли у ведущего задания точка, к которой можно полететь прямо сейчас.
static func can_route(state: Dictionary, layout: Dictionary = {}) -> bool:
	var q := guide_quest(state)
	if q.is_empty():
		return false
	if str(q.get("kind", "")) == "jump" and str(q.get("target", "")) != "":
		return true
	if object_id(state, q) != "":
		return true
	return layout_destination(layout, q) is Vector2


## Идентификатор объекта мира для задания. Пустая строка: в снимке его нет.
static func object_id(state: Dictionary, q: Dictionary) -> String:
	var target := str(q.get("target", ""))
	var marker := str(q.get("marker", ""))
	var kind := str(q.get("kind", ""))
	var me := Vector2.ZERO
	var ship: Variant = state.get("ship", {})
	if ship is Dictionary and (ship as Dictionary).get("pos") is Vector2:
		me = (ship as Dictionary)["pos"]
	var best := ""
	var best_d := 1.0e12
	for raw in state.get("objects", []):
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var o: Dictionary = raw
		if bool(o.get("dead", false)):
			continue
		var id := str(o.get("id", ""))
		var okind := str(o.get("kind", ""))
		if target != "" and id == target:
			return id
		var hit := false
		if marker != "" and okind == marker and (target == "" or target == okind or target == id):
			hit = true
		elif kind == "kill" and okind == "hostile" and target == "":
			hit = true
		elif kind == "mine" and okind == "deposit" and (target == "" or str(o.get("resource", "")) == target):
			hit = true
		elif kind == "visit" and (okind == target or (marker != "" and okind == marker and (target == "" or target == okind or target == id))):
			hit = true
		elif kind in ["sell", "deliver", "talk", "report"] and okind == "station":
			hit = true
		elif kind == "reach" and okind == "station" and (target == "" or target == "station"):
			hit = true
		if not hit:
			continue
		var pos: Variant = o.get("pos", Vector2.ZERO)
		var d := 0.0
		if pos is Vector2:
			d = me.distance_to(pos)
		if d < best_d:
			best_d = d
			best = id
	return best


## Точка из раскладки, если живой объект в снимок не попал.
static func layout_destination(layout: Dictionary, q: Dictionary) -> Variant:
	var target := str(q.get("target", ""))
	var kind := str(q.get("kind", ""))
	if target != "" and kind != "mine" and kind != "jump":
		var at: Variant = _layout_id(layout, target)
		if at is Vector2:
			return at
	if kind == "mine":
		for raw in layout.get("deposits", []):
			if typeof(raw) != TYPE_DICTIONARY:
				continue
			var d: Dictionary = raw
			if target == "" or str(d.get("resource", "")) == target:
				var pos: Variant = _layout_vec(d.get("pos"))
				if pos is Vector2:
					return pos
	if kind in ["sell", "deliver", "talk", "report", "reach"]:
		if target != "" and target != "station":
			var named: Variant = _layout_id(layout, target)
			if named is Vector2:
				return named
		for raw in layout.get("stations", []):
			if typeof(raw) != TYPE_DICTIONARY:
				continue
			var pos2: Variant = _layout_vec((raw as Dictionary).get("pos"))
			if pos2 is Vector2:
				return pos2
	return null


static func _system_id(state: Dictionary) -> String:
	var system: Variant = state.get("system", {})
	if system is Dictionary:
		return str((system as Dictionary).get("id", ""))
	return str(system)


static func _layout_id(layout: Dictionary, id: String) -> Variant:
	for key in ["points", "stations", "deposits"]:
		for raw in layout.get(key, []):
			if typeof(raw) != TYPE_DICTIONARY:
				continue
			var d: Dictionary = raw
			if str(d.get("id", "")) == id:
				return _layout_vec(d.get("pos"))
	return null


static func _layout_vec(p: Variant) -> Variant:
	if p is Vector2:
		return p
	if p is Array and p.size() >= 2:
		return Vector2(float(p[0]), float(p[1]))
	return null

