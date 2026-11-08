extends Node
## Прокрутка пальцем внутри списков (R-50). Рамки и карточки (PanelContainer и подобные) по умолчанию «глотают» касание
## (mouse_filter = STOP), и ScrollContainer не видит жест, если палец лёг на рамку. Этот узел переводит такие элементы
## внутри ScrollContainer в режим PASS: касание доходит до прокрутки, а содержимое по-прежнему получает нажатия.
## Кнопки, поля ввода, ползунки и полосы прокрутки не трогаются: нажатие на кнопку во время прокрутки не должно срабатывать случайно.


func _ready() -> void:
	get_tree().node_added.connect(_on_node_added)


static func needs_pass(n: Node) -> bool:
	if not (n is Control) or (n as Control).mouse_filter != Control.MOUSE_FILTER_STOP:
		return false
	return not (n is BaseButton or n is LineEdit or n is TextEdit or n is Range or n is ScrollBar or n is ItemList or n is Tree or n is TabBar or n is ScrollContainer)


static func inside_scroll(n: Node) -> bool:
	var p := n.get_parent()
	while p != null:
		if p is ScrollContainer:
			return true
		p = p.get_parent()
	return false


## Переводит в PASS уже существующие рамки внутри ScrollContainer (для узлов, добавленных до включения автозагрузки, и для тестов).
static func fix_tree(root: Node) -> int:
	var changed := 0
	var stack: Array[Node] = [root]
	while not stack.is_empty():
		var n: Node = stack.pop_back()
		if needs_pass(n) and inside_scroll(n):
			(n as Control).mouse_filter = Control.MOUSE_FILTER_PASS
			changed += 1
		stack.append_array(n.get_children())
	return changed


func _on_node_added(n: Node) -> void:
	if needs_pass(n) and inside_scroll(n):
		(n as Control).mouse_filter = Control.MOUSE_FILTER_PASS
