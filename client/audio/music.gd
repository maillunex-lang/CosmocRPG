extends RefCounted
## Чистые правила плеера М-1: адрес сервера, каталог, проверка суммы. Сеть и звук — в music_player.gd.


## ws(s)://хост[:порт]/ws -> http(s)://хост[:порт]. Пусто, если адрес нельзя использовать для скачивания.
static func http_base(ws_url: String) -> String:
	var s := ws_url.strip_edges()
	var https := false
	if s.begins_with("wss://"):
		https = true
		s = s.substr(6)
	elif s.begins_with("ws://"):
		s = s.substr(5)
	else:
		return ""
	var slash := s.find("/")
	var host := s if slash < 0 else s.substr(0, slash)
	if host == "" or host.contains("@") or host.contains(" ") or host.contains("\\") or host.begins_with("."):
		return ""
	return ("https://" if https else "http://") + host


static func is_track_file(ext: String, sha: String) -> bool:
	if ext not in ["mp3", "ogg", "opus"]:
		return false
	if sha.length() != 64:
		return false
	for i in sha.length():
		var c := sha.unicode_at(i)
		var hex := (c >= 48 and c <= 57) or (c >= 97 and c <= 102)
		if not hex:
			return false
	return true


## Из каталога сервера оставляет только включённую музыку с безопасным путём файла.
static func playlist(catalog: Dictionary) -> Array:
	var out: Array = []
	var raw: Variant = catalog.get("tracks", [])
	if typeof(raw) != TYPE_ARRAY:
		return out
	for item in raw:
		if typeof(item) != TYPE_DICTIONARY:
			continue
		var t: Dictionary = item
		if str(t.get("kind", "")) != "music":
			continue
		if not bool(t.get("enabled", true)):
			continue
		var ext := str(t.get("ext", ""))
		var sha := str(t.get("sha256", "")).to_lower()
		if not is_track_file(ext, sha):
			continue
		var path := "/media/files/%s.%s" % [sha, ext]
		if str(t.get("url", path)) != path:
			continue
		t["url"] = path
		t["sha256"] = sha
		out.append(t)
	return out


static func sha256_hex(data: PackedByteArray) -> String:
	var ctx := HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	ctx.update(data)
	return ctx.finish().hex_encode()
