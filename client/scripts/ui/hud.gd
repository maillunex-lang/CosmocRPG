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
signal quest_log_requested   # карточка «Задания»: описание и маршрут
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
var _attack_expect: Variant = null  # ожидаемое значение attacking после нажатия (null: не ждём)

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
	# чат по умолчанию свёрнут: космос занимает основную площадь
	_pref_collapsed["chat"] = true
	_chat_card.set_collapsed(true, false)

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

	# иконки боевого кластера и верхнего правого угла: интерфейс легче читается
	Icons.apply(_attack, "zap", Tokens.DANGER, 22)
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
	%SettingsMenuButton.pressed.connect(func() -> void: _set_menu_open(false); settings_requested.emit())
	Icons.apply(%SettingsMenuButton, "settings", Tokens.ACCENT, 22)
	_build_cargo_ui()
	_confirm.opened.connect(func() -> void: modal_opened.emit())
	_exit_button.pressed.connect(func() -> void: exit_requested.emit())
	_system_button.pressed.connect(func() -> void: if gateway != null and gateway.has_method("demo_next_system"): gateway.demo_next_system())
	_attack.pressed.connect(_on_attack_pressed)
	_autopilot.toggled.connect(func(on: bool) -> void: if gateway != null: gateway.set_autopilot(on))
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
	_chat_card.toggled.connect(func(c: bool) -> void: _on_card_toggled("chat", c))
	_set_target_empty()


## Окно чата внутри карточки «Чат» (на сервере); в демо остаются демонстрационные строки.
func _setup_chat(gw: UiGateway) -> void:
	if chat_panel != null:
		return
	chat_panel = ChatPanel.new()
	chat_panel.name = "ChatPanel"
	_chat_lines.get_parent().add_child(chat_panel)
	chat_panel.bind(gw)
	_chat_lines.visible = not gw.has_chat()
	chat_panel.visible = gw.has_chat()
	chat_panel.unread_changed.connect(func(n: int) -> void:
		_chat_card.title = "Чат" if n <= 0 else "Чат · %d" % n)
	chat_panel.expand_requested.connect(func() -> void: chat_requested.emit())


## Короткое сообщение игроку (всплывающая плашка).
func show_toast(text: String) -> void:
	_alerts.toast(text)


## Меню (R-35): полупрозрачная кнопка справа сверху раскрывает панель действий.
func _toggle_menu() -> void:
	_set_menu_open(not _menu_popup.visible)


func _set_menu_open(open: bool) -> void:
	_menu_popup.visible = open


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
	if gw.has_chat():
		gw.request_chat_history()   # история запрашивается при входе, до появления панели: запрашиваем ещё раз уже для неё
	apply_state(gw.snapshot())
	if gw.has_signal("server_updated"):
		gw.server_updated.connect(_on_server_updated)


## Груз и кнопки добычи/продажи: появляются, только если корабль может добывать или везёт груз.
func _build_cargo_ui() -> void:
	_cargo_row = HBoxContainer.new()
	_cargo_row.add_theme_constant_override("separation", 6)
	_cargo_row.add_child(Icons.rect("package", 20, Tokens.ACCENT))
	_cargo_label = Label.new()
	_cargo_label.name = "CargoLabel"
	_cargo_row.add_child(_cargo_label)
	_cargo_row.visible = false
	var sp := _speed_label.get_parent()
	sp.add_child(_cargo_row)
	sp.move_child(_cargo_row, _speed_label.get_index() + 1)
	var stack := _attack.get_parent()
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
	stack.move_child(_mine_button, _attack.get_index() + 1)
	stack.move_child(_sell_button, _attack.get_index() + 2)


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
	_cargo_label.text = "Груз %d/%d" % [int(round(used)), int(round(cap))]
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


func _build_skills() -> void:
	for i in 4:
		var b := ActionButton.new()
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
		b.pressed.connect(func() -> void: if gateway != null: gateway.use_skill(slot))
		_skills_grid.add_child(b)
		_skill_buttons.append(b)


func _process(_delta: float) -> void:
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


# --- отображение состояния ---

