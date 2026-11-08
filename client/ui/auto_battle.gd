extends RefCounted
## Автобой (R-48): при включённой кнопке «Авто» игра сама выбирает ближайшую подходящую цель и начинает атаку.
## Кого атаковать, задаёт настройка «auto_target_mode»: только враги / враги + вражеские игроки / все корабли подряд.
## Чистая логика выбора (без сети): команды отправляет шлюз.

const UiSettings := preload("res://ui/ui_settings.gd")

const SETTING_KEY := "auto_target_mode"
const MODE_ENEMIES := "enemies"
const MODE_ENEMIES_PLAYERS := "enemies_players"
const MODE_ALL := "all"
const MODES := [MODE_ENEMIES, MODE_ENEMIES_PLAYERS, MODE_ALL]
const RANGE_FACTOR := 1.25   # радиус поиска не меньше 1,25 дальности залпа
const MIN_SEARCH := 250.0     # и не меньше 250 единиц (25 км): цель выделяется заранее, огонь откроется, когда она подойдёт на дальность залпа


static func mode() -> String:
	var m := UiSettings.get_string(SETTING_KEY, MODE_ENEMIES)
	return m if m in MODES else MODE_ENEMIES


## Подходит ли объект как цель автобоя в режиме mode.
static func eligible(obj: Dictionary, mode_name: String) -> bool:
	if bool(obj.get("dead", false)):
		return false
	var stance := str(obj.get("stance", "neutral"))
	if stance == "friendly":
		return false
	match str(obj.get("kind", "")):
		"hostile":
			return true
		"player", "ai":
			return mode_name == MODE_ALL or (mode_name == MODE_ENEMIES_PLAYERS and stance == "hostile")
	return false   # станции, месторождения, аномалии и прочее не атакуются


## Идентификатор цели для автобоя: текущая цель, если она подходит, иначе ближайшая подходящая в радиусе поиска ("" — нет).
## skip: id -> время (мс), до которого цель пропускается (например, сервер отклонил атаку).
static func pick(state: Dictionary, mode_name: String, now_ms: int, skip: Dictionary = {}) -> String:
	var ship: Dictionary = state.get("ship", {})
	if not ship.has("pos"):
		return ""
	var reach := maxf(float(ship.get("pulse_range", 0.0)) * RANGE_FACTOR, MIN_SEARCH)
	var pos: Vector2 = ship["pos"]
	var current := str(state.get("target_id", ""))
	var best := ""
	var best_d := INF
	for o in state.get("objects", []):
		var id := str(o.get("id", ""))
		if int(skip.get(id, 0)) > now_ms or not eligible(o, mode_name):
			continue
		var d := pos.distance_to(o["pos"])
		if id == current and d <= reach:
			return id
		if d <= reach and d < best_d:
			best_d = d
			best = id
	return best
