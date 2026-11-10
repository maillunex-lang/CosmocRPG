extends SceneTree
## Окно чата поверх игры (R-61): геометрия «половина / весь экран», клавиатура, полупрозрачный фон, пропуск касаний мимо панели,
## перенос панели чата в окно и обратно, пункт меню «Чат».

const ChatOverlay := preload("res://ui/chat_overlay.gd")
const ChatPanel := preload("res://ui/chat_panel.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const Session := preload("res://ui/session.gd")
const ChatGateway := preload("res://tests/support_chat_gateway.gd")

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
	var vp := Vector2(1280, 720)
	# --- геометрия ---
	var half := ChatOverlay.panel_rect(vp, ChatOverlay.Mode.HALF)
	_c(half.position == Vector2(10, 10) and half.size.x == 1260 and absf(half.end.y - 360.0) < 0.01, "половина: верхняя половина экрана на всю ширину")
	var full := ChatOverlay.panel_rect(vp, ChatOverlay.Mode.FULL)
	_c(full.size.x == 1260 and absf(full.end.y - 710.0) < 0.01, "весь экран: почти вся высота с полями")
	var kb_full := ChatOverlay.panel_rect(vp, ChatOverlay.Mode.FULL, 300.0)
	_c(absf(kb_full.end.y - 420.0) < 0.01, "весь экран: панель поднимается над клавиатурой")
	var kb_half := ChatOverlay.panel_rect(vp, ChatOverlay.Mode.HALF, 200.0)
	_c(absf(kb_half.end.y - 360.0) < 0.01, "половина: небольшая клавиатура панель не двигает")
	var kb_big := ChatOverlay.panel_rect(vp, ChatOverlay.Mode.HALF, 600.0)
	_c(kb_big.size.y >= 220.0, "огромная клавиатура: панель не схлопывается ниже минимальной высоты (%d)" % int(kb_big.size.y))
	_c(ChatOverlay.panel_rect(Vector2(500, 300), ChatOverlay.Mode.FULL).end.y <= 290.0, "на низком экране панель не выходит за край")
	_c(absf(ChatOverlay.keyboard_height(540.0, 1080.0, 720.0) - 360.0) < 0.01 and ChatOverlay.keyboard_height(0.0, 1080.0, 720.0) == 0.0, "высота клавиатуры пересчитывается в единицы окна игры")

	# --- перенос панели и поведение окна ---
	var host := VBoxContainer.new()
	root.add_child(host)
	var before := Label.new()
	host.add_child(before)
	var chat: Control = ChatPanel.new()
	host.add_child(chat)
	var after := Label.new()
	host.add_child(after)
	await process_frame
	var expand := chat.find_child("ExpandButton", true, false) as Button
	_c(expand != null and expand.visible, "в карточке есть кнопка «Окно»")
	var asked := [0]
	chat.expand_requested.connect(func() -> void: asked[0] += 1)
	expand.pressed.emit()
	_c(asked[0] == 1, "кнопка «Окно» просит открыть окно чата")

	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	holder.disable_3d = true
	root.add_child(holder)
	var ov: Control = ChatOverlay.new()
	holder.add_child(ov)
	await process_frame
	var closed := [0]
	ov.closed.connect(func() -> void: closed[0] += 1)
	ov.open(chat, ChatOverlay.Mode.HALF)
	await process_frame
	await process_frame
	var panel := ov.find_child("Panel", true, false) as PanelContainer
	_c(chat.get_parent() == ov.find_child("Slot", true, false), "панель чата перенесена в окно")
	_c(ov.mouse_filter == Control.MOUSE_FILTER_IGNORE and panel.mouse_filter == Control.MOUSE_FILTER_STOP, "касания мимо панели проходят к игре, по панели — не проходят")
	var bg := panel.get_theme_stylebox("panel") as StyleBoxFlat
	_c(bg != null and bg.bg_color.a > 0.3 and bg.bg_color.a < 0.9, "фон полупрозрачный (прозрачность %.2f): игра видна позади" % bg.bg_color.a)
	_c(absf(panel.size.y - 350.0) < 2.0 and panel.position.y == 10.0, "в половинном режиме панель занимает верхнюю половину (%d)" % int(panel.size.y))
	_c(not expand.visible, "в окне кнопка «Окно» скрыта")
	var log := chat.find_children("*", "RichTextLabel", true, false)[0] as RichTextLabel
	_c(log.size_flags_vertical == Control.SIZE_EXPAND_FILL and log.custom_minimum_size.y == 0.0, "журнал занимает всё свободное место окна")
	var mode_btn := ov.find_child("ModeButton", true, false) as Button
	_c(mode_btn.text == "На весь экран", "в половинном режиме кнопка предлагает «На весь экран»")
	mode_btn.pressed.emit()
	await process_frame
	_c(ov.mode == ChatOverlay.Mode.FULL and panel.size.y > 650.0 and mode_btn.text == "Половина", "переключение на весь экран (%d) и подпись кнопки" % int(panel.size.y))
	mode_btn.pressed.emit()
	await process_frame
	_c(ov.mode == ChatOverlay.Mode.HALF and panel.size.y < 400.0, "и обратно на половину")

	# --- закрытие возвращает панель на место ---
	(ov.find_child("CloseButton", true, false) as Button).pressed.emit()
	await process_frame
	_c(closed[0] == 1 and chat.get_parent() == host and chat.get_index() == 1, "после закрытия панель чата на прежнем месте карточки")
	_c(expand.visible and log.custom_minimum_size.y == ChatPanel.INLINE_LOG_HEIGHT and chat.size_flags_vertical == Control.SIZE_FILL, "карточка вернулась к обычному виду")

	# --- окно убрали без закрытия: панель чата не теряется ---
	var ov2: Control = ChatOverlay.new()
	holder.add_child(ov2)
	await process_frame
	ov2.open(chat, ChatOverlay.Mode.FULL)
	await process_frame
	ov2.free()
	await process_frame
	_c(is_instance_valid(chat) and chat.get_parent() == host, "если окно удалено без закрытия, панель чата возвращается в карточку")

	# --- меню HUD и экран космоса ---
	var dg := DemoGateway.new()
	dg.auto_tick = false
	Session.gateway = dg
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	holder.add_child(screen)
	for i in 3:
		await process_frame
	var hud: Control = screen.get_node("%Hud")
	var asked_menu := [0]
	hud.chat_requested.connect(func() -> void: asked_menu[0] += 1)
	var menu_btn := hud.get_node("%ChatMenuButton") as Button
	_c(menu_btn != null and menu_btn.text == "Чат", "в меню есть пункт «Чат»")
	menu_btn.pressed.emit()
	_c(asked_menu[0] == 1, "пункт меню «Чат» запрашивает окно чата")
	_c(screen.open_chat() == null and screen.get_node_or_null("ChatLayer") == null, "в демо без сервера окно чата не открывается (и не ломает игру)")
	_c(not screen.close_chat(), "закрывать нечего — «Назад» работает как обычно")
	holder.queue_free()
	Session.gateway = null

	# --- регрессия: в игре с сервером панель чата создаётся и окно открывается ---
	var cg := ChatGateway.new()
	cg.auto_tick = false
	Session.gateway = cg
	var h2 := SubViewport.new()
	h2.size = Vector2i(1280, 720)
	h2.disable_3d = true
	root.add_child(h2)
	var screen2: Control = load("res://scenes/space.tscn").instantiate()
	screen2.auto_exit = false
	h2.add_child(screen2)
	for i in 4:
		await process_frame
	var hud2: Control = screen2.get_node("%Hud")
	_c(hud2.chat_panel != null and hud2.chat_panel.enabled and not hud2.chat_panel.visible and hud2.find_child("ChatPeek", true, false) != null, "в игре чат — плашка, полное окно открывается отдельно")
	_c(cg.history_requests >= 1, "история чата запрашивается для созданной панели")
	var ov3 = screen2.open_chat()
	for i in 3:
		await process_frame
	_c(ov3 != null and screen2.get_node_or_null("ChatLayer/ChatOverlay") != null, "окно чата открывается из экрана космоса")
	var edit: Array = ov3.find_children("*", "LineEdit", true, false)
	var typed: LineEdit = null
	for e in edit:
		if (e as LineEdit).placeholder_text.begins_with("Сообщение"):
			typed = e
	_c(typed != null and typed.editable, "в окне можно писать сообщение")
	typed.text = "Привет, космос!"
	typed.text_submitted.emit(typed.text)
	for i in 3:
		await process_frame
	_c(cg.sent.size() == 1 and cg.sent[0]["channel"] == "general" and cg.sent[0]["text"] == "Привет, космос!", "сообщение из окна уходит в общий чат")
	var rich: Array = ov3.find_children("*", "RichTextLabel", true, false)
	_c(rich.size() > 0 and (rich[0] as RichTextLabel).get_parsed_text().contains("Привет, космос!"), "своё сообщение видно в журнале окна")
	var closed_ok: bool = screen2.close_chat()
	await process_frame
	await process_frame
	_c(closed_ok and screen2.get_node_or_null("ChatLayer/ChatOverlay") == null, "«Назад» закрывает окно чата, а не выходит из игры")
	_c(hud2.chat_panel.get_parent() == hud2.get_node("%ChatCard").get_node("Box") or hud2.chat_panel.is_inside_tree(), "после закрытия панель чата снова в карточке HUD")
	h2.queue_free()
	Session.gateway = null
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
