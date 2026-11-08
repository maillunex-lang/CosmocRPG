extends SceneTree
## Докачка контента: подпись манифеста, ссылки, лимиты, откат, выбор пакетов, установка файла и применение пакета при запуске.

const Loader := preload("res://ui/content_loader.gd")
const Boot := preload("res://ui/content_boot.gd")

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


func _pack(over: Dictionary = {}) -> Dictionary:
	var p := {"id": "ships", "version": 2, "url": "https://rikagro.ru/download/content/ships-2.pck", "sha256": "b".repeat(64), "size": 1000, "min_app_code": 1}
	p.merge(over, true)
	return p


func _man(packs: Array) -> PackedByteArray:
	return JSON.stringify({"content_version": 7, "packs": packs}).to_utf8_buffer()


func _sign(key: CryptoKey, bytes: PackedByteArray) -> String:
	var ctx := HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	ctx.update(bytes)
	return Marshalls.raw_to_base64(_crypto.sign(HashingContext.HASH_SHA256, ctx.finish(), key))


func _write(path: String, data: PackedByteArray) -> void:
	var f := FileAccess.open(path, FileAccess.WRITE)
	f.store_buffer(data)
	f.close()


func _run() -> void:
	var key := _crypto.generate_rsa(2048)
	var pub := key.save_to_string(true)
	var other := _crypto.generate_rsa(2048)

	var good := _man([_pack()])
	var sig := _sign(key, good)
	var r := Loader.evaluate(good, sig, pub, 5)
	_c(r["status"] == "ok" and r["packs"].size() == 1 and r["content_version"] == 7, "верный манифест принимается")
	_c(Loader.evaluate(good, _sign(other, good), pub, 5)["status"] == "error", "подпись другим ключом отклоняется")
	_c(Loader.evaluate(_man([_pack({"size": 5})]), sig, pub, 5)["status"] == "error", "изменённый манифест отклоняется")
	_c(Loader.evaluate(good, "", pub, 5)["status"] == "error", "без подписи отклоняется")
	_c(Loader.evaluate(PackedByteArray(), sig, pub, 5)["status"] == "error", "пустой манифест отклоняется")

	for bad in [["url", "http://rikagro.ru/download/content/a.pck"], ["url", "https://evil.example/download/content/a.pck"],
			["url", "https://rikagro.ru@evil.example/download/content/a.pck"], ["url", "https://rikagro.ru/download/content/../a.pck"],
			["url", "https://rikagro.ru/download/content/a.apk"], ["url", "https://rikagro.ru/other/a.pck"],
			["sha256", "zz".repeat(32)], ["sha256", "abc"], ["size", 0], ["size", 65 * 1024 * 1024], ["version", 0],
			["id", "Bad Id"], ["id", ""], ["id", "../x"]]:
		var b := _man([_pack({bad[0]: bad[1]})])
		_c(Loader.evaluate(b, _sign(key, b), pub, 5)["status"] == "error", "отклоняется: %s = %s" % [bad[0], str(bad[1]).left(40)])

	var dup := _man([_pack(), _pack({"version": 3})])
	_c(Loader.evaluate(dup, _sign(key, dup), pub, 5)["status"] == "error", "повтор id пакета отклоняется")
	var big: Array = []
	for i in 4:
		big.append(_pack({"id": "p%d" % i, "size": 60 * 1024 * 1024}))
	var bigm := _man(big)
	_c(Loader.evaluate(bigm, _sign(key, bigm), pub, 5)["status"] == "error", "суммарный размер выше лимита отклоняется")

	var newer := _man([_pack({"min_app_code": 9})])
	var rn := Loader.evaluate(newer, _sign(key, newer), pub, 5)
	_c(rn["status"] == "ok" and rn["packs"].is_empty(), "пакет для более нового приложения пропускается")
	_c(Loader.evaluate(good, sig, pub, 5, {"ships": 3})["status"] == "error", "откат версии пакета отклоняется")
	_c(Loader.evaluate(good, sig, pub, 5, {"ships": 2})["status"] == "ok", "та же версия не считается откатом")

	var pk := [_pack(), _pack({"id": "audio", "version": 1})]
	_c(Loader.to_download(pk, {}).size() == 2, "ничего не установлено: нужны оба")
	_c(Loader.to_download(pk, {"ships": 2}).size() == 1, "установленная версия не качается заново")
	_c(Loader.to_download(pk, {"ships": 1, "audio": 1}).size() == 1, "качается только более новая версия")

	_c(Loader.is_safe_pack_url("https://rikagro.ru/download/content/x.pck"), "ссылка на пакет допустима")
	_c(not Loader.is_safe_pack_url("https://rikagro.ru:444/download/content/x.pck"), "нестандартный порт недопустим")

	# --- установка файла и применение пакета ---
	var tmp_dir := "user://content_test"
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(tmp_dir))
	var pck := tmp_dir + "/src.pck"
	var packer := PCKPacker.new()
	packer.pck_start(pck)
	var res_file := tmp_dir + "/hello.txt"
	_write(res_file, "из пакета".to_utf8_buffer())
	packer.add_file("res://content_test_marker.txt", res_file)
	packer.flush()
	var bytes := FileAccess.get_file_as_bytes(pck)
	var sha := FileAccess.get_sha256(pck)

	var l := Loader.new()
	l.state_path = tmp_dir + "/state.cfg"
	l.dir = tmp_dir + "/packs"
	DirAccess.remove_absolute(ProjectSettings.globalize_path(l.state_path))
	var info := {"id": "tst", "version": 1, "size": bytes.size(), "sha256": sha}
	_write(tmp_dir + "/dl1.tmp", bytes)
	_c(l.install_pack_file(info.merged({"sha256": "0".repeat(64)}, true), tmp_dir + "/dl1.tmp") != "", "неверная сумма отклоняется")
	_c(l.install_pack_file(info.merged({"size": 1}, true), tmp_dir + "/dl1.tmp") != "", "неверный размер отклоняется")
	_c(l.install_pack_file(info, tmp_dir + "/нет.tmp") != "", "отсутствующий файл отклоняется")
	_c(l.installed_versions().is_empty(), "после отказов ничего не записано")
	_c(l.install_pack_file(info, tmp_dir + "/dl1.tmp") == "", "верный файл устанавливается")
	_c(l.installed_versions().get("tst", 0) == 1, "версия записана в состояние")

	_write(tmp_dir + "/dl2.tmp", bytes)
	var info2 := info.merged({"version": 2}, true)
	_c(l.install_pack_file(info2, tmp_dir + "/dl2.tmp") == "", "новая версия устанавливается")
	_c(not FileAccess.file_exists(l.dir + "/tst-1.pck") and FileAccess.file_exists(l.dir + "/tst-2.pck"), "старая версия пакета удалена")

	_c(not FileAccess.file_exists("res://content_test_marker.txt"), "до применения файла из пакета нет")
	var ap := Boot.apply_ready_packs(l.state_path)
	_c(ap.get("tst", 0) == 2, "пакет применён при запуске")
	_c(FileAccess.get_file_as_string("res://content_test_marker.txt") == "из пакета", "файл из пакета доступен по res://")

	# повреждённый файл в каталоге контента не применяется
	var cfg := ConfigFile.new()
	cfg.load(l.state_path)
	cfg.set_value("bad", "version", 1)
	cfg.set_value("bad", "path", l.dir + "/bad-1.pck")
	cfg.set_value("bad", "size", 10)
	cfg.save(l.state_path)
	_write(l.dir + "/bad-1.pck", "мусор".to_utf8_buffer())
	_c(not Boot.apply_ready_packs(l.state_path).has("bad"), "пакет с неверным размером не применяется")

	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
