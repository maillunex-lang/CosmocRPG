extends Control
## HUD космоса (решение UI-02). Только отображение и ввод: все числа приходят из шлюза данных,
## команды уходят в шлюз. Никаких расчётов боя, энергии и наград на клиенте.
##
## Схема (Landscape, R-33): сверху слева карточка игрока (ник, корабль, «Выйти») и тонкий трекер
## заданий; сверху по центру только тревоги и плашка «Демо» (в игре с сервером пусто);
## сверху справа валюты; справа внизу кластер «большого пальца»: цель, атака, добыча, автопилот;
## снизу по центру слоты способностей с полосами состояния и тоггл «Авто»; снизу слева чат
## над зоной джойстика и кнопка вида камеры. В бою задания и чат сворачиваются, тревоги остаются.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")
const ChatPanel := preload("res://ui/chat_panel.gd")
const Icons := preload("res://ui/icons.gd")
const QuestHints := preload("res://ui/quest_hints.gd")
const LayoutStore := preload("res://ui/hud/layout_store.gd")
const HudEditor := preload("res://ui/hud/hud_editor.gd")

signal exit_requested
signal profile_requested   # тап по карточке игрока: экран профиля
signal camera_mode_requested   # кнопка «Вид»: следующий режим камеры
signal world_tapped(pos: Vector2)   # короткое касание в зоне джойстика: выбор объекта под пальцем
signal hangar_requested   # кнопка «Ангар»
signal shop_requested   # кнопка «Магазин»
signal map_requested
signal star_map_requested   # Э-1: звёздная карта
signal station_requested    # Э-2: экран станции (ремонт, покупка, улучшение)   # кнопка «Карта»
signal corporation_requested
signal settings_requested   # пункт меню «Настройки»
signal chat_requested   # пункт меню «Чат» или кнопка «Окно» в карточке: окно чата поверх игры
signal hold_requested
signal friends_requested
signal raids_requested
signal quest_log_requested   # карточка «Задания»: описание и маршрут
signal guide_autopilot_requested   # автопилот без выбранной цели: вести к заданию
signal modal_opened   # открыто модальное окно (жесты камеры нужно завершить)

const BUSY_TIMEOUT := 2.0
const METERS_PER_UNIT := 100.0   # 1 ед. мира = 100 м (как расстояние в км в карточке цели)
const SKILL_MIN_SIZE := Vector2(88, 56)

var gateway: UiGateway
var _last_state: Dictionary = {}
var _skill_buttons: Array = []
var _pref_collapsed := {"quests": false, "chat": false}  # выбор игрока (вне боя)
var _auto_collapsing := false
var _was_combat := false
## Преобразование вектора управления из экранных осей в мировые (камера повёрнута: «вверх» на экране — вперёд по камере).
var move_transform: Callable = Callable()
var _joy := Vector2.ZERO
var _sent_move := Vector2.ZERO
var _speed_mps := 0.0
var _speed_prev_pos := Vector2.INF
var _speed_prev_time := 0.0
var _quests_sig := ""
var _chat_sig := ""
var chat_panel: VBoxContainer
var _mine_button: ActionButton
var _sell_button: ActionButton
var _cargo_row: HBoxContainer
var _cargo_label: Label
var _flight_status: Label
var _clear_target: Button
var _attack_expect: Variant = null  # ожидаемое значение attacking после нажатия (null: не ждём)
var _peek_a: Label
var _peek_b: Label
var _chat_open: Button
var _party_box: PanelContainer
var _party_label: Label
var _party_invite: Button
var _party_join: Button
var _party_start: Button
var _party_leave: Button
var _party_follow: Button
var _layout_doc: Dictionary = {}
var _layout_homes: Dictionary = {}
var _layout_lifted: Dictionary = {}
var _layout_nodes: Dictionary = {}
var _layout_free: Control
var _hud_editor: Control
var _editing := false
var _layout_force := false
var _cycle_target: Button

@onready var _joystick: Control = %Joystick
@onready var _reset_button: ActionButton = %ResetButton
@onready var _map_hud_button: ActionButton = %MapHudButton
@onready var _menu_button: ActionButton = %MenuButton
@onready var _menu_popup: Control = %MenuPopup

@onready var _player_name: Label = %PlayerName
@onready var _player_level: Label = %PlayerLevel
@onready var _ship_name: Label = %ShipName
@onready var _title_label: Label = %TitleLabel
@onready var _avatar_rect: TextureRect = %AvatarRect
@onready var _profile_card: PanelContainer = %ProfileCard
@onready var _hull: ProgressBar = %HullBar
@onready var _shield: ProgressBar = %ShieldBar
@onready var _energy: ProgressBar = %EnergyBar
@onready var _quests_card: PanelContainer = %QuestsCard
@onready var _chat_card: PanelContainer = %ChatCard
@onready var _quests_list: VBoxContainer = %QuestsList
@onready var _chat_lines: VBoxContainer = %ChatLines
@onready var _demo_bar: Control = %DemoBar
@onready var _exit_button: ActionButton = %ExitButton
@onready var _system_button: ActionButton = %SystemButton
@onready var _alerts: VBoxContainer = %AlertBar
@onready var _credits: HBoxContainer = %Credits
var _xp_bar: ProgressBar
@onready var _target_name: Label = %TargetName
@onready var _target_info: Label = %TargetInfo
@onready var _target_hull: ProgressBar = %TargetHull
@onready var _attack: ActionButton = %AttackButton
@onready var _skills_grid: Container = %SkillsGrid
@onready var _speed_label: Label = %SpeedLabel
@onready var _autopilot: ActionButton = %AutopilotButton
@onready var _auto_skills: ActionButton = %AutoSkillsButton
@onready var _confirm: Control = %Confirm


## Верхний статус-чип: режим управления, скорость и груз в прозрачной рамке (вместо строки внутри нижней панели).
## Тонкая полоса опыта под именем пилота (есть только если сервер прислал опыт).
func _build_xp_bar() -> void:
	var box: Control = _player_name.get_parent().get_parent()
	_xp_bar = ProgressBar.new()
	_xp_bar.name = "XpBar"
	_xp_bar.show_percentage = false
	_xp_bar.custom_minimum_size = Vector2(0, 6)
	_xp_bar.min_value = 0.0
	_xp_bar.max_value = 1.0
	_xp_bar.step = 0.001
	_xp_bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_xp_bar.visible = false
	var bg := StyleBoxFlat.new()
	bg.bg_color = Color(Tokens.TEXT, 0.12)
	bg.set_corner_radius_all(3)
	var fill := StyleBoxFlat.new()
	fill.bg_color = Tokens.ACCENT
	fill.set_corner_radius_all(3)
	_xp_bar.add_theme_stylebox_override("background", bg)
	_xp_bar.add_theme_stylebox_override("fill", fill)
	box.add_child(_xp_bar)
	box.move_child(_xp_bar, 1)


func _apply_xp(player: Dictionary) -> void:
	if _xp_bar == null:
		return
	_xp_bar.visible = player.has("xp_level")
	if not _xp_bar.visible:
		return
	var need := int(player.get("xp_level", 0))
	_xp_bar.value = 1.0 if need <= 0 else clampf(float(player.get("xp_into", 0)) / float(need), 0.0, 1.0)
	_xp_bar.tooltip_text = "Максимальный уровень" if need <= 0 else "Опыт: %d / %d" % [int(player.get("xp_into", 0)), need]


func _build_status_chip() -> void:
	var center: Control = _speed_label.get_parent().get_parent().get_parent()   # CenterStack
	var chip := PanelContainer.new()
	chip.name = "StatusChip"
	chip.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	chip.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var row := HBoxContainer.new()
	row.name = "ChipRow"
	row.add_theme_constant_override("separation", 14)
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	chip.add_child(row)
	center.add_child(chip)
	center.move_child(chip, 0)
	_speed_label.reparent(row, false)
	_speed_label.owner = self   # сохраняет поиск по уникальному имени %SpeedLabel
	_speed_label.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	_speed_label.add_theme_color_override("font_color", Color(Tokens.TEXT, 0.9))
	_airy(chip, 0.34, false)


## Прозрачная рамка карточки: меньше сплошных плашек, больше «воздуха» (как в современных MMO).
func _airy(card: Control, alpha: float, border := true) -> void:
	# стиль собирается из токенов, а не берётся из темы: тема назначается экрану позже, чем срабатывает _ready карточки
	var sb := StyleBoxFlat.new()
	sb.bg_color = Color(Tokens.SURFACE, alpha)
	sb.set_corner_radius_all(Tokens.RADIUS)
	sb.set_content_margin_all(Tokens.SPACE_MD)
	if border:
		sb.border_color = Color(Tokens.ACCENT, Tokens.BORDER_ALPHA)
		sb.set_border_width_all(Tokens.BORDER_WIDTH)
	sb.anti_aliasing = true
	card.add_theme_stylebox_override("panel", sb)


