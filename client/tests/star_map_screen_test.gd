extends SceneTree

const StarMap := preload("res://ui/star_map_screen.gd")
const Settings := preload("res://ui/ui_settings.gd")

class Catalog extends "res://ui/gateway/ui_gateway.gd":
	func star_systems() -> Array:
		return [{"id": "aurora", "name": "Аврора", "faction": "Конкордат"}, {"id": "meridian", "name": "Меридиан", "faction": "Союз"}]
	func snapshot() -> Dictionary:
		return {"system": {"id": "aurora"}, "ship": {"pos": Vector2.ZERO}}

var failures := 0
var checks := 0

func _initialize() -> void:
	_run.call_deferred()

func check(ok: bool, title: String) -> void:
	checks += 1
	if not ok:
		failures += 1
	print("PASS  " if ok else "FAIL  ", title)

func _run() -> void:
	var catalog := Catalog.new()
	var screen := StarMap.new()
	root.add_child(catalog)
	root.add_child(screen)
	screen.bind(catalog)
	check(screen._rows.get_child_count() == 2, "показан каталог систем")
	screen._search.text = "  МЕРИДИАН "
	screen._rebuild()
	check(screen._rows.get_child_count() == 1, "поиск без учёта регистра и пробелов")
	check(StarMap.matches_system(catalog.star_systems()[0], "конкордат"), "поиск по фракции")
	screen._search.text = "нет такой системы"
	screen._rebuild()
	check(screen._rows.get_child(0) is Label, "понятное пустое состояние")
	screen._search.text = ""
	Settings.override("map_system_aurora", true)
	Settings.override("map_system_meridian", false)
	screen._favorites_only = true
	screen._rebuild()
	check(screen._rows.get_child_count() == 1, "фильтр избранных систем")
	screen._favorites_only = false
	screen._rebuild()
	check(screen._rows.get_child_count() == 2, "снятие фильтра возвращает каталог")
	Settings.override("map_system_aurora", null)
	Settings.override("map_system_meridian", null)
	screen.queue_free()
	catalog.queue_free()
	await process_frame
	print("Итого: проверок %d, провалов %d" % [checks, failures])
	quit(1 if failures else 0)
