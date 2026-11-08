extends CanvasLayer
## Отладочная панель производительности: реальные интервалы между кадрами (FPS, процентили, зависания)
## и длительность этапов загрузки сцены. Нужна, чтобы по записи экрана на телефоне получать точные цифры
## плавности. Включение: касание тремя пальцами или клавиша F3. Состояние запоминается (настройка debug_overlay).
##
## С 2026-11-08 панель по умолчанию скрыта (решение владельца); включается жестом тремя пальцами или F3.
## (пункт в docs/ROADMAP.md «Перед публикацией»; проверяется тестом frame_stats_test).

const Tokens := preload("res://ui/theme/tokens.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

const DEFAULT_VISIBLE := false
## Ключ настройки. Сменён с debug_overlay (2026-11-08): в старых тестовых сборках у игроков сохранилось «включено», и новое значение по умолчанию не действовало.
const SETTING_KEY := "debug_overlay2"
const RING := 720                 # сколько последних кадров хранится (около 12 с при 60 FPS)
const WINDOW_SEC := 10.0          # окно, по которому считаются показатели
const HITCH_MS := 25.0            # кадр дольше этого считается «заиканием»
const BIG_HITCH_MS := 50.0        # заметный фриз
const STARTUP_SEC := 3.0          # первые секунды после появления экрана анализируются отдельно
const MAX_STARTUP_LIST := 6
const REFRESH_SEC := 0.5
const MAX_WIDTH := 430.0          # панель не шире центральной колонки HUD (не налезает на кнопки справа)

var _ms := PackedFloat32Array()     # кольцо: длительность кадров, мс
var _at := PackedFloat32Array()     # кольцо: время кадра от старта, с
var _head := 0
var _count := 0
var _start_usec := 0
var _last_usec := 0
var _accum := 0.0

var startup_hitches := 0
var startup_max_ms := 0.0
var startup_list: Array = []        # [{t, ms}] самые заметные кадры при старте
var marks: Dictionary = {}          # этап -> длительность, мс

var _panel: PanelContainer
var _label: Label


static func percentile(sorted: PackedFloat32Array, p: float) -> float:
	if sorted.is_empty():
		return 0.0
	var idx := clampi(int(ceil(p * sorted.size())) - 1, 0, sorted.size() - 1)
	return sorted[idx]


func _ready() -> void:
	layer = 100
	_ms.resize(RING)
	_at.resize(RING)
	_start_usec = Time.get_ticks_usec()
	_last_usec = _start_usec
	_panel = PanelContainer.new()
	_panel.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var sb := StyleBoxFlat.new()
	sb.bg_color = Color(Tokens.BG, 0.78)
	sb.border_color = Color(Tokens.RARE, 0.6)
	sb.set_border_width_all(1)
	sb.set_corner_radius_all(6)
	sb.content_margin_left = 10
	sb.content_margin_right = 10
	sb.content_margin_top = 4
	sb.content_margin_bottom = 4
	_panel.add_theme_stylebox_override("panel", sb)
	_label = Label.new()
	_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_label.add_theme_font_size_override("font_size", 14)
	_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_label.custom_minimum_size.x = MAX_WIDTH
	_label.add_theme_color_override("font_color", Tokens.TEXT)
	_panel.add_child(_label)
	add_child(_panel)  # пока не прикреплена к HUD: сама по себе внизу по центру
	_panel.set_anchors_and_offsets_preset(Control.PRESET_CENTER_BOTTOM, Control.PRESET_MODE_MINSIZE)
	_panel.grow_horizontal = Control.GROW_DIRECTION_BOTH
	_panel.grow_vertical = Control.GROW_DIRECTION_BEGIN
	_panel.offset_bottom = -78.0
	_set_shown(UiSettings.get_bool(SETTING_KEY, DEFAULT_VISIBLE))
	_refresh()


## Переносит панель внутрь контейнера HUD: расположением управляет контейнер, поэтому панель не налезает
## на другие блоки при любых размерах экрана и вырезах.
func attach_to(container: Control) -> void:
	if _panel.get_parent() != null:
		_panel.get_parent().remove_child(_panel)
	_panel.set_anchors_preset(Control.PRESET_TOP_LEFT)
	_panel.offset_left = 0.0
	_panel.offset_top = 0.0
	_panel.offset_right = 0.0
	_panel.offset_bottom = 0.0
	_panel.grow_horizontal = Control.GROW_DIRECTION_END
	_panel.grow_vertical = Control.GROW_DIRECTION_END
	_panel.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	container.add_child(_panel)


func _set_shown(v: bool) -> void:
	visible = v
	_panel.visible = v  # панель может жить вне слоя (после attach_to)


func _process(_delta: float) -> void:
	var now := Time.get_ticks_usec()
	record(float(now - _last_usec) / 1000.0, float(now - _start_usec) / 1000000.0)
	_last_usec = now
	_accum += _delta
	if visible and _accum >= REFRESH_SEC:
		_accum = 0.0
		_refresh()


## Записать кадр: длительность в мс и время от старта в секундах.
func record(ms: float, t_sec: float) -> void:
	_ms[_head] = ms
	_at[_head] = t_sec
	_head = (_head + 1) % RING
	_count = mini(_count + 1, RING)
	if t_sec <= STARTUP_SEC:
		if ms > HITCH_MS:
			startup_hitches += 1
			startup_max_ms = maxf(startup_max_ms, ms)
			startup_list.append({"t": t_sec, "ms": ms})
			if startup_list.size() > MAX_STARTUP_LIST:
				startup_list.sort_custom(func(a: Dictionary, b: Dictionary) -> bool: return a["ms"] > b["ms"])
				startup_list.resize(MAX_STARTUP_LIST)


## Длительность этапа загрузки (мс), показывается в панели.
func mark(stage: String, ms: float) -> void:
	marks[stage] = ms
	_refresh()


## Показатели за последние WINDOW_SEC секунд.
func stats() -> Dictionary:
	var newest := _at[(_head - 1 + RING) % RING] if _count > 0 else 0.0
	var vals := PackedFloat32Array()
	var sum := 0.0
	var h25 := 0
	var h50 := 0
	for i in _count:
		var idx := (_head - 1 - i + RING) % RING
		if newest - _at[idx] > WINDOW_SEC:
			break
		var v := _ms[idx]
		vals.append(v)
		sum += v
		if v > HITCH_MS:
			h25 += 1
		if v > BIG_HITCH_MS:
			h50 += 1
	if vals.is_empty():
		return {"n": 0, "fps": 0.0, "p50": 0.0, "p95": 0.0, "p99": 0.0, "max": 0.0, "hitches25": 0, "hitches50": 0}
	vals.sort()
	return {
		"n": vals.size(), "fps": 1000.0 / (sum / vals.size()),
		"p50": percentile(vals, 0.50), "p95": percentile(vals, 0.95), "p99": percentile(vals, 0.99),
		"max": vals[vals.size() - 1], "hitches25": h25, "hitches50": h50,
	}


func text() -> String:
	var s := stats()
	var lines: Array[String] = []
	lines.append("FPS %.1f · кадр p50 %.1f / p95 %.1f / p99 %.1f / макс %.1f мс" % [s["fps"], s["p50"], s["p95"], s["p99"], s["max"]])
	lines.append("За 10 с: кадров >25 мс — %d, >50 мс — %d" % [s["hitches25"], s["hitches50"]])
	lines.append("Старт (%d с): >25 мс — %d, макс %.0f мс" % [int(STARTUP_SEC), startup_hitches, startup_max_ms])
	if not startup_list.is_empty():
		var parts: Array[String] = []
		for e in startup_list:
			parts.append("%.2fс/%.0fмс" % [e["t"], e["ms"]])
		lines.append("  " + ", ".join(parts))
	if not marks.is_empty():
		var parts: Array[String] = []
		for k in marks:
			parts.append("%s %.0f" % [k, marks[k]])
		lines.append("Загрузка, мс: " + " · ".join(parts))
	return "\n".join(lines)


func _refresh() -> void:
	if _label != null:
		_label.text = text()


## Показать или скрыть панель и запомнить выбор (из экрана настроек).
func set_shown(v: bool) -> void:
	_set_shown(v)
	UiSettings.set_bool(SETTING_KEY, v)
	if v:
		_refresh()


func toggle() -> void:
	_set_shown(not visible)
	UiSettings.set_bool(SETTING_KEY, visible)
	if visible:
		_refresh()


## Быстрое касание тремя пальцами: три касания начались почти одновременно (в пределах 250 мс) и закончились
## быстро (не позже 600 мс от первого) без заметного сдвига. Долгое удержание (джойстик, щипок) жестом не считается.
const TAP3_START_WINDOW_MS := 250
const TAP3_RELEASE_MS := 600
const TAP3_MOVE_PX := 40.0

var _touch_start: Dictionary = {}   # index -> {t, pos, moved}
var _tap3_armed := false
var _tap3_t0 := 0


func _input(event: InputEvent) -> void:
	if event is InputEventScreenTouch:
		_track_touch(event)
	elif event is InputEventScreenDrag:
		if _touch_start.has(event.index):
			_touch_start[event.index]["moved"] = (event.position as Vector2).distance_to(_touch_start[event.index]["pos"]) > TAP3_MOVE_PX \
				or _touch_start[event.index]["moved"]
	elif event is InputEventKey and event.pressed and not event.echo and event.keycode == KEY_F3:
		toggle()


## Часы жеста (мс); в тестах подменяются, чтобы проверки не зависели от скорости машины.
var clock: Callable = Callable()


func _now_ms() -> int:
	return int(clock.call()) if clock.is_valid() else Time.get_ticks_msec()


func _track_touch(event: InputEventScreenTouch) -> void:
	var now := _now_ms()
	if event.pressed:
		_touch_start[event.index] = {"t": now, "pos": event.position, "moved": false}
		if _touch_start.size() >= 3:
			var ts: Array = []
			for k in _touch_start:
				ts.append(_touch_start[k]["t"])
			ts.sort()
			# три самых недавних касания должны начаться почти одновременно
			var n := ts.size()
			if int(ts[n - 1]) - int(ts[n - 3]) <= TAP3_START_WINDOW_MS:
				_tap3_armed = true
				_tap3_t0 = int(ts[n - 3])
	else:
		if _tap3_armed and now - _tap3_t0 <= TAP3_RELEASE_MS:
			var moved := false
			for k in _touch_start:
				moved = moved or _touch_start[k]["moved"]
			if not moved:
				toggle()
		_tap3_armed = false
		_touch_start.erase(event.index)
		if _touch_start.is_empty():
			_tap3_armed = false
