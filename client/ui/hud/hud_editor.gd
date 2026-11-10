extends Control
## Визуальный редактор HUD. Касания здесь двигают элементы и не вызывают бой, прыжок или автопилот.

const Tokens := preload("res://ui/theme/tokens.gd")
const Store := preload("res://ui/hud/layout_store.gd")

var hud: Control
var _doc: Dictionary = {}
var _undo: Array = []
var _redo: Array = []
var _selected := ""
var _drag_id := ""
var _drag_from := Vector2.ZERO
var _drag_origin := Vector2.ZERO
var _pointer := -1
var _snap := true
var _grid := true
var _preview := false
var _hidden_open := false
var _panel: PanelContainer
var _status: Label
var _profiles: OptionButton
var _scale: HSlider
var _opacity: HSlider
var _hidden_box: VBoxContainer
var _rename: LineEdit


func open(host: Control) -> void:
	hud = host
	_doc = (hud.call("hud_editor_doc") as Dictionary).duplicate(true)
	_undo.clear()
	_redo.clear()
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	z_index = 60
	_build()
	hud.call("hud_editor_set_editing", true)
	queue_redraw()


func _build() -> void:
	for c in get_children():
		c.queue_free()
	_panel = PanelContainer.new()
	_panel.set_anchors_preset(Control.PRESET_BOTTOM_WIDE)
	_panel.offset_top = -250
	_panel.mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(_panel)
	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(0, 240)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	_panel.add_child(scroll)
	var box := VBoxContainer.new()
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	box.add_theme_constant_override("separation", 6)
	scroll.add_child(box)
	var head := HBoxContainer.new()
	var title := Label.new()
	title.text = "Настройка HUD"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	title.add_theme_font_size_override("font_size", Tokens.FONT_BODY)
	head.add_child(title)
	head.add_child(_btn("Закрыть", _close))
	box.add_child(head)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	_profiles = OptionButton.new()
	_profiles.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_profiles.custom_minimum_size = Vector2(160, 48)
	_fill_profiles()
	_profiles.item_selected.connect(_on_profile)
	row.add_child(_profiles)
	row.add_child(_btn("Копия", _copy_profile))
	row.add_child(_btn("Сброс", _reset_profile))
	row.add_child(_btn("Удалить", _delete_profile))
	box.add_child(row)
	var name_row := HBoxContainer.new()
	_rename = LineEdit.new()
	_rename.placeholder_text = "Имя своего профиля"
	_rename.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_rename.custom_minimum_size = Vector2(0, 48)
	name_row.add_child(_rename)
	name_row.add_child(_btn("Переименовать", _rename_profile))
	box.add_child(name_row)
	_status = Label.new()
	_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_status.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	box.add_child(_status)
	_scale = _slider(box, "Размер", 0.5, 2.0, 1.0, _on_scale)
	_opacity = _slider(box, "Прозрачность", 0.2, 1.0, 1.0, _on_opacity)
	var tools := HBoxContainer.new()
	tools.add_theme_constant_override("separation", 6)
	tools.add_child(_btn("Скрыть", _hide_selected))
	tools.add_child(_btn("Скрытые", _toggle_hidden))
	tools.add_child(_btn("Сброс элемента", _reset_selected))
	tools.add_child(_btn("Блок", _toggle_lock))
	box.add_child(tools)
	var tools2 := HBoxContainer.new()
	tools2.add_theme_constant_override("separation", 6)
	tools2.add_child(_btn("Сетка", _toggle_grid))
	tools2.add_child(_btn("Привязка", _toggle_snap))
	tools2.add_child(_btn("Отмена", _undo_change))
	tools2.add_child(_btn("Повтор", _redo_change))
	tools2.add_child(_btn("Без сетки", _toggle_preview))
	box.add_child(tools2)
	var io := HBoxContainer.new()
	io.add_theme_constant_override("separation", 6)
	io.add_child(_btn("Экспорт JSON", _export_json))
	io.add_child(_btn("Импорт JSON", _import_json))
	box.add_child(io)
	_hidden_box = VBoxContainer.new()
	_hidden_box.visible = false
	box.add_child(_hidden_box)
	_sync_status()


func _btn(text: String, cb: Callable) -> Button:
	var b := Button.new()
	b.text = text
	b.custom_minimum_size = Vector2(0, 48)
	b.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	b.pressed.connect(cb)
	return b


