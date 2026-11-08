extends SceneTree
## Корабли каталога (клиентские описания совпадают с серверными, GLB есть, окраска заглушек) и экран магазина.

const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ShopScreen := preload("res://ui/shop_screen.gd")
const Icons := preload("res://ui/icons.gd")

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


func _meshes(node: Node, out: Array) -> void:
	if node is MeshInstance3D:
		out.append(node)
	for c in node.get_children():
		_meshes(c, out)


func _run() -> void:
	# --- корабли ---
	var server_path := ProjectSettings.globalize_path("res://") + "../server/internal/world/data/ships.json"
	var doc: Variant = JSON.parse_string(FileAccess.get_file_as_string(server_path))
	var server_ids: Array = []
	var server_ships: Dictionary = {}
	if doc is Dictionary:
		for s in doc["ships"]:
			server_ids.append(str(s["id"]))
			server_ships[str(s["id"])] = s
	var client_ids: Array = ShipDef.PATHS.keys()
	server_ids.sort()
	client_ids.sort()
	_c(server_ids == client_ids and server_ids.size() >= 12, "клиентские описания кораблей совпадают с серверным каталогом (%d)" % server_ids.size())
	var temp_count := 0
	for id in client_ids:
		var d := ShipDef.get_def(id)
		var skins: Dictionary = d.get("skins", {})
		var ok: bool = str(d.get("id", "")) == id and not skins.is_empty() and (d.get("points", {}) as Dictionary).size() == 8
		for k in skins:
			ok = ok and ResourceLoader.exists(str(skins[k].get("glb", "")))
		_c(ok, "%s: описание полное, модель существует" % id)
		var srv: Dictionary = server_ships.get(id, {})
		_c((srv.get("skins", []) as Array).size() == skins.size() and bool(srv.get("temporary", false)) == bool(d.get("temporary", false)), "%s: облики и пометка «временная» совпадают с сервером" % id)
		_c((srv.get("specs", []) as Array).all(func(sp: Variant) -> bool: return (d.get("specs", {}) as Dictionary).has(str(sp))), "%s: специализации описаны для ангара" % id)
		if ShipDef.is_temporary(id):
			temp_count += 1
	_c(temp_count == 11 and not ShipDef.is_temporary("FTR-001"), "временными помечены все, кроме «Стрижа»")
	# окраска заглушек не портит оригинальный GLB
	var a := ShipVisual.new()
	root.add_child(a)
	a.build("FTR-001", "a")
	var b := ShipVisual.new()
	root.add_child(b)
	b.build("FTR-002", "a")
	await process_frame
	var ma: Array = []
	var mb: Array = []
	_meshes(a, ma)
	_meshes(b, mb)
	_c(ma.size() > 0 and mb.size() > 0, "модели строятся")
	var ca := ((ma[0] as MeshInstance3D).get_active_material(0) as BaseMaterial3D).albedo_color
	var cb := ((mb[0] as MeshInstance3D).get_active_material(0) as BaseMaterial3D).albedo_color
	_c(not ca.is_equal_approx(cb), "заглушка окрашена иначе, чем «Стриж»")
	_c(((ma[0] as MeshInstance3D).get_surface_override_material(0)) == null, "у «Стрижа» окраска не применяется")
	# --- магазин ---
	var catalog := {"free": true, "currency": "кристаллов", "items": [
		{"id": "vip_1", "kind": "vip", "name": "VIP I", "desc": "d", "price": 299, "icon": "vip"},
		{"id": "badge_star", "kind": "badge", "name": "Звезда", "desc": "d", "price": 99, "icon": "star"},
		{"id": "badge_crown", "kind": "badge", "name": "Корона", "desc": "d", "price": 199, "vip_min": 2, "icon": "crown"},
		{"id": "engine_cyan", "kind": "engine", "name": "Лазурь", "desc": "d", "price": 149, "color": "#40e0ff", "icon": "flame"}]}
	var sh: Control = ShopScreen.new()
	root.add_child(sh)
	await process_frame
	sh.set_data({"catalog": catalog, "state": {"vip_level": 0, "owned": [], "equipped": {}}}, 120)
	await process_frame
	_c(sh._grid.get_child_count() == 1, "на вкладке VIP один товар")
	var item: Dictionary = catalog["items"][1]
	_c(sh.action_for(item)["kind"] == "buy" and sh.action_for(item)["text"] == "Получить", "непокупленное: «Получить»")
	_c(sh.action_for(catalog["items"][2])["kind"] == "locked" and not sh.action_for(catalog["items"][2])["enabled"], "товар VIP II без VIP заблокирован")
	sh.set_data({"catalog": catalog, "state": {"vip_level": 2, "owned": ["badge_star"], "equipped": {}}}, 120)
	_c(sh.action_for(item)["kind"] == "equip" and sh.action_for(catalog["items"][2])["kind"] == "buy", "купленное: «Надеть»; VIP II открыл Корону")
	sh.set_data({"catalog": catalog, "state": {"vip_level": 2, "owned": ["badge_star"], "equipped": {"badge": "badge_star"}}}, 120)
	_c(sh.action_for(item)["kind"] == "unequip", "надетое: «Снять»")
	_c(sh.action_for(catalog["items"][0])["text"] == "Продлить", "VIP активен: «Продлить»")
	sh._select_tab("badge")
	await process_frame
	_c(sh._grid.get_child_count() == 2, "вкладка «Значки» показывает два товара")
	var asked: Array = []
	sh.equip_requested.connect(func(slot: String, id: String) -> void: asked.append([slot, id]))
	sh.buy_requested.connect(func(id: String) -> void: asked.append(["buy", id]))
	(sh._cards["badge_star"] as Button).pressed.emit()
	_c(asked == [["badge", ""]], "«Снять» отправляет пустой товар в слот значка")
	_c((sh._cards["badge_star"] as Button).disabled, "кнопка блокируется до ответа сервера")
	sh.show_error("нужен более высокий VIP-уровень")
	await process_frame
	_c(not (sh._cards["badge_star"] as Button).disabled and sh._notice.text.contains("VIP"), "ошибка сервера возвращает кнопки и показана игроку")
	_c(sh._price_text(item).begins_with("Бесплатно") and sh._credits_label.text == "120", "в режиме «бесплатно» цена помечена, баланс показан")
	_c(Icons.exists("crown") and Icons.exists("vip") and Icons.tex("unknown-name") != null, "иконки загружаются, неизвестное имя даёт запасную")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
