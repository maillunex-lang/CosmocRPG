extends Node
## Докачка контента в фоне: игра скачивает подписанный манифест content.json, затем нужные пакеты .pck (модели, текстуры, данные),
## проверяет подпись манифеста, размер и SHA-256 каждого пакета и складывает в user://content. Применяются пакеты при следующем запуске
## (ui/content_boot.gd). Так изменения, сделанные в админке и опубликованные как пакет, доходят до игроков без переустановки APK.
##
## Защита: только https и разрешённый хост, путь /download/content/…pck; подпись манифеста тем же ключом, что у обновлений приложения;
## пакет принимается только если совпали размер и SHA-256; версии пакетов не откатываются; пакет с min_app_code выше установленного
## пропускается (ждёт нового APK); лимиты размера. Публикуют пакеты только инструментом tools/publish_content.py, который не принимает
## скрипты и сцены (в пакеты попадают данные, модели, текстуры, звук).

const Updater := preload("res://ui/updater.gd")
const AppVersion := preload("res://ui/app_version.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const Boot := preload("res://ui/content_boot.gd")

const MANIFEST_URL := "https://rikagro.ru/download/content/content.json"
const SIG_URL := "https://rikagro.ru/download/content/content.json.sig"
const ALLOWED_HOSTS := ["rikagro.ru"]
const MAX_MANIFEST_BYTES := 32768
const MAX_PACK_BYTES := 64 * 1024 * 1024
const MAX_TOTAL_BYTES := 200 * 1024 * 1024
const AUTO_SETTING := "content_auto"

signal progress(pack_id: String, fraction: float)
signal finished(result: Dictionary)

var manifest_url := MANIFEST_URL
var sig_url := SIG_URL
var state_path := Boot.STATE_PATH
var dir := "user://content"
var busy := false

var _http: HTTPRequest


## Безопасный адрес пакета: https, разрешённый хост, каталог /download/content/, файл .pck.
## Только для локальных проверок: если задан, адреса, начинающиеся с этого префикса (например, http://127.0.0.1:18090), считаются допустимыми.
## В игре не задаётся никогда.
static var test_origin := ""


static func is_safe_pack_url(url: String) -> bool:
	if test_origin != "" and url.begins_with(test_origin) and url.ends_with(".pck") and not url.contains(".."):
		return true
	if not url.begins_with("https://") or url.length() > 300 or url.contains(" ") or url.contains("@") or url.contains("\\"):
		return false
	var rest := url.substr(8)
	var slash := rest.find("/")
	var host := rest if slash < 0 else rest.substr(0, slash)
	if host.contains(":") or not ALLOWED_HOSTS.has(host) or slash < 0:
		return false
	var path := rest.substr(slash)
	return path.begins_with("/download/content/") and path.ends_with(".pck") and not path.contains("..") and not path.contains("?")


static func _valid_id(id: String) -> bool:
	if id.length() < 1 or id.length() > 32:
		return false
	for i in id.length():
		var c := id.unicode_at(i)
		if not ((c >= 97 and c <= 122) or (c >= 48 and c <= 57) or c == 95 or c == 45):
			return false
	return true


## Проверка манифеста контента (без сети). Возвращает {status, error, packs[]}: status error | ok.
## installed_code — версия приложения; highest — id пакета -> наибольшая виденная версия (защита от отката).
static func evaluate(manifest: PackedByteArray, sig_b64: String, public_pem: String, installed_code: int, highest: Dictionary = {}) -> Dictionary:
	if manifest.size() == 0 or manifest.size() > MAX_MANIFEST_BYTES:
		return {"status": "error", "error": "Манифест контента пустой или слишком большой", "packs": []}
	var sig_err := Updater.verify_signature(manifest, sig_b64, public_pem)
	if sig_err != "":
		return {"status": "error", "error": sig_err, "packs": []}
	var parsed: Variant = JSON.parse_string(manifest.get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY or typeof((parsed as Dictionary).get("packs")) != TYPE_ARRAY:
		return {"status": "error", "error": "Манифест контента повреждён", "packs": []}
	var out: Array = []
	var total := 0
	var seen := {}
	for p in (parsed as Dictionary)["packs"]:
		if typeof(p) != TYPE_DICTIONARY:
			return {"status": "error", "error": "Запись пакета повреждена", "packs": []}
		var id := str(p.get("id", ""))
		var version := int(p.get("version", 0))
		var sha := str(p.get("sha256", "")).to_lower()
		var size := int(p.get("size", 0))
		if not _valid_id(id) or seen.has(id) or version < 1:
			return {"status": "error", "error": "В манифесте некорректный пакет «%s»" % id, "packs": []}
		seen[id] = true
		if not is_safe_pack_url(str(p.get("url", ""))):
			return {"status": "error", "error": "Небезопасная ссылка пакета «%s»" % id, "packs": []}
		if sha.length() != 64 or not sha.is_valid_hex_number() or size <= 0 or size > MAX_PACK_BYTES:
			return {"status": "error", "error": "Некорректные размер или сумма пакета «%s»" % id, "packs": []}
		total += size
		if int(p.get("min_app_code", 1)) > installed_code:
			continue   # для более новой версии приложения: ждёт обновления APK
		if version < int(highest.get(id, 0)):
			return {"status": "error", "error": "Пакет «%s» старее уже виденного: откат отклонён" % id, "packs": []}
		out.append({"id": id, "version": version, "url": str(p["url"]), "sha256": sha, "size": size})
	if total > MAX_TOTAL_BYTES:
		return {"status": "error", "error": "Суммарный размер пакетов превышает лимит", "packs": []}
	return {"status": "ok", "error": "", "packs": out, "content_version": int((parsed as Dictionary).get("content_version", 0))}


## Какие пакеты нужно скачать: версия выше установленной (installed: id -> версия).
static func to_download(packs: Array, installed: Dictionary) -> Array:
	var out: Array = []
	for p in packs:
		if int(p["version"]) > int(installed.get(str(p["id"]), 0)):
			out.append(p)
	return out


func _read_state() -> ConfigFile:
	var cfg := ConfigFile.new()
	cfg.load(state_path)
	return cfg


func installed_versions() -> Dictionary:
	var cfg := _read_state()
	var out := {}
	for id in cfg.get_sections():
		out[id] = int(cfg.get_value(id, "version", 0))
	return out


func _highest() -> Dictionary:
	var cfg := _read_state()
	var out := {}
	for id in cfg.get_sections():
		out[id] = maxi(int(cfg.get_value(id, "version", 0)), int(cfg.get_value(id, "highest", 0)))
	return out


## Проверяет файл пакета (размер, SHA-256), переносит в каталог контента и записывает состояние. Возвращает "" при успехе.
func install_pack_file(pack: Dictionary, tmp_path: String) -> String:
	var f := FileAccess.open(tmp_path, FileAccess.READ)
	if f == null:
		return "Файл пакета не найден"
	var size := int(f.get_length())
	f.close()
	if size != int(pack["size"]):
		return "Размер пакета «%s» не совпал" % pack["id"]
	if FileAccess.get_sha256(tmp_path).to_lower() != str(pack["sha256"]):
		return "Контрольная сумма пакета «%s» не совпала" % pack["id"]
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(dir))
	var final := "%s/%s-%d.pck" % [dir, pack["id"], int(pack["version"])]
	if DirAccess.rename_absolute(ProjectSettings.globalize_path(tmp_path), ProjectSettings.globalize_path(final)) != OK:
		return "Не удалось сохранить пакет"
	var cfg := _read_state()
	var old_path := str(cfg.get_value(str(pack["id"]), "path", ""))
	cfg.set_value(str(pack["id"]), "version", int(pack["version"]))
	cfg.set_value(str(pack["id"]), "highest", maxi(int(pack["version"]), int(cfg.get_value(str(pack["id"]), "highest", 0))))
	cfg.set_value(str(pack["id"]), "path", final)
	cfg.set_value(str(pack["id"]), "size", size)
	cfg.set_value(str(pack["id"]), "sha256", str(pack["sha256"]))
	cfg.save(state_path)
	if old_path != "" and old_path != final and FileAccess.file_exists(old_path):
		DirAccess.remove_absolute(ProjectSettings.globalize_path(old_path))   # старая версия того же пакета больше не нужна
	return ""


func _fetch(url: String, limit: int) -> Dictionary:
	var http := HTTPRequest.new()
	http.timeout = 15.0
	http.body_size_limit = limit
	add_child(http)
	if http.request(url) != OK:
		http.queue_free()
		return {"ok": false, "error": "Не удалось начать запрос"}
	var r: Array = await http.request_completed
	http.queue_free()
	if int(r[0]) != HTTPRequest.RESULT_SUCCESS or int(r[1]) != 200:
		return {"ok": false, "error": "Сервер контента недоступен (%d)" % int(r[1])}
	return {"ok": true, "body": r[3]}


func _download(pack: Dictionary) -> String:
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(dir))
	var tmp := "%s/%s-%d.part" % [dir, pack["id"], int(pack["version"])]
	_http = HTTPRequest.new()
	_http.download_file = tmp
	_http.body_size_limit = int(pack["size"]) + 1024
	_http.timeout = 120.0
	add_child(_http)
	if _http.request(str(pack["url"])) != OK:
		_http.queue_free()
		return "Не удалось начать загрузку"
	var poll := get_tree().create_timer(0.4)
	var done := [false]
	var res: Array = []
	_http.request_completed.connect(func(a: int, b: int, _c: PackedStringArray, _d: PackedByteArray) -> void:
		res.assign([a, b])
		done[0] = true)
	while not done[0]:
		progress.emit(str(pack["id"]), clampf(float(_http.get_downloaded_bytes()) / float(pack["size"]), 0.0, 1.0))
		await get_tree().create_timer(0.3).timeout
	_http.queue_free()
	if int(res[0]) != HTTPRequest.RESULT_SUCCESS or int(res[1]) != 200:
		DirAccess.remove_absolute(ProjectSettings.globalize_path(tmp))
		return "Загрузка пакета прервана (%d)" % int(res[1])
	var err := install_pack_file(pack, tmp)
	if err != "":
		DirAccess.remove_absolute(ProjectSettings.globalize_path(tmp))
	return err


