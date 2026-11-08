extends PanelContainer
## Сворачиваемая карточка. В сцене: PanelContainer с этим скриптом > VBoxContainer (содержимое).
## Заголовок и кнопка «Свернуть/Развернуть» вставляются в начало VBoxContainer.
## Открытие анимируется за ANIM_OPEN_MS (если не включено уменьшение анимаций).

const Tokens := preload("res://ui/theme/tokens.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

signal toggled(collapsed: bool)

@export var title: String = "":
	set(v):
		title = v
		if _title_label != null:
			_title_label.text = v

var collapsed: bool = false

var _box: VBoxContainer
var _header: HBoxContainer
var _title_label: Label
var _button: Button
var _tween: Tween


func _init() -> void:
	theme_type_variation = &"CardPanel"


func _ready() -> void:
	for c in get_children():
		if c is VBoxContainer:
			_box = c
			break
	assert(_box != null, "CollapsibleCard: нужен дочерний VBoxContainer")
	_header = HBoxContainer.new()
	_header.name = "CardHeader"
	_title_label = Label.new()
	_title_label.theme_type_variation = &"SectionLabel"
	_title_label.text = title
	_title_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_header.add_child(_title_label)
	_button = Button.new()
	_button.theme_type_variation = &"FlatButton"
	_button.custom_minimum_size = Vector2(0, 48)  # удобная область касания
	_button.focus_mode = Control.FOCUS_ALL
	_button.pressed.connect(func() -> void: set_collapsed(not collapsed))
	_header.add_child(_button)
	_box.add_child(_header)
	_box.move_child(_header, 0)
	_apply(false)


func set_collapsed(value: bool, animate: bool = true) -> void:
	if value == collapsed:
		return
	collapsed = value
	_apply(animate and not value)
	toggled.emit(collapsed)


func _apply(animate_open: bool) -> void:
	_button.text = "Развернуть" if collapsed else "Свернуть"
	_button.tooltip_text = "Развернуть «%s»" % title if collapsed else "Свернуть «%s»" % title
	for c in _box.get_children():
		if c == _header or not (c is CanvasItem):
			continue
		c.visible = not collapsed
	if _tween != null and _tween.is_valid():
		_tween.kill()
	if animate_open and not UiSettings.reduce_motion():
		_tween = create_tween().set_parallel(true)
		for c in _box.get_children():
			if c != _header and c is CanvasItem:
				c.modulate.a = 0.0
				_tween.tween_property(c, "modulate:a", 1.0, Tokens.ANIM_OPEN_MS / 1000.0)
	else:
		for c in _box.get_children():
			if c != _header and c is CanvasItem:
				c.modulate.a = 1.0
