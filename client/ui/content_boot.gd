extends Node
## Автозагрузка (autoload): при каждом запуске, ДО загрузки сцен, подключает скачанные пакеты контента (.pck) поверх ресурсов игры.
## Пакеты скачивает ui/content_loader.gd (подпись и контрольные суммы проверены при скачивании); здесь они только применяются.
## Применение при запуске, а не на лету: так ресурсы, уже загруженные в память, не оказываются наполовину старыми.

const STATE_PATH := "user://content/state.cfg"

## id пакета -> применённая версия (для экрана настроек и проверок).
static var applied: Dictionary = {}


func _init() -> void:
	apply_ready_packs()


## Подключает готовые пакеты из состояния. Повреждённый или пропавший файл пропускается (игра работает на встроенных ресурсах).
static func apply_ready_packs(state_path: String = STATE_PATH) -> Dictionary:
	applied.clear()
	var cfg := ConfigFile.new()
	if cfg.load(state_path) != OK:
		return applied
	for id in cfg.get_sections():
		var path := str(cfg.get_value(id, "path", ""))
		var size := int(cfg.get_value(id, "size", 0))
		if path == "" or not FileAccess.file_exists(path):
			continue
		var f := FileAccess.open(path, FileAccess.READ)
		if f == null or int(f.get_length()) != size:
			continue   # размер не совпал: файл не доверяем
		f.close()
		if ProjectSettings.load_resource_pack(path, true):
			applied[id] = int(cfg.get_value(id, "version", 0))
	return applied


## Короткая сводка применённого контента: «ships 3, audio 1» или пусто.
static func summary() -> String:
	var parts: PackedStringArray = []
	for id in applied:
		parts.append("%s %d" % [id, applied[id]])
	return ", ".join(parts)