func _ready() -> void:
	_build_status_chip()
	_build_xp_bar()
	for c in [%ProfileCard, %QuestsCard, %ChatCard, %TargetCard, %CreditsCard]:
		_airy(c, 0.55)
	_airy(%CombatBlock, 0.28, false)
	for bar in [%ShieldBar, %HullBar, %EnergyBar]:
		bar.custom_minimum_size.y = 18.0
	_skills_grid.add_theme_constant_override("separation", 10)
	_build_chat_strip()
	_build_party_box()
	_target_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_target_info.max_lines_visible = 2
	_target_info.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_autopilot.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_autopilot.custom_minimum_size = Vector2(168, Tokens.TOUCH_MIN)

	_hull.fill_color = Tokens.SUCCESS
	_hull.warn_below = 0.25   # критический корпус краснеет
	_shield.fill_color = Tokens.ACCENT
	_energy.fill_color = Tokens.ACCENT_2
	_target_hull.fill_color = Tokens.DANGER

	# тоггл «Авто» стоит в ряду слотов: компактная подпись
	_auto_skills.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 4)
	_auto_skills.tooltip_text = "Автобой: сам выбирает ближайшую цель (вид — в настройках) и атакует"

	for b in [_attack, _autopilot, _auto_skills]:
		b.disabled = true
	_attack.variant = ActionButton.Variant.DANGER
	_build_skills()
	_dock_skills()

	# иконки боевого кластера и верхнего правого угла: интерфейс легче читается
	Icons.apply(_attack, "zap", Color.WHITE, 28)
	Icons.apply(_autopilot, "rocket", Tokens.ACCENT, 22)
	Icons.apply(_exit_button, "x", Tokens.TEXT, 18)
	_exit_button.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 4)
	Icons.apply(_map_hud_button, "globe", Tokens.ACCENT, 22)
	Icons.apply(_menu_button, "menu", Tokens.ACCENT, 22)
	# полупрозрачные, чтобы не спорить с космосом; при наведении/нажатии читаются полностью
	for b in [_map_hud_button, _menu_button]:
		b.modulate = Color(1, 1, 1, 0.85)
		b.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 4)
	_avatar_rect.custom_minimum_size = Vector2(26, 26)
	_avatar_rect.texture = Icons.tex("user")
	_avatar_rect.modulate = Tokens.TEXT_MUTED

	_joystick.visible = UiSettings.joystick_enabled()
	_joystick.vector_changed.connect(func(v: Vector2) -> void: _joy = v)

	_reset_button.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 2)
	_reset_button.pressed.connect(func() -> void: camera_mode_requested.emit())
	_joystick.tapped.connect(func(pos: Vector2) -> void: world_tapped.emit(pos))
	# карточка игрока открывает профиль; кнопки внутри карточки работают как раньше
	_profile_card.gui_input.connect(func(ev: InputEvent) -> void:
		if ev is InputEventMouseButton and ev.pressed and ev.button_index == MOUSE_BUTTON_LEFT:
			profile_requested.emit())
	_map_hud_button.pressed.connect(func() -> void: map_requested.emit())
	_menu_button.pressed.connect(_toggle_menu)
	(%MenuPopup/Backdrop as Control).gui_input.connect(func(ev: InputEvent) -> void:
		if ev is InputEventMouseButton and ev.pressed:
			_set_menu_open(false))
	%CorporationMenuButton.pressed.connect(func() -> void: _set_menu_open(false); corporation_requested.emit())
	Icons.apply(%CorporationMenuButton, "user", Tokens.ACCENT, 22)
	%ProfileMenuButton.pressed.connect(func() -> void: _set_menu_open(false); profile_requested.emit())
	%StarMapMenuButton.pressed.connect(func() -> void: _set_menu_open(false); star_map_requested.emit())
	Icons.apply(%StarMapMenuButton, "globe", Tokens.ACCENT, 22)
	%StationMenuButton.pressed.connect(func() -> void: _set_menu_open(false); station_requested.emit())
	Icons.apply(%StationMenuButton, "store", Tokens.ACCENT, 22)
	%ShopMenuButton.pressed.connect(func() -> void: _set_menu_open(false); shop_requested.emit())
	%HangarMenuButton.pressed.connect(func() -> void: _set_menu_open(false); hangar_requested.emit())
	%ChatMenuButton.pressed.connect(func() -> void: _set_menu_open(false); chat_requested.emit())
	Icons.apply(%ChatMenuButton, "mail", Tokens.ACCENT, 22)
	Icons.apply(%ProfileMenuButton, "user", Tokens.ACCENT, 22)
	Icons.apply(%ShopMenuButton, "store", Tokens.ACCENT, 22)
	Icons.apply(%HangarMenuButton, "rocket", Tokens.ACCENT, 22)
	Icons.apply(%StorageMenuButton, "package", Tokens.ACCENT, 22)
	Icons.apply(%MailMenuButton, "mail", Tokens.ACCENT, 22)
	%StorageMenuButton.visible = true
	%StorageMenuButton.disabled = false
	%StorageMenuButton.text = "Трюм"
	%StorageMenuButton.pressed.connect(func() -> void: _set_menu_open(false); hold_requested.emit())
	%MailMenuButton.visible = false
	%StationMenuButton.visible = false
	%HangarMenuButton.visible = false
	_order_menu()
	%SettingsMenuButton.pressed.connect(func() -> void: _set_menu_open(false); settings_requested.emit())
	Icons.apply(%SettingsMenuButton, "settings", Tokens.ACCENT, 22)
	_build_cargo_ui()
	_dock_target()
	_confirm.opened.connect(func() -> void: modal_opened.emit())
	_exit_button.pressed.connect(func() -> void: exit_requested.emit())
	_system_button.pressed.connect(func() -> void: if gateway != null and gateway.has_method("demo_next_system"): gateway.demo_next_system())
	_attack.pressed.connect(_on_attack_pressed)
	_autopilot.toggled.connect(func(on: bool) -> void:
		if gateway == null:
			return
		if on and str(gateway.snapshot().get("target_id", "")) == "":
			var lay := {}
			if "layout" in gateway:
				lay = gateway.layout
			if QuestHints.can_route(gateway.snapshot(), lay):
				guide_autopilot_requested.emit()
				return
		gateway.set_autopilot(on))
	_auto_skills.toggled.connect(func(on: bool) -> void: if gateway != null: gateway.set_auto_skills(on))
	_quests_card.toggled.connect(func(c: bool) -> void: _on_card_toggled("quests", c))
	_quests_card.gui_input.connect(func(ev: InputEvent) -> void:
		if ev is InputEventMouseButton and ev.pressed and ev.button_index == MOUSE_BUTTON_LEFT:
			quest_log_requested.emit())
	var quest_title := _quests_card.find_child("CardHeader", true, false)
	if quest_title != null and quest_title.get_child_count() > 0:
		var title_label := quest_title.get_child(0) as Control
		if title_label != null:
			title_label.mouse_filter = Control.MOUSE_FILTER_STOP
			title_label.gui_input.connect(func(ev: InputEvent) -> void:
				if ev is InputEventMouseButton and ev.pressed and ev.button_index == MOUSE_BUTTON_LEFT:
					quest_log_requested.emit())
	_chat_card.toggled.connect(func(c: bool) -> void:
		_on_card_toggled("chat", c)
		_keep_chat_strip())
	_set_target_empty()
	_boot_layout()
	_adapt_viewport()
	get_viewport().size_changed.connect(_adapt_viewport)


## Короткая плашка: два последних сообщения на прозрачном фоне и круглая иконка чата сбоку.
func _adapt_viewport() -> void:
	var viewport_size := get_viewport_rect().size
	var safe_root := get_node("%SafeArea") as Control
	var safe_size := safe_root.size if safe_root != null and safe_root.size.x > 0.0 else viewport_size
	var w: float = safe_size.x
	var h: float = safe_size.y
	var compact: bool = w < 1040.0
	var narrow: bool = w < 860.0
	# Правый боевой веер занимает больше обычной карточки: колонка должна быть не уже веера,
	# иначе последние кнопки физически выходят за край Android viewport.
	var left_w: float = 220.0 if not compact else 184.0
	var right_w: float = 252.0 if not compact else 224.0
	var columns := get_node("SafeArea/Columns") as HBoxContainer
	var left_stack := columns.get_node("LeftStack") as Control
	var center_stack := columns.get_node("CenterStack") as Control
	var right_stack := columns.get_node("RightStack") as Control
	columns.add_theme_constant_override("separation", 8)
	columns.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	left_stack.custom_minimum_size.x = left_w
	right_stack.custom_minimum_size.x = right_w
	center_stack.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	center_stack.custom_minimum_size.x = 0.0
	_profile_card.custom_minimum_size.x = left_w
	_quests_card.custom_minimum_size.x = left_w
	_chat_card.custom_minimum_size.x = left_w
	%CreditsCard.custom_minimum_size.x = right_w
	%TargetCard.custom_minimum_size.x = right_w
	_attack.custom_minimum_size.x = right_w
	_autopilot.custom_minimum_size.x = right_w
	_quests_card.visible = not narrow
	_chat_card.visible = not narrow
	_target_info.max_lines_visible = 1 if compact else 2
	var compact_scale: float = 1.0
	if w < 760.0 or h < 420.0:
		compact_scale = 0.82
	elif compact:
		compact_scale = 0.90
	%CombatBlock.scale = Vector2.ONE * compact_scale
	var skill_fan := get_node_or_null("SafeArea/Columns/RightStack/SkillFan") as Control
	if skill_fan != null:
		skill_fan.custom_minimum_size.x = right_w
		skill_fan.pivot_offset = Vector2.ZERO
		skill_fan.scale = Vector2.ONE * minf(1.0, right_w / 312.0)
	_reflow_layout()


