extends SceneTree
## Тест базовых компонентов интерфейса. Запуск:
##   godot --headless --path client --script res://tests/components_test.gd

const ActionButton := preload("res://ui/components/action_button.gd")
const PanelCard := preload("res://ui/components/panel_card.gd")
const ResourceBadge := preload("res://ui/components/resource_badge.gd")
const HudBar := preload("res://ui/components/hud_bar.gd")
const Tokens := preload("res://ui/theme/tokens.gd")

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


func _run() -> void:
	# --- ActionButton ---
	var b: Button = ActionButton.new()
	b.text = "Атаковать"
	root.add_child(b)
	await process_frame
	_check(b.custom_minimum_size.y >= Tokens.TOUCH_MIN, "кнопка: минимальная высота %d" % Tokens.TOUCH_MIN)
	_check(b.theme_type_variation == &"", "кнопка: по умолчанию вторичный вариант")
	b.variant = ActionButton.Variant.PRIMARY
	_check(b.theme_type_variation == &"PrimaryButton", "кнопка: вариант PRIMARY")
	b.variant = ActionButton.Variant.DANGER
	_check(b.theme_type_variation == &"DangerButton", "кнопка: вариант DANGER")

	var presses := 0
	b.pressed.connect(func() -> void: presses += 1)
	b.begin_busy("Выполняется…")
	_check(b.is_busy() and b.disabled and b.text == "Выполняется…", "кнопка: busy блокирует и меняет текст")
	b.begin_busy("другое")  # повторный вызов не должен затирать сохранённое состояние
	b.end_busy()
	_check(not b.is_busy() and not b.disabled and b.text == "Атаковать", "кнопка: end_busy возвращает текст и состояние")
	b.end_busy()  # повторный вызов безопасен
	_check(not b.disabled and b.text == "Атаковать", "кнопка: повторный end_busy безопасен")

	b.disabled = true
	b.begin_busy()
	b.end_busy()
	_check(b.disabled, "кнопка: была неактивна до busy, остаётся неактивной после")
	b.disabled = false

	b.soon = true
	_check(b.disabled and b.text.ends_with("скоро") and b.tooltip_text != "", "кнопка: «Скоро» неактивна, помечена и с подсказкой")
	b.soon = false
	_check(not b.disabled and b.text == "Атаковать" and b.tooltip_text == "", "кнопка: «Скоро» снимается, текст возвращается")

	# --- ResourceBadge ---
	_check(ResourceBadge.format_number(0) == "0", "бейдж: 0")
	_check(ResourceBadge.format_number(999) == "999", "бейдж: 999")
	_check(ResourceBadge.format_number(1000) == "1 000", "бейдж: 1 000 с неразрывным пробелом")
	_check(ResourceBadge.format_number(1234567) == "1 234 567", "бейдж: 1 234 567")
	_check(ResourceBadge.format_number(-45000) == "-45 000", "бейдж: отрицательное")
	var badge: HBoxContainer = ResourceBadge.new()
	badge.caption = "Кредиты"
	root.add_child(badge)
	await process_frame
	badge.set_value(12500)
	var label: Label = badge.get_child(1)
	_check(label.text == "Кредиты  12 500", "бейдж: подпись и значение в тексте (%s)" % label.text)
	_check(badge.get_child_count() == 2, "бейдж: значок и текст")

	# --- HudBar ---
	var bar: ProgressBar = HudBar.new()
	root.add_child(bar)
	await process_frame
	bar.set_values(30.0, 120.0)
	_check(bar.max_value == 120.0 and bar.value == 30.0, "полоска: значения")
	bar.set_values(500.0, 120.0)
	_check(bar.value == 120.0, "полоска: значение не больше максимума")
	bar.set_values(-5.0, 0.0)
	_check(bar.max_value == 1.0 and bar.value == 0.0, "полоска: максимум не меньше 1, значение не меньше 0")
	_check(bar.custom_minimum_size.y >= 22.0, "полоска: минимальная высота")
	_check(bar.has_theme_stylebox_override("fill"), "полоска: цвет заливки задан")

	# --- PanelCard ---
	var card: PanelContainer = PanelCard.new()
	card.title = "Корабль"
	var box := VBoxContainer.new()
	var content := Label.new()
	content.text = "содержимое"
	box.add_child(content)
	card.add_child(box)
	root.add_child(card)
	await process_frame
	_check(card.theme_type_variation == &"CardPanel", "карточка: стиль CardPanel")
	_check(box.get_child_count() == 2 and box.get_child(0) is Label and (box.get_child(0) as Label).text == "Корабль", "карточка: заголовок вставлен первым")
	card.title = "Флагман"
	_check((box.get_child(0) as Label).text == "Флагман" and box.get_child_count() == 2, "карточка: заголовок обновляется без дублей")
	card.title = ""
	await process_frame
	_check(box.get_child_count() == 1, "карточка: пустой заголовок убирает подпись")

	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