func apply_state(state: Dictionary) -> void:
	_last_state = state
	var player: Dictionary = state.get("player", {})
	_player_name.text = str(player.get("name", ""))
	_player_level.visible = player.has("level")   # уровня нет в данных: не выдумываем
	_player_level.text = "Ур. %d" % int(player.get("level", 1)) + ((" · +%d" % int(player["attr_free"])) if int(player.get("attr_free", 0)) > 0 else "")
	_apply_xp(player)
	var ship: Dictionary = state.get("ship", {})
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
	var mode := "Автопилот" if bool(state.get("autopilot", false)) else "Ручное управление"
	_speed_label.text = "%d м/с · %s" % [int(round(_speed_mps)), mode]


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


func _apply_target(state: Dictionary, target: Dictionary) -> void:
	if target.is_empty():
		_set_target_empty()
		return
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
		_attack.text = ("Прекратить атаку" if attacking else "Атаковать") if combat_ok else "Бой · скоро"
		_attack.variant = ActionButton.Variant.SECONDARY if attacking else ActionButton.Variant.DANGER
		_attack.disabled = not combat_ok or (not attacking and (not has_target or friendly))

	for i in _skill_buttons.size():
		var b: ActionButton = _skill_buttons[i]
		var skills: Array = state.get("skills", [])
		if i >= skills.size():
			b.disabled = true
			continue
		var s: Dictionary = skills[i]
		var cd: float = s["cooldown_left"]
		var fill := b.get_node_or_null("CooldownFill") as ColorRect
		if fill != null:
			fill.anchor_right = clampf(cd / maxf(float(s["cooldown"]), 0.001), 0.0, 1.0) if cd > 0.0 else 0.0
			fill.offset_right = 0.0
		b.text = ("%s\n%d с" % [s["name"], int(ceil(cd))]) if cd > 0.0 else ("%s\n%d" % [s["name"], int(s["energy_cost"])])
		var blocked: bool = not bool(s.get("enabled", true)) or bool(ship.get("dead", false)) or cd > 0.0 or float(ship.get("energy", 0.0)) < float(s["energy_cost"])
		if s["needs_target"]:
			blocked = blocked or not has_target or friendly or not state.get("target_in_range", false)
		b.disabled = blocked
		b.tooltip_text = "Энергия: %d, перезарядка: %d с" % [int(s["energy_cost"]), int(s["cooldown"])]
		if not bool(s.get("enabled", true)):
			b.tooltip_text = "Навык не изучен" if str(s.get("unavailable_reason", "")) == "ability_not_learned" else "Требуется установленный модуль"


	_apply_cargo(state, target, ship)
	_autopilot.disabled = not has_target and not state.get("autopilot", false)
	_set_toggle(_autopilot, "Автопилот", bool(state.get("autopilot", false)))
	_auto_skills.disabled = not combat_ok
	_set_toggle(_auto_skills, "Авто" if combat_ok else "Авто · скоро", bool(state.get("auto_skills", false)))


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
		_chat_card.set_collapsed(true, false)
	else:
		_quests_card.set_collapsed(_pref_collapsed["quests"], true)
		_chat_card.set_collapsed(_pref_collapsed["chat"], true)
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
		for q in quests:
			var b := Button.new()
			b.alignment = HORIZONTAL_ALIGNMENT_LEFT
			b.theme_type_variation = &"FlatButton"
			b.text = "%s (%d/%d)" % [q["title"], int(q["progress"]), int(q["goal"])]
			b.clip_text = true
			b.custom_minimum_size = Vector2(0, 48)
			var tip := str(q.get("text", ""))
			b.tooltip_text = tip if tip != "" else b.text
			b.pressed.connect(func() -> void: quest_log_requested.emit())
			_quests_list.add_child(b)
	var chat: Array = state.get("chat", [])
	var csig := JSON.stringify(chat)
	if csig != _chat_sig:
		_chat_sig = csig
		for c in _chat_lines.get_children():
			c.queue_free()
		if chat.is_empty():
			_add_muted(_chat_lines, "Сообщений нет")
		for m in chat.slice(maxi(0, chat.size() - 3)):
			var who: String = ("%s: " % m["author"]) if str(m["author"]) != "" else ""
			_add_muted(_chat_lines, "[%s] %s%s" % [m["channel"], who, m["text"]])


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
	if gateway == null or _attack.is_busy():
		return  # защита от повторного нажатия
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


func _begin_attack_busy(text: String, expect: bool) -> void:
	_attack.begin_busy(text)
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
		_event_label.custom_minimum_size = Vector2(340, 0)
		_event_label.add_theme_font_size_override("font_size", 14)
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
	_event_label.text = "%s · %s\n%s" % [str(e.get("title", "")), when, str(e.get("text", ""))]
