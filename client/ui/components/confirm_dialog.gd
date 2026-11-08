extends Control
## Модальное окно подтверждения (опасные действия, покупки, атака нейтрального игрока).
## Использование:  var ok: bool = await dialog.ask("Заголовок", "Текст", "Подтвердить")
## Игра за окном продолжается: окно не ставит дерево на паузу (PvP не останавливается).

const Tokens := preload("res://ui/theme/tokens.gd")
const ActionButton := preload("res://ui/components/action_button.gd")

signal answered(confirmed: bool)

var _title: Label
var _message: Label
var _confirm: Button
var _cancel: Button
var _open := false


func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP  # блокирует нажатия под окном
	visible = false

	var dim := ColorRect.new()
	dim.color = Color(Tokens.BG, 0.72)
	dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(dim)

	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(center)

	var panel := PanelContainer.new()
	panel.theme_type_variation = &"RaisedPanel"
	panel.custom_minimum_size = Vector2(520, 0)
	center.add_child(panel)

	var box := VBoxContainer.new()
	panel.add_child(box)
	_title = Label.new()
	_title.theme_type_variation = &"SectionLabel"
	_title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_title)
	_message = Label.new()
	_message.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(_message)
	var row := HBoxContainer.new()
	box.add_child(row)
	_cancel = ActionButton.new()
	_cancel.text = "Отмена"
	_cancel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_cancel.pressed.connect(func() -> void: _close(false))
	row.add_child(_cancel)
	_confirm = ActionButton.new()
	_confirm.variant = ActionButton.Variant.DANGER
	_confirm.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_confirm.pressed.connect(func() -> void: _close(true))
	row.add_child(_confirm)


func is_open() -> bool:
	return _open


## Показывает окно и ждёт ответа. Если окно уже открыто, возвращает false (повторный вызов игнорируется).
func ask(title: String, message: String, confirm_text: String = "Подтвердить") -> bool:
	if _open:
		return false
	_open = true
	_title.text = title
	_message.text = message
	_confirm.text = confirm_text
	visible = true
	_cancel.grab_focus()  # по умолчанию «Отмена»: случайное нажатие Enter не подтверждает опасное действие
	return await answered


func _close(confirmed: bool) -> void:
	if not _open:
		return
	_open = false
	visible = false
	answered.emit(confirmed)


func _unhandled_key_input(event: InputEvent) -> void:
	if _open and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		_close(false)
