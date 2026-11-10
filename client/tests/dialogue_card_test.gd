extends SceneTree
## Карточка разговора: пропуск шлёт серверу skip и не подменяет ответы.

const Card := preload("res://ui/dialogue_card.gd")

var _f := 0
var _n := 0


class Gate extends Node:
	var choice := ""

	func talk(id: String) -> void:
		choice = id

	func snapshot() -> Dictionary:
		return {}


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _buttons(n: Node) -> Array:
	var out: Array = []
	for c in n.get_children():
		if c is Button:
			out.append(c)
		out.append_array(_buttons(c))
	return out


func _run() -> void:
	var gw := Gate.new()
	var holder := Control.new()
	root.add_child(holder)
	var card: Control = Card.new()
	holder.add_child(card)
	card.bind(gw)
	await process_frame
	card._show({
		"speaker": "Ирина Ларс",
		"text": "Первый вылет?",
		"done": false,
		"choices": [{"id": "ready", "text": "Я готов."}],
	})
	var skip: Button = null
	var answer: Button = null
	for b in _buttons(card):
		if b.text == "Пропустить":
			skip = b
		if b.text == "Я готов.":
			answer = b
	_c(skip != null and skip.visible and answer != null, "в разговоре есть пропуск и ответ")
	skip.pressed.emit()
	_c(gw.choice == "skip", "пропуск просит сервер закрыть текущий разговор")
	card._show({"speaker": "Ирина Ларс", "text": "Итог.", "done": true, "choices": []})
	skip = null
	var ok := false
	for b in _buttons(card):
		if b.text == "Пропустить" and b.visible:
			skip = b
		if b.text == "Понятно":
			ok = true
	_c(skip == null and ok, "после итога пропуска больше нет")
	var routed := [false]
	card.route_ready.connect(func() -> void: routed[0] = true)
	card._close()
	_c(routed[0], "закрытие итога запускает маршрут следующего задания")
	routed[0] = false
	card._show({"speaker": "Ирина Ларс", "text": "Ещё вопрос.", "done": false, "choices": [{"id": "ready", "text": "Я готов."}]})
	card._close()
	_c(not routed[0], "незакрытый разговор маршрут не включает")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
