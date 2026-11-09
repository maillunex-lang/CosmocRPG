extends VBoxContainer
## Панель тревог и коротких уведомлений. Боевые предупреждения (set_alert) остаются на экране,
## пока условие не снято (clear_alert); обычные уведомления (toast) исчезают сами.
## Панель не перехватывает нажатия: PvP и управление продолжаются.

const Tokens := preload("res://ui/theme/tokens.gd")

const TOAST_SECONDS := 3.5
const MAX_TOASTS := 4

var _alerts: Dictionary = {}  # id -> PanelContainer


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_theme_constant_override("separation", Tokens.SPACE_SM)


func alert_ids() -> Array:
	return _alerts.keys()


func has_alert(id: String) -> bool:
	return _alerts.has(id)


func _make_panel(text: String, danger: bool) -> PanelContainer:
	var p := PanelContainer.new()
	p.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var s := StyleBoxFlat.new()
	s.bg_color = Color(Tokens.BG, 0.88)
	s.border_color = Tokens.DANGER if danger else Color(Tokens.ACCENT, 0.5)
	s.set_border_width_all(2 if danger else 1)
	s.set_corner_radius_all(Tokens.RADIUS)
	s.content_margin_left = Tokens.SPACE_LG
	s.content_margin_right = Tokens.SPACE_LG
	s.content_margin_top = Tokens.SPACE_SM
	s.content_margin_bottom = Tokens.SPACE_SM
	p.add_theme_stylebox_override("panel", s)
	var l := Label.new()
	l.text = text
	l.theme_type_variation = &"DangerLabel" if danger else &"Label"
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.mouse_filter = Control.MOUSE_FILTER_IGNORE
	p.add_child(l)
	return p


## Постоянное предупреждение (повторный вызов с тем же id обновляет текст).
func set_alert(id: String, text: String) -> void:
	if _alerts.has(id):
		(_alerts[id].get_child(0) as Label).text = text
		return
	var p := _make_panel(text, true)
	_alerts[id] = p
	add_child(p)
	move_child(p, 0)  # тревоги выше обычных уведомлений


func clear_alert(id: String) -> void:
	if not _alerts.has(id):
		return
	_alerts[id].queue_free()
	_alerts.erase(id)


## Кратковременное уведомление.
func toast(text: String) -> void:
	var toasts := get_children().filter(func(c: Node) -> bool: return c.has_meta("toast"))
	while toasts.size() >= MAX_TOASTS:
		toasts.pop_front().queue_free()
	var p := _make_panel(text, false)
	p.set_meta("toast", true)
	add_child(p)
	# A bound Node method disconnects when the panel is freed (screen close or
	# toast limit). A lambda captures the freed panel before its validity guard.
	get_tree().create_timer(TOAST_SECONDS).timeout.connect(p.queue_free)
