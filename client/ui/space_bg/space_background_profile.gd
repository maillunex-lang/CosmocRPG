extends Resource
## Профиль космического фона звёздной системы: набор слоёв (от дальнего к ближнему) и общие
## множители. Профили — данные: их можно заменить для каждой системы без выпуска нового APK.

const LayerConfig := preload("res://ui/space_bg/space_layer_config.gd")

@export var id: String = ""
@export var display_name: String = ""
@export var base_color: Color = Color("#030A14")     # глубокий тёмно-синий под слоями
@export_range(0.0, 2.0, 0.01) var brightness: float = 1.0   # общий множитель яркости профиля
@export_range(0.0, 2.0, 0.01) var intensity: float = 1.0    # общий множитель интенсивности профиля
@export_range(1.0, 60.0, 0.1) var px_per_unit: float = 14.4  # сколько пикселей экрана в одной единице мира
@export var layers: Array = []                               # Array[SpaceLayerConfig], от дальнего к ближнему

var warnings: PackedStringArray = PackedStringArray()


static func from_dict(d: Dictionary) -> Resource:
	var p = load("res://ui/space_bg/space_background_profile.gd").new()
	var w := PackedStringArray()
	p.id = str(d.get("id", ""))
	if p.id == "":
		w.append("id: не задан")
	p.display_name = str(d.get("name", p.id))
	var bc: Variant = d.get("base_color", null)
	if bc is Array and bc.size() >= 3:
		p.base_color = Color(clampf(float(bc[0]), 0.0, 1.0), clampf(float(bc[1]), 0.0, 1.0), clampf(float(bc[2]), 0.0, 1.0), 1.0)
	elif bc is String and bc.begins_with("#"):
		p.base_color = Color(bc)
	p.brightness = clampf(float(d.get("brightness", 1.0)), 0.0, 2.0)
	p.intensity = clampf(float(d.get("intensity", 1.0)), 0.0, 2.0)
	p.px_per_unit = clampf(float(d.get("px_per_unit", 14.4)), 1.0, 60.0)
	var arr: Variant = d.get("layers", [])
	if arr is Array:
		for item in arr:
			if item is Dictionary:
				var layer = LayerConfig.from_dict(item)
				for lw in layer.warnings:
					w.append("слой «%s»: %s" % [layer.texture_key, lw])
				p.layers.append(layer)
	else:
		w.append("layers: ожидается массив")
	if p.layers.is_empty():
		w.append("layers: нет ни одного слоя")
	p.warnings = w
	return p


func to_dict() -> Dictionary:
	var ls: Array = []
	for l in layers:
		ls.append(l.to_dict())
	return {
		"id": id, "name": display_name,
		"base_color": [base_color.r, base_color.g, base_color.b],
		"brightness": brightness, "intensity": intensity, "px_per_unit": px_per_unit,
		"layers": ls,
	}
