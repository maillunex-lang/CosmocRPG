extends Node
## Эффекты М-3: кнопки, двигатель своего корабля, свой выстрел и попадание по себе.
## Файлы качаются из библиотеки (вид sfx, теги ui/engine/weapon/hit) и играются на шине Effects.

const Lib := preload("res://audio/sfx.gd")
const Music := preload("res://audio/music.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

const CACHE := "user://sfx"
const GAPS := {"ui": 45, "weapon": 90, "hit": 120}

var status := ""

var _base := ""
var _cues: Dictionary = {}
var _bytes := {}
var _generation := 0
var _http: HTTPRequest
var _oneshots: Array[AudioStreamPlayer] = []
var _cursor := 0
var _engine: AudioStreamPlayer
var _engine_sha := ""
var _engine_want := false
var _pitch := 1.0
var _last := {}
var _paused := false


func _ready() -> void:
	_ensure_bus()
	_engine = AudioStreamPlayer.new()
	_engine.bus = "Effects"
	add_child(_engine)
	for _i in 6:
		var p := AudioStreamPlayer.new()
		p.bus = "Effects"
		add_child(p)
		_oneshots.append(p)
	_http = HTTPRequest.new()
	_http.timeout = 30
	add_child(_http)
	apply_mix()
	var tree := get_tree()
	if tree != null and not tree.node_added.is_connected(_watch):
		tree.node_added.connect(_watch)
		if tree.root != null:
			_watch_existing(tree.root)


func configure(ws_url: String) -> void:
	var base := Music.http_base(ws_url)
	if base == "":
		status = "Нет адреса сервера для эффектов"
		return
	if base != _base:
		_base = base
		_cues = {}
		_bytes.clear()
		_engine_sha = ""
	_generation += 1
	_fetch_catalog(_generation)


func apply_mix() -> void:
	var v := clampf(UiSettings.get_float("sfx_volume", 0.5), 0.0, 1.0)
	var idx := AudioServer.get_bus_index(_ensure_bus())
	AudioServer.set_bus_mute(idx, v <= 0.001)
	AudioServer.set_bus_volume_db(idx, linear_to_db(maxf(v, 0.001)))
	_sync_engine()


func play(role: String) -> void:
	if _paused or role == "engine" or role not in Lib.ROLES:
		return
	if UiSettings.get_float("sfx_volume", 0.5) <= 0.001:
		return
	var now := Time.get_ticks_msec()
	var last := int(_last.get(role, -1))
	if not Lib.allow(now, last, int(GAPS.get(role, 80))):
		return
	var t := Lib.pick(_cues, role)
	if t.is_empty():
		return
	var sha := str(t.get("sha256", ""))
	if not _bytes.has(sha):
		return
	var stream := _stream(str(t.get("ext", "")), _bytes[sha], false)
	if stream == null:
		return
	_last[role] = now
	var player := _oneshots[_cursor]
	_cursor = (_cursor + 1) % _oneshots.size()
	player.stop()
	player.stream = stream
	player.pitch_scale = 1.0
	player.play()


func set_engine(active: bool, speed_frac: float, boosting: bool) -> void:
	_engine_want = active
	_pitch = Lib.engine_pitch(speed_frac, boosting)
	_sync_engine()


func _notification(what: int) -> void:
	if what == NOTIFICATION_APPLICATION_PAUSED or what == NOTIFICATION_WM_WINDOW_FOCUS_OUT:
		_paused = true
		if _engine != null and _engine.playing:
			_engine.stream_paused = true
	elif what == NOTIFICATION_APPLICATION_RESUMED or what == NOTIFICATION_WM_WINDOW_FOCUS_IN:
		_paused = false
		if _engine != null:
			_engine.stream_paused = false
		_sync_engine()


func _ensure_bus() -> String:
	if AudioServer.get_bus_index("Effects") == -1:
		var idx := AudioServer.bus_count
		AudioServer.add_bus(idx)
		AudioServer.set_bus_name(idx, "Effects")
		AudioServer.set_bus_send(idx, "Master")
	return "Effects"


func _watch(n: Node) -> void:
	if n is BaseButton and not (n as BaseButton).pressed.is_connected(_on_ui_pressed):
		(n as BaseButton).pressed.connect(_on_ui_pressed)


func _watch_existing(n: Node) -> void:
	_watch(n)
	for c in n.get_children():
		_watch_existing(c)


func _on_ui_pressed() -> void:
	play("ui")


func _fetch_catalog(gen: int) -> void:
	if _base == "":
		return
	var err := _http.request(_base + "/media/catalog.json")
	if err == ERR_BUSY:
		await _http.request_completed
		if gen != _generation:
			return
		_fetch_catalog(gen)
		return
	if err != OK:
		status = "Каталог эффектов недоступен"
		return
	var result: Array = await _http.request_completed
	if gen != _generation:
		return
	if result.size() < 4 or int(result[1]) != 200:
		status = "Каталог эффектов недоступен"
		return
	var parsed: Variant = JSON.parse_string(result[3].get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY:
		status = "Каталог эффектов повреждён"
		return
	_cues = Lib.cues(parsed)
	status = ""
	await _cache_all(gen)


func _cache_all(gen: int) -> void:
	var wanted: Array = []
	var seen := {}
	for role in Lib.ROLES:
		var t := Lib.pick(_cues, role)
		if t.is_empty():
			continue
		var sha := str(t["sha256"])
		if seen.has(sha):
			continue
		seen[sha] = true
		if _bytes.has(sha) or _load_cached(t):
			continue
		wanted.append(t)
	for item in wanted:
		if gen != _generation:
			return
		var t: Dictionary = item
		if not await _download(t, gen):
			status = "Не удалось скачать эффект «%s»" % str(t.get("title", ""))
	if gen == _generation:
		_sync_engine()


func _load_cached(t: Dictionary) -> bool:
	var path := "%s/%s.%s" % [CACHE, t["sha256"], t["ext"]]
	if not FileAccess.file_exists(path):
		return false
	var cached := FileAccess.get_file_as_bytes(path)
	if int(t.get("size", -1)) != cached.size() or Music.sha256_hex(cached) != str(t["sha256"]):
		DirAccess.remove_absolute(path)
		return false
	_bytes[str(t["sha256"])] = cached
	return true


func _download(t: Dictionary, gen: int) -> bool:
	DirAccess.make_dir_recursive_absolute(CACHE)
	var err := _http.request(_base + str(t["url"]))
	if err == ERR_BUSY:
		await _http.request_completed
		if gen != _generation:
			return false
		return await _download(t, gen)
	if err != OK:
		return false
	var result: Array = await _http.request_completed
	if gen != _generation or result.size() < 4 or int(result[1]) != 200:
		return false
	var body: PackedByteArray = result[3]
	if int(t.get("size", -1)) != body.size():
		return false
	if Music.sha256_hex(body) != str(t["sha256"]):
		return false
	var path := "%s/%s.%s" % [CACHE, t["sha256"], t["ext"]]
	var f := FileAccess.open(path, FileAccess.WRITE)
	if f == null:
		return false
	f.store_buffer(body)
	f.close()
	var check := FileAccess.get_file_as_bytes(path)
	if Music.sha256_hex(check) != str(t["sha256"]):
		DirAccess.remove_absolute(path)
		return false
	_bytes[str(t["sha256"])] = check
	return true


func _sync_engine() -> void:
	if _engine == null:
		return
	var audible := _engine_want and not _paused and UiSettings.get_float("sfx_volume", 0.5) > 0.001
	if not audible:
		if _engine.playing:
			_engine.stop()
		return
	var t := Lib.pick(_cues, "engine")
	if t.is_empty():
		if _engine.playing:
			_engine.stop()
		return
	var sha := str(t["sha256"])
	if not _bytes.has(sha):
		if _engine.playing and _engine_sha != "" and _engine_sha != sha:
			_engine.stop()
		return
	if _engine_sha != sha or _engine.stream == null:
		var stream := _stream(str(t["ext"]), _bytes[sha], true)
		if stream == null:
			return
		_engine.stream = stream
		_engine_sha = sha
		_engine.pitch_scale = _pitch
		_engine.play()
		return
	_engine.pitch_scale = _pitch
	if not _engine.playing:
		_engine.play()


func _stream(ext: String, bytes: PackedByteArray, looped: bool) -> AudioStream:
	if ext == "mp3":
		var mp3 := AudioStreamMP3.new()
		mp3.data = bytes
		mp3.loop = looped
		return mp3
	if ext == "ogg":
		var ogg := AudioStreamOggVorbis.load_from_buffer(bytes)
		if ogg != null:
			ogg.loop = looped
		return ogg
	return null