## Проверка и докачка в фоне. Итог в сигнале finished: {status: up_to_date | downloaded | error | disabled, error, downloaded[]}.
func check(force: bool = false) -> void:
	if busy:
		return
	if not force and not UiSettings.get_bool(AUTO_SETTING, true):
		finished.emit({"status": "disabled", "error": "", "downloaded": []})
		return
	busy = true
	var result := {"status": "up_to_date", "error": "", "downloaded": []}
	var m := await _fetch(manifest_url, MAX_MANIFEST_BYTES)
	if not bool(m["ok"]):
		result = {"status": "error", "error": str(m["error"]), "downloaded": []}
	else:
		var s := await _fetch(sig_url, 4096)
		if not bool(s["ok"]):
			result = {"status": "error", "error": str(s["error"]), "downloaded": []}
		else:
			var ev := evaluate(m["body"], (s["body"] as PackedByteArray).get_string_from_utf8(), Updater.PUBLIC_KEY_PEM, AppVersion.code(), _highest())
			if ev["status"] != "ok":
				result = {"status": "error", "error": ev["error"], "downloaded": []}
			else:
				for p in to_download(ev["packs"], installed_versions()):
					var err := await _download(p)
					if err != "":
						result = {"status": "error", "error": err, "downloaded": result["downloaded"]}
						break
					(result["downloaded"] as Array).append("%s %d" % [p["id"], int(p["version"])])
				if result["status"] == "up_to_date" and (result["downloaded"] as Array).size() > 0:
					result["status"] = "downloaded"
	busy = false
	finished.emit(result)
