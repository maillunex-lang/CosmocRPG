extends SceneTree
## Прокрутка пальцем (R-50): рамки внутри ScrollContainer пропускают касание, кнопки остаются кнопками; профиль помещается на экран 1280×720.

const TouchScroll := preload("res://ui/touch_scroll.gd")
const ProfileScreen := preload("res://ui/profile_screen.gd")
const SettingsScreen := preload("res://ui/settings_screen.gd")
const ShopScreen := preload("res://ui/shop_screen.gd")

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
	var ts := TouchScroll.new()
	root.add_child(ts)
	var scroll := ScrollContainer.new()
	root.add_child(scroll)
	var card := PanelContainer.new()
	var inner := PanelContainer.new()
	var btn := Button.new()
	var label := Label.new()
	var edit := LineEdit.new()
	scroll.add_child(card)
	card.add_child(inner)
	inner.add_child(btn)
	inner.add_child(label)
	inner.add_child(edit)
	await process_frame
	_c(card.mouse_filter == Control.MOUSE_FILTER_PASS and inner.mouse_filter == Control.MOUSE_FILTER_PASS, "рамки внутри ScrollContainer пропускают касание")
	_c(btn.mouse_filter == Control.MOUSE_FILTER_STOP, "кнопка остаётся кнопкой: случайных нажатий при прокрутке нет")
	_c(edit.mouse_filter == Control.MOUSE_FILTER_STOP, "поле ввода не трогается")
	var outside := PanelContainer.new()
	root.add_child(outside)
	await process_frame
	_c(outside.mouse_filter == Control.MOUSE_FILTER_STOP, "рамка вне прокрутки не меняется")
	# узлы, созданные до включения: fix_tree
	var scroll2 := ScrollContainer.new()
	var late := PanelContainer.new()
	scroll2.add_child(late)
	_c(TouchScroll.fix_tree(scroll2) == 1 and late.mouse_filter == Control.MOUSE_FILTER_PASS, "fix_tree исправляет уже созданные рамки")
	scroll2.free()

	# профиль на экране 1280×720 с развитием пилота
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	holder.disable_3d = true
	root.add_child(holder)
	var p: Control = ProfileScreen.new()
	holder.add_child(p)
	p.set_data({"player": {"name": "Тестовый"}, "credits": 580, "ship": {"name": "Стриж", "cosmetics": {"vip": 7, "title": "Ас фронтира"}}}, {"subscription_until": "2027-01-07T00:00:00Z"})
	p.set_pilot({"level": 1, "max_level": 60, "into_level": 0, "level_xp": 100, "attr_free": 0, "attributes": {}})
	for i in 4:
		await process_frame
	var panel := p.find_child("ProfileScroll", true, false).get_parent() as Control
	_c(panel.size.y <= 720.0 and panel.get_global_rect().end.y <= 720.0, "панель профиля помещается по высоте (%d из 720)" % int(panel.size.y))
	var sc := p.find_child("ProfileScroll", true, false) as ScrollContainer
	var content := sc.get_child(0) as Control
	_c(content.size.y > sc.size.y, "содержимое выше области: прокрутка нужна (%d > %d)" % [int(content.size.y), int(sc.size.y)])
	sc.scroll_vertical = 10000
	await process_frame
	_c(sc.scroll_vertical > 0, "профиль прокручивается вниз до кнопки магазина")
	_c(sc.find_child("ShopButton", true, false) != null, "кнопка магазина внутри прокрутки")

	# настройки на экране 1280×720
	var st: Control = SettingsScreen.new()
	holder.add_child(st)
	for i in 3:
		await process_frame
	var spanel := st.find_child("*", true, false)
	var biggest := 0.0
	for c in st.find_children("*", "PanelContainer", true, false):
		biggest = maxf(biggest, (c as Control).size.y)
	_c(biggest <= 720.0, "панель настроек помещается по высоте (%d из 720)" % int(biggest))
	# магазин: карточки товаров — рамки внутри прокрутки
	var cat := {"vip_tiers": [], "items": [
		{"id": "vip_1", "kind": "vip", "name": "VIP I", "desc": "d", "price": 299, "icon": "vip"},
		{"id": "vip_2", "kind": "vip", "name": "VIP II", "desc": "d", "price": 599, "icon": "vip"}]}
	var sh: Control = ShopScreen.new()
	holder.add_child(sh)
	await process_frame
	sh.set_data({"catalog": cat, "state": {"vip_level": 0, "owned": [], "equipped": {}}}, 120)
	for i in 3:
		await process_frame
	var cards: Array = sh._grid.find_children("*", "PanelContainer", true, false)
	var blocking := 0
	for c in cards:
		if (c as Control).mouse_filter == Control.MOUSE_FILTER_STOP:
			blocking += 1
	_c(cards.size() >= 2 and blocking == 0, "карточки товаров магазина пропускают касание (%d карточек, блокируют: %d)" % [cards.size(), blocking])
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