func _build_chat_strip() -> void:
	var header := _chat_card.find_child("CardHeader", true, false)
	if header != null:
		header.visible = false
	_chat_lines.visible = false
	var sb := StyleBoxFlat.new()
	sb.bg_color = Color(Tokens.BG, 0.22)
	sb.set_corner_radius_all(14)
	sb.set_content_margin_all(8)
	sb.border_color = Color(Tokens.TEXT, 0.10)
	sb.set_border_width_all(1)
	sb.anti_aliasing = true
	_chat_card.add_theme_stylebox_override("panel", sb)
	var row := HBoxContainer.new()
	row.name = "ChatPeek"
	row.add_theme_constant_override("separation", 8)
	row.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var lines := VBoxContainer.new()
	lines.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	lines.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	lines.add_theme_constant_override("separation", 2)
	lines.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_peek_a = _peek_label()
	_peek_b = _peek_label()
	lines.add_child(_peek_a)
	lines.add_child(_peek_b)
	row.add_child(lines)
	var icon := ActionButton.new()
	icon.name = "ChatOpenButton"
	icon.text = ""
	icon.tooltip_text = "Открыть чат"
	icon.focus_mode = Control.FOCUS_ALL
	_round_button(icon, 56, Color(Tokens.SURFACE, 0.35), Color(Tokens.ACCENT, 0.55))
	Icons.apply(icon, "mail", Tokens.ACCENT, 22)
	icon.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
	icon.vertical_icon_alignment = VERTICAL_ALIGNMENT_CENTER
	icon.pressed.connect(func() -> void: chat_requested.emit())
	row.add_child(icon)
	_chat_open = icon
	_chat_lines.get_parent().add_child(row)
	_show_chat_lines(PackedStringArray())


func _peek_label() -> Label:
	var l := Label.new()
	l.mouse_filter = Control.MOUSE_FILTER_IGNORE
	l.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	l.clip_text = true
	l.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	l.add_theme_color_override("font_color", Color(Tokens.TEXT, 0.94))
	l.custom_minimum_size = Vector2(160, 22)
	return l


func _show_chat_lines(lines: PackedStringArray) -> void:
	if _peek_a == null or _peek_b == null:
		return
	if lines.is_empty():
		_peek_a.text = "Чат"
		_peek_a.add_theme_color_override("font_color", Tokens.TEXT_MUTED)
		_peek_b.text = ""
		_peek_b.visible = false
		return
	_peek_a.add_theme_color_override("font_color", Color(Tokens.TEXT, 0.94))
	var older := lines[0] if lines.size() == 1 else lines[lines.size() - 2]
	var newer := "" if lines.size() == 1 else lines[lines.size() - 1]
	_peek_a.text = older
	_peek_b.text = newer
	_peek_b.visible = newer != ""


func _keep_chat_strip() -> void:
	var header := _chat_card.find_child("CardHeader", true, false)
	if header != null:
		header.visible = false
	_chat_lines.visible = false
	var peek := _chat_card.find_child("ChatPeek", true, false)
	if peek != null:
		peek.visible = true
	if chat_panel != null and chat_panel.get_parent() == self:
		chat_panel.visible = false


## Окно чата живёт отдельно и открывается иконкой. В карточке остаются только две строки.
func _has_live_chat(gw: Node) -> bool:
	return gw != null and gw.has_method("has_chat") and gw.has_chat()


func _setup_chat(gw: UiGateway) -> void:
	if chat_panel != null:
		return
	chat_panel = ChatPanel.new()
	chat_panel.name = "ChatPanel"
	chat_panel.visible = false
	add_child(chat_panel)
	chat_panel.bind(gw)
	_keep_chat_strip()
	if not _has_live_chat(gw):
		return
	chat_panel.unread_changed.connect(func(n: int) -> void:
		if _chat_open != null:
			_chat_open.tooltip_text = "Открыть чат" if n <= 0 else "Открыть чат · %d" % n)
	chat_panel.expand_requested.connect(func() -> void: chat_requested.emit())
	chat_panel.preview_changed.connect(func(_t: String) -> void:
		_show_chat_lines(chat_panel.preview_lines(2)))
	_show_chat_lines(chat_panel.preview_lines(2))


## Короткое сообщение игроку (всплывающая плашка).
func show_toast(text: String) -> void:
	_alerts.toast(text)


## Меню (R-35): полупрозрачная кнопка справа сверху раскрывает панель действий.
func _order_menu() -> void:
	var box := %StationMenuButton.get_parent()
	var fly := ActionButton.new()
	fly.name = "SystemMapMenuButton"
	fly.text = "Карта"
	fly.tooltip_text = "Карта системы"
	fly.pressed.connect(func() -> void: _set_menu_open(false); map_requested.emit())
	Icons.apply(fly, "globe", Tokens.ACCENT, 22)
	box.add_child(fly)
	var friends := ActionButton.new()
	friends.name = "FriendsMenuButton"
	friends.text = "Друзья"
	friends.pressed.connect(func() -> void: _set_menu_open(false); friends_requested.emit())
	Icons.apply(friends, "user", Tokens.ACCENT, 22)
	box.add_child(friends)
	var raids := ActionButton.new()
	raids.name = "RaidsMenuButton"
	raids.text = "Коридоры"
	raids.pressed.connect(func() -> void: _set_menu_open(false); raids_requested.emit())
	Icons.apply(raids, "swords", Tokens.RARE, 22)
	box.add_child(raids)
	var order: Array[Node] = [
		%StationMenuButton, fly, %HangarMenuButton, %ProfileMenuButton,
		%ShopMenuButton, %CorporationMenuButton, raids, %StarMapMenuButton, %ChatMenuButton,
		friends, %StorageMenuButton, %SettingsMenuButton, %ExitButton,
		%MailMenuButton,
	]
	for i in order.size():
		box.move_child(order[i], i)


func _toggle_menu() -> void:
	_set_menu_open(not _menu_popup.visible)


func _set_menu_open(open: bool) -> void:
	_menu_popup.visible = open
	var panel := _menu_popup.get_node_or_null("Panel") as CanvasItem
	if panel == null:
		return
	if not open or UiSettings.reduce_motion():
		panel.modulate.a = 1.0
		return
	panel.modulate.a = 0.0
	var tw := create_tween()
	tw.tween_property(panel, "modulate:a", 1.0, 0.16).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_OUT)


## Меню открыто (для тестов и жестов камеры).
func is_menu_open() -> bool:
	return _menu_popup.visible


## Подпись кнопки «Вид» показывает текущий режим камеры.
func set_camera_mode_name(mode_name: String) -> void:
	_reset_button.text = "Вид: " + mode_name


var _restart_offer := false


func _on_server_updated(reason: String) -> void:
	if _restart_offer or _confirm.is_open():
		return
	_restart_offer = true
	var why := reason.strip_edges()
	if why == "":
		why = "настройки"
	var ok: bool = await _confirm.ask("Сервер обновился", "Уже применено: %s.\nПерезапустить игру и зайти заново?" % why, "Перезапустить")
	if ok:
		if gateway != null and gateway.has_method("close"):
			gateway.close()
		get_tree().change_scene_to_file("res://scenes/connect.tscn")
		return
	_restart_offer = false


## Привязка к шлюзу данных (демо или сервер).
func bind(gw: UiGateway) -> void:
	gateway = gw
	gw.state_updated.connect(apply_state)
	gw.alert_raised.connect(func(id: String, text: String) -> void: _alerts.set_alert(id, text))
	gw.alert_cleared.connect(func(id: String) -> void: _alerts.clear_alert(id))
	gw.notification.connect(func(text: String) -> void: _alerts.toast(text))
	gw.command_failed.connect(_on_command_failed)
	# плашка «Демо» нужна только в демо; в игре с сервером топ-центр остаётся свободным (тревоги)
	_demo_bar.visible = gw.is_demo()
	%DemoLabel.visible = gw.is_demo()
	_system_button.visible = gw.is_demo()
	_setup_chat(gw)   # панель чата создаётся при привязке к шлюзу (с 0.3.2 вызов был забыт: в игре оставалась демо-заглушка)
	if _has_live_chat(gw):
		gw.request_chat_history()   # история запрашивается при входе, до появления панели: запрашиваем ещё раз уже для неё
	apply_state(gw.snapshot())
	if gw.has_signal("server_updated"):
		gw.server_updated.connect(_on_server_updated)


