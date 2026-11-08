extends Node
## Проверка обновлений приложения. Godot скачивает **подписанный** манифест версии по HTTPS, проверяет подпись (RSA-SHA256,
## публичный ключ вшит в приложение), сравнивает версии и сообщает результат. «Обновить» открывает защищённую ссылку на APK;
## установку выполняет Android с подтверждением пользователя (тихой установки нет).
##
## Защита: только HTTPS и разрешённый хост; подпись манифеста; запрет отката (манифест со старой версией отклоняется, в том числе
## старее уже виденного); контрольная сумма и размер APK берутся из подписанного манифеста; при установке Android сам проверяет
## подпись приложения (обновление принимается только с тем же ключом, данные игры сохраняются).

const AppVersion := preload("res://ui/app_version.gd")

const MANIFEST_URL := "https://rikagro.ru/download/latest.json"
const SIG_URL := "https://rikagro.ru/download/latest.json.sig"
## Публичный ключ проверки подписи манифеста. Вшит в код, потому что файлы .pem не попадают в APK без отдельной настройки экспорта
## (так уже случилось в версии 0.3.0). Файл assets/update_public.pem остаётся эталоном; тест updater_test сверяет их.
const PUBLIC_KEY_PEM := """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvQOphIqyew4ImE0bMrX6
c9jH9pQ9ePtV2aXF8q1bsgITF10lFOmoD/4gn9JAW+VjYaIYGR+8o0zRXg52EtVV
B2YKeWnIfSIvQWklUphsFer21dwwsHmBKGm+PI3fwenLGcEKbI2IBm0BJ8x08zsx
MeGfImudMEeN3LQikqlld43Zg8lpTO2Q3pm9r9sfZuGje4xONGIa0vR3eS3a7Zez
bArh8jjlmWw650VbRhulMI3MiSSlsrAIYvbT1xqKIH5BxUEYjvkTZuQrJHDJMLR9
vKMiW8CW3D0VchpUxQvkk4GAKKLs0oMeNnGF8CSwe32hTETZsHDZzK7cHN3LF3TJ
gQIDAQAB
-----END PUBLIC KEY-----
"""
const PUBLIC_KEY_PATH := "res://assets/update_public.pem"
const ALLOWED_HOSTS := ["rikagro.ru"]
const MAX_MANIFEST_BYTES := 16384
const MAX_APK_BYTES := 95 * 1024 * 1024   # целевой предел базового APK (решение владельца)
const STATE_PATH := "user://update_state.cfg"
const TIMEOUT_SEC := 15.0

signal finished(result: Dictionary)

var manifest_url := MANIFEST_URL
var sig_url := SIG_URL
var state_path := STATE_PATH
var busy := false


## Безопасный адрес скачивания: https, разрешённый хост, без логина в адресе, файл .apk.
static func is_safe_url(url: String) -> bool:
	if not url.begins_with("https://") or url.length() > 300 or url.contains(" ") or url.contains("@") or url.contains("\\"):
		return false
	var rest := url.substr(8)
	var slash := rest.find("/")
	var host := rest if slash < 0 else rest.substr(0, slash)
	if host.contains(":") or not ALLOWED_HOSTS.has(host):
		return false
	var path := "" if slash < 0 else rest.substr(slash)
	return path.begins_with("/") and path.split("?")[0].ends_with(".apk") and not path.contains("..")


static func _digest(bytes: PackedByteArray) -> PackedByteArray:
	var ctx := HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	ctx.update(bytes)
	return ctx.finish()


static func _err(msg: String) -> Dictionary:
	return {"status": "error", "error": msg, "info": {}}


