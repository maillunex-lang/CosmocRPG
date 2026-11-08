extends Control
## HUD космоса (решение UI-02). Только отображение и ввод: все числа приходят из шлюза данных,
## команды уходят в шлюз. Никаких расчётов боя, энергии и наград на клиенте.
##
## Схема (Landscape): сверху слева профиль и состояние корабля, под ним задания и чат;
## сверху по центру тревоги и плашка «Демо»; сверху справа валюты и настройки; справа внизу цель,
## атака, способности, автопилот; снизу слева джойстик; снизу по центру «Карта», «Флот», «Меню».
## В бою задания и чат сворачиваются, тревоги остаются.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

signal exit_requested

const BUSY_TIMEOUT := 2.0

var gateway: UiGateway
var _last_state: Dictionary = {}
var _skill_buttons: Array = []
var _pref_collapsed := {"quests": false, "chat": false}  # выбор игрока (вне боя)
var _auto_collapsing := false
var _was_combat := false
var _joy := Vector2.ZERO
var _sent_move := Vector2.ZERO
var _quests_sig := ""
var _chat_sig := ""
var _attack_expect: Variant = null  # ожидаемое значение attacking после нажатия (null: не ждём)

@onready var _joystick: Control = %Joystick
@onready var _bottom_nav: Control = %BottomNav

@onready var _player_name: Label = %PlayerName
@onready var _player_level: Label = %PlayerLevel
@onready var _ship_name: Label = %ShipName
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
@onready var _target_name: Label = %TargetName
@onready var _target_info: Label = %TargetInfo
@onready var _target_hull: ProgressBar = %TargetHull
@onready var _attack: ActionButton = %AttackButton
@onready var _skills_grid: GridContainer = %SkillsGrid
@onready var _autopilot: ActionButton = %AutopilotButton
@onready var _auto_skills: ActionButton = %AutoSkillsButton
@onready var _confirm: Control = %Confirm


func _ready() -> void:
	# компактность: шрифт кнопок нижней панели чуть меньше
	for b in _bottom_nav.get_children():
		b.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 2)
	%SettingsButton.add_theme_font_size_override("font_size", Tokens.FONT_BODY - 2)
	# чат по умолчанию свёрнут: космос занимает основную площадь
	_pref_collapsed["chat"] = true
	_chat_card.set_collapsed(true, false)

	_hull.fill_color = Tokens.SUCCESS
	_shield.fill_color = Tokens.ACCENT
	_energy.fill_color = Tokens.ACCENT_2
	_target_hull.fill_color = Tokens.DANGER

	for b in [_attack, _autopilot, _auto_skills]:
		b.disabled = true
	_attack.variant = ActionButton.Variant.DANGER
	_build_skills()

	_joystick.visible = UiSettings.joystick_enabled()
	_joystick.vector_changed.connect(func(v: Vector2) -> void: _joy = v)

	_exit_button.pressed.connect(func() -> void: exit_requested.emit())
	_system_button.pressed.connect(func() -> void: if gateway != null and gateway.has_method("demo_next_system"): gateway.demo_next_system())
	_attack.pressed.connect(_on_attack_pressed)
	_autopilot.toggled.connect(func(on: bool) -> void: if gateway != null: gateway.set_autopilot(on))
	_auto_skills.toggled.connect(func(on: bool) -> void: if gateway != null: gateway.set_auto_skills(on))
	_quests_card.toggled.connect(func(c: bool) -> void: _on_card_toggled("quests", c))
	_chat_card.toggled.connect(func(c: bool) -> void: _on_card_toggled("chat", c))
	_set_target_empty()


## Привязка к шлюзу данных (демо или сервер).
func bind(gw: UiGateway) -> void:
	gateway = gw
	gw.state_updated.connect(apply_state)
	gw.alert_raised.connect(func(id: String, text: String) -> void: _alerts.set_alert(id, text))
	gw.alert_cleared.connect(func(id: String) -> void: _alerts.clear_alert(id))
	gw.notification.connect(func(text: String) -> void: _alerts.toast(text))
	gw.command_failed.connect(_on_command_failed)
	_demo_bar.visible = gw.is_demo()
	apply_state(gw.snapshot())


