extends RefCounted
## Правила эффектов М-3. Каталог и сумма — как у музыки; колонки и сеть — в sfx_player.gd.
## Роли задаются тегами библиотеки: ui, engine, weapon, hit. Файлы в APK не вшиваются.

const Music := preload("res://audio/music.gd")

const ROLES: Array[String] = ["ui", "engine", "weapon", "hit"]
const ENGINE_SPEED := 0.08


static func cues(catalog: Dictionary) -> Dictionary:
	var buckets := {"ui": [], "engine": [], "weapon": [], "hit": []}
	var raw: Variant = catalog.get("tracks", [])
	if typeof(raw) != TYPE_ARRAY:
		return buckets
	for item in raw:
		if typeof(item) != TYPE_DICTIONARY:
			continue
		var t: Dictionary = item
		if str(t.get("kind", "")) != "sfx":
			continue
		if not bool(t.get("enabled", true)):
			continue
		var ext := str(t.get("ext", ""))
		if ext not in ["mp3", "ogg"]:
			continue
		var sha := str(t.get("sha256", "")).to_lower()
		if not Music.is_track_file(ext, sha):
			continue
		var path := "/media/files/%s.%s" % [sha, ext]
		if str(t.get("url", path)) != path:
			continue
		var roles := roles_in(str(t.get("tags", "")))
		if roles.is_empty():
			continue
		var copy := t.duplicate()
		copy["url"] = path
		copy["sha256"] = sha
		copy["ext"] = ext
		for role in roles:
			(buckets[role] as Array).append(copy)
	for role in ROLES:
		(buckets[role] as Array).sort_custom(func(a: Dictionary, b: Dictionary) -> bool:
			return str(a["sha256"]) < str(b["sha256"]))
	return buckets


static func roles_in(tags: String) -> Array[String]:
	var out: Array[String] = []
	var norm := tags.to_lower().replace(",", " ").replace(";", " ")
	for part in norm.split(" ", false):
		var p := str(part).strip_edges()
		if p in ROLES and p not in out:
			out.append(p)
	return out


static func pick(bucket: Dictionary, role: String) -> Dictionary:
	var list: Variant = bucket.get(role, [])
	if typeof(list) != TYPE_ARRAY or (list as Array).is_empty():
		return {}
	var first: Variant = (list as Array)[0]
	if typeof(first) != TYPE_DICTIONARY:
		return {}
	return first


static func allow(now_ms: int, last_ms: int, gap_ms: int) -> bool:
	if last_ms < 0:
		return true
	return now_ms - last_ms >= gap_ms


static func engine_active(speed_frac: float, boosting: bool) -> bool:
	return boosting or speed_frac > ENGINE_SPEED


static func engine_pitch(speed_frac: float, boosting: bool) -> float:
	var p := 0.82 + clampf(speed_frac, 0.0, 1.6) * 0.28
	if boosting:
		p += 0.18
	return clampf(p, 0.75, 1.45)


## Свой выстрел и попадание по себе. Чужие выстрелы, в том числе ИИ, остаются только картинкой.
static func cue_for(event_type: String, from_id: String, to_id: String, me_ids: Array) -> String:
	var from_me := from_id != "" and from_id in me_ids
	var to_me := to_id != "" and to_id in me_ids
	match event_type:
		"fire", "intercept":
			if from_me:
				return "weapon"
		"hit_shield", "hit_hull":
			if to_me:
				return "hit"
	return ""