## Чистая проверка манифеста (без сети): подпись, поля, версия. Возвращает {status, error, info}.
## status: up_to_date | available | required | error. highest_seen — наибольшая ранее виденная версия манифеста (защита от отката).
static func evaluate(manifest: PackedByteArray, sig_b64: String, public_pem: String, installed_code: int, highest_seen: int = 0) -> Dictionary:
	if manifest.size() == 0 or manifest.size() > MAX_MANIFEST_BYTES:
		return _err("Манифест пустой или слишком большой")
	var sig := Marshalls.base64_to_raw(sig_b64.strip_edges())
	if sig.size() == 0:
		return _err("Подпись манифеста отсутствует")
	var key := CryptoKey.new()
	if key.load_from_string(public_pem, true) != OK:
		return _err("Не удалось загрузить публичный ключ")
	if not Crypto.new().verify(HashingContext.HASH_SHA256, _digest(manifest), sig, key):
		return _err("Подпись манифеста не подтверждена: обновление отклонено")
	var parsed: Variant = JSON.parse_string(manifest.get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY:
		return _err("Манифест повреждён")
	var m: Dictionary = parsed
	var code := int(m.get("version_code", 0))
	var min_code := int(m.get("min_version_code", 1))
	var url := str(m.get("url", ""))
	var sha := str(m.get("sha256", "")).to_lower()
	var size := int(m.get("size", 0))
	if code < 1 or min_code < 1 or min_code > code:
		return _err("В манифесте некорректные номера версий")
	if not is_safe_url(url):
		return _err("В манифесте небезопасная ссылка на APK")
	if sha.length() != 64 or not sha.is_valid_hex_number():
		return _err("В манифесте некорректная контрольная сумма")
	if size <= 0 or size > MAX_APK_BYTES:
		return _err("В манифесте некорректный размер APK")
	if code < installed_code:
		return _err("Опубликованная версия старее установленной: откат отклонён")
	if code < highest_seen:
		return _err("Манифест старее уже виденного: откат отклонён")
	var info := {"version_name": str(m.get("version_name", "")).left(32), "version_code": code, "min_version_code": min_code, "url": url,
			"sha256": sha, "size": size, "notes": str(m.get("notes", "")).left(1000), "published": str(m.get("published", "")).left(40)}
	var status := "up_to_date"
	if code > installed_code:
		status = "available"
	if min_code > installed_code:
		status = "required"
	return {"status": status, "error": "", "info": info}


func _load_highest() -> int:
	var cfg := ConfigFile.new()
	if cfg.load(state_path) == OK:
		return int(cfg.get_value("update", "highest_seen", 0))
	return 0


func _save_highest(code: int) -> void:
	var cfg := ConfigFile.new()
	cfg.load(state_path)
	cfg.set_value("update", "highest_seen", maxi(code, int(cfg.get_value("update", "highest_seen", 0))))
	cfg.save(state_path)


func _fetch(url: String) -> Dictionary:
	var http := HTTPRequest.new()
	http.timeout = TIMEOUT_SEC
	http.body_size_limit = MAX_MANIFEST_BYTES
	add_child(http)
	if http.request(url) != OK:
		http.queue_free()
		return {"ok": false, "error": "Не удалось начать запрос"}
	var r: Array = await http.request_completed
	http.queue_free()
	if int(r[0]) != HTTPRequest.RESULT_SUCCESS:
		return {"ok": false, "error": "Нет связи с сервером обновлений"}
	if int(r[1]) != 200:
		return {"ok": false, "error": "Сервер обновлений ответил кодом %d" % int(r[1])}
	return {"ok": true, "body": r[3]}


## Проверка обновлений по сети. Результат приходит в сигнале finished.
func check() -> void:
	if busy:
		return
	busy = true
	var res: Dictionary
	var m := await _fetch(manifest_url)
	if not bool(m["ok"]):
		res = _err(str(m["error"]))
	else:
		var s := await _fetch(sig_url)
		if not bool(s["ok"]):
			res = _err(str(s["error"]))
		else:
			res = evaluate(m["body"], (s["body"] as PackedByteArray).get_string_from_utf8(), PUBLIC_KEY_PEM, AppVersion.code(), _load_highest())
			if res["status"] != "error":
				_save_highest(int(res["info"]["version_code"]))
	busy = false
	finished.emit(res)


## «Обновить»: открывает защищённую ссылку на APK в системном браузере; дальше установка средствами Android.
func open_download(info: Dictionary) -> bool:
	var url := str(info.get("url", ""))
	if not is_safe_url(url):
		return false
	OS.shell_open(url)
	return true
