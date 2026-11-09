extends Control
## Окно чата поверх игры (R-61): на весь экран или наполовину, с полупрозрачным фоном, чтобы можно было и писать, и читать чат,
## и видеть игру на заднем плане. Корень окна не перехватывает касания (мир под ним остаётся управляемым), перехватывает только панель.
## «Половина» — верхняя половина экрана: экранная клавиатура на телефоне закрывает нижнюю часть, и поле ввода не должно оказаться под ней.
## В полноэкранном режиме панель поднимается над клавиатурой. Сама панель чата (ui/chat_panel.gd) переносится сюда из карточки HUD
## и возвращается обратно при закрытии: история и счётчики непрочитанных остаются общими.

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")
const Icons := preload("res://ui/icons.gd")

signal closed

enum Mode { HALF, FULL }

const MARGIN := 10.0
const HALF_SHARE := 0.5
const MIN_HEIGHT := 220.0
const BG_ALPHA := 0.74   # фон полупрозрачный: игра остаётся видна, но карточки интерфейса под окном не мешают читать

var mode: int = Mode.HALF
var _panel: PanelContainer
var _slot: VBoxContainer
var _title: Label
var _mode_button: ActionButton
var _chat: Control
var _origin: Node
var _origin_index := 0


## Прямоугольник панели в координатах экрана. kb — высота экранной клавиатуры в тех же единицах (0 — клавиатуры нет).
static func panel_rect(viewport: Vector2, m: int, kb: float = 0.0) -> Rect2:
	var bottom := viewport.y - kb
	if m == Mode.HALF:
		bottom = minf(viewport.y * HALF_SHARE, bottom)
	bottom = maxf(bottom, MARGIN + MIN_HEIGHT)
	bottom = minf(bottom, viewport.y - MARGIN)
	return Rect2(MARGIN, MARGIN, maxf(viewport.x - MARGIN * 2.0, 0.0), maxf(bottom - MARGIN, 0.0))


## Высота клавиатуры в единицах окна игры: пиксели экрана делятся на масштаб растяжения.
static func keyboard_height(kb_px: float, window_h: float, viewport_h: float) -> float:
	if kb_px <= 0.0 or window_h <= 0.0 or viewport_h <= 0.0:
		return 0.0
	return kb_px / (window_h / viewport_h)


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_IGNORE   # касания вне панели достаются игре
	_panel = PanelContainer.new()
	_panel.name = "Panel"
	_panel.mouse_filter = Control.MOUSE_FILTER_STOP
	var sb := StyleBoxFlat.new()
	sb.bg_color = Color(Tokens.BG, BG_ALPHA)
	sb.set_corner_radius_all(Tokens.RADIUS)
	sb.border_color = Color(Tokens.ACCENT, 0.35)
	sb.set_border_width_all(1)
	sb.set_content_margin_all(Tokens.SPACE_SM)
	sb.anti_aliasing = true
	_panel.add_theme_stylebox_override("panel", sb)
	add_child(_panel)
	var col := VBoxContainer.new()
	col.add_theme_constant_override("separation", 6)
	_panel.add_child(col)
	var head := HBoxContainer.new()
	head.add_theme_constant_override("separation", 8)
	col.add_child(head)
	head.add_child(Icons.rect("mail", 22, Tokens.ACCENT))
	_title = Label.new()
	_title.text = "Чат"
	_title.theme_type_variation = &"TitleLabel"
	_title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	head.add_child(_title)
	_mode_button = ActionButton.new()
	_mode_button.name = "ModeButton"
	_mode_button.custom_minimum_size = Vector2(150, 44)
	_mode_button.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	_mode_button.pressed.connect(func() -> void: set_mode(Mode.FULL if mode == Mode.HALF else Mode.HALF))
	head.add_child(_mode_button)
	var close := ActionButton.new()
	close.name = "CloseButton"
	close.text = "Закрыть"
	Icons.apply(close, "x")
	close.custom_minimum_size = Vector2(140, 44)
	close.add_theme_font_size_override("font_size", Tokens.FONT_SMALL)
	close.pressed.connect(close_overlay)
	head.add_child(close)
	_slot = VBoxContainer.new()
	_slot.name = "Slot"
	_slot.size_flags_vertical = Control.SIZE_EXPAND_FILL
	col.add_child(_slot)
	_sync_button()
	_apply_layout()


## Показывает окно: панель чата переносится из карточки HUD сюда.
func open(chat: Control, m: int = Mode.HALF) -> void:
	_chat = chat
	_origin = chat.get_parent()
	_origin_index = chat.get_index()
	mode = m
	chat.reparent(_slot, false)
	chat.size_flags_vertical = Control.SIZE_EXPAND_FILL
	chat.visible = true
	if chat.has_method("set_overlay"):
		chat.set_overlay(true)
	_sync_button()
	_apply_layout()


func set_mode(m: int) -> void:
	mode = m
	_sync_button()
	_apply_layout()


## Закрывает окно и возвращает панель чата в карточку HUD.
func close_overlay() -> void:
	if _chat != null and is_instance_valid(_chat):
		if _chat.has_method("set_overlay"):
			_chat.set_overlay(false)
		_chat.size_flags_vertical = Control.SIZE_FILL
		if _origin != null and is_instance_valid(_origin):
			_chat.reparent(_origin, false)
			_origin.move_child(_chat, mini(_origin_index, _origin.get_child_count() - 1))
	_chat = null
	closed.emit()


func _exit_tree() -> void:
	# если окно убрали без закрытия, панель чата не должна пропасть вместе с ним
	if _chat != null and is_instance_valid(_chat) and _origin != null and is_instance_valid(_origin) and _chat.get_parent() == _slot:
		close_overlay()


func _sync_button() -> void:
	if _mode_button != null:
		_mode_button.text = "На весь экран" if mode == Mode.HALF else "Половина"


func _apply_layout() -> void:
	if _panel == null:
		return
	var vp := get_viewport_rect().size
	if vp.x <= 0.0:
		vp = Vector2(1280, 720)
	var kb := keyboard_height(float(DisplayServer.virtual_keyboard_get_height()), float(DisplayServer.window_get_size().y), vp.y)
	var r := panel_rect(vp, mode, kb)
	_panel.set_anchors_preset(Control.PRESET_TOP_LEFT)
	_panel.position = r.position
	_panel.size = r.size


func _process(_delta: float) -> void:
	_apply_layout()   # клавиатура выезжает и убирается: панель следует за ней
