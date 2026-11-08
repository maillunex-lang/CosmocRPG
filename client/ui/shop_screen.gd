extends Control
## Магазин и VIP. Показывает каталог и состояние игрока, которые прислал сервер; все покупки и экипировка подтверждаются сервером.
## Пока покупки бесплатны (флаг free в каталоге). Боевых преимуществ магазин не даёт: только внешний вид и VIP-оформление.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")

signal buy_requested(item_id: String)
signal equip_requested(slot: String, item_id: String)
signal closed

const SLOT_OF := {"badge": "badge", "engine": "engine", "namecolor": "name", "avatar": "avatar", "frame": "frame", "title": "title"}
const TABS := [["vip", "VIP", "crown"], ["subscription", "Пропуск", "zap"], ["engine", "Двигатели", "flame"], ["namecolor", "Цвет ника", "palette"], ["badge", "Значки", "star"], ["avatar", "Аватарки", "user"], ["frame", "Рамки", "gem"], ["title", "Титулы", "badge-check"]]
const COLUMNS := 3

var tab := "vip"
var credits := -1   # -1: баланс неизвестен (сервер не прислал)
var catalog: Dictionary = {}
var state: Dictionary = {}

var _notice: Label
var _vip_label: Label
var _credits_label: Label
var _grid: GridContainer
var _tab_buttons: Dictionary = {}
var _cards: Dictionary = {}   # id -> кнопка действия


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 0.97)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var margin := MarginContainer.new()
	margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 20)
	add_child(margin)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 10)
	margin.add_child(col)

	# шапка: название, VIP, баланс, закрыть
	var head := HBoxContainer.new()
	head.add_theme_constant_override("separation", 12)
	col.add_child(head)
	head.add_child(Icons.rect("store", 34, Tokens.ACCENT))
	var title := Label.new()
	title.text = "Магазин"
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title)
	head.add_child(Icons.rect("crown", 24, Tokens.RARE))
	_vip_label = Label.new()
	head.add_child(_vip_label)
	head.add_child(Icons.rect("coins", 24, Tokens.RARE))
	_credits_label = Label.new()
	head.add_child(_credits_label)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x")
	close.custom_minimum_size = Vector2(150, 52)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)

	_notice = Label.new()
	_notice.theme_type_variation = &"MutedLabel"
	_notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	col.add_child(_notice)

	var tabs := HBoxContainer.new()
	tabs.add_theme_constant_override("separation", 8)
	col.add_child(tabs)
	for t in TABS:
		var b := ActionButton.new()
		b.toggle_mode = true
		b.text = t[1]
		Icons.apply(b, t[2], Tokens.ACCENT, 22)
		b.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 4)
		b.custom_minimum_size = Vector2(112, 52)
		b.pressed.connect(_select_tab.bind(str(t[0])))
		tabs.add_child(b)
		_tab_buttons[t[0]] = b

	var scroll := ScrollContainer.new()
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	col.add_child(scroll)
	_grid = GridContainer.new()
	_grid.columns = COLUMNS
	_grid.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_grid.add_theme_constant_override("h_separation", 10)
	_grid.add_theme_constant_override("v_separation", 10)
	scroll.add_child(_grid)
	_refresh()


## Каталог и состояние от сервера (сообщение shop) и баланс кредитов (из состояния игры).
func set_data(data: Dictionary, balance: int = -1) -> void:
	catalog = data.get("catalog", {}) if data.get("catalog") is Dictionary else {}
	state = data.get("state", {}) if data.get("state") is Dictionary else {}
	if balance >= 0:
		credits = balance
	if is_inside_tree():
		_refresh()


func set_credits(balance: int) -> void:
	credits = balance
	if _credits_label != null:
		_credits_label.text = str(credits) if credits >= 0 else "—"


## Ошибка операции от сервера: кнопки снова доступны, причина показана в строке над вкладками.
func show_error(text: String) -> void:
	if _notice != null:
		_notice.text = text
		_notice.theme_type_variation = &"DangerLabel"
	_refresh(false)


func _select_tab(t: String) -> void:
	tab = t
	_refresh()


func vip_level() -> int:
	return int(state.get("vip_level", 0))


func _owned(id: String) -> bool:
	return (state.get("owned", []) as Array).has(id)


func _equipped(item: Dictionary) -> bool:
	var slot: String = SLOT_OF.get(str(item.get("kind", "")), "")
	return slot != "" and str((state.get("equipped", {}) as Dictionary).get(slot, "")) == str(item.get("id", ""))


