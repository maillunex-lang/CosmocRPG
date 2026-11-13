extends Control
## Профиль пилота (R-34): аватарка в рамке, титул, направление, корабль, VIP, подписка, кредиты.
## Все данные — из снимка состояния игры (сервер); экран только показывает.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")

signal shop_requested
signal closed
## Игрок нажал «+» у характеристики пилота (сервер сам проверит очки).
signal attribute_requested(attr: String)

const ROMAN := ["", "I", "II", "III", "IV", "V", "VI", "VII"]
const ATTRIBUTES := [
	["piloting", "Пилотирование"], ["tactics", "Тактика"], ["engineering", "Инженерия"],
	["energetics", "Энергетика"], ["analytics", "Аналитика"], ["logistics", "Логистика"],
]

var _nick: Label
var _title: Label
var _spec: Label
var _corporation: Label
var _ship: Label
var _ship_progress: Label
var _vip: Label
var _sub: Label
var _credits: Label
var _avatar: TextureRect
var _frame: PanelContainer
var _shop_button: ActionButton
var _dev: VBoxContainer
var _scroll: ScrollContainer
var _box: VBoxContainer
var _dev_level: Label
var _dev_free: Label
var _dev_bar: ProgressBar
var _attr_values: Dictionary = {}
var _attr_buttons: Dictionary = {}


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var bg := ColorRect.new()
	bg.color = Color(Tokens.BG, 0.97)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(bg)
	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(center)
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(460, 0)
	center.add_child(panel)
	# содержимое прокручивается: на низком экране (альбомная ориентация) оно не помещается целиком
	_scroll = ScrollContainer.new()
	_scroll.name = "ProfileScroll"
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	panel.add_child(_scroll)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_scroll.add_child(box)
	_box = box
	resized.connect(_fit_scroll)

	var head := HBoxContainer.new()
	box.add_child(head)
	head.add_child(Icons.rect("user", 30, Tokens.ACCENT))
	var title_l := Label.new()
	title_l.text = "Профиль пилота"
	title_l.theme_type_variation = &"TitleLabel"
	title_l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(title_l)
	var close := ActionButton.new()
	close.text = "Закрыть"
	Icons.apply(close, "x")
	close.custom_minimum_size = Vector2(140, 52)
	close.pressed.connect(func() -> void: closed.emit())
	head.add_child(close)

	# аватарка в рамке + ник + титул
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 14)
	box.add_child(row)
	_frame = PanelContainer.new()
	row.add_child(_frame)
	var av_margin := MarginContainer.new()
	for side in ["left", "right", "top", "bottom"]:
		av_margin.add_theme_constant_override("margin_" + side, 5)
	_frame.add_child(av_margin)
	_avatar = TextureRect.new()
	_avatar.custom_minimum_size = Vector2(72, 72)
	_avatar.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	_avatar.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	av_margin.add_child(_avatar)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 4)
	col.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(col)
	_nick = Label.new()
	_nick.name = "Nick"
	_nick.add_theme_font_size_override("font_size", Tokens.FONT_TITLE)
	col.add_child(_nick)
	_title = Label.new()
	_title.name = "TitleLabel"
	_title.add_theme_color_override("font_color", Tokens.RARE)
	col.add_child(_title)
	_spec = Label.new()
	_spec.theme_type_variation = &"MutedLabel"
	col.add_child(_spec)

	_ship = _row(box, "rocket", "Корабль")
	_ship.name = "ShipValue"
	_ship_progress = _row(box, "gauge", "Развитие")
	_ship_progress.name = "ShipProgress"
	_vip = _row(box, "crown", "VIP")
	_vip.name = "VipValue"
	_sub = _row(box, "zap", "Пропуск")
	_sub.name = "SubValue"
	_credits = _row(box, "coins", "Кредиты")
	_credits.name = "CreditsValue"

	_corporation = _row(box, "user", "Корпорация")
	_corporation.name = "CorporationValue"
	set_corporation({"known": false})

	_build_development(box)
	_fit_scroll.call_deferred()

	_shop_button = ActionButton.new()
	_shop_button.name = "ShopButton"
	_shop_button.text = "Открыть магазин"
	Icons.apply(_shop_button, "store", Tokens.ACCENT, 22)
	_shop_button.pressed.connect(func() -> void: shop_requested.emit())
	box.add_child(_shop_button)


## Высота области прокрутки: по содержимому, но не выше экрана (минус поля панели).
func _fit_scroll() -> void:
	if _scroll == null or _box == null:
		return
	var avail := maxf(size.y - 56.0, 200.0)
	_scroll.custom_minimum_size.y = minf(_box.get_combined_minimum_size().y, avail)


## Развитие пилота: уровень, опыт, свободные очки и шесть характеристик. Показывается, только если сервер прислал данные.
func _build_development(box: Control) -> void:
	_dev = VBoxContainer.new()
	_dev.name = "Development"
	_dev.add_theme_constant_override("separation", 6)
	_dev.visible = false
	box.add_child(_dev)
	var head := HBoxContainer.new()
	_dev.add_child(head)
	_dev_level = Label.new()
	_dev_level.name = "DevLevel"
	_dev_level.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_dev_level.add_theme_font_size_override("font_size", Tokens.FONT_BODY + 2)
	head.add_child(_dev_level)
	_dev_free = Label.new()
	_dev_free.name = "DevFree"
	_dev_free.add_theme_color_override("font_color", Tokens.ACCENT)
	head.add_child(_dev_free)
	_dev_bar = ProgressBar.new()
	_dev_bar.show_percentage = false
	_dev_bar.custom_minimum_size = Vector2(0, 8)
	_dev_bar.max_value = 1.0
	_dev_bar.step = 0.001
	_dev.add_child(_dev_bar)
	for a in ATTRIBUTES:
		var row := HBoxContainer.new()
		row.add_theme_constant_override("separation", 10)
		_dev.add_child(row)
		var name_l := Label.new()
		name_l.text = a[1]
		name_l.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(name_l)
		var val := Label.new()
		val.custom_minimum_size.x = 44
		val.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		row.add_child(val)
		_attr_values[a[0]] = val
		var b := ActionButton.new()
		b.name = "Plus_" + a[0]
		b.text = "+"
		b.custom_minimum_size = Vector2(56, 44)
		var attr_id: String = a[0]
		b.pressed.connect(func() -> void: attribute_requested.emit(attr_id))
		row.add_child(b)
		_attr_buttons[a[0]] = b
	var note := Label.new()
	note.text = "Очки сохраняются. Влияние характеристик на корабль подключается на следующих этапах развития."
	note.theme_type_variation = &"MutedLabel"
	note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_dev.add_child(note)


