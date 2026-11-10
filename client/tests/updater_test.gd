extends SceneTree
## Проверка обновлений: подпись манифеста, версии, защита от отката, безопасные ссылки, совпадение версии в проекте и пресете экспорта,
## совместимость с подписью, которую создаёт tools/publish_release.sh (openssl).

const Updater := preload("res://ui/updater.gd")
const AppVersion := preload("res://ui/app_version.gd")

var _f := 0
var _n := 0
var _crypto := Crypto.new()


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _manifest(over: Dictionary = {}) -> Dictionary:
	var m := {"version_name": "0.4.0", "version_code": 4, "min_version_code": 1, "url": "https://rikagro.ru/download/zvezdnye-rubezhi-0.4.0-4.apk",
			"sha256": "a".repeat(64), "size": 38000000, "notes": "Тест", "published": "2026-11-08T20:00:00Z"}
	m.merge(over, true)
	return m


func _sign(key: CryptoKey, bytes: PackedByteArray) -> String:
	var ctx := HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	ctx.update(bytes)
	return Marshalls.raw_to_base64(_crypto.sign(HashingContext.HASH_SHA256, ctx.finish(), key))


func _run() -> void:
	var key := _crypto.generate_rsa(2048)
	var pub := key.save_to_string(true)
	var other := _crypto.generate_rsa(2048)

	var good := JSON.stringify(_manifest()).to_utf8_buffer()
	var sig := _sign(key, good)
	var r := Updater.evaluate(good, sig, pub, 3)
	_c(r["status"] == "available" and r["info"]["version_code"] == 4 and r["info"]["version_name"] == "0.4.0", "новая версия: «доступно обновление»")
	_c(Updater.evaluate(good, sig, pub, 4)["status"] == "up_to_date", "та же версия: «последняя версия»")
	var req := JSON.stringify(_manifest({"min_version_code": 4})).to_utf8_buffer()
	_c(Updater.evaluate(req, _sign(key, req), pub, 3)["status"] == "required", "установленная версия ниже минимальной: «обязательное обновление»")

	# --- подпись ---
	var tampered := JSON.stringify(_manifest({"size": 39000000})).to_utf8_buffer()
	_c(Updater.evaluate(tampered, sig, pub, 3)["status"] == "error", "изменённый манифест с прежней подписью отклоняется")
	_c(Updater.evaluate(good, _sign(other, good), pub, 3)["status"] == "error", "подпись другим ключом отклоняется")
	_c(Updater.evaluate(good, "", pub, 3)["status"] == "error", "без подписи отклоняется")
	_c(Updater.evaluate(good, "не-base64!!", pub, 3)["status"] == "error", "мусор вместо подписи отклоняется")
	_c(Updater.evaluate(PackedByteArray(), sig, pub, 3)["status"] == "error", "пустой манифест отклоняется")
	_c(Updater.evaluate(("x".repeat(20000)).to_utf8_buffer(), sig, pub, 3)["status"] == "error", "слишком большой манифест отклоняется")

	# --- содержимое (подписано верно, но данные плохие) ---
	for bad in [["url", "http://rikagro.ru/download/a.apk"], ["url", "https://evil.example/download/a.apk"], ["url", "https://rikagro.ru@evil.example/a.apk"],
			["url", "https://rikagro.ru:8443/download/a.apk"], ["url", "https://rikagro.ru/download/a.zip"], ["url", "https://rikagro.ru/download/../x.apk"],
			["url", "https://rikagro.ru.evil.example/a.apk"], ["sha256", "zz".repeat(32)], ["sha256", "abc"], ["size", 0], ["size", 200 * 1024 * 1024],
			["version_code", 0], ["min_version_code", 9]]:
		var b := JSON.stringify(_manifest({bad[0]: bad[1]})).to_utf8_buffer()
		_c(Updater.evaluate(b, _sign(key, b), pub, 3)["status"] == "error", "отклоняется: %s = %s" % [bad[0], str(bad[1]).left(40)])

	# --- откат ---
	var old := JSON.stringify(_manifest({"version_code": 2, "version_name": "0.2.0"})).to_utf8_buffer()
	_c(Updater.evaluate(old, _sign(key, old), pub, 3)["status"] == "error", "манифест старее установленной версии: откат отклонён")
	_c(Updater.evaluate(good, sig, pub, 3, 5)["status"] == "error", "манифест старее уже виденного: откат отклонён")
	_c(Updater.evaluate(good, sig, pub, 3, 4)["status"] == "available", "виденная версия не мешает той же версии")

	# --- ссылки ---
	_c(Updater.is_safe_url("https://rikagro.ru/download/zvezdnye-rubezhi-0.3.0-3.apk"), "обычная ссылка на APK допустима")
	_c(not Updater.is_safe_url("") and not Updater.is_safe_url("https://rikagro.ru/") and not Updater.is_safe_url("javascript:alert(1)"), "пустая, корень и чужая схема недопустимы")
	var u := Updater.new()
	root.add_child(u)
	_c(not u.open_download({"url": "http://rikagro.ru/download/a.apk"}) and not u.open_download({}), "небезопасная ссылка не открывается")

	# --- проект: версия и ключ ---
	var pem := Updater.PUBLIC_KEY_PEM
	var pk := CryptoKey.new()
	# файл ключа (*.pem) не хранится в git: в CI его нет, сверка только там, где файл есть (на машине разработчика)
	_c(FileAccess.file_exists(Updater.PUBLIC_KEY_PATH), "публичный ключ обновлений присутствует в checkout для publish_release.sh")
	if FileAccess.file_exists(Updater.PUBLIC_KEY_PATH):
		_c(FileAccess.get_file_as_string(Updater.PUBLIC_KEY_PATH).strip_edges() == pem.strip_edges(), "вшитый в код ключ совпадает с файлом ключа")
	else:
		print("SKIP  файл ключа отсутствует (CI): сверка пропущена")
	_c(pem.begins_with("-----BEGIN PUBLIC KEY-----") and pk.load_from_string(pem, true) == OK, "вшитый публичный ключ загружается")
	_c(not pem.contains("PRIVATE"), "в приложении нет приватного ключа")
	var cfg := ConfigFile.new()
	var preset_ok := cfg.load("res://export_presets.cfg") == OK
	var code := int(cfg.get_value("preset.0.options", "version/code", -1)) if preset_ok else -1
	var vname := str(cfg.get_value("preset.0.options", "version/name", "")) if preset_ok else ""
	_c(code == AppVersion.code() and vname == AppVersion.version_name() and code >= 1, "версия в пресете экспорта (%s, %d) совпадает с project.godot (%s)" % [vname, code, AppVersion.label()])

	# --- совместимость с подписью openssl (как tools/publish_release.sh). Фиктивный APK скрипт публикации больше не принимает. ---
	var key_path := OS.get_environment("HOME").path_join("keys/cosmoc/manifest_private.pem")
	if FileAccess.file_exists(key_path):
		var tmp := OS.get_user_data_dir().path_join("pubtest")
		DirAccess.make_dir_recursive_absolute(tmp)
		var body := JSON.stringify(_manifest({"version_code": AppVersion.code(), "version_name": AppVersion.version_name()})).to_utf8_buffer()
		var mf_path := tmp.path_join("manifest.json")
		var fa := FileAccess.open(mf_path, FileAccess.WRITE)
		fa.store_buffer(body)
		fa.close()
		var out: Array = []
		var rc := OS.execute("openssl", ["dgst", "-sha256", "-sign", key_path, "-out", tmp.path_join("manifest.sig"), mf_path], out, true)
		_c(rc == 0, "openssl подписывает манифест тем же ключом, что и публикация")
		if rc == 0:
			var sig_b64 := Marshalls.raw_to_base64(FileAccess.get_file_as_bytes(tmp.path_join("manifest.sig")))
			var rr := Updater.evaluate(body, sig_b64, pem, AppVersion.code() - 1)
			_c(rr["status"] == "available" and rr["info"]["version_code"] == AppVersion.code(), "подпись openssl принимается Godot")
			var bad := body.duplicate()
			bad[20] = bad[20] ^ 1
			_c(Updater.evaluate(bad, sig_b64, pem, 1)["status"] == "error", "один изменённый байт манифеста ломает подпись")
	else:
		print("ключа подписи манифеста нет: проверка совместимости с openssl пропущена")

	# --- обязательность переживает сбой сети и снимается только успешным манифестом ---
	var state_a := "user://updater_settle_a.cfg"
	var state_b := "user://updater_settle_b.cfg"
	DirAccess.remove_absolute(ProjectSettings.globalize_path(state_a))
	DirAccess.remove_absolute(ProjectSettings.globalize_path(state_b))
	var ua := Updater.new()
	ua.state_path = state_a
	var installed := AppVersion.code()
	var none := JSON.stringify(_manifest({"version_code": installed, "version_name": AppVersion.version_name(), "min_version_code": 1})).to_utf8_buffer()
	var rn := ua.settle(Updater.evaluate(none, _sign(key, none), pub, installed))
	_c(rn["status"] == "up_to_date" and not bool(rn["play_blocked"]), "нет обновления: вход не блокируется")
	var opt_body := JSON.stringify(_manifest({"version_code": installed + 1, "min_version_code": 1})).to_utf8_buffer()
	var ro := ua.settle(Updater.evaluate(opt_body, _sign(key, opt_body), pub, installed))
	_c(ro["status"] == "available" and not bool(ro["play_blocked"]), "необязательное обновление не блокирует вход")
	var req_body := JSON.stringify(_manifest({"version_code": installed + 10, "min_version_code": installed + 9})).to_utf8_buffer()
	var rq := ua.settle(Updater.evaluate(req_body, _sign(key, req_body), pub, installed))
	_c(rq["status"] == "required" and bool(rq["play_blocked"]) and ua.play_blocked(), "обязательное обновление сохраняется")
	var down := ua.settle({"status": "error", "error": "Нет связи с сервером обновлений", "info": {}})
	_c(down["status"] == "error" and bool(down["play_blocked"]), "недоступный сервер не снимает подтверждённое требование")
	var forged := ua.settle(Updater.evaluate(req_body, _sign(other, req_body), pub, installed))
	_c(forged["status"] == "error" and ua.play_blocked(), "неверная подпись не снимает обязательное обновление")
	var rolled := ua.settle(Updater.evaluate(old, _sign(key, old), pub, installed))
	_c(rolled["status"] == "error" and ua.play_blocked(), "откат версии не снимает обязательное обновление")
	var fixed := JSON.stringify(_manifest({"version_code": installed + 10, "min_version_code": 1})).to_utf8_buffer()
	var done := ua.settle(Updater.evaluate(fixed, _sign(key, fixed), pub, installed))
	_c(done["status"] == "available" and not ua.play_blocked(), "успешный манифест снимает обязательность, если минимум уже не выше установки")
	var ub := Updater.new()
	ub.state_path = state_b
	var down2 := ub.settle({"status": "error", "error": "Сервер обновлений не ответил вовремя", "info": {}})
	_c(down2["status"] == "error" and not bool(down2["play_blocked"]), "недоступный сервер без требования не блокирует вход")
	var forged2 := ub.settle(Updater.evaluate(good, _sign(other, good), pub, installed))
	_c(forged2["status"] == "error" and not ub.play_blocked(), "неверная подпись сама по себе не блокирует вход")
	ub.busy = true
	var extra := []
	ub.finished.connect(func(_r: Dictionary) -> void: extra.append(1))
	var started: bool = await ub.check()
	_c(not started and extra.is_empty(), "повторная проверка не запускается")
	DirAccess.remove_absolute(ProjectSettings.globalize_path(state_a))
	DirAccess.remove_absolute(ProjectSettings.globalize_path(state_b))
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
