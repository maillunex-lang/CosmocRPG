extends SceneTree
## Экран настроек: значения по умолчанию, переключение, сохранение, сигналы; автоответ уходит серверу при входе.

const SettingsScreen := preload("res://ui/settings_screen.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const FrameStats := preload("res://ui/debug/frame_stats.gd")

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
	UiSettings.set_bool("auto_retaliate", true)
	UiSettings.set_bool(FrameStats.SETTING_KEY, false)
	_c(SettingsScreen.get_value("auto_retaliate") and not SettingsScreen.get_value(FrameStats.SETTING_KEY), "по умолчанию: автоответ включён, панель FPS выключена")
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	root.add_child(holder)
	var s: Control = SettingsScreen.new()
	holder.add_child(s)
	await process_frame
	await process_frame
	var ar: Button = s._toggles["auto_retaliate"]
	var fps: Button = s._toggles[FrameStats.SETTING_KEY]
	_c(ar.button_pressed and ar.text == "Вкл" and not fps.button_pressed and fps.text == "Выкл", "переключатели показывают текущие значения")
	var got: Array = []
	s.option_changed.connect(func(k: String, v: bool) -> void: got.append([k, v]))
	ar.button_pressed = false   # как при нажатии: состояние кнопки меняется до сигнала pressed
	ar.pressed.emit()
	_c(got.size() >= 1 and got[-1][0] == "auto_retaliate" and got[-1][1] == false, "выключение автоответа сообщает об изменении")
	_c(not SettingsScreen.get_value("auto_retaliate") and ar.text == "Выкл", "выбор сохранён и показан")
	fps.button_pressed = true
	fps.pressed.emit()
	_c(SettingsScreen.get_value(FrameStats.SETTING_KEY) and got[-1][0] == FrameStats.SETTING_KEY and got[-1][1] == true, "включение панели FPS сохраняется и сообщается")
	var closed := [false]
	s.closed.connect(func() -> void: closed[0] = true)
	s.closed.emit()
	_c(closed[0], "закрытие")
	# восстановить значения по умолчанию для следующих тестов
	UiSettings.set_bool("auto_retaliate", true)
	UiSettings.set_bool(FrameStats.SETTING_KEY, false)
	# в боевом интерфейсе пункт меню «Настройки» активен и открывает экран
	var hud_scene: PackedScene = load("res://scenes/hud.tscn")
	var hud: Control = hud_scene.instantiate()
	holder.add_child(hud)
	await process_frame
	var btn: Button = hud.get_node("%SettingsMenuButton")
	var asked := [false]
	hud.settings_requested.connect(func() -> void: asked[0] = true)
	_c(not btn.disabled and btn.text == "Настройки", "пункт меню «Настройки» активен")
	btn.pressed.emit()
	_c(asked[0], "«Настройки» запрашивают экран настроек")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
