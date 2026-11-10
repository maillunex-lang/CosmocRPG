extends SceneTree
## Экран карты: соответствие мир-экран, выбор объектов нажатием, кнопка автопилота, закрытие.

const MapScreen := preload("res://ui/map_screen.gd")

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
	var data := {"layout": {"radius": 1400, "hub": [-12, 14],
			"bodies": [{"id": "pl-1", "name": "Планета", "kind": "planet", "pos": [-300, -500], "radius": 40, "ring": true, "color": [0.4, 0.5, 0.8]}],
			"stations": [{"id": "st1", "name": "Рубеж", "pos": [-12, 14], "safe_radius": 40}, {"id": "st2", "name": "Порт", "pos": [500, 100], "safe_radius": 35}],
			"zones": [{"id": "z1", "type": "mining", "pos": [-20, -200], "radius": 67}, {"id": "z2", "type": "pve", "pos": [600, -400], "radius": 100}]},
		"state": {"ship": {"pos": Vector2(0, 0), "dir": Vector2(1, 0)}, "objects": [
			{"id": "dep-1", "kind": "deposit", "name": "Месторождение", "pos": Vector2(-30, -190), "resource": "crystals", "dead": false},
			{"id": "en-sp-1", "kind": "hostile", "name": "Рейдер", "pos": Vector2(620, -380), "dead": false},
			{"id": "en-dead", "kind": "hostile", "name": "Убит", "pos": Vector2(0, 600), "dead": true}]}}
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	root.add_child(holder)
	var m: Control = MapScreen.new()
	holder.add_child(m)
	await process_frame
	m.set_data(data["layout"], data["state"])
	await process_frame
	await process_frame
	var inside := Rect2(Vector2.ZERO, Vector2(1280 - 320, 720))
	var edge: Vector2 = m.to_screen(Vector2(1400, 0))
	var edge2: Vector2 = m.to_screen(Vector2(-1400, 0))
	_c(inside.encloses(Rect2(edge2, edge - edge2).abs()), "вся система помещается на экране (края %s и %s)" % [edge, edge2])
	_c(m.to_world(m.to_screen(Vector2(123, -456))).is_equal_approx(Vector2(123, -456)), "преобразование мир-экран обратимо")
	_c(m.to_screen(Vector2(500, 100)).x > m.to_screen(Vector2(-12, 14)).x and m.to_screen(Vector2(0, -300)).y < m.to_screen(Vector2(0, 0)).y, "север сверху, восток справа")
	var got: Array = []
	m.target_selected.connect(func(id: String) -> void: got.append(id))
	_c(m.pick_at(m.to_screen(Vector2(500, 100)) + Vector2(5, 3)) == "st2" and got == ["st2"], "нажатие на станцию выбирает её")
	_c(m.pick_at(m.to_screen(Vector2(-30, -190))) == "dep-1", "нажатие на месторождение выбирает его")
	_c(m.pick_at(m.to_screen(Vector2(620, -380))) == "en-sp-1", "нажатие на врага выбирает его")
	_c(m.pick_at(m.to_screen(Vector2(0, 600))) == "", "убитый враг не выбирается")
	_c(m.pick_at(m.to_screen(Vector2(-900, 900))) == "" , "нажатие в пустоту ничего не выбирает")
	m.select("st2")
	_c(not m._auto.disabled and m._info.text.contains("Порт"), "для выбранной цели доступен автопилот, подпись показана")
	var auto: Array = []
	m.autopilot_requested.connect(func(id: String) -> void: auto.append(id))
	m._auto.pressed.emit()
	_c(auto == ["st2"], "кнопка «Автопилот» отправляет выбранную цель")
	m.selected = ""
	m._refresh_info()
	_c(m._auto.disabled, "без цели автопилот недоступен")
	# Скрытая цель не остаётся доступной автопилоту и не выбирается.
	m.select("dep-1")
	m.set_filter("deposit", false)
	await process_frame
	await process_frame
	_c(m.pick_at(m.to_screen(Vector2(-30, -190))) == "" and m._auto.disabled, "фильтр скрывает добычу и блокирует автопилот к скрытой цели")
	m.set_filter("deposit", true)
	await process_frame
	await process_frame
	_c(m.pick_at(m.to_screen(Vector2(-30, -190))) == "dep-1", "возврат фильтра восстанавливает выбор")
	_c(m.object_visible("anomaly") and m.object_visible("ai"), "категории исследования и пилотов включены")
	m.set_filter("explore", false)
	_c(not m.object_visible("signal") and not m.object_visible("derelict"), "единый фильтр всех исследуемых объектов")
	m.set_filter("explore", true)
	await process_frame
	await process_frame
	m.select("st2")
	var was_favorite: bool = m._is_favorite("st2")
	m._toggle_favorite()
	_c(m._is_favorite("st2") != was_favorite, "переключение избранной точки")
	if not m._is_favorite("st2"):
		m._toggle_favorite()
	_c(m._favorites.item_count >= 2, "избранная точка доступна в списке")
	m._choose_favorite(1)
	_c(m.selected == "st2", "избранное выбирает существующую цель")
	m._point_stars.clear()
	_c(m._is_favorite("st2"), "избранное загружается с диска")
	if not was_favorite:
		m._toggle_favorite()
	m.reset_view()
	var scroll: ScrollContainer = m.get_child(0)
	holder.size = Vector2i(960, 540)
	await process_frame
	await process_frame
	_c(scroll.get_global_rect().end.x <= 960 and scroll.get_global_rect().position.x >= 0, "панель карты помещается на компактном экране")
	_c(scroll.get_v_scroll_bar().max_value > scroll.size.y, "длинная панель доступна прокруткой")
	# малая система без раскладки (демо) тоже рисуется
	m.set_data({}, {"ship": {"pos": Vector2(10, 10), "dir": Vector2(0, 1)}, "objects": []})
	await process_frame
	_c(is_equal_approx(m.radius(), 400.0), "без раскладки радиус по умолчанию 400")
	var closed := [false]
	m.closed.connect(func() -> void: closed[0] = true)
	m.closed.emit()
	_c(closed[0], "закрытие")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
