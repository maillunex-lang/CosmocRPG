extends RefCounted
## Сборка общей Theme из токенов. Тема создаётся кодом, поэтому цвета и размеры
## нельзя «забыть» поправить в нескольких местах. Подключение: root.theme = UiTheme.build().

const Tokens := preload("res://ui/theme/tokens.gd")
const FONT_PATH := "res://ui/fonts/GolosText-VF.ttf"

static var _font_cache: Dictionary = {}


## Golos Text заданного веса (ось wght задаётся числовым тегом).
static func make_font(weight: int) -> FontVariation:
	if _font_cache.has(weight):
		return _font_cache[weight]
	var fv := FontVariation.new()
	fv.base_font = load(FONT_PATH)
	var tag := TextServerManager.get_primary_interface().name_to_tag("wght")
	fv.variation_opentype = {tag: weight}
	_font_cache[weight] = fv
	return fv


static func _fs(base: int, text_scale: float) -> int:
	return maxi(1, int(round(base * text_scale)))


static func _box(bg: Color, border: Color = Color(0, 0, 0, 0), radius: int = Tokens.RADIUS, margins: Vector4 = Vector4.ZERO) -> StyleBoxFlat:
	var s := StyleBoxFlat.new()
	s.bg_color = bg
	s.border_color = border
	s.set_border_width_all(Tokens.BORDER_WIDTH if border.a > 0.0 else 0)
	s.set_corner_radius_all(radius)
	s.content_margin_left = margins.x
	s.content_margin_top = margins.y
	s.content_margin_right = margins.z
	s.content_margin_bottom = margins.w
	s.anti_aliasing = true
	return s


static func _variation(t: Theme, name: StringName, base: StringName) -> void:
	t.add_type(name)
	t.set_type_variation(name, base)


