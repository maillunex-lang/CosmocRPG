extends SceneTree
## Профиль пилота (R-34): аватарка, рамка, титул, VIP (в т.ч. VII), подписка, кредиты.

const ProfileScreen := preload("res://ui/profile_screen.gd")

var _f := 0
var _n := 0


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _run() -> void:
	var p: Control = ProfileScreen.new()
	root.add_child(p)
	p.theme = load("res://ui/theme/space_theme_fallback.tres") if ResourceLoader.exists("res://ui/theme/space_theme_fallback.tres") else null
	await process_frame

	var state := {
		"player": {"name": "Ковбой"},
		"credits": 125400,
		"ship": {"name": "Учебный дрон", "spec": "fighter",
			"cosmetics": {"vip": 7, "avatar": "zap", "frame": "#c06bff", "title": "Легенда рубежей"}},
	}
	p.set_data(state)
	await process_frame
	_c(p.find_child("Nick", true, false).text == "Ковбой", "ник из состояния")
	_c(p.find_child("TitleLabel", true, false).visible and p.find_child("TitleLabel", true, false).text == "«Легенда рубежей»", "титул показан")
	_c(p.find_child("VipValue", true, false).text == "VIP VII", "VIP VII римской цифрой")
	_c(p.find_child("CreditsValue", true, false).text == "125 400", "кредиты с разделителями")
	_c(p.find_child("SubValue", true, false).get_parent().visible == false, "подписка скрыта без данных магазина")

	p.set_data(state, {"subscription_until": "2026-12-08T00:00:00Z"})
	await process_frame
	_c(p.find_child("SubValue", true, false).get_parent().visible and p.find_child("SubValue", true, false).text.begins_with("активен до 2026-12-08"), "срок подписки из магазина")

	# без косметики: титул скрыт, VIP нет, аватарка по умолчанию
	p.set_data({"player": {"name": "Новичок"}, "credits": 0, "ship": {"name": "Стриж", "cosmetics": {}}})
	await process_frame
	_c(not p.find_child("TitleLabel", true, false).visible, "без титула строка скрыта")
	_c(p.find_child("VipValue", true, false).text == "нет", "без VIP честно «нет»")

	# кнопка магазина: сигнал и скрытие в демо
	var asked := [false]
	p.shop_requested.connect(func() -> void: asked[0] = true)
	p.find_child("ShopButton", true, false).pressed.emit()
	_c(asked[0], "кнопка магазина шлёт сигнал")
	p.set_shop_available(false)
	_c(not p.find_child("ShopButton", true, false).visible, "кнопка магазина скрывается")

	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
