extends SceneTree
## Звёздная карта (граф галактики): поиск, избранное, пустое состояние, выбор и прыжок.

const StarMap := preload("res://ui/star_map_screen.gd")
const Settings := preload("res://ui/ui_settings.gd")

class Catalog extends "res://ui/gateway/ui_gateway.gd":
	var systems: Array = [
		{"id": "aurora", "name": "Аврора", "faction": "Солярный Конкордат"},
		{"id": "meridian", "name": "Меридиан", "faction": "Союз Свободных Систем"},
	]
	var jumped := ""
	func star_systems() -> Array:
		return systems
	func snapshot() -> Dictionary:
		return {"system": {"id": "aurora"}, "ship": {"pos": Vector2(1150, 200)}, "credits": 100}
	func jump_gates() -> Array:
		return [{"to": "meridian", "pos": [1150.0, 200.0]}]
	func gate_links() -> Array:
		return [["aurora", "meridian"]]
	func jump(to: String) -> void:
		jumped = to

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
	var screen: Control = StarMap.new()
	root.add_child(catalog)
	root.add_child(screen)
	await process_frame
	screen.bind(catalog)
	await process_frame

	check(screen._visible_systems().size() == 2, "показан каталог систем")
	check(StarMap.matches_system(catalog.star_systems()[0], "  конкордат "), "поиск по фракции без учёта регистра/пробелов")
	check(StarMap.matches_system(catalog.star_systems()[1], "меридиан"), "поиск по имени")
	check(not StarMap.matches_system(catalog.star_systems()[0], "шторм"), "несовпадение отклоняется")

	screen._search.text = "меридиан"
	check(screen._visible_systems().size() == 1, "поиск фильтрует каталог")
	screen._search.text = "нет такой системы"
	check(screen._visible_systems().is_empty(), "пустой результат поиска")
	screen._search.text = ""

	Settings.override("map_system_aurora", true)
	Settings.override("map_system_meridian", false)
	screen._favorites_only = true
	check(screen._visible_systems().size() == 1 and str(screen._visible_systems()[0].get("id")) == "aurora", "фильтр избранных")
	screen._favorites_only = false
	Settings.override("map_system_aurora", null)
	Settings.override("map_system_meridian", null)
	check(screen._visible_systems().size() == 2, "снятие фильтра возвращает каталог")

	# выбор системы тапом: маршрут и прыжок
	screen._selected = "meridian"
	screen._update_notice()
	check(screen._jump_btn.visible, "кнопка прыжка появляется для связанной системы")
	check(screen._notice.text.contains("Меридиан"), "подсказка маршрута с дистанцией")
	screen._on_jump_pressed()
	check(catalog.jumped == "meridian", "прыжок отправляется шлюзу")

	# несвязанная система: без кнопки прыжка
	screen._selected = ""
	catalog.systems.append({"id": "redstorm", "name": "Красный Шторм", "faction": "Доминион Веспер"})
	screen._selected = "redstorm"
	screen._update_notice()
	check(not screen._jump_btn.visible and screen._notice.text.contains("сосед"), "без прямого коридора прыжок недоступен")

	screen.queue_free()
	catalog.queue_free()
	await process_frame
	print("Итого: проверок %d, провалов %d" % [checks, failures])
	quit(1 if failures else 0)
