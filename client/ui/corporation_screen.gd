extends Control
## К-1Б: интерфейс существующего сервиса корпораций. Все изменения подтверждает сервер.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const ConfirmDialog := preload("res://ui/components/confirm_dialog.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

signal closed

var _gateway: UiGateway
var _data: Dictionary = {"known": false}
var _busy := false
var _notice: Label
var _empty: VBoxContainer
var _details: VBoxContainer
var _name_input: LineEdit
var _tag_input: LineEdit
var _join_input: LineEdit
var _create: Button
var _join: Button
var _leave: Button
var _refresh: Button
var _corp_title: Label
var _treasury: Label
var _donate_input: LineEdit
var _donate: Button
var _journal: Label
var _warehouse: Label
var _warehouse_resource: OptionButton
var _warehouse_amount: LineEdit
var _task: Label
var _alliance: Label
var _members: VBoxContainer
var _confirm: Control


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
	for side in ["left", "right"]:
		margin.add_theme_constant_override("margin_" + side, 20)
	for side in ["top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 12)
	add_child(margin)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	margin.add_child(box)
	var head := HBoxContainer.new()
	box.add_child(head)
	var title := _label(head, "Корпорация")
	title.theme_type_variation = &"TitleLabel"
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_refresh = _button(head, "RefreshCorporation", "Обновить", _reload)
	_button(head, "CloseCorporation", "Закрыть", _close_screen)
	_notice = _label(box, "Загрузка…")
	_notice.name = "Notice"
	var scroll := ScrollContainer.new()
	scroll.name = "CorporationScroll"
	scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	box.add_child(scroll)
	var body := VBoxContainer.new()
	body.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body.add_theme_constant_override("separation", 16)
	scroll.add_child(body)

	_empty = VBoxContainer.new()
	_empty.name = "NoCorporation"
	_empty.add_theme_constant_override("separation", 10)
	body.add_child(_empty)
	_label(_empty, "Вы пока не состоите в корпорации")
	_label(_empty, "Вступить в корпорацию — бесплатно, по точному названию или тегу")
	_join_input = _make_input(_empty, "JoinName", "Название или тег", 32)
	_join = _button(_empty, "JoinCorporation", "Вступить", _join_pressed)
	_label(_empty, "Создать корпорацию — 10 000 кредитов")
	_name_input = _make_input(_empty, "CorporationName", "Название: буквы, цифры, пробел, дефис или _", 32)
	_tag_input = _make_input(_empty, "CorporationTag", "Тег: 2–4 латинские буквы или цифры", 4)
	_create = _button(_empty, "CreateCorporation", "Создать за 10 000 кредитов", _create_pressed)

	_details = VBoxContainer.new()
	_details.name = "CorporationDetails"
	_details.add_theme_constant_override("separation", 10)
	body.add_child(_details)
	_corp_title = _label(_details, "")
	_corp_title.name = "CorporationTitle"
	_corp_title.theme_type_variation = &"SectionLabel"
	_treasury = _label(_details, "")
	_treasury.name = "Treasury"
	var donate_row := HBoxContainer.new()
	_details.add_child(donate_row)
	_donate_input = _make_input(donate_row, "DonationAmount", "Сумма пожертвования", 12)
	_donate_input.custom_minimum_size.x = 220
	_donate = _button(donate_row, "DonateTreasury", "Пополнить казну", _donate_pressed)
	_journal = _label(_details, "")
	_journal.name = "TreasuryJournal"
	var warehouse_title := _label(_details, "Общий склад корпорации")
	warehouse_title.theme_type_variation = &"SectionLabel"
	_warehouse = _label(_details, "Склад пуст")
	_warehouse.name = "Warehouse"
	var warehouse_row := VBoxContainer.new()
	_details.add_child(warehouse_row)
	_warehouse_resource = OptionButton.new()
	for resource in ["iron_ore", "crystals", "gas", "scrap"]:
		_warehouse_resource.add_item({"iron_ore":"Железная руда","crystals":"Кристаллы","gas":"Газ","scrap":"Лом"}[resource])
	_warehouse_resource.set_meta("resources", ["iron_ore","crystals","gas","scrap"])
	warehouse_row.add_child(_warehouse_resource)
	_warehouse_amount = _make_input(warehouse_row, "WarehouseAmount", "Количество", 8)
	_warehouse_amount.custom_minimum_size.x = 150
	var deposit := _button(warehouse_row, "WarehouseDeposit", "Положить", _warehouse_deposit)
	var withdraw := _button(warehouse_row, "WarehouseWithdraw", "Забрать", _warehouse_withdraw)
	_task = _label(_details, "")
	_task.name = "CorporationTask"
	_alliance = _label(_details, "")
	_alliance.name = "CorporationAlliance"
	_label(_details, "Состав корпорации")
	_members = VBoxContainer.new()
	_members.name = "Members"
	_details.add_child(_members)
	_label(_details, "В этом разделе доступны просмотр состава, ролей и казны. Вступление открытое.")
	_leave = _button(_details, "LeaveCorporation", "Покинуть корпорацию", _leave_pressed)
	_leave.variant = ActionButton.Variant.DANGER
	_confirm = ConfirmDialog.new()
	_confirm.name = "CorporationConfirm"
	add_child(_confirm)
	_on_data({"known": false})


func _label(parent: Node, text: String) -> Label:
	var label := Label.new()
	label.text = text
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	parent.add_child(label)
	return label


func _make_input(parent: Node, node_name: String, hint: String, limit: int) -> LineEdit:
	var input := LineEdit.new()
	input.name = node_name
	input.placeholder_text = hint
	input.max_length = limit
	input.custom_minimum_size.y = 52
	input.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	parent.add_child(input)
	input.text_changed.connect(func(_text: String) -> void: _update_actions())
	return input


func _button(parent: Node, node_name: String, text: String, action: Callable) -> Button:
	var button := ActionButton.new()
	button.name = node_name
	button.text = text
	button.pressed.connect(action)
	parent.add_child(button)
	return button


func bind(gateway: UiGateway) -> void:
	_gateway = gateway
	gateway.corporation_updated.connect(_on_data)
	gateway.corporation_failed.connect(_on_error)
	gateway.corporation_busy_changed.connect(_on_busy)
	_on_data(gateway.corporation_snapshot())
	_on_busy(gateway.corporation_busy())
	_reload()


func _exit_tree() -> void:
	if is_instance_valid(_gateway):
		_gateway.corporation_updated.disconnect(_on_data)
		_gateway.corporation_failed.disconnect(_on_error)
		_gateway.corporation_busy_changed.disconnect(_on_busy)


func _reload() -> void:
	if _gateway != null and not _busy:
		_notice.text = "Загрузка…"
		_gateway.corporation_info()


func _on_data(data: Dictionary) -> void:
	_data = data
	var known := bool(data.get("known", false))
	var corp: Dictionary = data.get("corp", {})
	_empty.visible = known and corp.is_empty()
	_details.visible = known and not corp.is_empty()
	_notice.text = "" if known else "Сведения о корпорации ещё не получены"
	for child in _members.get_children():
		_members.remove_child(child)
		child.queue_free()
	if not corp.is_empty():
		_corp_title.text = "[%s] %s" % [str(corp.get("tag", "")), str(corp.get("name", ""))]
		_treasury.text = "Казна: %s кредитов" % str(int(corp.get("treasury", 0)))
		var journal: Array = data.get("treasury_journal", []) if data.get("treasury_journal") is Array else []
		if journal.is_empty():
			_journal.text = "История казны: пока нет операций"
		else:
			var rows := PackedStringArray(["История казны:"])
			for entry in journal.slice(0, 5):
				if entry is Dictionary:
					rows.append("• %s +%d кр. · баланс %d" % [str(entry.get("player_id", "участник")), int(entry.get("amount", 0)), int(entry.get("balance", 0))])
			_journal.text = "\n".join(rows)
		var warehouse: Array = data.get("warehouse", []) if data.get("warehouse") is Array else []
		if warehouse.is_empty():
			_warehouse.text = "Склад пуст"
		else:
			var stock := PackedStringArray(["Склад:"])
			for item in warehouse:
				if item is Dictionary:
					stock.append("• %s: %.1f" % [str(item.get("resource", "")), float(item.get("amount", 0.0))])
			_warehouse.text = "\n".join(stock)
		var task: Dictionary = data.get("task", {}) if data.get("task") is Dictionary else {}
		if task.is_empty():
			_task.text = ""
		else:
			_task.text = "Задание клана: %s (%d/%d)" % [str(task.get("title", "")), int(task.get("progress", 0)), int(task.get("goal", 0))]
		var alliance: Dictionary = data.get("alliance", {}) if data.get("alliance") is Dictionary else {}
		if alliance.is_empty():
			_alliance.text = ""
		else:
			var governor := str(alliance.get("governor_name", ""))
			if governor == "":
				governor = str(alliance.get("governor", ""))
			_alliance.text = "Союз: %s · кланов %d/3 · управляющий %s" % [str(alliance.get("name", "")), int(alliance.get("clans", 0)), governor]
		var members: Array = data.get("members", []) if data.get("members") is Array else []
		if not data.has("members"):
			_label(_members, "Состав загружается…")
		for member in members:
			if member is Dictionary:
				var member_name := str(member.get("name", ""))
				if member_name == "":
					member_name = str(member.get("id", "Пилот"))
				var role: String = {"owner": "Основатель", "member": "Участник"}.get(str(member.get("role", "")), "Неизвестная роль")
				_label(_members, "%s · %s" % [member_name, role])
	_update_actions()


func _on_error(text: String) -> void:
	_notice.text = text


func _on_busy(busy: bool) -> void:
	_busy = busy
	if busy:
		_notice.text = "Ожидание ответа сервера…"
	_update_actions()


func _update_actions() -> void:
	if _create == null or _join == null or _leave == null or _donate == null:
		return
	var empty := bool(_data.get("known", false)) and (_data.get("corp", {}) as Dictionary).is_empty()
	_create.disabled = _busy or not empty or _name_input.text.strip_edges().is_empty() or _tag_input.text.strip_edges().length() < 2
	_join.disabled = _busy or not empty or _join_input.text.strip_edges().is_empty()
	_leave.disabled = _busy or not _details.visible
	_donate.disabled = _busy or not _details.visible or not _donate_input.text.is_valid_int() or int(_donate_input.text) <= 0
	_refresh.disabled = _busy
	for input in [_name_input, _tag_input, _join_input, _donate_input]:
		input.editable = not _busy


func _warehouse_resource_id() -> String:
	var resources: Array = _warehouse_resource.get_meta("resources", [])
	return str(resources[_warehouse_resource.selected]) if _warehouse_resource.selected >= 0 and _warehouse_resource.selected < resources.size() else "iron_ore"


func _warehouse_amount_value() -> float:
	return float(_warehouse_amount.text) if _warehouse_amount.text.is_valid_float() else 0.0


func _warehouse_deposit() -> void:
	if _gateway != null and _warehouse_amount_value() > 0:
		_gateway.corporation_deposit(_warehouse_resource_id(), _warehouse_amount_value())
		_warehouse_amount.clear()


func _warehouse_withdraw() -> void:
	if _gateway != null and _warehouse_amount_value() > 0:
		_gateway.corporation_withdraw(_warehouse_resource_id(), _warehouse_amount_value())
		_warehouse_amount.clear()


func _donate_pressed() -> void:
	if _donate.disabled or _gateway == null:
		return
	_notice.text = "Пополнение казны…"
	_gateway.corporation_donate(int(_donate_input.text))
	_donate_input.clear()


func _join_pressed() -> void:
	if _join.disabled or _gateway == null:
		return
	_notice.text = "Вступление…"
	_gateway.corporation_join(_join_input.text)


func _create_pressed() -> void:
	if _create.disabled or _gateway == null:
		return
	var corp_name := _name_input.text.strip_edges()
	var tag := _tag_input.text.strip_edges().to_upper()
	if corp_name.to_utf8_buffer().size() > 32:
		_on_error("Сократите название: сервер не принимает такую длину")
		return
	var approved: bool = await _confirm.ask("Создать корпорацию?", "[%s] %s\nС вашего баланса будет списано 10 000 кредитов." % [tag, corp_name], "Создать")
	if approved and not _create.disabled:
		_notice.text = "Создание…"
		_gateway.corporation_create(corp_name, tag)


func _leave_pressed() -> void:
	if _leave.disabled or _gateway == null:
		return
	var corp: Dictionary = _data.get("corp", {})
	var approved: bool = await _confirm.ask("Покинуть корпорацию?", "Вы выйдете из «%s». Передача прав основателя пока недоступна." % str(corp.get("name", "")), "Покинуть")
	if approved and not _leave.disabled and corp.get("id") == (_data.get("corp", {}) as Dictionary).get("id"):
		_notice.text = "Выход…"
		_gateway.corporation_leave()


func _close_screen() -> void:
	if _confirm.is_open():
		_confirm._close(false)
	else:
		closed.emit()


func _unhandled_key_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		_close_screen()
