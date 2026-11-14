extends PanelContainer
## Компактная карточка ежедневных задач. Клиент показывает только серверные данные.
signal closed
var gateway: Node
var _body: VBoxContainer
var _status: Label

func bind(source: Node) -> void:
	gateway = source
	if gateway.has_signal("daily_updated"):
		gateway.daily_updated.connect(_on_daily)
	if gateway.has_signal("daily_rewarded"):
		gateway.daily_rewarded.connect(_on_reward)
	gateway.request_daily()

func _ready() -> void:
	custom_minimum_size = Vector2(300, 0)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var margin := MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 16)
	margin.add_theme_constant_override("margin_right", 16)
	margin.add_theme_constant_override("margin_top", 12)
	margin.add_theme_constant_override("margin_bottom", 12)
	add_child(margin)
	_body = VBoxContainer.new()
	_body.add_theme_constant_override("separation", 8)
	margin.add_child(_body)
	var title := Label.new()
	title.text = "Сегодня"
	title.add_theme_font_size_override("font_size", 20)
	_body.add_child(title)
	_status = Label.new()
	_status.text = "Загрузка задач…"
	_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_body.add_child(_status)

func _on_daily(data: Dictionary) -> void:
	for child in _body.get_children():
		if child != _body.get_child(0) and child != _body.get_child(1):
			child.queue_free()
	_status.text = "Серия возвращений: %d дней\nНаграда за вход: %d кредитов" % [int(data.get("streak", 1)), int(data.get("return_reward", 0))]
	if bool(data.get("can_claim_return", false)):
		_add_action("Забрать награду за вход", "return")
	var claimed: Dictionary = data.get("claimed", {})
	for activity in data.get("activities", []):
		if not activity is Dictionary:
			continue
		var id := str(activity.get("id", ""))
		var done := bool(claimed.get(id, false))
		_add_action("%s  ·  +%d" % [str(activity.get("title", "Активность")), int(activity.get("reward", 0))], id, done)

func _add_action(text: String, id: String, disabled := false) -> void:
	var b := Button.new()
	b.text = ("✓ " if disabled else "") + text
	b.disabled = disabled
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.pressed.connect(func() -> void:
		if gateway != null:
			gateway.claim_daily(id)
	)
	_body.add_child(b)

func _on_reward(_data: Dictionary) -> void:
	gateway.request_daily()