## Груз и кнопки добычи/продажи: появляются, только если корабль может добывать или везёт груз.
func _build_cargo_ui() -> void:
	_cargo_row = HBoxContainer.new()
	_cargo_row.add_theme_constant_override("separation", 8)
	_flight_status = Label.new()
	_flight_status.name = "FlightStatus"
	_flight_status.theme_type_variation = &"SectionLabel"
	_flight_status.custom_minimum_size.x = 170
	_cargo_row.add_child(_flight_status)
	_cargo_row.add_child(Icons.rect("package", 20, Tokens.ACCENT))
	_cargo_label = Label.new()
	_cargo_label.name = "CargoLabel"
	_cargo_row.add_child(_cargo_label)
	var hold := ActionButton.new()
	hold.name = "HoldButton"
	hold.text = "Трюм"
	hold.custom_minimum_size = Vector2(88, 44)
	hold.pressed.connect(func() -> void: hold_requested.emit())
	_cargo_row.add_child(hold)
	_cargo_row.visible = false
	_flight_status.visible = false
	%BarsRow.add_child(_cargo_row)
	var stack := _autopilot.get_parent()
	_mine_button = ActionButton.new()
	_mine_button.name = "MineButton"
	_mine_button.text = "Добыть"
	Icons.apply(_mine_button, "pickaxe", Tokens.ACCENT, 22)
	_mine_button.visible = false
	_mine_button.pressed.connect(_on_mine_pressed)
	_sell_button = ActionButton.new()
	_sell_button.name = "SellButton"
	_sell_button.text = "Продать груз"
	Icons.apply(_sell_button, "coins", Tokens.RARE, 22)
	_sell_button.visible = false
	_sell_button.pressed.connect(func() -> void: if gateway != null: gateway.sell())
	stack.add_child(_mine_button)
	stack.add_child(_sell_button)
	var at := _autopilot.get_index()
	stack.move_child(_mine_button, at)
	stack.move_child(_sell_button, at + 1)


func _on_mine_pressed() -> void:
	if gateway == null:
		return
	var ship: Dictionary = _last_state.get("ship", {})
	if str(ship.get("mining", "")) != "":
		gateway.stop_mining()
	else:
		gateway.mine(str(_last_state.get("target_id", "")))


## Состояние кнопок добычи и груза. Вынесено отдельно для тестов.
func _apply_cargo(state: Dictionary, target: Dictionary, ship: Dictionary) -> void:
	var cap := float(ship.get("cargo", 0.0))
	var used := float(ship.get("cargo_used", 0.0))
	_cargo_row.visible = cap > 0.0 and state.get("mode", "") == "server"
	_flight_status.visible = _cargo_row.visible
	var mining_now := str(ship.get("mining", "")) != ""
	if mining_now:
		_flight_status.text = "Добыча"
		_flight_status.add_theme_color_override("font_color", Tokens.RARE)
	elif bool(ship.get("in_safe", false)) and used > 0.0:
		_flight_status.text = "Сдать груз"
		_flight_status.add_theme_color_override("font_color", Tokens.SUCCESS)
	elif bool(state.get("autopilot", false)):
		_flight_status.text = "Маршрут"
		_flight_status.add_theme_color_override("font_color", Tokens.ACCENT_2)
	else:
		_flight_status.text = "Свободный полёт"
		_flight_status.add_theme_color_override("font_color", Tokens.TEXT_MUTED)
	var bay_text := ""
	var bays: Array = ship.get("bays", [])
	if bays.size() > 0:
		var bits: PackedStringArray = []
		for bay in bays:
			if bay is Dictionary and float(bay.get("cap", 0.0)) > 0.0:
				bits.append("%s %d/%d" % [str(bay.get("name", "")), int(round(float(bay.get("used", 0.0)))), int(round(float(bay.get("cap", 0.0))))])
		bay_text = " · ".join(bits)
	_cargo_label.text = bay_text if bay_text != "" else "Груз %d/%d" % [int(round(used)), int(round(cap))]
	_cargo_label.add_theme_color_override("font_color", Tokens.DANGER if cap > 0.0 and used >= cap - 0.01 else Tokens.TEXT)
	var can_mine := bool(ship.get("can_mine", false))
	_mine_button.visible = can_mine
	if can_mine:
		var mining := str(ship.get("mining", "")) != ""
		var is_dep := not target.is_empty() and str(target.get("kind", "")) == "deposit"
		var in_reach := is_dep and (ship["pos"] as Vector2).distance_to(target["pos"]) <= float(ship.get("mine_range", 0.0))
		_mine_button.text = "Остановить добычу" if mining else "Добыть"
		_mine_button.disabled = not mining and not (is_dep and in_reach and used < cap - 0.01 and float(target.get("amount", 0.0)) >= 1.0)
		_mine_button.tooltip_text = "Выберите месторождение и подлетите ближе" if not is_dep else ("Подлетите ближе" if not in_reach else "")
	_sell_button.visible = used > 0.0
	_sell_button.disabled = not bool(ship.get("in_safe", false))
	_sell_button.tooltip_text = "" if bool(ship.get("in_safe", false)) else "Продавать груз можно только у станции"



func _dock_target() -> void:
	var row := _speed_label.get_parent()
	var card := %TargetCard as Control
	card.reparent(row, false)
	card.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_target_info.custom_minimum_size.x = 200
	var box := _target_name.get_parent()
	var head := HBoxContainer.new()
	head.name = "TargetHead"
	head.add_theme_constant_override("separation", 8)
	box.add_child(head)
	box.move_child(head, 0)
	_target_name.reparent(head, false)
	_target_name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_clear_target = Button.new()
	_clear_target.name = "ClearTarget"
	_clear_target.text = "×"
	_clear_target.tooltip_text = "Снять цель"
	_clear_target.focus_mode = Control.FOCUS_NONE
	_clear_target.custom_minimum_size = Vector2(40, 40)
	_clear_target.pressed.connect(func() -> void:
		if gateway != null:
			gateway.set_target(""))
	head.add_child(_clear_target)
	card.visible = false


func _dock_skills() -> void:
	var right := _attack.get_parent()
	var fan := Control.new()
	fan.name = "SkillFan"
	fan.mouse_filter = Control.MOUSE_FILTER_IGNORE
	fan.custom_minimum_size = Vector2(312, 280)
	fan.size_flags_horizontal = Control.SIZE_SHRINK_END
	right.add_child(fan)
	right.move_child(fan, _attack.get_index())
	for b in _skill_buttons:
		(b as Control).reparent(fan, false)
	_attack.reparent(fan, false)
	_attack.text = ""
	_style_attack(false)
	_auto_skills.reparent(fan, false)
	_auto_skills.text = "Авто"
	_cycle_target = Button.new()
	_cycle_target.name = "CycleTargetButton"
	_cycle_target.focus_mode = Control.FOCUS_ALL
	_cycle_target.tooltip_text = "Ближайшая цель. Если цель уже выбрана — следующая по дальности."
	_cycle_target.pressed.connect(_cycle_target_pressed)
	fan.add_child(_cycle_target)
	_place_fan()
	_paint_auto()
	_skills_grid.visible = false


## Дуга большого пальца: умения крупными кругами, удар — самый большой справа снизу.
func _place_fan() -> void:
	var spots := [Vector2(0, 0), Vector2(88, 0), Vector2(0, 88), Vector2(88, 92)]
	var sides := [78.0, 84.0, 78.0, 78.0]
	var extra_spots := [Vector2(0, 176), Vector2(168, 88)]
	for i in _skill_buttons.size():
		var b := _skill_buttons[i] as Control
		if i < spots.size():
			b.position = spots[i]
			b.set_meta("fan_side", sides[i])
			_round_button(b, sides[i])
		else:
			b.position = extra_spots[i - spots.size()]
			b.set_meta("fan_side", 64.0)
			_round_button(b, 64.0)
			b.visible = false
	_attack.position = Vector2(188, 152)
	_auto_skills.position = Vector2(100, 204)
	_round_button(_auto_skills, 56)
	if _cycle_target != null:
		_cycle_target.position = Vector2(232, 22)
		_round_button(_cycle_target, 62, Color(Tokens.SURFACE, 0.82), Color(Tokens.ACCENT, 0.8))
		Icons.apply(_cycle_target, "maximize-2", Tokens.ACCENT, 32)
		_cycle_target.text = ""
		_cycle_target.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
		_cycle_target.vertical_icon_alignment = VERTICAL_ALIGNMENT_CENTER


