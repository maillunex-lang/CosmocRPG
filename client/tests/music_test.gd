extends SceneTree
## Плеер М-1: адрес, каталог, сумма. Сеть и колонки не нужны.

const Lib := preload("res://audio/music.gd")

var _f := 0
var _n := 0


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _run() -> void:
	_c(Lib.http_base("wss://rikagro.ru/ws") == "https://rikagro.ru", "wss без порта")
	_c(Lib.http_base("ws://127.0.0.1:8080/ws") == "http://127.0.0.1:8080", "ws с портом")
	_c(Lib.http_base("https://evil.example/ws") == "", "не websocket")
	_c(Lib.http_base("ws://user@host/ws") == "", "адрес с логином")
	var sha := "ab".repeat(32)
	_c(Lib.is_track_file("mp3", sha) and not Lib.is_track_file("wav", sha) and not Lib.is_track_file("mp3", "abc"), "имя файла")
	var good := {"tracks": [
		{"kind": "music", "title": "Тишина", "enabled": true, "ext": "mp3", "sha256": sha, "url": "/media/files/%s.mp3" % sha, "size": 3},
		{"kind": "ad", "title": "Реклама", "enabled": true, "ext": "mp3", "sha256": sha, "url": "/media/files/%s.mp3" % sha},
		{"kind": "music", "title": "Выкл", "enabled": false, "ext": "ogg", "sha256": sha, "url": "/media/files/%s.ogg" % sha},
		{"kind": "music", "title": "Чужой адрес", "enabled": true, "ext": "mp3", "sha256": sha, "url": "https://evil.example/a.mp3"},
	]}
	var list: Array = Lib.playlist(good)
	_c(list.size() == 1 and list[0]["title"] == "Тишина", "в плейлист попадает только включённая музыка с безопасным путём")
	var bytes := PackedByteArray([1, 2, 3])
	_c(Lib.sha256_hex(bytes).length() == 64, "контрольная сумма")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
