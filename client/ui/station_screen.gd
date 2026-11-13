extends Control
## Экран станции (Э-2): ремонт, покупка ресурсов, улучшение корабля.
## Все операции подтверждает сервер; причины отказа приходят через command_failed и показываются тостом снаружи.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

signal closed

const RESOURCE_LABELS := {"iron_ore": "Железная руда", "crystals": "Кристаллы", "gas": "Газ", "scrap": "Лом"}
const BUY_AMOUNT := 5.0

var _gateway: UiGateway
var _credits := -1
var _notice: Label
var _repair_btn: ActionButton
var _repair_info: Label
var _upgrade_btn: ActionButton
var _upgrade_info: Label
var _sell_btn: ActionButton
var _rows: VBoxContainer
var _routes: VBoxContainer
var _contracts: VBoxContainer


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 0.96)
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

	var head := HBoxContainer.new()
	col.add_child(head)
	head.add_child(Icons.rect("store", 32, Tokens.ACCENT))
	var title := Label.new()
	title.text = "Станция"
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title)
	var refresh := ActionButton.new()
	refresh.text = "Обновить"
	Icons.apply(refresh, "check", Tokens.ACCENT, 20)
	refresh.pressed.connect(_reload)
	head.add_child(refresh)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x")
	close.custom_minimum_size = Vector2(140, 52)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)

	_notice = Label.new()
	_notice.theme_type_variation = &"MutedLabel"
	col.add_child(_notice)

	_repair_info = Label.new()
	col.add_child(_repair_info)
	_repair_btn = ActionButton.new()
	_repair_btn.text = "Починить"
	Icons.apply(_repair_btn, "shield", Tokens.ACCENT, 22)
	_repair_btn.pressed.connect(func() -> void: if _gateway != null: _gateway.repair())
	col.add_child(_repair_btn)

	_upgrade_info = Label.new()
	col.add_child(_upgrade_info)
	_upgrade_btn = ActionButton.new()
	_upgrade_btn.text = "Улучшить корабль"
	Icons.apply(_upgrade_btn, "rocket", Tokens.RARE, 22)
	_upgrade_btn.pressed.connect(func() -> void: if _gateway != null: _gateway.ship_upgrade())
	col.add_child(_upgrade_btn)

	_sell_btn = ActionButton.new()
	_sell_btn.text = "Продать весь груз"
	Icons.apply(_sell_btn, "coins", Tokens.SUCCESS, 22)
	_sell_btn.pressed.connect(func() -> void: if _gateway != null: _gateway.sell())
	col.add_child(_sell_btn)

	var contract_title := Label.new()
	contract_title.text = "Контракты перевозки"
	contract_title.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	col.add_child(contract_title)
	_contracts = VBoxContainer.new()
	_contracts.add_theme_constant_override("separation", 6)
	col.add_child(_contracts)
	var res_title := Label.new()
	res_title.text = "Склад ресурсов (цена за единицу)"
	res_title.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	col.add_child(res_title)
	var scroll := ScrollContainer.new()
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	col.add_child(scroll)
	_routes = VBoxContainer.new()
	_routes.add_theme_constant_override("separation", 4)
	col.add_child(_routes)
	_rows = VBoxContainer.new()
	_rows.add_theme_constant_override("separation", 6)
	_rows.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	scroll.add_child(_rows)


func bind(gw: UiGateway) -> void:
	_gateway = gw
	if not gw.station_market.is_connected(_on_market):
		gw.station_market.connect(_on_market)
		gw.station_done.connect(_on_done)
		gw.contracts_updated.connect(_on_contracts)
		gw.contract_failed.connect(_on_contract_failed)
	_reload()


func _reload() -> void:
	_notice.text = "Загрузка…"
	if _gateway != null:
		_gateway.request_market()
		_gateway.request_contracts()


