extends Node2D
## Звёздный фон с параллаксом: два слоя точек. Рисуется один раз, затем только сдвигается.

const Tokens := preload("res://ui/theme/tokens.gd")

const AREA := 1500.0       # размер рисуемой области (px)
const STARS_FAR := 520
const STARS_NEAR := 200

var _far: Node2D
var _near: Node2D


func _ready() -> void:
	var rng := RandomNumberGenerator.new()
	rng.seed = 2026
	_far = _make_layer(rng, STARS_FAR, 1.4, 0.45)
	_near = _make_layer(rng, STARS_NEAR, 2.4, 0.85)


func _make_layer(rng: RandomNumberGenerator, count: int, max_size: float, alpha: float) -> Node2D:
	var layer := StarLayer.new()
	layer.points = PackedVector2Array()
	layer.sizes = PackedFloat32Array()
	layer.colors = PackedColorArray()
	for i in count:
		layer.points.append(Vector2(rng.randf_range(-AREA, AREA), rng.randf_range(-AREA, AREA)))
		layer.sizes.append(rng.randf_range(0.6, max_size))
		var c := Tokens.TEXT if rng.randf() > 0.2 else Tokens.ACCENT
		layer.colors.append(Color(c, rng.randf_range(0.4, 1.0) * alpha))
	add_child(layer)
	return layer


## Сдвиг по положению камеры в мире (параллакс: дальний слой движется медленнее).
func set_scroll(cam: Vector2) -> void:
	if _far == null:
		return
	_far.position = -cam * 4.0
	_near.position = -cam * 10.0


class StarLayer extends Node2D:
	var points: PackedVector2Array
	var sizes: PackedFloat32Array
	var colors: PackedColorArray

	func _draw() -> void:
		var center := get_viewport_rect().size * 0.5
		for i in points.size():
			draw_circle(center + points[i], sizes[i], colors[i])
