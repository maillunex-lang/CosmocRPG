extends CanvasLayer
## Экран «Подготовка космоса»: закрывает первые секунды после входа в мир, пока сцена прогревается
## (компиляция шейдеров и конвейеров, загрузка моделей и текстур). Игра при этом уже запущена и
## рисует мир под экраном, поэтому микрозаморозки происходят за ним.
## Закрывается по факту: когда кадры стали ровными (все последние кадры быстрее порога),
## но не раньше минимального времени и не позже максимального. Ничего не имитирует: время
## определяется реальной стабильностью кадров.

const Tokens := preload("res://ui/theme/tokens.gd")

signal finished(seconds: float)

const MIN_TIME := 1.2          # не показывать мельком
const MAX_TIME := 5.0          # не держать дольше, даже если кадры неровные
const STABLE_FRAMES := 24      # столько подряд ровных кадров нужно для завершения
const STABLE_DT := 0.034       # «ровный» кадр: быстрее 34 мс
const FADE := 0.25
const STATUSES: Array[String] = [
	"Подготовка космоса…",
	"Загрузка кораблей и станций…",
	"Подготовка графики…",
	"Калибровка камеры…",
]

var _root: Control
var _bar: ColorRect
var _status: Label
var _t := 0.0
var _stable := 0
var _done := false
var _fade := 0.0


func _ready() -> void:
	layer = 100
	_root = Control.new()
	_root.set_anchors_preset(Control.PRESET_FULL_RECT)
	_root.mouse_filter = Control.MOUSE_FILTER_STOP   # касания не проходят в мир, пока идёт подготовка
	add_child(_root)
	var bg := ColorRect.new()
	bg.color = Tokens.BG
	bg.set_anchors_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_root.add_child(bg)
	var box := VBoxContainer.new()
	box.set_anchors_preset(Control.PRESET_CENTER)
	box.grow_horizontal = Control.GROW_DIRECTION_BOTH
	box.grow_vertical = Control.GROW_DIRECTION_BOTH
	box.alignment = BoxContainer.ALIGNMENT_CENTER
	box.add_theme_constant_override("separation", 14)
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_root.add_child(box)
	var title := Label.new()
	title.text = "ЗВЁЗДНЫЕ РУБЕЖИ"
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	title.add_theme_font_size_override("font_size", 40)
	title.add_theme_color_override("font_color", Tokens.ACCENT)
	box.add_child(title)
	_status = Label.new()
	_status.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_status.add_theme_color_override("font_color", Tokens.TEXT_MUTED)
	box.add_child(_status)
	var track := Control.new()
	track.custom_minimum_size = Vector2(320, 4)
	track.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	track.mouse_filter = Control.MOUSE_FILTER_IGNORE
	track.clip_contents = true
	var tbg := ColorRect.new()
	tbg.color = Color(Tokens.ACCENT, 0.15)
	tbg.set_anchors_preset(Control.PRESET_FULL_RECT)
	tbg.mouse_filter = Control.MOUSE_FILTER_IGNORE
	track.add_child(tbg)
	_bar = ColorRect.new()
	_bar.color = Tokens.ACCENT
	_bar.size = Vector2(110, 4)
	_bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
	track.add_child(_bar)
	box.add_child(track)
	_update_visuals()


func _process(delta: float) -> void:
	if _root == null:
		return
	if _done:
		_fade += delta
		_root.modulate.a = clampf(1.0 - _fade / FADE, 0.0, 1.0)
		if _fade >= FADE:
			queue_free()
		return
	_t += delta
	_stable = _stable + 1 if delta < STABLE_DT else 0
	_update_visuals()
	if _t >= MAX_TIME or (_t >= MIN_TIME and _stable >= STABLE_FRAMES):
		_done = true
		_root.mouse_filter = Control.MOUSE_FILTER_IGNORE
		finished.emit(_t)


func _update_visuals() -> void:
	_status.text = STATUSES[int(_t / 0.7) % STATUSES.size()]
	var travel := 320.0 + 110.0
	_bar.position.x = fposmod(_t * 380.0, travel) - 110.0


func is_done() -> bool:
	return _done