## Состояние пилота от сервера (опыт, уровень, очки).
func set_pilot(p: Dictionary) -> void:
	_dev.visible = not p.is_empty()
	_fit_scroll.call_deferred()
	if p.is_empty():
		return
	var need := int(p.get("level_xp", 0))
	_dev_level.text = "Уровень %d из %d" % [int(p.get("level", 1)), int(p.get("max_level", 60))]
	_dev_bar.value = 1.0 if need <= 0 else clampf(float(p.get("into_level", 0)) / float(need), 0.0, 1.0)
	_dev_bar.tooltip_text = "Максимальный уровень" if need <= 0 else "Опыт: %d / %d" % [int(p.get("into_level", 0)), need]
	var free := int(p.get("attr_free", 0))
	_dev_free.text = ("Свободно очков: %d" % free) if free > 0 else ""
	var attrs: Dictionary = p.get("attributes", {})
	for a in ATTRIBUTES:
		(_attr_values[a[0]] as Label).text = str(int(attrs.get(a[0], 0)))
		(_attr_buttons[a[0]] as Button).disabled = free <= 0


func _row(parent: Control, icon: String, caption: String) -> Label:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 10)
	parent.add_child(row)
	row.add_child(Icons.rect(icon, 22, Tokens.ACCENT))
	var cap := Label.new()
	cap.text = caption
	cap.custom_minimum_size.x = 110
	cap.theme_type_variation = &"MutedLabel"
	row.add_child(cap)
	var value := Label.new()
	value.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(value)
	return value


## Данные из снимка состояния игры: player/ship/credits. Срок подписки — из состояния магазина (если загружено).
func set_data(state: Dictionary, shop_state: Dictionary = {}) -> void:
	_fit_scroll.call_deferred()
	var player: Dictionary = state.get("player", {})
	var ship: Dictionary = state.get("ship", {})
	var cos: Dictionary = ship.get("cosmetics", {})
	_nick.text = str(player.get("name", "Пилот"))
	var title := str(cos.get("title", ""))
	_title.visible = title != ""
	_title.text = ("«%s»" % title) if title != "" else ""
	_spec.text = "Направление: %s" % str(ship.get("spec", player.get("pilot_spec", "—"))).capitalize()
	_ship.text = str(ship.get("name", "—"))
	var ship_level := int(ship.get("level", 0))
	var modules: Array = ship.get("modules", []) if ship.get("modules") is Array else []
	var equipped := 0
	for module in modules:
		if module is Dictionary and bool(module.get("equipped", false)):
			equipped += 1
	var spec_name := str(ship.get("spec", ""))
	_ship_progress.text = ("Ур. %d · %s · модулей %d" % [ship_level, spec_name.capitalize() if spec_name != "" else "без специализации", equipped]) if ship_level > 0 else "Сервер ещё не прислал прогресс"
	_ship_progress.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	var vip := int(cos.get("vip", 0))
	var roman: String = ROMAN[clampi(vip, 0, 7)]
	_vip.text = ("VIP %s" % roman) if vip > 0 else "нет"
	_vip.add_theme_color_override("font_color", Tokens.RARE if vip > 0 else Tokens.TEXT_MUTED)
	_sub.get_parent().visible = not shop_state.is_empty()
	if not shop_state.is_empty():
		var sub_until := str(shop_state.get("subscription_until", ""))
		_sub.text = ("активен до %s" % sub_until.substr(0, 10)) if sub_until != "" else "нет"
	var frame_color := str(cos.get("frame", ""))
	_frame.modulate = Color(frame_color) if frame_color != "" else Color(1, 1, 1, 0.0)
	var avatar := str(cos.get("avatar", ""))
	_avatar.texture = Icons.tex(avatar if avatar != "" else "user")
	_avatar.modulate = Tokens.ACCENT if avatar != "" else Tokens.TEXT_MUTED
	_credits.text = "%s" % _fmt(int(state.get("credits", 0)))
	if state.get("pilot") is Dictionary:
		set_pilot(state["pilot"])


static func _fmt(n: int) -> String:
	var s := str(abs(n))
	var out := ""
	while s.length() > 3:
		out = " " + s.substr(s.length() - 3, 3) + out
		s = s.substr(0, s.length() - 3)
	return s + out


## Магазин доступен (серверный режим): скрывает кнопку перехода в магазин, если его нет.
func set_shop_available(ok: bool) -> void:
	if _shop_button != null:
		_shop_button.visible = ok


func set_corporation(data: Dictionary) -> void:
	var corp: Dictionary = data.get("corp", {})
	_corporation.text = "[%s] %s" % [str(corp.get("tag", "")), str(corp.get("name", ""))] if not corp.is_empty() else ("нет" if bool(data.get("known", false)) else "—")
	_corporation.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_fit_scroll.call_deferred()
