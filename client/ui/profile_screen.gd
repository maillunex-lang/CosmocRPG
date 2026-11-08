extends Control
## Профиль пилота (R-34): аватарка в рамке, титул, направление, корабль, VIP, подписка, кредиты.
## Все данные — из снимка состояния игры (сервер); экран только показывает.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")

signal shop_requested
signal closed

const ROMAN := ["", "I", "II", "III", "IV", "V", "VI", "VII"]

var _nick: Label
var _title: Label
var _spec: Label
var _ship: Label
var _vip: Label
var _sub: Label
var _credits: Label
var _avatar: TextureRect
var _frame: PanelContainer
var _shop_button: ActionButton


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
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	panel.add_child(box)

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
	_vip = _row(box, "crown", "VIP")
	_vip.name = "VipValue"
	_sub = _row(box, "zap", "Пропуск")
	_sub.name = "SubValue"
	_credits = _row(box, "coins", "Кредиты")
	_credits.name = "CreditsValue"

	_shop_button = ActionButton.new()
	_shop_button.name = "ShopButton"
	_shop_button.text = "Открыть магазин"
	Icons.apply(_shop_button, "store", Tokens.ACCENT, 22)
	_shop_button.pressed.connect(func() -> void: shop_requested.emit())
	box.add_child(_shop_button)


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
	var player: Dictionary = state.get("player", {})
	var ship: Dictionary = state.get("ship", {})
	var cos: Dictionary = ship.get("cosmetics", {})
	_nick.text = str(player.get("name", "Пилот"))
	var title := str(cos.get("title", ""))
	_title.visible = title != ""
	_title.text = ("«%s»" % title) if title != "" else ""
	_spec.text = "Направление: %s" % str(ship.get("spec", player.get("pilot_spec", "—"))).capitalize()
	_ship.text = str(ship.get("name", "—"))
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
