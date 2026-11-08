extends SceneTree
## Установка APK из игры: имя файла, проверка размера и SHA-256, очистка, запасной вариант без плагина.

const Inst := preload("res://ui/apk_installer.gd")

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
	var dir := "user://apk_installer_test"
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(dir))
	_c(Inst.file_path({"version_code": 7}, dir) == dir + "/zvezdnye-rubezhi-7.apk", "имя файла из номера версии")
	_c(Inst.file_path({"version_code": 7, "url": "../../x"}, dir).find("..") == -1, "имя файла не содержит недоверенных символов")
	var p := dir + "/a.apk"
	var f := FileAccess.open(p, FileAccess.WRITE)
	f.store_string("apk-bytes")
	f.close()
	var sha := FileAccess.get_sha256(p)
	_c(Inst.verify_file({"size": 9, "sha256": sha}, p) == "", "верный файл принимается")
	_c(Inst.verify_file({"size": 8, "sha256": sha}, p) != "", "неверный размер отклоняется")
	_c(Inst.verify_file({"size": 9, "sha256": "0".repeat(64)}, p) != "", "неверная сумма отклоняется")
	_c(Inst.verify_file({"size": 9, "sha256": sha}, dir + "/нет.apk") != "", "отсутствующий файл отклоняется")
	var g := FileAccess.open(dir + "/old.apk", FileAccess.WRITE)
	g.store_string("x")
	g.close()
	Inst.clean(dir, p)
	_c(FileAccess.file_exists(p) and not FileAccess.file_exists(dir + "/old.apk"), "очистка оставляет нужный файл и убирает прочие")
	_c(not Inst.available(), "вне Android плагина нет")
	var inst := Inst.new()
	root.add_child(inst)
	var got: Array = []
	inst.finished.connect(func(s: String, m: String) -> void: got.assign([s, m]))
	inst.start({"url": "https://rikagro.ru/download/a.apk", "size": 9, "sha256": sha, "version_code": 7}, dir)
	_c(got.size() == 2 and got[0] == "fallback", "без плагина — запасной вариант через браузер")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