func _build_skills() -> void:
	for i in 4:
		var b := ActionButton.new()
		b.text = "Навык %d" % (i + 1)
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		b.add_theme_font_size_override("font_size", Tokens.FONT_BODY)
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
	if not total.is_equal_approx(_sent_move):
		_sent_move = total
		gateway.move_input(total)


# --- отображение состояния ---

func apply_state(state: Dictionary) -> void:
	_last_state = state
	var player: Dictionary = state.get("player", {})
	_player_name.text = str(player.get("name", ""))
	_player_level.text = "Ур. %d" % int(player.get("level", 1))
	var ship: Dictionary = state.get("ship", {})
	_ship_name.text = str(ship.get("name", ""))
	_hull.set_values(ship.get("hull", 0.0), ship.get("hull_max", 1.0))
	_shield.set_values(ship.get("shield", 0.0), ship.get("shield_max", 1.0))
	_energy.set_values(ship.get("energy", 0.0), ship.get("energy_max", 1.0))
	_credits.set_value(int(state.get("credits", 0)))

	var target := _target_object(state)
	_apply_target(state, target)
	_apply_buttons(state, target, ship)
	_apply_combat_layout(bool(state.get("combat", {}).get("in_combat", false)))
	_apply_lists(state)


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
	return "Объект"


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
	var rng := "в зоне огня" if state.get("target_in_range", false) else "вне дальности"
	_target_info.text = "%s · %s · %.1f км · %s" % [kind_label(target), stance_label(str(target["stance"])), dist, rng]
	_target_hull.visible = true
	_target_hull.set_values(target["hull"], target["hull_max"])


func _apply_buttons(state: Dictionary, target: Dictionary, ship: Dictionary) -> void:
	var attacking: bool = bool(state.get("attacking", false))
	var has_target := not target.is_empty()
	var friendly := has_target and str(target["stance"]) == "friendly"

	# кнопка атаки: «Прекратить» во время атаки; busy снимается, когда состояние подтвердило команду
	if _attack.is_busy() and _attack_expect != null and attacking == _attack_expect:
		_attack.end_busy()
		_attack_expect = null
	if not _attack.is_busy():
		_attack.text = "Прекратить атаку" if attacking else "Атаковать"
		_attack.variant = ActionButton.Variant.SECONDARY if attacking else ActionButton.Variant.DANGER
		_attack.disabled = not attacking and (not has_target or friendly)

	for i in _skill_buttons.size():
		var b: ActionButton = _skill_buttons[i]
		var skills: Array = state.get("skills", [])
		if i >= skills.size():
			b.disabled = true
			continue
		var s: Dictionary = skills[i]
		var cd: float = s["cooldown_left"]
		b.text = ("%s · %d с" % [s["name"], int(ceil(cd))]) if cd > 0.0 else ("%s · %d" % [s["name"], int(s["energy_cost"])])
		var blocked: bool = cd > 0.0 or float(ship.get("energy", 0.0)) < float(s["energy_cost"])
		if s["needs_target"]:
			blocked = blocked or not has_target or friendly or not state.get("target_in_range", false)
		b.disabled = blocked
		b.tooltip_text = "Энергия: %d, перезарядка: %d с" % [int(s["energy_cost"]), int(s["cooldown"])]

	_autopilot.disabled = not has_target and not state.get("autopilot", false)
	_set_toggle(_autopilot, "Автопилот", bool(state.get("autopilot", false)))
	_auto_skills.disabled = false
	_set_toggle(_auto_skills, "Авто-навыки", bool(state.get("auto_skills", false)))


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
			var l := Label.new()
			l.text = "%s (%d/%d)" % [q["title"], int(q["progress"]), int(q["goal"])]
			l.theme_type_variation = &"MutedLabel"
			l.add_theme_color_override("font_color", Tokens.TEXT)
			l.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
			l.custom_minimum_size.x = 260
			l.tooltip_text = l.text
			_quests_list.add_child(l)
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
	l.custom_minimum_size.x = 260
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
