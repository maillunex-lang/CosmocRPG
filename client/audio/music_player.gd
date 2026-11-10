extends Node
## Фоновая музыка М-1. Каталог и файлы качаются с того же сервера, куда игрок подключился.
## Громкость и выключатель — в настройках (user://settings.cfg). Радиоэфир и реклама — этап М-2, здесь их нет.

const Lib := preload("res://audio/music.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

const CACHE := "user://music"

var track_title := ""
var status := ""

var _base := ""
var _playlist: Array = []
var _index := -1
var _player: AudioStreamPlayer
var _http: HTTPRequest
var _busy := false
var _paused_by_app := false
var _generation := 0


func _ready() -> void:
	_player = AudioStreamPlayer.new()
	_player.bus = _bus("Music")
	_player.finished.connect(_on_finished)
	add_child(_player)
	_http = HTTPRequest.new()
	_http.timeout = 60
	add_child(_http)
	apply_mix()


func configure(ws_url: String) -> void:
	var base := Lib.http_base(ws_url)
	if base == "":
		status = "Нет адреса сервера для музыки"
		return
	if base != _base:
		_base = base
		_playlist = []
		_index = -1
	_generation += 1
	_fetch_catalog(_generation)


func apply_mix() -> void:
	_set_bus("Music", UiSettings.get_float("music_volume", 0.5), not UiSettings.get_bool("music_enabled", true))
	_set_bus("Radio", UiSettings.get_float("radio_volume", 0.5), false)
	_set_bus("Effects", UiSettings.get_float("sfx_volume", 0.5), false)
	if not UiSettings.get_bool("music_enabled", true):
		if _player != null:
			_player.stop()
		track_title = ""
		return
	if _player != null and not _player.playing and not _playlist.is_empty() and not _busy:
		_play_index(_index if _index >= 0 else 0)


func next_track() -> void:
	if _playlist.is_empty():
		return
	_play_index((_index + 1) % _playlist.size())


func prev_track() -> void:
	if _playlist.is_empty():
		return
	_play_index((_index - 1 + _playlist.size()) % _playlist.size())


func _notification(what: int) -> void:
	if _player == null:
		return
	if what == NOTIFICATION_APPLICATION_PAUSED or what == NOTIFICATION_WM_WINDOW_FOCUS_OUT:
		if _player.playing:
			_player.stream_paused = true
			_paused_by_app = true
	elif what == NOTIFICATION_APPLICATION_RESUMED or what == NOTIFICATION_WM_WINDOW_FOCUS_IN:
		if _paused_by_app and UiSettings.get_bool("music_enabled", true):
			_player.stream_paused = false
		_paused_by_app = false


func _bus(bus_name: String) -> String:
	if AudioServer.get_bus_index(bus_name) == -1:
		var idx := AudioServer.bus_count
		AudioServer.add_bus(idx)
		AudioServer.set_bus_name(idx, bus_name)
		AudioServer.set_bus_send(idx, "Master")
	return bus_name


func _set_bus(bus_name: String, linear: float, mute: bool) -> void:
	var idx := AudioServer.get_bus_index(_bus(bus_name))
	var v := clampf(linear, 0.0, 1.0)
	AudioServer.set_bus_mute(idx, mute or v <= 0.001)
	AudioServer.set_bus_volume_db(idx, linear_to_db(maxf(v, 0.001)))


func _fetch_catalog(gen: int) -> void:
	if _base == "":
		return
	_busy = true
	var err := _http.request(_base + "/media/catalog.json")
	if err != OK:
		_busy = false
		status = "Каталог музыки недоступен"
		return
	var result: Array = await _http.request_completed
	if gen != _generation:
		return
	_busy = false
	if result.size() < 4 or int(result[1]) != 200:
		status = "Каталог музыки недоступен"
		return
	var parsed: Variant = JSON.parse_string(result[3].get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY:
		status = "Каталог музыки повреждён"
		return
	_playlist = Lib.playlist(parsed)
	if _playlist.is_empty():
		status = "В библиотеке пока нет музыки"
		track_title = ""
		return
	status = ""
	if UiSettings.get_bool("music_enabled", true):
		_play_index(0 if _index < 0 or _index >= _playlist.size() else _index)


func _play_index(i: int) -> void:
	if _playlist.is_empty() or not UiSettings.get_bool("music_enabled", true):
		return
	_index = posmod(i, _playlist.size())
	var t: Dictionary = _playlist[_index]
	track_title = str(t.get("title", "Музыка"))
	var ext := str(t.get("ext", ""))
	var sha := str(t.get("sha256", ""))
	var path := "%s/%s.%s" % [CACHE, sha, ext]
	if not FileAccess.file_exists(path):
		var ok := await _download(t)
		if not ok:
			status = "Не удалось скачать «%s»" % track_title
			return
	var bytes := FileAccess.get_file_as_bytes(path)
	if Lib.sha256_hex(bytes) != sha:
		DirAccess.remove_absolute(path)
		status = "Файл «%s» не совпал с контрольной суммой" % track_title
		return
	var stream := _stream(ext, bytes)
	if stream == null:
		status = "Этот формат плеер пока не играет"
		return
	_player.stream = stream
	_player.play()
	_player.stream_paused = false
	status = ""


func _download(t: Dictionary) -> bool:
	DirAccess.make_dir_recursive_absolute(CACHE)
	var gen := _generation
	_busy = true
	var err := _http.request(_base + str(t["url"]))
	if err != OK:
		_busy = false
		return false
	var result: Array = await _http.request_completed
	_busy = false
	if gen != _generation or result.size() < 4 or int(result[1]) != 200:
		return false
	var body: PackedByteArray = result[3]
	if int(t.get("size", body.size())) != body.size():
		return false
	if Lib.sha256_hex(body) != str(t["sha256"]):
		return false
	var path := "%s/%s.%s" % [CACHE, t["sha256"], t["ext"]]
	var f := FileAccess.open(path, FileAccess.WRITE)
	if f == null:
		return false
	f.store_buffer(body)
	f.close()
	return true


func _stream(ext: String, bytes: PackedByteArray) -> AudioStream:
	if ext == "mp3":
		var s := AudioStreamMP3.new()
		s.data = bytes
		return s
	if ext == "ogg":
		return AudioStreamOggVorbis.load_from_buffer(bytes)
	return null


func _on_finished() -> void:
	if _playlist.size() <= 1:
		if not _playlist.is_empty() and UiSettings.get_bool("music_enabled", true):
			_play_index(_index)
		return
	next_track()