func _slider(parent: Node, caption: String, lo: float, hi: float, value: float, cb: Callable) -> HSlider:
	var row := HBoxContainer.new()
	var l := Label.new()
	l.text = caption
	l.custom_minimum_size = Vector2(140, 0)
	row.add_child(l)
	var s := HSlider.new()
	s.min_value = lo
	s.max_value = hi
	s.step = 0.05
	s.value = value
	s.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	s.custom_minimum_size = Vector2(120, 48)
	s.drag_started.connect(_push_undo)
	s.value_changed.connect(cb)
	row.add_child(s)
	parent.add_child(row)
	return s


func _fill_profiles() -> void:
	_profiles.clear()
	var profiles: Dictionary = _doc.get("profiles", {})
	var active := str(_doc.get("active", "standard"))
	var i := 0
	var pick := 0
	for pid in profiles.keys():
		_profiles.add_item(str(profiles[pid].get("name", pid)), i)
		_profiles.set_item_metadata(i, str(pid))
		if str(pid) == active:
			pick = i
		i += 1
	_profiles.select(pick)


func _selected_profile_id() -> String:
	if _profiles.selected < 0:
		return "standard"
	return str(_profiles.get_item_metadata(_profiles.selected))


func _on_profile(index: int) -> void:
	var pid := str(_profiles.get_item_metadata(index))
	_push_undo()
	_doc["active"] = pid
	_commit()


func _copy_profile() -> void:
	var profiles: Dictionary = _doc["profiles"]
	var src: Dictionary = profiles.get(_selected_profile_id(), {})
	var n := 1
	while profiles.has("user-%d" % n):
		n += 1
	var id := "user-%d" % n
	_push_undo()
	profiles[id] = {"name": "Свой %d" % n, "builtin": false, "items": (src.get("items", {}) as Dictionary).duplicate(true)}
	_doc["active"] = id
	_commit()
	_fill_profiles()


func _reset_profile() -> void:
	var pid := _selected_profile_id()
	_push_undo()
	if pid == "minimal":
		_doc["profiles"][pid]["items"] = Store._minimal_items()
	elif pid == "combat":
		_doc["profiles"][pid]["items"] = Store._combat_items()
	else:
		_doc["profiles"][pid]["items"] = {}
	_commit()


func _delete_profile() -> void:
	var pid := _selected_profile_id()
	var profile: Dictionary = _doc["profiles"].get(pid, {})
	if bool(profile.get("builtin", false)):
		_status.text = "Стандартный профиль нельзя удалить. Его можно сбросить."
		return
	_push_undo()
	_doc["profiles"].erase(pid)
	_doc["active"] = "standard"
	_commit()
	_fill_profiles()


func _rename_profile() -> void:
	var pid := _selected_profile_id()
	var profile: Dictionary = _doc["profiles"].get(pid, {})
	if bool(profile.get("builtin", false)):
		_status.text = "Стандартные имена не меняются. Сделайте копию."
		return
	var title := _rename.text.strip_edges()
	if title == "":
		return
	_push_undo()
	profile["name"] = title
	_doc["profiles"][pid] = profile
	_commit()
	_fill_profiles()


func _items() -> Dictionary:
	return Store.active_items(_doc)


func _item(id: String) -> Dictionary:
	var raw: Variant = _items().get(id, {})
	return raw if raw is Dictionary else {}


func _put_item(id: String, item: Dictionary) -> void:
	var pid := str(_doc.get("active", "standard"))
	var profile: Dictionary = _doc["profiles"][pid]
	var items: Dictionary = profile.get("items", {})
	items[id] = Store.clamp_item(id, item)
	profile["items"] = items
	_doc["profiles"][pid] = profile


func _push_undo() -> void:
	_undo.append(_doc.duplicate(true))
	if _undo.size() > 40:
		_undo.pop_front()
	_redo.clear()


func _undo_change() -> void:
	if _undo.is_empty():
		return
	_redo.append(_doc.duplicate(true))
	_doc = _undo.pop_back()
	_commit()
	_fill_profiles()


func _redo_change() -> void:
	if _redo.is_empty():
		return
	_undo.append(_doc.duplicate(true))
	_doc = _redo.pop_back()
	_commit()
	_fill_profiles()


func _commit() -> void:
	_doc = Store.sanitize(_doc)
	hud.call("hud_editor_set_doc", _doc)
	_sync_status()
	_refresh_hidden()
	queue_redraw()


