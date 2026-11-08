extends Node
## Скачивание и установка обновления APK прямо из игры (Android). Файл качается в user://updates, проверяется размер и SHA-256
## из подписанного манифеста, затем запускается системный установщик (плагин ApkInstaller). Подтверждение установки даёт сама Android.
## Если плагина нет (не Android, старая сборка) — вызывающий код открывает ссылку в браузере.

const Updater := preload("res://ui/updater.gd")

const DIR := "user://updates"

## status: installing | need_permission | fallback | error
signal progress(fraction: float)
signal finished(status: String, message: String)

var busy := false
var _http: HTTPRequest


static func plugin() -> Object:
	return Engine.get_singleton("ApkInstaller") if Engine.has_singleton("ApkInstaller") else null


static func available() -> bool:
	return plugin() != null


## Имя файла из номера версии: без недоверенных символов из манифеста.
static func file_path(info: Dictionary, dir: String = DIR) -> String:
	return "%s/zvezdnye-rubezhi-%d.apk" % [dir, int(info.get("version_code", 0))]


## Проверка скачанного файла по манифесту: "" — всё верно.
static func verify_file(info: Dictionary, path: String) -> String:
	var f := FileAccess.open(path, FileAccess.READ)
	if f == null:
		return "Файл обновления не найден"
	var size := int(f.get_length())
	f.close()
	if size != int(info.get("size", -1)):
		return "Размер файла обновления не совпал"
	if FileAccess.get_sha256(path).to_lower() != str(info.get("sha256", "")).to_lower():
		return "Контрольная сумма обновления не совпала"
	return ""


## Очистка прежних файлов обновлений (каталог только для них).
static func clean(dir: String = DIR, keep: String = "") -> void:
	var d := DirAccess.open(dir)
	if d == null:
		return
	for n in d.get_files():
		if n != keep.get_file():
			d.remove(n)


func start(info: Dictionary, dir: String = DIR) -> void:
	if busy:
		return
	var p := plugin()
	var url := str(info.get("url", ""))
	if p == null or not Updater.is_safe_url(url):
		finished.emit("fallback", "")
		return
	if not bool(p.canInstall()):
		p.openInstallSettings()
		finished.emit("need_permission", "Разрешите установку из этого источника для игры и вернитесь: загрузка продолжится.")
		return
	busy = true
	var path := file_path(info, dir)
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(dir))
	clean(dir, path)
	if verify_file(info, path) != "":
		var err := await _download(url, path, int(info.get("size", 0)))
		if err != "":
			busy = false
			finished.emit("error", err)
			return
	var bad := verify_file(info, path)
	if bad != "":
		DirAccess.remove_absolute(ProjectSettings.globalize_path(path))
		busy = false
		finished.emit("error", bad)
		return
	var msg := str(p.install(ProjectSettings.globalize_path(path)))
	busy = false
	if msg != "":
		finished.emit("error", msg)
	else:
		finished.emit("installing", "Подтвердите установку в окне Android. Прогресс игры сохранится.")


func _download(url: String, path: String, size: int) -> String:
	if size <= 0 or size > Updater.MAX_APK_BYTES:
		return "Некорректный размер обновления"
	var tmp := path + ".part"
	_http = HTTPRequest.new()
	_http.download_file = tmp
	_http.body_size_limit = size + 1024
	_http.timeout = 180.0
	add_child(_http)
	if _http.request(url) != OK:
		_http.queue_free()
		return "Не удалось начать загрузку"
	var done := [false]
	var res: Array = []
	_http.request_completed.connect(func(a: int, b: int, _c: PackedStringArray, _d: PackedByteArray) -> void:
		res.assign([a, b])
		done[0] = true)
	while not done[0]:
		progress.emit(clampf(float(_http.get_downloaded_bytes()) / float(size), 0.0, 1.0))
		await get_tree().create_timer(0.3).timeout
	_http.queue_free()
	if int(res[0]) != HTTPRequest.RESULT_SUCCESS or int(res[1]) != 200:
		DirAccess.remove_absolute(ProjectSettings.globalize_path(tmp))
		return "Загрузка прервана (%d). Проверьте соединение." % int(res[1])
	if DirAccess.rename_absolute(ProjectSettings.globalize_path(tmp), ProjectSettings.globalize_path(path)) != OK:
		return "Не удалось сохранить файл обновления"
	return ""
