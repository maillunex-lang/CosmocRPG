extends SceneTree
## Раскладка подписей: без пересечений, лимит, приоритеты, цель всегда видна, границы экрана, детерминизм.

const Nameplates := preload("res://ui/nameplates.gd")

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


func _item(id: String, anchor: Vector2, prio: float, force := false, size := Vector2(150, 28)) -> Dictionary:
	return {"id": id, "anchor": anchor, "size": size, "priority": prio, "force": force}


func _overlap_free(placed: Dictionary) -> bool:
	var ids := placed.keys()
	for i in ids.size():
		for j in range(i + 1, ids.size()):
			if (placed[ids[i]] as Rect2).intersects(placed[ids[j]]):
				return false
	return true


func _run() -> void:
	var area := Rect2(0, 0, 1280, 720)
	# толпа в одной точке (как три пиратских разведчика на скриншоте)
	var crowd: Array = []
	for i in 12:
		crowd.append(_item("e%d" % i, Vector2(640, 300), 500.0 - i))
	var p := Nameplates.place(crowd, area)
	_c(p.size() <= Nameplates.MAX_PLATES, "подписей не больше лимита (%d)" % p.size())
	_c(_overlap_free(p), "подписи в одной точке не пересекаются")
	_c(p.has("e0") and p.has("e1") and not p.has("e11"), "остаются самые приоритетные")
	# цель и свой корабль всегда
	var crowd2 := crowd.duplicate()
	crowd2.append(_item("target", Vector2(640, 300), 1000.0, true))
	crowd2.append(_item("me", Vector2(640, 300), 900.0, true))
	var p2 := Nameplates.place(crowd2, area)
	_c(p2.has("target") and p2.has("me"), "цель и свой корабль подписаны всегда")
	_c(_overlap_free(p2), "с целью и своим кораблём подписи тоже не пересекаются")
	# цель в самом приоритетном месте: ровно над своей точкой
	var solo := Nameplates.place([_item("t", Vector2(500, 400), 1000.0, true, Vector2(100, 30))], area)
	_c((solo["t"] as Rect2).position.is_equal_approx(Vector2(450, 370)), "одиночная подпись стоит над своей точкой")
	# границы экрана
	var edge := Nameplates.place([_item("a", Vector2(5, 5), 10.0), _item("b", Vector2(1275, 715), 9.0)], area)
	_c(area.encloses(edge["a"]) and area.encloses(edge["b"]), "подписи у краёв остаются внутри экрана")
	# лимит и принудительные
	var forced_many: Array = []
	for i in 10:
		forced_many.append(_item("f%d" % i, Vector2(300 + i * 5, 300), 1000.0 - i, true))
	var p3 := Nameplates.place(forced_many, area)
	_c(p3.size() == 10, "принудительные подписи не отбрасываются лимитом")
	# детерминизм
	var again := Nameplates.place(crowd, area)
	var same := again.size() == p.size()
	for k in p:
		same = same and again.has(k) and (again[k] as Rect2).is_equal_approx(p[k])
	_c(same, "одинаковый вход даёт одинаковую раскладку")
	# равные приоритеты: порядок по идентификатору
	var eq := Nameplates.place([_item("b", Vector2(640, 300), 10.0), _item("a", Vector2(640, 300), 10.0)], area)
	_c((eq["a"] as Rect2).position.is_equal_approx(Vector2(565, 272)), "при равных приоритетах первым по идентификатору стоит «a» над своей точкой")
	_c(_overlap_free(eq), "равные приоритеты: без пересечений")
	# сокращение длинных имён
	_c(Nameplates.short_name("Пиратский разведчик усиленный элитный").length() <= Nameplates.MAX_NAME_CHARS and Nameplates.short_name("Ковбой") == "Ковбой", "длинные имена сокращаются, короткие целы")
	# узел: пул подписей, скрытие лишних, освобождение исчезнувших
	var node: Control = Nameplates.new()
	root.add_child(node)
	await process_frame
	var items: Array = []
	for i in 10:
		items.append({"id": "o%d" % i, "text": "Пиратский разведчик %d" % i, "anchor": Vector2(640, 300), "priority": 500.0 - i, "force": false, "alpha": 0.9})
	node.update_plates(items, area)
	_c(node.visible_count() <= Nameplates.MAX_PLATES and node.visible_count() > 0, "на экране не больше лимита подписей (%d)" % node.visible_count())
	node.update_plates(items.slice(0, 2), area)
	await process_frame
	_c(node.visible_count() == 2, "после ухода объектов подписи снимаются")
	# метка ИИ-капитана: иконка перед именем, отступ слева; у обычных подписей иконки нет
	var np: Control = Nameplates.new()
	root.add_child(np)
	await process_frame
	np.update_plates([
		{"id": "ai1", "text": "Валун", "anchor": Vector2(300, 300), "priority": 10.0, "icon": "bot"},
		{"id": "pl1", "text": "Игрок", "anchor": Vector2(700, 300), "priority": 9.0}], area)
	await process_frame
	var l_ai := np._labels["ai1"] as Label
	var l_pl := np._labels["pl1"] as Label
	_c(l_ai.get_node_or_null("Icon") != null and (l_ai.get_node("Icon") as TextureRect).visible and (l_ai.get_node("Icon") as TextureRect).texture != null, "у ИИ-капитана перед именем иконка")
	_c(l_pl.get_node_or_null("Icon") == null, "у обычной подписи иконки нет")
	_c(l_ai.size.x > l_pl.size.x - 1.0 and np._measure("Валун", 17, true).x - np._measure("Валун", 17, false).x == Nameplates.ICON_SPACE, "под иконку отводится место")
	np.update_plates([{"id": "ai1", "text": "Валун", "anchor": Vector2(300, 300), "priority": 10.0}], area)
	await process_frame
	_c(not (l_ai.get_node("Icon") as TextureRect).visible, "иконка пропадает, когда метки нет")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