## Собирает тему. text_scale: масштаб текста (доступность, UI-11), 1.0 по умолчанию.
static func build(text_scale: float = 1.0) -> Theme:
	var t := Theme.new()
	var regular := make_font(400)
	var semibold := make_font(600)
	var bold := make_font(700)
	var border := Color(Tokens.ACCENT, Tokens.BORDER_ALPHA)
	var margin := float(Tokens.SPACE_LG)

	t.default_font = regular
	t.default_font_size = _fs(Tokens.FONT_BODY, text_scale)

	# --- Label и его варианты ---
	t.set_color("font_color", "Label", Tokens.TEXT)
	_variation(t, &"TitleLabel", &"Label")
	t.set_font("font", "TitleLabel", bold)
	t.set_font_size("font_size", "TitleLabel", _fs(Tokens.FONT_TITLE, text_scale))
	_variation(t, &"SectionLabel", &"Label")
	t.set_font("font", "SectionLabel", semibold)
	t.set_font_size("font_size", "SectionLabel", _fs(Tokens.FONT_H2, text_scale))
	t.set_color("font_color", "SectionLabel", Tokens.ACCENT)
	_variation(t, &"MutedLabel", &"Label")
	t.set_font_size("font_size", "MutedLabel", _fs(Tokens.FONT_SMALL, text_scale))
	t.set_color("font_color", "MutedLabel", Tokens.TEXT_MUTED)
	_variation(t, &"DangerLabel", &"Label")
	t.set_color("font_color", "DangerLabel", Tokens.DANGER)
	_variation(t, &"SuccessLabel", &"Label")
	t.set_color("font_color", "SuccessLabel", Tokens.SUCCESS)
	_variation(t, &"RareLabel", &"Label")
	t.set_color("font_color", "RareLabel", Tokens.RARE)

	# --- Панели ---
	t.set_stylebox("panel", "PanelContainer", _box(Color(Tokens.SURFACE, Tokens.PANEL_ALPHA), border, Tokens.RADIUS, Vector4(margin, margin, margin, margin)))
	_variation(t, &"CardPanel", &"PanelContainer")
	t.set_stylebox("panel", "CardPanel", _box(Color(Tokens.SURFACE, Tokens.PANEL_ALPHA), border, Tokens.RADIUS, Vector4(margin, margin, margin, margin)))
	_variation(t, &"RaisedPanel", &"PanelContainer")
	t.set_stylebox("panel", "RaisedPanel", _box(Color(Tokens.SURFACE_RAISED, Tokens.PANEL_ALPHA), border, Tokens.RADIUS, Vector4(margin, margin, margin, margin)))
	t.set_constant("separation", "VBoxContainer", Tokens.SPACE_MD)
	t.set_constant("separation", "HBoxContainer", Tokens.SPACE_MD)

	# --- Кнопки ---
	var bm := Vector4(Tokens.SPACE_LG, Tokens.SPACE_MD, Tokens.SPACE_LG, Tokens.SPACE_MD)
	t.set_stylebox("normal", "Button", _box(Tokens.SURFACE_RAISED, border, Tokens.RADIUS, bm))
	t.set_stylebox("hover", "Button", _box(Tokens.SURFACE_RAISED.lightened(0.12), Color(Tokens.ACCENT, 0.6), Tokens.RADIUS, bm))
	t.set_stylebox("pressed", "Button", _box(Tokens.SURFACE.darkened(0.2), Tokens.ACCENT, Tokens.RADIUS, bm))
	t.set_stylebox("disabled", "Button", _box(Color(Tokens.SURFACE, 0.55), Color(Tokens.ACCENT, 0.12), Tokens.RADIUS, bm))
	var focus := _box(Color(0, 0, 0, 0), Tokens.ACCENT, Tokens.RADIUS, bm)
	focus.draw_center = false
	focus.set_border_width_all(2)
	t.set_stylebox("focus", "Button", focus)
	t.set_font("font", "Button", semibold)
	t.set_font_size("font_size", "Button", _fs(Tokens.FONT_BUTTON, text_scale))
	t.set_color("font_color", "Button", Tokens.TEXT)
	t.set_color("font_hover_color", "Button", Tokens.TEXT)
	t.set_color("font_pressed_color", "Button", Tokens.TEXT)
	t.set_color("font_focus_color", "Button", Tokens.TEXT)
	t.set_color("font_disabled_color", "Button", Tokens.TEXT_MUTED)

	_variation(t, &"PrimaryButton", &"Button")
	t.set_stylebox("normal", "PrimaryButton", _box(Tokens.ACCENT, Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	t.set_stylebox("hover", "PrimaryButton", _box(Tokens.ACCENT.lightened(0.15), Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	t.set_stylebox("pressed", "PrimaryButton", _box(Tokens.ACCENT.darkened(0.2), Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	t.set_stylebox("disabled", "PrimaryButton", _box(Color(Tokens.ACCENT, 0.25), Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	for c in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color"]:
		t.set_color(c, "PrimaryButton", Tokens.BG)
	t.set_color("font_disabled_color", "PrimaryButton", Tokens.TEXT_MUTED)

	_variation(t, &"DangerButton", &"Button")
	t.set_stylebox("normal", "DangerButton", _box(Tokens.DANGER, Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	t.set_stylebox("hover", "DangerButton", _box(Tokens.DANGER.lightened(0.12), Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	t.set_stylebox("pressed", "DangerButton", _box(Tokens.DANGER.darkened(0.2), Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	t.set_stylebox("disabled", "DangerButton", _box(Color(Tokens.DANGER, 0.25), Color(0, 0, 0, 0), Tokens.RADIUS, bm))
	for c in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color"]:
		t.set_color(c, "DangerButton", Tokens.BG)
	t.set_color("font_disabled_color", "DangerButton", Tokens.TEXT_MUTED)

	# Плоская кнопка (заголовки сворачиваемых карточек): без фона, акцентный текст, высота касания не ниже 48
	_variation(t, &"FlatButton", &"Button")
	var fm := Vector4(Tokens.SPACE_MD, Tokens.SPACE_MD, Tokens.SPACE_MD, Tokens.SPACE_MD)
	t.set_stylebox("normal", "FlatButton", _box(Color(0, 0, 0, 0), Color(0, 0, 0, 0), Tokens.RADIUS, fm))
	t.set_stylebox("hover", "FlatButton", _box(Color(Tokens.SURFACE_RAISED, 0.6), Color(0, 0, 0, 0), Tokens.RADIUS, fm))
	t.set_stylebox("pressed", "FlatButton", _box(Color(Tokens.SURFACE_RAISED, 0.9), Color(0, 0, 0, 0), Tokens.RADIUS, fm))
	t.set_stylebox("disabled", "FlatButton", _box(Color(0, 0, 0, 0), Color(0, 0, 0, 0), Tokens.RADIUS, fm))
	for c in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color"]:
		t.set_color(c, "FlatButton", Tokens.ACCENT)
	t.set_font_size("font_size", "FlatButton", _fs(Tokens.FONT_SMALL, text_scale))
	t.set_constant("h_separation", "FlatButton", Tokens.SPACE_SM)

	# --- Поле ввода ---
	var lm := Vector4(Tokens.SPACE_LG, Tokens.SPACE_MD, Tokens.SPACE_LG, Tokens.SPACE_MD)
	t.set_stylebox("normal", "LineEdit", _box(Tokens.SURFACE_SUNKEN, border, Tokens.RADIUS, lm))
	var le_focus := _box(Tokens.SURFACE_SUNKEN, Tokens.ACCENT, Tokens.RADIUS, lm)
	le_focus.set_border_width_all(2)
	t.set_stylebox("focus", "LineEdit", le_focus)
	t.set_stylebox("read_only", "LineEdit", _box(Color(Tokens.SURFACE_SUNKEN, 0.6), Color(Tokens.ACCENT, 0.12), Tokens.RADIUS, lm))
	t.set_color("font_color", "LineEdit", Tokens.TEXT)
	t.set_color("font_uneditable_color", "LineEdit", Tokens.TEXT_MUTED)
	t.set_color("font_placeholder_color", "LineEdit", Tokens.TEXT_MUTED)
	t.set_color("caret_color", "LineEdit", Tokens.ACCENT)
	t.set_color("selection_color", "LineEdit", Color(Tokens.ACCENT, 0.35))
	t.set_font_size("font_size", "LineEdit", _fs(Tokens.FONT_BODY, text_scale))
	t.set_constant("minimum_character_width", "LineEdit", 4)

	# --- Многострочный текст ---
	t.set_color("default_color", "RichTextLabel", Tokens.TEXT)
	t.set_font("normal_font", "RichTextLabel", regular)
	t.set_font("bold_font", "RichTextLabel", semibold)
	t.set_font_size("normal_font_size", "RichTextLabel", _fs(Tokens.FONT_BODY, text_scale))
	t.set_font_size("bold_font_size", "RichTextLabel", _fs(Tokens.FONT_BODY, text_scale))
	t.set_stylebox("normal", "RichTextLabel", StyleBoxEmpty.new())

	# --- Индикатор прогресса ---
	t.set_stylebox("background", "ProgressBar", _box(Tokens.SURFACE_SUNKEN, Color(Tokens.ACCENT, 0.18), 6))
	t.set_stylebox("fill", "ProgressBar", _box(Tokens.ACCENT, Color(0, 0, 0, 0), 6))
	t.set_font_size("font_size", "ProgressBar", _fs(Tokens.FONT_SMALL, text_scale))
	t.set_color("font_color", "ProgressBar", Tokens.TEXT)

	# --- Разделитель ---
	var line := StyleBoxLine.new()
	line.color = Color(Tokens.ACCENT, 0.2)
	line.thickness = 1
	t.set_stylebox("separator", "HSeparator", line)
	t.set_constant("separation", "HSeparator", Tokens.SPACE_MD)
	return t