func _round_button(b: Button, side: float, fill: Variant = null, edge: Variant = null) -> void:
	b.custom_minimum_size = Vector2(side, side)
	b.clip_text = true
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.add_theme_font_size_override("font_size", 26 if side >= 100.0 else (20 if side >= 72.0 else 14))
	var bg := Color(Tokens.SURFACE, 0.78)
	var border := Color(Tokens.ACCENT, 0.55)
	if fill is Color:
		bg = fill
	if edge is Color:
		border = edge
	var sb := StyleBoxFlat.new()
	sb.bg_color = bg
	sb.border_color = border
	sb.set_border_width_all(1)
	sb.set_corner_radius_all(int(side * 0.5))
	sb.content_margin_left = 4
	sb.content_margin_right = 4
	sb.content_margin_top = 4
	sb.content_margin_bottom = 4
	var hover := sb.duplicate() as StyleBoxFlat
	hover.bg_color = bg.lightened(0.12)
	var pressed := sb.duplicate() as StyleBoxFlat
	pressed.bg_color = bg.lightened(0.2)
	var disabled := sb.duplicate() as StyleBoxFlat
	disabled.bg_color = Color(Tokens.BG, 0.45)
	disabled.border_color = Color(Tokens.TEXT_MUTED, 0.25)
	b.add_theme_stylebox_override("normal", sb)
	b.add_theme_stylebox_override("hover", hover)
	b.add_theme_stylebox_override("pressed", pressed)
	b.add_theme_stylebox_override("disabled", disabled)
	b.add_theme_stylebox_override("focus", sb)


func _style_attack(attacking: bool) -> void:
	var fill := Color(Tokens.DANGER, 0.95) if attacking else Color(Tokens.DANGER, 0.72)
	var edge := Color(1, 1, 1, 0.9) if attacking else Color(Tokens.DANGER.lightened(0.2), 0.85)
	_round_button(_attack, 108, fill, edge)
	Icons.apply(_attack, "zap", Color.WHITE, 58)
	_attack.text = ""
	_attack.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_attack.vertical_icon_alignment = VERTICAL_ALIGNMENT_CENTER


func _paint_auto() -> void:
	if _auto_skills == null:
		return
	var on := _auto_skills.button_pressed
	var sb := StyleBoxFlat.new()
	sb.set_corner_radius_all(int(_auto_skills.custom_minimum_size.x * 0.5))
	sb.set_border_width_all(2)
	sb.content_margin_left = 2
	sb.content_margin_right = 2
	sb.content_margin_top = 2
	sb.content_margin_bottom = 2
	if on:
		sb.bg_color = Color(0.96, 0.97, 1.0, 0.96)
		sb.border_color = Color(1, 1, 1, 0.95)
	else:
		sb.bg_color = Color(0.28, 0.31, 0.36, 0.88)
		sb.border_color = Color(0.55, 0.58, 0.64, 0.45)
	for state_name in ["normal", "hover", "pressed", "focus", "disabled"]:
		_auto_skills.add_theme_stylebox_override(state_name, sb)
	_auto_skills.add_theme_color_override("font_color", Color(0.12, 0.14, 0.18) if on else Color(0.75, 0.78, 0.82))
	_auto_skills.add_theme_color_override("font_disabled_color", Color(0.55, 0.58, 0.62))
	_auto_skills.add_theme_font_size_override("font_size", 12)


func _build_skills() -> void:
	for i in 6:
		var b := ActionButton.new()
		b.name = "Skill%d" % i
		b.text = "Навык %d" % (i + 1)
		b.custom_minimum_size = SKILL_MIN_SIZE
		b.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 2)
		var fill := ColorRect.new()   # затемнение убывает по мере перезарядки: готовность видна без чтения текста
		fill.name = "CooldownFill"
		fill.color = Color(Tokens.BG, 0.6)
		fill.mouse_filter = Control.MOUSE_FILTER_IGNORE
		fill.set_anchors_preset(Control.PRESET_FULL_RECT)
		fill.anchor_right = 0.0
		fill.offset_right = 0.0
		b.add_child(fill)
		b.disabled = true
		var slot := i
		b.pressed.connect(func() -> void:
			if gateway != null and not _editing:
				gateway.use_skill(slot))
		_skills_grid.add_child(b)
		_skill_buttons.append(b)


func _process(_delta: float) -> void:
	if _auto_skills != null and _auto_skills.button_pressed and not _auto_skills.disabled and not UiSettings.reduce_motion():
		var pulse := 0.5 + 0.5 * sin(float(Time.get_ticks_msec()) / 220.0)
		_auto_skills.modulate = Color(1.0, 1.0, 1.0, 0.82 + 0.18 * pulse)
		if not _layout_has("combat.auto", "scale"):
			_auto_skills.scale = Vector2.ONE * (1.0 + 0.04 * pulse)
			_auto_skills.pivot_offset = _auto_skills.size * 0.5
	elif _auto_skills != null:
		_auto_skills.modulate = Color(0.72, 0.75, 0.8) if not _auto_skills.button_pressed else Color.WHITE
		if not _layout_has("combat.auto", "scale"):
			_auto_skills.scale = Vector2.ONE
	if _editing:
		if gateway != null and _sent_move != Vector2.ZERO:
			_sent_move = Vector2.ZERO
			gateway.move_input(Vector2.ZERO)
		return
	if gateway == null:
		return
	# ручное управление с клавиатуры (проверка на компьютере): W A S D плюс джойстик
	var kb := Vector2(
		float(Input.is_key_pressed(KEY_D)) - float(Input.is_key_pressed(KEY_A)),
		float(Input.is_key_pressed(KEY_S)) - float(Input.is_key_pressed(KEY_W)))
	var total := (_joy + kb).limit_length(1.0)
	if move_transform.is_valid() and total != Vector2.ZERO:
		total = move_transform.call(total)
	# при повороте камеры вектор меняется плавно: мелкие изменения не отправляем, чтобы не засорять канал
	var changed := total.distance_to(_sent_move) > 0.04 or (total == Vector2.ZERO) != (_sent_move == Vector2.ZERO)
	if changed:
		_sent_move = total
		gateway.move_input(total)


func _build_party_box() -> void:
	_party_box = PanelContainer.new()
	_party_box.name = "PartyBox"
	_party_box.visible = false
	_party_box.mouse_filter = Control.MOUSE_FILTER_STOP
	_party_box.position = Vector2(16, 168)
	_party_box.custom_minimum_size = Vector2(228, 0)
	_airy(_party_box, 0.72)
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 6)
	_party_box.add_child(box)
	_party_label = Label.new()
	_party_label.name = "PartyLabel"
	_party_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_party_label.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	box.add_child(_party_label)
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 6)
	box.add_child(row)
	_party_invite = _party_button(row, "PartyInvite", "В пати", _on_party_invite)
	_party_join = _party_button(row, "PartyJoin", "Принять", _on_party_join)
	_party_start = _party_button(row, "PartyStart", "В бой", _on_party_start)
	_party_leave = _party_button(row, "PartyLeave", "Выйти", _on_party_leave)
	_party_follow = _party_button(box, "PartyFollow", "Не следовать", _on_party_follow)
	add_child(_party_box)


func _party_button(row: Node, node_name: String, caption: String, cb: Callable) -> Button:
	var b := Button.new()
	b.name = node_name
	b.text = caption
	b.custom_minimum_size = Vector2(0, Tokens.TOUCH_MIN)
	b.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	b.pressed.connect(cb)
	row.add_child(b)
	return b


func _party_call(method: String, arg: String = "") -> void:
	if gateway == null or not gateway.has_method(method):
		return
	if arg == "":
		gateway.call(method)
	else:
		gateway.call(method, arg)


func _on_party_invite() -> void:
	var target_id: String = str(_last_state.get("target_id", ""))
	_party_call("party_invite", target_id)


func _on_party_join() -> void:
	_party_call("party_join")


func _on_party_start() -> void:
	_party_call("party_start")


func _on_party_leave() -> void:
	_party_call("party_leave")


func _apply_party(state: Dictionary) -> void:
	if _party_box == null:
		return
	var party: Dictionary = {}
	if state.get("party") is Dictionary:
		party = state["party"]
	var members: Array = []
	var raw_members: Variant = party.get("members", [])
	if raw_members is Array:
		members = raw_members
	var pending: bool = bool(party.get("pending", false))
	var in_inst: bool = bool(party.get("instance", false))
	var target_id: String = str(state.get("target_id", ""))
	var target_kind: String = ""
	var target_ai: bool = false
	for o in state.get("objects", []):
		if o is Dictionary and str(o.get("id", "")) == target_id:
			target_kind = str(o.get("kind", ""))
			target_ai = bool(o.get("ai", false))
			break
	var can_invite: bool = target_id != "" and not in_inst and (target_kind == "ai" or target_kind == "player" or target_ai)
	var mine: String = ""
	if gateway != null:
		mine = str(gateway.get("player_id"))
	var leader_id: String = str(party.get("leader", ""))
	var am_leader: bool = mine != "" and mine == leader_id
	var show_box: bool = members.size() > 0 or pending or can_invite
	_party_box.visible = show_box
	if not show_box:
		return
	var title: String = str(party.get("title", ""))
	if title == "":
		title = "Пати"
	var lines: String = title
	var goal: String = str(party.get("goal", ""))
	if goal != "":
		lines += "\n" + goal
	var need: int = int(party.get("need", 0))
	if need > 0:
		lines += "  %d/%d" % [int(party.get("progress", 0)), need]
	for m in members:
		if m is Dictionary:
			var nm: String = str(m.get("name", "пилот"))
			if bool(m.get("leader", false)):
				nm += " · ведущий"
			if bool(m.get("ai", false)):
				nm += " · ИИ"
			if bool(m.get("pending", false)):
				nm += " · ждёт"
			lines += "\n" + nm
	_party_label.text = lines
	var alone: bool = members.is_empty()
	_party_invite.visible = can_invite and (alone or am_leader)
	_party_join.visible = pending
	_party_start.visible = false
	_party_leave.visible = members.size() > 0
	_party_leave.text = "Покинуть стычку" if in_inst else "Выйти"
	var following := true
	if party.has("you_follow"):
		following = bool(party.get("you_follow"))
	_party_follow.visible = members.size() > 0 and not pending and not am_leader and mine != ""
	_party_follow.text = "Не следовать" if following else "Следовать"