func _sync_status() -> void:
	if _status == null:
		return
	var title := "Нажмите элемент на экране"
	if _selected != "":
		var row := Store.meta(_selected)
		title = str(row.get("title", _selected))
		var item := _item(_selected)
		if bool(item.get("locked", false)):
			title += " · заблокирован"
		if _scale != null and item.has("scale"):
			_scale.set_value_no_signal(float(item["scale"]))
		if _opacity != null and item.has("opacity"):
			_opacity.set_value_no_signal(float(item["opacity"]))
	_status.text = title + " · профиль «%s»" % str(_doc["profiles"][str(_doc.get("active", "standard"))].get("name", ""))


func _hide_selected() -> void:
	if _selected == "" or not Store.hideable(_selected):
		_status.text = "Этот элемент нельзя скрыть: иначе пропадёт вход в настройки."
		return
	var item := _item(_selected)
	_push_undo()
	item["visible"] = false
	_put_item(_selected, item)
	_commit()


func _reset_selected() -> void:
	if _selected == "":
		return
	_push_undo()
	var pid := str(_doc.get("active", "standard"))
	(_doc["profiles"][pid]["items"] as Dictionary).erase(_selected)
	_commit()


func _toggle_lock() -> void:
	if _selected == "":
		return
	var item := _item(_selected)
	_push_undo()
	item["locked"] = not bool(item.get("locked", false))
	_put_item(_selected, item)
	_commit()


func _toggle_grid() -> void:
	_grid = not _grid
	queue_redraw()


func _toggle_snap() -> void:
	_snap = not _snap


func _toggle_preview() -> void:
	_preview = not _preview
	queue_redraw()


func _toggle_hidden() -> void:
	_hidden_open = not _hidden_open
	_refresh_hidden()


func _refresh_hidden() -> void:
	if _hidden_box == null:
		return
	_hidden_box.visible = _hidden_open
	for c in _hidden_box.get_children():
		c.queue_free()
	if not _hidden_open:
		return
	var any := false
	for id in Store.ids():
		var item := _item(id)
		if item.has("visible") and not bool(item["visible"]):
			any = true
			var sid := id
			var b := _btn("Вернуть: " + str(Store.meta(sid).get("title", sid)), func() -> void:
				var back := _item(sid)
				_push_undo()
				back["visible"] = true
				_put_item(sid, back)
				_commit())
			_hidden_box.add_child(b)
	if not any:
		var empty := Label.new()
		empty.text = "Скрытых элементов нет"
		_hidden_box.add_child(empty)


func _export_json() -> void:
	var text := Store.export_text(_doc)
	var f := FileAccess.open("user://hud_layout_export.json", FileAccess.WRITE)
	if f == null:
		_status.text = "Не удалось записать экспорт"
		return
	f.store_string(text)
	f.close()
	_status.text = "Экспорт записан на устройство: hud_layout_export.json"


func _import_json() -> void:
	if not FileAccess.file_exists("user://hud_layout_export.json"):
		_status.text = "Нет файла импорта. Сначала сделайте экспорт на этом устройстве."
		return
	var got: Dictionary = Store.import_text(FileAccess.get_file_as_string("user://hud_layout_export.json"))
	if not bool(got.get("ok", false)):
		_status.text = str(got.get("error", "Импорт отклонён"))
		return
	_push_undo()
	_doc = got["doc"]
	_commit()
	_fill_profiles()
	_status.text = "Раскладка импортирована"


func _on_scale(v: float) -> void:
	if _selected == "":
		return
	var item := _item(_selected)
	item["scale"] = v
	_put_item(_selected, item)
	hud.call("hud_editor_set_doc", Store.sanitize(_doc))
	queue_redraw()


func _on_opacity(v: float) -> void:
	if _selected == "":
		return
	var item := _item(_selected)
	item["opacity"] = v
	_put_item(_selected, item)
	hud.call("hud_editor_set_doc", Store.sanitize(_doc))
	queue_redraw()


func _close() -> void:
	_commit()
	hud.call("hud_editor_set_editing", false)
	queue_free()