func _on_market(data: Dictionary) -> void:
	_notice.text = ""
	_credits = int(_gateway.snapshot().get("credits", -1))
	var ship: Dictionary = _gateway.snapshot().get("ship", {}) if _gateway.snapshot().get("ship") is Dictionary else {}
	var cargo_used := float(ship.get("cargo_used", 0.0))
	_sell_btn.disabled = cargo_used <= 0.0
	_sell_btn.text = "Продать груз (%d ед.)" % int(round(cargo_used)) if cargo_used > 0.0 else "Продать весь груз"
	var rc := int(data.get("repair_cost", -1))
	if rc < 0:
		_repair_info.text = "Ремонт: только у станции"
		_repair_btn.disabled = true
	elif rc == 0:
		_repair_info.text = "Корабль в полном порядке"
		_repair_btn.disabled = true
	else:
		_repair_info.text = "Ремонт: %d кр." % rc
		_repair_btn.disabled = _credits >= 0 and _credits < rc
	var uc := int(data.get("upgrade_cost", -1))
	if uc < 0:
		_upgrade_info.text = "Улучшение: недоступно"
		_upgrade_btn.disabled = true
	else:
		_upgrade_info.text = "Улучшение корабля: %d кр. (следующий уровень)" % uc
		_upgrade_btn.disabled = _credits >= 0 and _credits < uc
	for c in _rows.get_children():
		c.queue_free()
	for c in _routes.get_children():
		c.queue_free()
	var routes: Array = data.get("routes", []) if data.get("routes") is Array else []
	for route in routes:
		if route is Dictionary:
			var hint := Label.new()
			hint.text = "Маршрут: %s → %s · %s · риск: %s" % [str(route.get("resource", "")), str(route.get("to", "")), str(route.get("reason", "")), str(route.get("risk", ""))]
			hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
			hint.add_theme_color_override("font_color", Tokens.RARE)
			_routes.add_child(hint)
	var prices: Dictionary = data.get("prices", {})
	var trends: Dictionary = data.get("trends", {})
	var history: Dictionary = data.get("history", {})
	for res in prices:
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 10)
		_rows.add_child(row)
		var name := Label.new()
		name.text = RESOURCE_LABELS.get(res, str(res))
		name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(name)
		var price := Label.new()
		var trend := str(trends.get(res, "стабильно"))
		var points: Array = history.get(res, []) if history.get(res) is Array else []
		var spark := ""
		if points.size() >= 2:
			var first := float(points[0])
			var last := float(points[points.size() - 1])
			spark = " ↑" if last > first * 1.02 else (" ↓" if last < first * 0.98 else " →")
		price.text = "%d кр. · %s%s" % [int(ceil(float(prices[res]))), trend, spark]
		price.custom_minimum_size.x = 110
		row.add_child(price)
		var b := ActionButton.new()
		b.text = "Купить %d" % int(BUY_AMOUNT)
		b.disabled = _credits >= 0 and _credits < int(ceil(float(prices[res]) * BUY_AMOUNT))
		b.pressed.connect(func() -> void: if _gateway != null: _gateway.buy(str(res), BUY_AMOUNT))
		row.add_child(b)
	if prices.is_empty():
		_notice.text = "У станции пустой склад (нужна игра на сервере 0.3.11+)."



func _on_contract_failed(reason: String) -> void:
	_notice.text = reason


func _on_contracts(data: Dictionary) -> void:
	if _contracts == null:
		return
	for child in _contracts.get_children():
		child.queue_free()
	var active: Dictionary = data.get("active", {}) if data.get("active") is Dictionary else {}
	var has_active := bool(data.get("has_active", false))
	if has_active:
		var line := Label.new()
		line.text = "Активный: %s → %s · %s %.0f ед. · награда %d кр." % [str(active.get("from_system", "")), str(active.get("to_system", "")), RESOURCE_LABELS.get(str(active.get("resource", "")), str(active.get("resource", ""))), float(active.get("amount", 0.0)), int(active.get("reward_credits", 0))]
		line.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		_contracts.add_child(line)
		var actions := HBoxContainer.new()
		var deliver := ActionButton.new()
		deliver.text = "Сдать груз"
		deliver.pressed.connect(func() -> void: if _gateway != null: _gateway.deliver_contract())
		actions.add_child(deliver)
		var cancel := ActionButton.new()
		cancel.text = "Отменить"
		cancel.pressed.connect(func() -> void: if _gateway != null: _gateway.cancel_contract())
		actions.add_child(cancel)
		_contracts.add_child(actions)
	else:
		var available: Array = data.get("available", []) if data.get("available") is Array else []
		for item in available:
			if item is Dictionary:
				var row := HBoxContainer.new()
				var label := Label.new()
				label.text = "%s → %s · %s %.0f · %d кр. · риск %s" % [str(item.get("from_system", "")), str(item.get("to_system", "")), RESOURCE_LABELS.get(str(item.get("resource", "")), str(item.get("resource", ""))), float(item.get("amount", 0.0)), int(item.get("reward_credits", 0)), str(item.get("risk", ""))]
				label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
				label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
				row.add_child(label)
				var id := str(item.get("id", ""))
				var accept := ActionButton.new()
				accept.text = "Принять"
				accept.pressed.connect(func() -> void: if _gateway != null: _gateway.accept_contract(id))
				row.add_child(accept)
				_contracts.add_child(row)
	if _contracts.get_child_count() == 0:
		var empty := Label.new()
		empty.text = "В этой системе пока нет доступных перевозок."
		empty.theme_type_variation = &"MutedLabel"
		_contracts.add_child(empty)


func _on_done(data: Dictionary) -> void:
	_notice.text = "Готово: %s, остаток %d кр." % [str(data.get("action", "")), int(data.get("credits", 0))]
	_reload()