func _on_party_follow() -> void:
	var party: Dictionary = _last_state.get("party", {}) if _last_state.get("party") is Dictionary else {}
	var following := true
	if party.has("you_follow"):
		following = bool(party.get("you_follow"))
	if gateway != null and gateway.has_method("party_follow"):
		gateway.party_follow(not following)


# --- отображение состояния ---

func apply_state(state: Dictionary) -> void:
	_last_state = state
	var player: Dictionary = state.get("player", {})
	_player_name.text = str(player.get("name", ""))
	_player_level.visible = player.has("level")   # уровня нет в данных: не выдумываем
	_player_level.text = "Ур. %d" % int(player.get("level", 1)) + ((" · +%d" % int(player["attr_free"])) if int(player.get("attr_free", 0)) > 0 else "")
	_apply_xp(player)
	var ship: Dictionary = state.get("ship", {})
	var at_station := bool(ship.get("in_safe", false))
	%StationMenuButton.visible = at_station
	%HangarMenuButton.visible = at_station
	_ship_name.text = str(ship.get("name", "")) + ((" · ур. %d" % int(ship["level"])) if ship.has("level") else "")
	# косметика профиля (R-34): аватарка, рамка, титул — прислал сервер
	var cos: Dictionary = ship.get("cosmetics", {})
	var avatar := str(cos.get("avatar", ""))
	_avatar_rect.texture = Icons.tex(avatar if avatar != "" else "user")
	_avatar_rect.modulate = Color(str(cos.get("frame", ""))) if str(cos.get("frame", "")) != "" else (Tokens.ACCENT if avatar != "" else Tokens.TEXT_MUTED)
	var title := str(cos.get("title", ""))
	_title_label.visible = title != ""
	_title_label.text = ("«%s»" % title) if title != "" else ""
	# шкалы и кредиты показываются только если сервер прислал данные
	_hull.visible = ship.has("hull")
	_shield.visible = ship.has("shield")
	_energy.visible = ship.has("energy")
	%BarsRow.visible = _hull.visible or _shield.visible or _energy.visible
	_hull.set_values(ship.get("hull", 0.0), ship.get("hull_max", 1.0))
	_shield.set_values(ship.get("shield", 0.0), ship.get("shield_max", 1.0))
	_energy.set_values(ship.get("energy", 0.0), ship.get("energy_max", 1.0))
	%CreditsCard.visible = state.has("credits")
	_credits.set_value(int(state.get("credits", 0)))
	_skills_grid.visible = not state.get("skills", []).is_empty() or not state.has("combat_available")
	_update_speed(state, ship)

	var target := _target_object(state)
	_apply_target(state, target)
	_apply_buttons(state, target, ship)
	_apply_combat_layout(bool(state.get("combat", {}).get("in_combat", false)))
	_apply_lists(state)
	_apply_event(state)
	_apply_party(state)
	_enforce_user_hides()
	_reflow_layout()


## Скорость считается по реальному перемещению корабля между снимками состояния (не выдуманное число).
func _update_speed(state: Dictionary, ship: Dictionary) -> void:
	if ship.is_empty() or not ship.has("pos"):
		_speed_label.text = "Скорость недоступна"
		return
	var pos: Vector2 = ship["pos"]
	var t: float = float(state.get("time", -1.0))
	if _speed_prev_pos != Vector2.INF and t > _speed_prev_time + 0.001:
		var inst := pos.distance_to(_speed_prev_pos) / (t - _speed_prev_time) * METERS_PER_UNIT
		_speed_mps = lerpf(_speed_mps, inst, 0.5)
		if _speed_mps < 0.5:
			_speed_mps = 0.0
	if t != _speed_prev_time or _speed_prev_pos == Vector2.INF:
		_speed_prev_pos = pos
		_speed_prev_time = t
	_speed_label.text = "%d м/с" % int(round(_speed_mps))


func _target_object(state: Dictionary) -> Dictionary:
	var id: String = str(state.get("target_id", ""))
	if id == "":
		return {}
	for o in state.get("objects", []):
		if o["id"] == id:
			return o
	return {}


static func kind_label(obj: Dictionary) -> String:
	match str(obj.get("kind", "")):
		"player":
			return "Игрок"
		"ai":
			return "AI-капитан"
		"hostile":
			return "Враг"
		"station":
			return "Станция"
		"deposit":
			return "Месторождение"
		"anomaly":
			return "Аномалия"
		"derelict":
			return "Заброшенная станция"
		"signal":
			return "Сигнал"
	return "Объект"


static func tier_label(tier: String) -> String:
	return {"normal": "обычный", "reinforced": "усиленный", "elite": "элитный", "miniboss": "мини-босс", "worldboss": "мировой босс"}.get(tier, "")


static func stance_label(stance: String) -> String:
	match stance:
		"hostile":
			return "враждебный"
		"friendly":
			return "союзный"
	return "нейтральный"


func _set_target_empty() -> void:
	_target_name.text = "Цель не выбрана"
	_target_info.text = "Нажмите на объект в космосе"
	_target_hull.visible = false
	(%TargetCard as Control).visible = false
	_speed_label.visible = true


func _apply_target(state: Dictionary, target: Dictionary) -> void:
	if target.is_empty():
		_set_target_empty()
		return
	(%TargetCard as Control).visible = true
	_speed_label.visible = false
	_target_name.text = str(target["name"])
	var dist := ((state["ship"]["pos"] as Vector2).distance_to(target["pos"])) / 10.0
	var info := "%s · %s · %.1f км" % [kind_label(target), stance_label(str(target["stance"])), dist]
	if str(target.get("kind", "")) == "deposit":
		info = "Месторождение · %d из %d · %.1f км" % [int(target.get("amount", 0.0)), int(target.get("amount_max", 0.0)), dist]
	elif tier_label(str(target.get("tier", ""))) != "":
		info += " · " + tier_label(str(target["tier"]))
	if state.get("combat_available", true):
		info += " · " + ("в зоне огня" if state.get("target_in_range", false) else "вне дальности")
	_target_info.text = info
	_target_hull.visible = target.has("hull")
	if target.has("hull"):
		_target_hull.set_values(target["hull"], target["hull_max"])


