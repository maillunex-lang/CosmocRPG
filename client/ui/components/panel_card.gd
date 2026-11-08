extends PanelContainer
## Карточка-панель с необязательным заголовком. Использование в сцене:
##   PanelCard (этот скрипт, title = "Сервер")
##     └ VBoxContainer  — содержимое (заголовок вставляется в него первым элементом)

@export var title: String = "":
	set(v):
		title = v
		_refresh_title()

var _title_label: Label


func _init() -> void:
	theme_type_variation = &"CardPanel"


func _ready() -> void:
	_refresh_title()


func _refresh_title() -> void:
	if not is_node_ready():
		return
	var box := _content_box()
	if box == null:
		return
	if title.is_empty():
		if _title_label != null:
			_title_label.queue_free()
			_title_label = null
		return
	if _title_label == null:
		_title_label = Label.new()
		_title_label.name = "CardTitle"
		_title_label.theme_type_variation = &"SectionLabel"
		box.add_child(_title_label)
		box.move_child(_title_label, 0)
	_title_label.text = title


func _content_box() -> Container:
	for c in get_children():
		if c is VBoxContainer:
			return c
	return null
