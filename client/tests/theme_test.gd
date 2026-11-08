extends SceneTree
## Тест токенов и темы: палитра, контраст (WCAG), шрифт, варианты, масштаб текста, Landscape.
## Запуск: godot --headless --path client --script res://tests/theme_test.gd

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")

var _failures := 0
var _checks := 0


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _blend(top: Color, bottom: Color) -> Color:
	return Color(top.r * top.a + bottom.r * (1.0 - top.a), top.g * top.a + bottom.g * (1.0 - top.a),
		top.b * top.a + bottom.b * (1.0 - top.a), 1.0)


func _run() -> void:
	# --- палитра утверждена владельцем (UI-11) ---
	_check(Tokens.hex(Tokens.BG) == "#081421", "палитра: фон #081421")
	_check(Tokens.hex(Tokens.SURFACE) == "#14283c", "палитра: поверхности #14283C")
	_check(Tokens.hex(Tokens.ACCENT) == "#42c6e5", "палитра: акцент #42C6E5")
	_check(Tokens.hex(Tokens.ACCENT_2) == "#9575e8", "палитра: вторичный акцент #9575E8")
	_check(Tokens.hex(Tokens.RARE) == "#f3c46b", "палитра: редкие награды #F3C46B")
	_check(Tokens.hex(Tokens.DANGER) == "#f16c79", "палитра: опасность #F16C79")

	# --- контраст (WCAG): обычный текст >= 4.5, крупный/декоративный >= 3 ---
	var panel := _blend(Color(Tokens.SURFACE, Tokens.PANEL_ALPHA), Tokens.BG)  # панель поверх фона
	var pairs := [
		["основной текст на фоне", Tokens.TEXT, Tokens.BG, 7.0],
		["основной текст на панели", Tokens.TEXT, panel, 7.0],
		["вторичный текст на панели", Tokens.TEXT_MUTED, panel, 4.5],
		["вторичный текст в поле ввода", Tokens.TEXT_MUTED, Tokens.SURFACE_SUNKEN, 4.5],
		["акцент (заголовки секций) на панели", Tokens.ACCENT, panel, 4.5],
		["текст основной кнопки (фон на акценте)", Tokens.BG, Tokens.ACCENT, 4.5],
		["текст опасной кнопки (фон на красном)", Tokens.BG, Tokens.DANGER, 4.5],
		["опасность (ошибки) на панели", Tokens.DANGER, panel, 4.5],
		["успех на панели", Tokens.SUCCESS, panel, 4.5],
		["редкие награды на панели", Tokens.RARE, panel, 4.5],
		["вторичный акцент на панели (крупное)", Tokens.ACCENT_2, panel, 3.0],
	]
	for p in pairs:
		var ratio := Tokens.contrast(p[1], p[2])
		_check(ratio >= p[3], "контраст %s: %.2f (нужно >= %.1f)" % [p[0], ratio, p[3]])

	# --- размеры и анимации ---
	_check(Tokens.TOUCH_MIN >= 48, "размер нажимаемой кнопки не меньше 48 px")
	_check(Tokens.ANIM_OPEN_MS >= 150 and Tokens.ANIM_OPEN_MS <= 250, "анимация открытия 150–250 мс")

	# --- шрифт ---
	var regular := UiTheme.make_font(400)
	var bold := UiTheme.make_font(700)
	var sample := "Звёздные рубежи — командный центр"
	var w400 := regular.get_string_size(sample, HORIZONTAL_ALIGNMENT_LEFT, -1, 24).x
	var w700 := bold.get_string_size(sample, HORIZONTAL_ALIGNMENT_LEFT, -1, 24).x
	_check(w700 > w400, "вариативный шрифт: вес 700 шире веса 400 (%.0f > %.0f)" % [w700, w400])
	_check(UiTheme.make_font(400) == regular, "шрифты кэшируются")
	var base_font: FontFile = load("res://ui/fonts/GolosText-VF.ttf")
	_check(base_font != null and base_font.get_font_name() == "Golos Text", "подключён шрифт Golos Text")
	var missing := []
	for ch in "АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдеёжзийклмнопрстуфхцчшщъыьэюя№«»—…₽":
		if not base_font.has_char(ch.unicode_at(0)):
			missing.append(ch)
	_check(missing.is_empty(), "в шрифте есть все русские буквы и знаки (нет: %s)" % str(missing))

	# --- тема ---
	var th := UiTheme.build()
	_check(th != null and th.default_font != null, "тема собирается, шрифт по умолчанию задан")
	_check(th.default_font_size == Tokens.FONT_BODY, "размер шрифта по умолчанию из токенов")
	for v in [["TitleLabel", "Label"], ["SectionLabel", "Label"], ["MutedLabel", "Label"], ["DangerLabel", "Label"],
			["SuccessLabel", "Label"], ["RareLabel", "Label"], ["CardPanel", "PanelContainer"],
			["RaisedPanel", "PanelContainer"], ["PrimaryButton", "Button"], ["DangerButton", "Button"]]:
		_check(th.get_type_variation_base(v[0]) == StringName(v[1]), "вариант темы %s наследует %s" % [v[0], v[1]])
	_check(th.has_stylebox("panel", "CardPanel") and th.has_stylebox("normal", "PrimaryButton"), "стили панели и кнопки заданы")
	_check(th.get_color("font_color", "PrimaryButton").is_equal_approx(Tokens.BG), "текст основной кнопки тёмный")
	_check(th.get_color("font_color", "DangerLabel").is_equal_approx(Tokens.DANGER), "цвет DangerLabel из токена")
	var scaled := UiTheme.build(1.5)
	_check(scaled.default_font_size == 30, "масштаб текста 1.5 увеличивает шрифт (30)")
	_check(scaled.get_font_size("font_size", "Button") == int(round(Tokens.FONT_BUTTON * 1.5)), "масштаб текста действует на кнопки")
	_check(UiTheme.build(0.0).default_font_size >= 1, "нулевой масштаб не даёт нулевой размер шрифта")

	# --- ориентация: только горизонтальная, поворот на 180° ---
	var orient := int(ProjectSettings.get_setting("display/window/handheld/orientation"))
	_check(orient == 4, "ориентация: альбомная по датчику (4), сейчас %d" % orient)
	var w := int(ProjectSettings.get_setting("display/window/size/viewport_width"))
	var h := int(ProjectSettings.get_setting("display/window/size/viewport_height"))
	_check(w > h and absf(float(w) / h - 16.0 / 9.0) < 0.01, "базовое разрешение 16:9 горизонтальное (%dx%d)" % [w, h])
	_check(str(ProjectSettings.get_setting("display/window/stretch/aspect")) == "expand", "stretch aspect = expand")

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
