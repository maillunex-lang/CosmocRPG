extends RefCounted
## Иконки интерфейса (набор Lucide, ISC; см. assets/icons/README.md). Иконки белые: цвет задаётся через modulate / icon_*_color.

const DIR := "res://assets/icons/"

## Ключ иконки товара магазина (с сервера) -> файл набора.
const ALIASES := {"vip": "crown", "planet": "globe", "cargo": "package", "mine": "pickaxe", "credits": "coins", "shop": "store"}

static var _cache: Dictionary = {}


static func path(icon: String) -> String:
	return DIR + str(ALIASES.get(icon, icon)) + ".svg"


static func exists(icon: String) -> bool:
	return ResourceLoader.exists(path(icon))


## Текстура иконки. Неизвестное имя даёт общую иконку «sparkles», чтобы интерфейс не оставался пустым.
static func tex(icon: String) -> Texture2D:
	if _cache.has(icon):
		return _cache[icon]
	var p := path(icon) if exists(icon) else DIR + "sparkles.svg"
	var t: Texture2D = load(p)
	_cache[icon] = t
	return t


## Иконка на кнопке (слева от текста).
static func apply(button: Button, icon: String, color: Color = Color.WHITE, size: int = 24) -> void:
	button.icon = tex(icon)
	button.expand_icon = true
	button.add_theme_constant_override("icon_max_width", size)
	button.add_theme_color_override("icon_normal_color", color)
	button.add_theme_color_override("icon_hover_color", color)
	button.add_theme_color_override("icon_pressed_color", color)
	button.add_theme_color_override("icon_disabled_color", Color(color, 0.4))


## Узел TextureRect с иконкой заданного размера и цвета.
static func rect(icon: String, size: int = 28, color: Color = Color.WHITE) -> TextureRect:
	var r := TextureRect.new()
	r.texture = tex(icon)
	r.custom_minimum_size = Vector2(size, size)
	r.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	r.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	r.modulate = color
	r.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return r
