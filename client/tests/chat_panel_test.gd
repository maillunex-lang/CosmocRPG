extends SceneTree
## Окно чата: форматирование и защита от разметки, вкладки, непрочитанные, отправка, ошибки, метка ИИ.

const ChatPanel := preload("res://ui/chat_panel.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

var _f := 0
var _n := 0


class FakeGateway extends UiGateway:
	var sent: Array = []
	func has_chat() -> bool:
		return true
	func own_id() -> String:
		return "me"
	func chat_send(channel: String, to: String, text: String) -> void:
		sent.append([channel, to, text])


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
	# --- чистые функции ---
	_c(ChatPanel.escape("[b]жирный[/b] [url=x]ссылка[/url]") == "[lb]b]жирный[lb]/b] [lb]url=x]ссылка[lb]/url]", "теги разметки в тексте экранируются")
	var line := ChatPanel.format_line({"channel": "general", "from_id": "p2", "from_name": "Вега", "text": "Привет [img]x[/img]", "at": 1794160000000}, "me")
	_c(line.contains("Вега:") and not line.contains("[img]x") and line.contains("[lb]img]"), "в строке нет чужих тегов, имя и текст на месте")
	var ai := ChatPanel.format_line({"channel": "general", "from_id": "ai1", "from_name": "Орион-7", "from_ai": true, "text": "Добрый вечер", "at": 1794160000000}, "me")
	_c(ai.contains("bot.svg") and not line.contains("bot.svg"), "у сообщений ИИ есть иконка, у игроков нет")
	var evil_name := ChatPanel.format_line({"channel": "general", "from_id": "x", "from_name": "[color=red]Хак", "text": "т", "at": 1}, "me")
	_c(not evil_name.contains("[color=red]Хак"), "имя отправителя тоже экранируется")
	var mine := ChatPanel.format_line({"channel": "dm", "from_id": "me", "to_id": "p2", "to_name": "Вега", "text": "лично", "at": 1794160000000}, "me")
	_c(mine.contains("→ Вега:"), "своё личное сообщение показано как «→ Кому»")
	_c(ChatPanel.format_line({"channel": "system", "from_id": "system", "from_name": "Система", "text": "Новости", "at": 1}, "me").contains("Новости"), "системное сообщение")
	# --- панель ---
	var holder := SubViewport.new()
	holder.size = Vector2i(800, 600)
	root.add_child(holder)
	var gw := FakeGateway.new()
	holder.add_child(gw)
	var p: VBoxContainer = ChatPanel.new()
	holder.add_child(p)
	await process_frame
	p.bind(gw)
	var counts := []
	p.unread_changed.connect(func(n: int) -> void: counts.append(n))
	p.on_history([{"channel": "general", "from_id": "p2", "from_name": "Вега", "text": "раньше", "at": 1794160000000}], [], {})
	_c(p._log.get_parsed_text().contains("Вега") and p._log.get_parsed_text().contains("раньше"), "история показана")
	# отправка
	p._edit.text = "  Всем привет  "
	p.submit()
	_c(gw.sent == [["general", "", "Всем привет"]] and p._edit.text == "", "общее сообщение отправлено, поле очищено, пробелы обрезаны")
	p._edit.text = "   "
	p.submit()
	_c(gw.sent.size() == 1, "пустой текст не отправляется")
	# личные: нужен адресат
	p._select_tab("dm")
	p._edit.text = "привет лично"
	p.submit()
	_c(gw.sent.size() == 1 and p._error.visible, "в личных без адресата отправки нет, показана подсказка")
	p._to.text = "Вега"
	p.submit()
	_c(gw.sent.size() == 2 and gw.sent[1] == ["dm", "Вега", "привет лично"], "личное сообщение уходит с адресатом")
	# непрочитанные
	p._select_tab("general")
	p._to.text = ""   # адресат не задан: входящее личное подставит отправителя для ответа
	p.on_message({"channel": "dm", "from_id": "p3", "from_name": "Лира", "text": "эй", "at": 1794160002000})
	_c(p.dm_unread == 1 and counts.back() == 1 and p._tab_buttons["dm"].text.contains("(1)"), "личное сообщение при открытом общем: счётчик непрочитанных")
	_c(p._to.text == "Лира", "адресат для ответа подставлен автоматически")
	p._select_tab("dm")
	_c(p.dm_unread == 0 and counts.back() == 0 and p._log.get_parsed_text().contains("эй"), "открытие вкладки сбрасывает счётчик и показывает сообщение")
	# ограничение числа строк
	for i in 130:
		p.on_message({"channel": "general", "from_id": "p2", "from_name": "Вега", "text": "т%d" % i, "at": 1794160003000 + i})
	_c(p._general.size() <= ChatPanel.MAX_LINES, "в журнале не больше %d строк" % ChatPanel.MAX_LINES)
	# ошибка сервера
	p.show_error("слишком часто: подождите пару секунд")
	_c(p._error.visible and p._error.text.contains("слишком часто"), "ошибка сервера показана")
	# без чата (демо)
	var demo := UiGateway.new()
	holder.add_child(demo)
	var p2: VBoxContainer = ChatPanel.new()
	holder.add_child(p2)
	await process_frame
	p2.bind(demo)
	p2._edit.text = "привет"
	p2.submit()
	_c(not p2.enabled and not p2._edit.editable and p2._send.disabled, "в демо ввод отключён")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
