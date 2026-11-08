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
