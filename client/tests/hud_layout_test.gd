extends SceneTree
## Раскладка HUD: профили, границы значений, редактор, выбор цели, запрет прятать меню.

const Store := preload("res://ui/hud/layout_store.gd")
const Demo := preload("res://ui/gateway/demo_gateway.gd")

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
	var old_path := Store.path
	Store.path = "user://hud_layout_test.json"
	DirAccess.remove_absolute(ProjectSettings.globalize_path("user://hud_layout_test.json"))
	DirAccess.remove_absolute(ProjectSettings.globalize_path("user://hud_layout_test.json.bak"))
	var bad: Dictionary = Store.import_text("{")
	_c(not bool(bad.get("ok", true)), "битый JSON импорта отклоняется")
	var future: Dictionary = Store.import_text(JSON.stringify({"version": 99, "profiles": {}}))
	_c(not bool(future.get("ok", true)), "слишком новая версия отклоняется")
	var doc := Store.default_doc()
	doc["profiles"]["standard"]["items"] = {
		"nope": {"visible": false},
		"nav.menu": {"visible": false, "opacity": 0.01},
		"player.quests": {"visible": false, "scale": 9.0, "nx": 5.0},
	}
	var clean := Store.sanitize(doc)
	var items: Dictionary = clean["profiles"]["standard"]["items"]
	_c(not items.has("nope"), "неизвестный элемент не сохраняется")
	_c(bool(items["nav.menu"]["visible"]) and float(items["nav.menu"]["opacity"]) == 0.2, "меню нельзя скрыть, прозрачность не ниже 20%")
	_c(float(items["player.quests"]["scale"]) == 2.0 and float(items["player.quests"]["nx"]) == 1.25, "размер и позиция зажимаются")
	var back: Dictionary = Store.import_text(Store.export_text(clean))
	_c(bool(back.get("ok", false)) and back["doc"]["profiles"]["standard"]["items"]["player.quests"]["visible"] == false, "экспорт и импорт сохраняют скрытие заданий")
	var dumped := JSON.stringify({"version": 1, "active": "standard", "profiles": {"custom": {"name": "Свой", "builtin": true, "items": {}}}})
	var imported: Dictionary = Store.import_text(dumped)
	_c(bool(imported.get("ok", false)) and bool(imported["doc"]["profiles"].has("minimal")) and not bool(imported["doc"]["profiles"]["custom"]["builtin"]), "импорт дополняет стандартные профили и не делает свой профиль защищённым")
	Store.save_doc(clean)
	var garbage := FileAccess.open(Store.path, FileAccess.WRITE)
	garbage.store_string("{")
	garbage.close()
	var fallback := Store.load_doc()
	_c(str(fallback.get("active", "")) == "standard" and fallback["profiles"].has("combat"), "повреждённый файл откатывается к рабочей копии или стандарту")

	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	root.add_child(holder)
	var hud: Control = load("res://scenes/hud.tscn").instantiate()
	holder.add_child(hud)
	for i in 3:
		await process_frame
	_c(hud.get_node_or_null("SkillFan/CycleTargetButton") != null or hud.find_child("CycleTargetButton", true, false) != null, "кнопка выбора цели есть в боевом блоке")
	var gw = Demo.new()
	gw.auto_tick = false
	hud.gateway = gw
	hud._cycle_target_pressed()
	_c(str(gw.snapshot().get("target_id", "")) == "pl1", "первая цель — ближайшая")
	hud._cycle_target_pressed()
	_c(str(gw.snapshot().get("target_id", "")) == "pi1", "следующая цель — соседняя по дальности")
	hud.open_hud_editor()
	await process_frame
	_c(hud._editing and hud.get_node_or_null("HudEditor") != null, "редактор открывается поверх HUD")
	var edited: Dictionary = hud.hud_editor_doc()
	edited["profiles"]["standard"]["items"]["player.quests"] = {"visible": false}
	edited["profiles"]["standard"]["items"]["nav.menu"] = {"visible": false}
	hud.hud_editor_set_doc(edited)
	_c(not hud._quests_card.visible, "задания скрываются раскладкой")
	_c(hud._menu_button.visible, "кнопка меню остаётся на экране")
	hud.hud_editor_lift("player.card")
	hud.hud_editor_move("player.card", hud.hud_editor_safe().position + Vector2(30, 28))
	_c(hud._profile_card.get_parent().name == "HudFreeLayer", "перемещение выносит карточку из колонки")
	var reset: Dictionary = hud.hud_editor_doc()
	(reset["profiles"]["standard"]["items"] as Dictionary).erase("player.card")
	hud.hud_editor_set_doc(reset)
	_c(hud._profile_card.get_parent().name == "LeftStack", "сброс возвращает карточку на место")
	var ed: Control = hud.get_node("HudEditor")
	ed._close()
	await process_frame
	_c(not hud._editing and hud.get_node_or_null("HudEditor") == null, "редактор закрывается и снова пускает бой")
	Store.path = old_path
	DirAccess.remove_absolute(ProjectSettings.globalize_path("user://hud_layout_test.json"))
	DirAccess.remove_absolute(ProjectSettings.globalize_path("user://hud_layout_test.json.bak"))
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