func _apply_buttons(state: Dictionary, target: Dictionary, ship: Dictionary) -> void:
	var attacking: bool = bool(state.get("attacking", false))
	var has_target := not target.is_empty()
	var friendly := has_target and str(target["stance"]) == "friendly"

	# кнопка атаки: «Прекратить» во время атаки; busy снимается, когда состояние подтвердило команду
	if _attack.is_busy() and _attack_expect != null and attacking == _attack_expect:
		_attack.end_busy()
		_attack_expect = null
	var combat_ok: bool = state.get("combat_available", true)
	if not _attack.is_busy():
		var phrase := ("Прекратить атаку" if attacking else "Атаковать") if combat_ok else "Бой скоро"
		_attack.tooltip_text = phrase
		_attack.disabled = not combat_ok or (not attacking and (not has_target or friendly))
		_style_attack(attacking and combat_ok)

	for i in _skill_buttons.size():
		var b: ActionButton = _skill_buttons[i]
		var skills: Array = state.get("skills", [])
		if i >= skills.size():
			b.disabled = true
			b.text = ""
			b.icon = null
			b.visible = i < 4
			continue
		b.visible = true
		var s: Dictionary = skills[i]
		var cd: float = s["cooldown_left"]
		var fill := b.get_node_or_null("CooldownFill") as ColorRect
		if fill != null:
			fill.anchor_right = clampf(cd / maxf(float(s["cooldown"]), 0.001), 0.0, 1.0) if cd > 0.0 else 0.0
			fill.offset_right = 0.0
		var skill_name := str(s["name"])
		var icon := str(s.get("icon", ""))
		if cd > 0.0:
			b.text = str(int(ceil(cd)))
			if icon != "":
				b.icon = null
		else:
			b.text = "" if icon != "" else skill_name
			if icon != "":
				var tint := Color.WHITE
				if str(s.get("role", "")) == "ult":
					tint = Color(0.95, 0.78, 0.35)
				elif str(s.get("role", "")) == "module":
					tint = Color(0.65, 0.9, 1.0)
				var icon_px := 46
				if b.has_meta("fan_side"):
					icon_px = int(round(float(b.get_meta("fan_side")) * 0.58))
				Icons.apply(b, icon, tint, icon_px)
				b.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
				b.vertical_icon_alignment = VERTICAL_ALIGNMENT_CENTER
		if b.has_meta("fan_side"):
			var side := float(b.get_meta("fan_side"))
			b.custom_minimum_size = Vector2(side, side)
			b.size = Vector2(side, side)
		var role_name: String = str({"strike": "атака", "ult": "ульта", "module": "модуль"}.get(str(s.get("role", "")), ""))
		var kind := ("%s · " % role_name) if role_name != "" else ""
		b.tooltip_text = "%s%s · энергия %d · перезарядка %d с" % [kind, skill_name, int(s["energy_cost"]), int(s["cooldown"])]
		var blocked: bool = not bool(s.get("enabled", true)) or bool(ship.get("dead", false)) or cd > 0.0 or float(ship.get("energy", 0.0)) < float(s["energy_cost"])
		var reach := float(s.get("range", 0.0))
		var near := bool(state.get("target_in_range", false))
		if reach > 0.0 and has_target and ship.get("pos") is Vector2 and target.get("pos") is Vector2:
			near = (ship["pos"] as Vector2).distance_to(target["pos"]) <= reach
		if s["needs_target"]:
			blocked = blocked or not has_target or friendly or not near
		b.disabled = blocked
		if not bool(s.get("enabled", true)):
			b.tooltip_text = "Навык не изучен" if str(s.get("unavailable_reason", "")) == "ability_not_learned" else "Требуется установленный модуль"
	_layout_skill_spots(state.get("skills", []))


	_apply_cargo(state, target, ship)
	var lay := {}
	if gateway != null and "layout" in gateway:
		lay = gateway.layout
	var guided := QuestHints.can_route(state, lay)
	_autopilot.disabled = not has_target and not guided and not state.get("autopilot", false)
	_set_toggle(_autopilot, "Автопилот", bool(state.get("autopilot", false)))
	_auto_skills.disabled = not combat_ok
	_auto_skills.set_pressed_no_signal(bool(state.get("auto_skills", false)))
	_auto_skills.text = "Авто"
	_paint_auto()


func _set_toggle(b: ActionButton, label: String, on: bool) -> void:
	b.set_pressed_no_signal(on)
	b.text = label + (" · вкл" if on else "")
	b.variant = ActionButton.Variant.PRIMARY if on else ActionButton.Variant.SECONDARY


## В бою второстепенные панели сворачиваются, после боя возвращается выбор игрока.
func _apply_combat_layout(in_combat: bool) -> void:
	if in_combat == _was_combat:
		return
	_was_combat = in_combat
	_auto_collapsing = true
	if in_combat:
		_quests_card.set_collapsed(true, false)
	else:
		_quests_card.set_collapsed(_pref_collapsed["quests"], true)
	_keep_chat_strip()
	_auto_collapsing = false


func _on_card_toggled(which: String, collapsed: bool) -> void:
	if not _auto_collapsing:
		_pref_collapsed[which] = collapsed  # выбор игрока запоминается


func _apply_lists(state: Dictionary) -> void:
	var quests: Array = state.get("quests", [])
	var qsig := JSON.stringify(quests)
	if qsig != _quests_sig:
		_quests_sig = qsig
		for c in _quests_list.get_children():
			c.queue_free()
		if quests.is_empty():
			_add_muted(_quests_list, "Нет активных заданий")
		for raw in quests:
			var q: Dictionary = QuestHints.delivery_step(raw, state)
			var b := Button.new()
			b.alignment = HORIZONTAL_ALIGNMENT_LEFT
			b.theme_type_variation = &"FlatButton"
			b.text = "%s (%d/%d)" % [q["title"], int(raw.get("progress", 0)), int(raw.get("goal", 1))]
			b.clip_text = true
			b.custom_minimum_size = Vector2(0, 56)
			var tip := str(q.get("text", ""))
			b.tooltip_text = tip if tip != "" else b.text
			b.pressed.connect(func() -> void: quest_log_requested.emit())
			_quests_list.add_child(b)
	var chat: Array = state.get("chat", [])
	var csig := JSON.stringify(chat)
	if csig != _chat_sig:
		_chat_sig = csig
		var live := _has_live_chat(gateway)
		if not live:
			var lines := PackedStringArray()
			for m in chat.slice(maxi(0, chat.size() - 2)):
				var who := str(m.get("author", "")).strip_edges()
				var text := str(m.get("text", "")).strip_edges().replace("\n", " ")
				var line := text if who == "" else "%s: %s" % [who, text]
				if line.length() > 80:
					line = line.substr(0, 79) + "…"
				lines.append(line)
			_show_chat_lines(lines)
		_keep_chat_strip()


func _add_muted(parent: Control, text: String) -> void:
	var l := Label.new()
	l.text = text
	l.theme_type_variation = &"MutedLabel"
	l.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	l.custom_minimum_size.x = 220
	l.tooltip_text = text
	parent.add_child(l)


# --- действия ---

func _on_attack_pressed() -> void:
	if _editing or gateway == null or _attack.is_busy():
		return  # в редакторе HUD бой не запускается; повторное нажатие тоже отсекается
	if bool(_last_state.get("attacking", false)):
		_begin_attack_busy("…", false)
		gateway.stop_attack()
		return
	var target := _target_object(_last_state)
	if target.is_empty():
		return
	if str(target["stance"]) != "hostile":
		var title := "Атаковать AI-капитана?" if target["kind"] == "ai" else "Атаковать нейтрального игрока?"
		var text := "Открытое PvP: цель и её союзники могут ответить. Последствия атаки (ответный бой, потеря репутации, потери корабля и груза) определяет сервер."
		var ok: bool = await _confirm.ask(title, text, "Атаковать")
		if not ok:
			return
		# пока окно было открыто, цель могла измениться или исчезнуть
		if _target_object(_last_state).get("id", "") != target["id"]:
			_alerts.toast("Цель изменилась, атака отменена")
			return
	_begin_attack_busy("Атака…", true)
	gateway.attack(str(target["id"]))


func _begin_attack_busy(_text: String, expect: bool) -> void:
	_attack.begin_busy("")
	_attack_expect = expect
	get_tree().create_timer(BUSY_TIMEOUT).timeout.connect(func() -> void:
		if _attack.is_busy():
			_attack.end_busy()  # ответ не пришёл: кнопка снова доступна
			_attack_expect = null)


func _on_command_failed(_action: String, reason: String) -> void:
	_alerts.toast(reason)
	if _attack.is_busy():
		_attack.end_busy()
		_attack_expect = null


var _event_label: Label


func _apply_event(state: Dictionary) -> void:
	if _event_label == null:
		_event_label = Label.new()
		_event_label.name = "WorldEvent"
		_event_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		_event_label.custom_minimum_size = Vector2(300, 0)
		_event_label.add_theme_font_size_override("font_size", 15)
		_event_label.max_lines_visible = 4
		_event_label.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
		_event_label.add_theme_color_override("font_color", Tokens.RARE)
		var host := _quests_card.get_parent()
		host.add_child(_event_label)
		host.move_child(_event_label, _quests_card.get_index() + 1)
	var ev: Variant = state.get("event", null)
	if typeof(ev) != TYPE_DICTIONARY or (ev as Dictionary).is_empty():
		_event_label.visible = false
		return
	var e: Dictionary = ev
	var left := int(e.get("left", 0))
	var when := "%d с" % left
	if left >= 60:
		when = "%d мин" % int(left / 60.0)
	_event_label.visible = true
	_event_label.text = "⚡ %s · %s\n%s" % [str(e.get("title", "")), when, str(e.get("text", ""))]


func _boot_layout() -> void:
	_layout_free = Control.new()
	_layout_free.name = "HudFreeLayer"
	_layout_free.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_layout_free.position = Vector2.ZERO
	_layout_free.size = Vector2.ZERO
	_layout_free.z_index = 8
	add_child(_layout_free)
	_bind_layout_nodes()
	for id in _layout_nodes.keys():
		var node := _layout_nodes[id] as Control
		if node == null:
			continue
		_layout_homes[id] = {
			"parent": node.get_parent(),
			"index": node.get_index(),
			"visible": node.visible,
			"self_modulate": node.self_modulate,
			"scale": node.scale,
			"position": node.position,
			"anchor_left": node.anchor_left,
			"anchor_top": node.anchor_top,
			"anchor_right": node.anchor_right,
			"anchor_bottom": node.anchor_bottom,
			"offset_left": node.offset_left,
			"offset_top": node.offset_top,
			"offset_right": node.offset_right,
			"offset_bottom": node.offset_bottom,
			"grow_horizontal": node.grow_horizontal,
			"grow_vertical": node.grow_vertical,
			"min": node.custom_minimum_size,
		}
	_layout_doc = LayoutStore.load_doc()
	if not get_viewport().size_changed.is_connected(_reflow_layout):
		get_viewport().size_changed.connect(_reflow_layout)
	call_deferred("_reflow_layout")