## Описание действия карточки: {text, enabled, kind} (kind: buy | equip | unequip | locked). Вынесено отдельно для тестов.
func action_for(item: Dictionary) -> Dictionary:
	var kind := str(item.get("kind", ""))
	var need := int(item.get("vip_min", 0))
	if need > vip_level():
		return {"text": "Нужен VIP %d" % need, "enabled": false, "kind": "locked"}
	if kind == "vip":
		var active := vip_level() > 0
		return {"text": "Продлить" if active else "Получить", "enabled": true, "kind": "buy"}
	if kind == "subscription":
		# подписка покупается повторно для продления; слота экипировки не имеет
		var sub_active := str(state.get("subscription_until", "")) != ""
		return {"text": "Продлить" if sub_active else "Получить", "enabled": true, "kind": "buy"}
	if not _owned(str(item.get("id", ""))):
		return {"text": "Получить", "enabled": true, "kind": "buy"}
	if _equipped(item):
		return {"text": "Снять", "enabled": true, "kind": "unequip"}
	return {"text": "Надеть", "enabled": true, "kind": "equip"}


func _price_text(item: Dictionary) -> String:
	var price := int(item.get("price", 0))
	if bool(catalog.get("free", false)):
		return "Бесплатно (обычная цена %d)" % price if price > 0 else "Бесплатно"
	return "%d %s" % [price, str(catalog.get("currency", ""))]


func _refresh(reset_notice: bool = true) -> void:
	if _grid == null:
		return
	for t in _tab_buttons:
		(_tab_buttons[t] as Button).set_pressed_no_signal(t == tab)
	var lvl := vip_level()
	_vip_label.text = ("VIP %d" % lvl) if lvl > 0 else "Без VIP"
	set_credits(credits)
	if reset_notice:
		_notice.theme_type_variation = &"MutedLabel"
		_notice.text = ("Покупки пока бесплатны. Магазин не даёт боевых преимуществ: только внешний вид и оформление." if bool(catalog.get("free", false))
				else "Магазин не даёт боевых преимуществ: только внешний вид и оформление.") if not catalog.is_empty() else "Загрузка каталога…"
	for c in _grid.get_children():
		c.queue_free()
	_cards.clear()
	for item in catalog.get("items", []):
		if item is Dictionary and str(item.get("kind", "")) == tab:
			_grid.add_child(_card(item))


func _card(item: Dictionary) -> Control:
	var panel := PanelContainer.new()
	panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 6)
	panel.add_child(box)
	var top := HBoxContainer.new()
	top.add_theme_constant_override("separation", 10)
	box.add_child(top)
	var tint := Tokens.ACCENT
	if str(item.get("color", "")) != "":
		tint = Color(str(item["color"]))
	elif str(item.get("kind", "")) == "vip":
		tint = Tokens.RARE
	top.add_child(Icons.rect(str(item.get("icon", "sparkles")), 44, tint))
	var name_l := Label.new()
	name_l.text = str(item.get("name", ""))
	name_l.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	name_l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	name_l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	top.add_child(name_l)
	var desc := Label.new()
	desc.text = str(item.get("desc", ""))
	desc.theme_type_variation = &"MutedLabel"
	desc.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	desc.custom_minimum_size = Vector2(0, 44)
	box.add_child(desc)
	var price := Label.new()
	price.text = _price_text(item)
	price.add_theme_color_override("font_color", Tokens.RARE)
	box.add_child(price)
	var act := action_for(item)
	var b := ActionButton.new()
	b.text = str(act["text"])
	b.disabled = not bool(act["enabled"])
	match str(act["kind"]):
		"locked":
			Icons.apply(b, "lock", Tokens.TEXT_MUTED, 20)
		"unequip":
			b.variant = ActionButton.Variant.PRIMARY
			Icons.apply(b, "check", Tokens.BG, 20)
		"buy":
			Icons.apply(b, "shopping-cart", Color.WHITE, 20)
		_:
			pass
	b.pressed.connect(_on_action.bind(item, str(act["kind"]), b))
	box.add_child(b)
	_cards[str(item.get("id", ""))] = b
	return panel


func _on_action(item: Dictionary, kind: String, b: ActionButton) -> void:
	if b.is_busy():
		return
	b.begin_busy("…")
	match kind:
		"buy":
			buy_requested.emit(str(item["id"]))
		"equip":
			equip_requested.emit(SLOT_OF.get(str(item["kind"]), ""), str(item["id"]))
		"unequip":
			equip_requested.emit(SLOT_OF.get(str(item["kind"]), ""), "")
