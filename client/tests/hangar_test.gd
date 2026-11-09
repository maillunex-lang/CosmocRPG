extends SceneTree
## Ангар: открывается поверх космоса, показывает характеристики, переключает облик и специализацию.

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")

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
	var gw := DemoGateway.new()
	gw.auto_tick = false
	Session.gateway = gw
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	root.add_child(screen)
	for i in 5:
		await process_frame
	var me: Node3D = screen.ship_view("me")
	_c(me != null and me.visual != null and me.ship_type == "FTR-001", "свой корабль — «Стриж»")
	var h: Control = screen.open_hangar()
	await process_frame
	_c(h != null and h.is_inside_tree(), "ангар открылся поверх космоса")
	_c(screen.open_hangar() == h, "повторное открытие не создаёт второй ангар")
	_c(h._stats.text.contains("Корпус") and h._stats.text.contains("щит"), "характеристики показаны")
	_c(h._skin_buttons.size() == 2 and h._spec_buttons.size() == 2, "два облика и две специализации")
	_c(h._visual != null and h._visual.point("Dock_Point") != null, "3D-предпросмотр корабля создан")
	h._skin_buttons["b"].pressed.emit()
	await process_frame
	_c(h.skin == "b" and h._visual.skin_id == "b", "в ангаре выбран облик «b»")
	_c(me.skin == "b", "облик сразу применён к кораблю в космосе")
	_c((h._skin_buttons["b"] as Button).button_pressed and not (h._skin_buttons["a"] as Button).button_pressed, "кнопки облика отражают выбор")
	var specs: Array = []
	h.spec_selected.connect(func(k: String) -> void: specs.append(k))
	h._spec_buttons["scout"].pressed.emit()
	_c(specs == ["scout"] and h.spec == "scout", "выбрана специализация «Разведчик»")
	var before_hull: float = h._data.get("hull_max", 0.0)
	_c(before_hull > 0.0, "характеристики не зависят от выбора (корпус %d)" % int(before_hull))
	# The server-provided plan is optional; old servers/demo keep the old view.
	_c(not h._passport_info.visible, "без паспорта новая секция скрыта")
	h.set_data({"type": "FTR-001", "level": 31, "hull_max": 2000, "shield_max": 700, "energy_max": 150})
	h.set_fleet([{"id": "FTR-001", "name": "Стриж", "max_level": 50, "rpg_passport": {
		"id": "FTR-001", "description": "Серверный паспорт", "mass_tons": 18, "reactor_power_mw": 100,
		"module_mass_budget_tons": 12, "base": {"crew": 3, "engines": 2, "weapons": "Серверное оружие", "dimensions_m": [12, 8, 3]},
		"slots": [{"group": "weapon"}, {"group": "technical"}]}}])
	_c(h._passport_info.visible and h._passport_info.text.contains("Серверный паспорт"), "паспорт сервера отображён")
	_c(h._passport_info.text.contains("предварительно") and h._passport_info.text.contains("100"), "предварительный бюджет обозначен")
	_c(h._info.text.contains("Экипаж 3") and h._info.text.contains("Серверное оружие"), "паспорт использует серверный base")
	_c(h._stats.text.contains("31 из 50") and h._stats.text.contains("2000"), "уровень и живые статы определяет сервер")
	h.set_fleet([{"id": "FTR-001", "rpg_passport": {"id": "TMP-MINE"}}])
	_c(not h._passport_info.visible and not h._stats.text.contains("из 30"), "чужой паспорт и устаревший cap не показаны")
	h.set_fleet([{"id": "FTR-001", "name": "Стриж"}])
	_c(not h._passport_info.visible and h._passport.is_empty(), "старый ответ очищает прежний паспорт")
	var server_gateway: Node = load("res://ui/gateway/server_gateway.gd").new()
	var received: Array = []
	server_gateway.ships_updated.connect(func(fleet: Array) -> void: received.append(fleet))
	server_gateway._on_message("ships", {"ships": [{"id": "FTR-001"}], "ship_max_level": 50,
		"levels": {"FTR-001": {"level": 31}}, "passport": {"id": "FTR-001", "description": "Из сети"}}, "")
	_c(received.size() == 1 and received[0][0].get("max_level", 0) == 50 and received[0][0].get("level", 0) == 31, "шлюз сохраняет серверный cap и уровень")
	_c(received[0][0].get("rpg_passport", {}).get("description", "") == "Из сети", "паспорт проходит через сетевой шлюз")
	server_gateway._on_message("ships", {"ships": [{"id": "TMP-MINE"}], "passport": {"id": "FTR-001"}}, "")
	_c(not received[1][0].has("rpg_passport"), "шлюз не прикрепляет чужой паспорт")
	server_gateway._on_message("ships", {"ships": [{"id": "FTR-001"}]}, "")
	_c(not received[2][0].has("rpg_passport") and not received[2][0].has("max_level"), "шлюз совместим со старым сервером")
	var wire_ship := {"type": "FTR-001", "hull": 1200, "hull_max": 1200, "energy": 100, "energy_max": 100,
		"abilities": [{"id": "boost", "catalog_id": "M01", "name": "Форсаж", "energy_cost": 20, "cooldown": 12,
			"cooldown_left": 0, "enabled": false, "unavailable_reason": "module_unavailable"}],
		"modules": [{"prototype_id": "ENG-S-START", "owned": true, "equipped": true}]}
	server_gateway._on_message("state", {"ship": wire_ship}, "")
	var unavailable: Dictionary = server_gateway.snapshot()
	_c(server_gateway._ability_ids[0] == "M01" and not unavailable["skills"][0]["enabled"], "шлюз сохраняет M01 и серверный запрет")
	var hud: Control = screen.get_node("%Hud")
	hud._apply_buttons(unavailable, {}, unavailable["ship"])
	_c(hud._skill_buttons[0].disabled and hud._skill_buttons[0].tooltip_text.contains("модуль"), "HUD блокирует навык без модуля и объясняет причину")
	h.set_data(unavailable["ship"])
	_c(h._module_info.visible and h._module_info.text.contains("M01") and h._module_info.text.contains("установлен"), "ангар показывает действующий двигатель отдельно от плана")
	wire_ship["abilities"] = [{"id": "boost", "name": "Форсаж", "energy_cost": 20, "cooldown": 12}]
	server_gateway._on_message("state", {"ship": wire_ship}, "")
	var old_state: Dictionary = server_gateway.snapshot()
	_c(server_gateway._ability_ids[0] == "boost" and old_state["skills"][0]["enabled"], "старый сервер без новых полей сохраняет прежний навык")
	hud._apply_buttons(old_state, {}, old_state["ship"])
	_c(not hud._skill_buttons[0].disabled, "HUD не блокирует старый совместимый навык")
	_c(not h._engine_button.visible, "старый сервер не показывает неподдерживаемую команду")
	wire_ship["equipment"] = {"revision": 4, "can_change": true}
	wire_ship["modules"] = [{"prototype_id": "ENG-S-START", "owned": true, "equipped": true}]
	server_gateway._on_message("state", {"ship": wire_ship}, "")
	var saved_state: Dictionary = server_gateway.snapshot()
	h.set_data(saved_state["ship"])
	var changes: Array = []
	h.engine_selected.connect(func(on: bool, revision: int) -> void: changes.append([on, revision]))
	_c(h._engine_button.visible and not h._engine_button.disabled and h._engine_button.text.contains("Снять"), "сервер разрешил снятие в доке")
	h._engine_button.pressed.emit()
	_c(changes == [[false, 4]] and h._data["modules"][0]["equipped"], "команда несёт ревизию и не меняет отображение до подтверждения")
	wire_ship["equipment"] = {"revision": 5, "can_change": false, "reason": "equipment_in_combat"}
	wire_ship["modules"][0]["equipped"] = false
	server_gateway._on_message("state", {"ship": wire_ship}, "")
	h.set_data(server_gateway.snapshot()["ship"])
	_c(h._engine_button.disabled and h._engine_reason.text.contains("10 секунд") and h._engine_button.text.contains("Установить"), "снятый двигатель и запрет боя подтверждает сервер")
	h._engine_button.pressed.emit()
	_c(changes.size() == 1, "disabled state не отправляет команду даже при прямом сигнале")
	wire_ship["equipment"] = {"revision": 5, "can_change": false, "pending": true, "reason": "equipment_busy"}
	server_gateway._on_message("state", {"ship": wire_ship}, "")
	h.set_data(server_gateway.snapshot()["ship"])
	_c(h._engine_button.disabled and h._engine_reason.text.contains("Сохраняем"), "pending status объясняет сохранение оснащения")
	wire_ship["equipment"] = {"can_change": false, "reason": "equipment_unavailable"}
	server_gateway._on_message("state", {"ship": wire_ship}, "")
	h.set_data(server_gateway.snapshot()["ship"])
	_c(h._equipment_refresh.visible and h._engine_button.disabled, "неопределённое состояние предлагает перечитать оснащение")
	server_gateway.free()
	h.closed.emit()
	await process_frame
	await process_frame
	_c(screen.get_node_or_null("HangarLayer/HangarOverlay") == null, "ангар закрывается")
	# события боя доходят до эффектов
	var dead_before: bool = me.visual.alive
	screen._on_combat_event({"type": "hit_hull", "to": "me", "amount": 10})
	_c(me.visual._sparks != null and me.visual._sparks.emitting, "событие попадания по корпусу показывает искры")
	screen._on_combat_event({"type": "hit_shield", "to": "me"})
	_c(me.visual._shell != null and me.visual._shell.visible, "событие попадания по щиту показывает оболочку")
	screen._on_combat_event({"type": "death", "to": "me"})
	_c(dead_before and not me.visual.alive, "событие гибели запускает разрушение")
	screen._on_combat_event({"type": "respawn", "to": "me"})
	_c(me.visual.alive, "событие возрождения возвращает корабль")
	screen._on_combat_event({"type": "fire", "from": "me", "to": "pi1"})
	_c(me.visual._flash.size() == 2, "событие выстрела показывает вспышки обеих пушек")
	Session.gateway = null
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