func _bind_layout_nodes() -> void:
	_layout_nodes = {
		"ship.joystick": _joystick,
		"ship.bars": %BarsRow,
		"ship.cargo": _cargo_row,
		"ship.speed": _speed_label,
		"player.card": _profile_card,
		"player.quests": _quests_card,
		"player.credits": %CreditsCard,
		"social.chat": _chat_card,
		"social.party": _party_box,
		"nav.map": _map_hud_button,
		"nav.menu": _menu_button,
		"nav.camera": _reset_button,
		"nav.demo": _demo_bar,
		"combat.target": %TargetCard,
		"combat.cycle": _cycle_target,
		"combat.attack": _attack,
		"combat.auto": _auto_skills,
		"combat.autopilot": _autopilot,
		"combat.mine": _mine_button,
		"combat.sell": _sell_button,
	}
	for i in _skill_buttons.size():
		_layout_nodes["combat.skill.%d" % i] = _skill_buttons[i]


func _layout_item(id: String) -> Dictionary:
	var items := LayoutStore.active_items(_layout_doc)
	var raw: Variant = items.get(id, {})
	return raw if raw is Dictionary else {}


func _layout_has(id: String, key: String) -> bool:
	return _layout_item(id).has(key)


func _layout_skill_spots(skills: Array) -> void:
	if _editing or skills.is_empty():
		return
	# Все точки внутри веера 312×280, справа от них крупная атака. За край экрана не выходят.
	var pool: Array = [
		[Vector2(0, 0), 78.0],
		[Vector2(88, 0), 84.0],
		[Vector2(0, 88), 78.0],
		[Vector2(88, 92), 78.0],
		[Vector2(0, 176), 70.0],
		[Vector2(168, 88), 62.0],
	]
	var used: Array = []
	for i in mini(skills.size(), _skill_buttons.size()):
		var id := "combat.skill.%d" % i
		if _layout_has(id, "nx"):
			continue
		var role := str((skills[i] as Dictionary).get("role", ""))
		var slot := 3
		if role == "strike":
			slot = 0
		elif role == "ult":
			slot = 1
		elif role == "module" and not used.has(2):
			slot = 2
		else:
			for n in pool.size():
				if not used.has(n):
					slot = n
					break
		if used.has(slot):
			for n in pool.size():
				if not used.has(n):
					slot = n
					break
		used.append(slot)
		var spot: Array = pool[slot]
		var b := _skill_buttons[i] as Control
		var side := float(spot[1])
		b.position = spot[0]
		b.set_meta("fan_side", side)
		b.size = Vector2(side, side)
		_round_button(b, side)


func _cycle_target_pressed() -> void:
	if _editing or gateway == null:
		return
	var state: Dictionary = gateway.snapshot()
	var ship: Dictionary = state.get("ship", {})
	if not (ship.get("pos") is Vector2):
		return
	var me: Vector2 = ship["pos"]
	var cur := str(state.get("target_id", ""))
	var rows: Array = []
	for ob in state.get("objects", []):
		if not (ob is Dictionary) or not (ob.get("pos") is Vector2):
			continue
		var id := str(ob.get("id", ""))
		var kind := str(ob.get("kind", ""))
		if id == "" or not (kind in ["ship", "player", "ai", "hostile"]):
			continue
		if str(ob.get("stance", "")) == "friendly":
			continue
		rows.append({"id": id, "d": me.distance_squared_to(ob["pos"])})
	if rows.is_empty():
		show_toast("Нет целей рядом")
		return
	rows.sort_custom(func(a: Dictionary, b: Dictionary) -> bool: return float(a["d"]) < float(b["d"]))
	var pick := str(rows[0]["id"])
	if cur != "":
		for i in rows.size():
			if str(rows[i]["id"]) == cur:
				pick = str(rows[(i + 1) % rows.size()]["id"])
				break
	gateway.set_target(pick)


func open_hud_editor() -> void:
	if _hud_editor != null and is_instance_valid(_hud_editor):
		return
	_set_menu_open(false)
	var ed: Control = HudEditor.new()
	ed.name = "HudEditor"
	_hud_editor = ed
	add_child(ed)
	ed.call("open", self)


func hud_editor_doc() -> Dictionary:
	return _layout_doc


func hud_editor_set_editing(on: bool) -> void:
	_editing = on
	if not on:
		_hud_editor = null


func hud_editor_set_doc(doc: Dictionary) -> void:
	_layout_doc = LayoutStore.sanitize(doc)
	LayoutStore.save_doc(_layout_doc)
	_layout_force = true
	_reflow_layout()
	_layout_force = false


func hud_editor_nodes() -> Dictionary:
	return _layout_nodes


func hud_editor_safe() -> Rect2:
	var safe := get_node("%SafeArea") as Control
	return safe.get_global_rect()


func hud_editor_lift(id: String) -> void:
	_lift_layout(id)


func hud_editor_move(id: String, global_pos: Vector2) -> void:
	var node := _layout_nodes.get(id) as Control
	if node == null:
		return
	_lift_layout(id)
	node.global_position = _clamp_layout_pos(node, global_pos)


func _lift_layout(id: String) -> void:
	if _layout_lifted.has(id):
		return
	var node := _layout_nodes.get(id) as Control
	if node == null or _layout_free == null:
		return
	var gp := node.global_position
	node.reparent(_layout_free, false)
	node.set_anchors_preset(Control.PRESET_TOP_LEFT)
	node.grow_horizontal = Control.GROW_DIRECTION_END
	node.grow_vertical = Control.GROW_DIRECTION_END
	node.global_position = gp
	_layout_lifted[id] = true


func _restore_layout_home(id: String) -> void:
	if not _layout_lifted.has(id):
		var node0 := _layout_nodes.get(id) as Control
		var home0: Dictionary = _layout_homes.get(id, {})
		if node0 != null and not home0.is_empty():
			node0.self_modulate = home0["self_modulate"]
			node0.scale = home0["scale"]
			node0.visible = bool(home0["visible"])
		return
	var node := _layout_nodes.get(id) as Control
	var home: Dictionary = _layout_homes.get(id, {})
	if node == null or home.is_empty():
		return
	var parent := home["parent"] as Node
	if parent == null or not is_instance_valid(parent):
		return
	node.reparent(parent, false)
	parent.move_child(node, mini(int(home["index"]), parent.get_child_count() - 1))
	node.anchor_left = float(home["anchor_left"])
	node.anchor_top = float(home["anchor_top"])
	node.anchor_right = float(home["anchor_right"])
	node.anchor_bottom = float(home["anchor_bottom"])
	node.offset_left = float(home["offset_left"])
	node.offset_top = float(home["offset_top"])
	node.offset_right = float(home["offset_right"])
	node.offset_bottom = float(home["offset_bottom"])
	node.grow_horizontal = int(home["grow_horizontal"])
	node.grow_vertical = int(home["grow_vertical"])
	node.position = home["position"]
	node.scale = home["scale"]
	node.self_modulate = home["self_modulate"]
	node.visible = bool(home["visible"])
	node.custom_minimum_size = home["min"]
	node.z_index = 0
	_layout_lifted.erase(id)


func _enforce_user_hides() -> void:
	for id in _layout_nodes.keys():
		var item := _layout_item(id)
		if item.has("visible") and not bool(item["visible"]):
			var node := _layout_nodes[id] as Control
			if node != null:
				node.visible = false


func _reflow_layout() -> void:
	if _editing and not _layout_force:
		return
	if _layout_nodes.is_empty():
		return
	var items := LayoutStore.active_items(_layout_doc)
	var safe := hud_editor_safe()
	for id in _layout_nodes.keys():
		var node := _layout_nodes[id] as Control
		if node == null:
			continue
		var raw: Variant = items.get(id, {})
		if not (raw is Dictionary) or (raw as Dictionary).is_empty():
			if _layout_lifted.has(id):
				_restore_layout_home(id)
			continue
		var item: Dictionary = raw
		if item.has("nx"):
			_lift_layout(id)
			var pos := safe.position + Vector2(float(item["nx"]), float(item.get("ny", 0.0))) * safe.size
			node.global_position = _clamp_layout_pos(node, pos)
		elif _layout_lifted.has(id):
			_restore_layout_home(id)
		if item.has("scale"):
			node.scale = Vector2.ONE * float(item["scale"])
		if item.has("opacity"):
			node.self_modulate.a = float(item["opacity"])
		if item.has("z"):
			node.z_index = int(item["z"])
		if item.has("visible"):
			node.visible = bool(item["visible"])


func _clamp_layout_pos(node: Control, pos: Vector2) -> Vector2:
	var safe := hud_editor_safe()
	var sz := node.size * node.scale.abs()
	return Vector2(
		clampf(pos.x, safe.position.x, maxf(safe.position.x, safe.end.x - sz.x)),
		clampf(pos.y, safe.position.y, maxf(safe.position.y, safe.end.y - sz.y)))

