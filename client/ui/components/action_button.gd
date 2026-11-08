extends Button
## Кнопка действия: варианты оформления, защита от повторного нажатия на время
## длительной операции и состояние «Скоро» для ещё не реализованных возможностей
## (никаких декоративных кнопок, которые выглядят рабочими, но ничего не делают).

const Tokens := preload("res://ui/theme/tokens.gd")

enum Variant { SECONDARY, PRIMARY, DANGER }

@export var variant: Variant = Variant.SECONDARY:
	set(v):
		variant = v
		_apply_variant()

## Функция ещё не реализована: кнопка неактивна и помечена «скоро».
@export var soon: bool = false:
	set(v):
		if v == soon:
			return
		soon = v
		_apply_soon()

const SOON_SUFFIX := " · скоро"
const SOON_HINT := "Скоро: функция ещё не реализована"

var _busy := false
var _saved_text := ""
var _saved_disabled := false
var _soon_base := ""


func _ready() -> void:
	custom_minimum_size.y = maxf(custom_minimum_size.y, Tokens.TOUCH_MIN)
	focus_mode = Control.FOCUS_ALL
	_apply_variant()
	if soon:
		_apply_soon()


func _apply_variant() -> void:
	match variant:
		Variant.PRIMARY:
			theme_type_variation = &"PrimaryButton"
		Variant.DANGER:
			theme_type_variation = &"DangerButton"
		_:
			theme_type_variation = &""


func _apply_soon() -> void:
	if soon:
		_soon_base = text
		text = _soon_base + SOON_SUFFIX
		tooltip_text = SOON_HINT
		disabled = true
	else:
		text = _soon_base
		tooltip_text = ""
		disabled = false


func is_busy() -> bool:
	return _busy


## Начало длительной операции: кнопка блокируется, повторные нажатия игнорируются.
func begin_busy(busy_text: String = "…") -> void:
	if _busy:
		return
	_busy = true
	_saved_text = text
	_saved_disabled = disabled
	text = busy_text
	disabled = true


## Конец операции: возвращается прежний текст и состояние.
func end_busy() -> void:
	if not _busy:
		return
	_busy = false
	text = _saved_text
	disabled = _saved_disabled