func _gui_input(ev: InputEvent) -> void:
	if not (ev is InputEventMouseButton) and not (ev is InputEventMouseMotion):
		return
	if ev is InputEventMouseButton and ev.button_index != MOUSE_BUTTON_LEFT:
		return
	var pos := (ev as InputEventMouse).position
	if _panel.get_global_rect().has_point(get_global_transform() * pos):
		return
	if ev is InputEventMouseButton and ev.pressed:
		var id := _hit(get_global_transform() * pos)
		_selected = id
		_sync_status()
		if id != "" and not bool(_item(id).get("locked", false)):
			_push_undo()
			_drag_id = id
			_pointer = ev.button_index
			_drag_from = get_global_transform() * pos
			var node := _nodes().get(id) as Control
			_drag_origin = node.global_position if node != null else Vector2.ZERO
			hud.call("hud_editor_lift", id)
		queue_redraw()
		accept_event()
	elif ev is InputEventMouseButton and not ev.pressed:
		if _drag_id != "":
			_write_dragged()
			_drag_id = ""
			_pointer = -1
			_commit()
		accept_event()
	elif ev is InputEventMouseMotion and _drag_id != "":
		var node := _nodes().get(_drag_id) as Control
		if node == null:
			return
		var delta: Vector2 = (get_global_transform() * pos) - _drag_from
		var next: Vector2 = _drag_origin + delta
		if _snap:
			next = Vector2(snapped(next.x, 16.0), snapped(next.y, 16.0))
		next = _guide(next, node)
		hud.call("hud_editor_move", _drag_id, next)
		queue_redraw()
		accept_event()


func _guide(pos: Vector2, node: Control) -> Vector2:
	var safe: Rect2 = hud.call("hud_editor_safe")
	var size := node.size * node.scale
	var center := pos + size * 0.5
	var xs: Array = [safe.position.x, safe.position.x + safe.size.x * 0.5, safe.end.x - size.x]
	var ys: Array = [safe.position.y, safe.position.y + safe.size.y * 0.5, safe.end.y - size.y]
	for other_id in _nodes():
		if other_id == _drag_id:
			continue
		var other := _nodes()[other_id] as Control
		if other == null or not other.visible:
			continue
		var r := other.get_global_rect()
		xs.append(r.position.x)
		ys.append(r.position.y)
	var p := pos
	for x in xs:
		if absf(center.x - (float(x) + size.x * 0.5)) < 8.0 or absf(pos.x - float(x)) < 8.0:
			p.x = float(x)
	for y in ys:
		if absf(pos.y - float(y)) < 8.0:
			p.y = float(y)
	return p


func _write_dragged() -> void:
	var node := _nodes().get(_drag_id) as Control
	if node == null:
		return
	var safe: Rect2 = hud.call("hud_editor_safe")
	var gp := node.global_position
	var item := _item(_drag_id)
	item["nx"] = (gp.x - safe.position.x) / maxf(safe.size.x, 1.0)
	item["ny"] = (gp.y - safe.position.y) / maxf(safe.size.y, 1.0)
	item["scale"] = node.scale.x
	item["opacity"] = node.self_modulate.a
	item["visible"] = node.visible
	item["z"] = node.z_index
	_put_item(_drag_id, item)


func _hit(global_pos: Vector2) -> String:
	var best := ""
	var best_z := -999
	for id in _nodes():
		var node := _nodes()[id] as Control
		if node == null or not node.visible:
			continue
		if node.get_global_rect().has_point(global_pos) and node.z_index >= best_z:
			best = id
			best_z = node.z_index
	return best


func _nodes() -> Dictionary:
	return hud.call("hud_editor_nodes")


func _draw() -> void:
	if hud == null or _preview:
		return
	if _grid:
		var safe: Rect2 = hud.call("hud_editor_safe")
		var local := get_global_transform().affine_inverse()
		draw_rect(Rect2(local * safe.position, safe.size), Color(Tokens.ACCENT, 0.35), false, 1.0)
		var x := safe.position.x
		while x < safe.end.x:
			draw_line(local * Vector2(x, safe.position.y), local * Vector2(x, safe.end.y), Color(Tokens.ACCENT, 0.12), 1.0)
			x += 32.0
		var y := safe.position.y
		while y < safe.end.y:
			draw_line(local * Vector2(safe.position.x, y), local * Vector2(safe.end.x, y), Color(Tokens.ACCENT, 0.12), 1.0)
			y += 32.0
	for id in _nodes():
		var node := _nodes()[id] as Control
		if node == null or not node.visible:
			continue
		var r := node.get_global_rect()
		var lr := Rect2(get_global_transform().affine_inverse() * r.position, r.size)
		var hot: bool = str(id) == _selected
		draw_rect(lr, Color(Tokens.RARE if hot else Tokens.ACCENT, 0.9 if hot else 0.35), false, 2.0 if hot else 1.0)
		if hot:
			for other_id in _nodes():
				if other_id == id:
					continue
				var other := _nodes()[other_id] as Control
				if other != null and other.visible and other.get_global_rect().intersects(r):
					draw_rect(lr, Color(Tokens.DANGER, 0.85), false, 2.0)
					break
