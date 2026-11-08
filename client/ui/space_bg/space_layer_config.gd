extends Resource
## Настройки одного слоя космического фона. Хранится в данных (JSON), редактируется в админке.
## Все числа проходят проверку диапазонов (from_dict): неверные значения ограничиваются,
## а замечание попадает в `warnings`.

enum Wrap { CLAMP, MIRROR }   # CLAMP: края не показываются (мягкое ограничение сдвига); MIRROR: бесконечное зеркальное повторение
enum Blend { ADD, MIX }       # ADD: складывается (звёзды, туманность); MIX: с прозрачностью (газ)

const QUALITIES := ["low", "medium", "high"]

@export var texture_key: String = ""
@export var blend: Blend = Blend.ADD
@export_range(0.0, 2.0, 0.01) var parallax: float = 0.2     # 0: не двигается, 1: двигается как мир
@export_range(1.0, 3.0, 0.01) var zoom: float = 1.2         # запас по размеру для сдвига
@export_range(0.0, 2.0, 0.01) var brightness: float = 1.0
@export_range(0.0, 2.0, 0.01) var intensity: float = 1.0
@export var tint: Color = Color.WHITE
@export var drift: Vector2 = Vector2.ZERO                  # медленное самодвижение, px/с
@export var wrap: Wrap = Wrap.CLAMP
@export var flip: Vector2i = Vector2i.ONE                  # (1|-1, 1|-1): отражение по осям
@export var min_quality: String = "low"                    # с какого пресета графики слой показывается

var warnings: PackedStringArray = PackedStringArray()


static func _clamp(d: Dictionary, key: String, default: float, lo: float, hi: float, w: PackedStringArray) -> float:
	var v := float(d.get(key, default))
	if is_nan(v) or is_inf(v):
		w.append("%s: некорректное число, взято %s" % [key, default])
		return default
	var c := clampf(v, lo, hi)
	if c != v:
		w.append("%s: %s вне диапазона [%s, %s], ограничено до %s" % [key, v, lo, hi, c])
	return c


static func from_dict(d: Dictionary) -> Resource:
	var s = load("res://ui/space_bg/space_layer_config.gd").new()
	var w := PackedStringArray()
	s.texture_key = str(d.get("texture", ""))
	if s.texture_key == "":
		w.append("texture: не задана")
	s.blend = Blend.MIX if str(d.get("blend", "add")) == "mix" else Blend.ADD
	s.parallax = _clamp(d, "parallax", 0.2, 0.0, 2.0, w)
	s.zoom = _clamp(d, "zoom", 1.2, 1.0, 3.0, w)
	s.brightness = _clamp(d, "brightness", 1.0, 0.0, 2.0, w)
	s.intensity = _clamp(d, "intensity", 1.0, 0.0, 2.0, w)
	var t: Variant = d.get("tint", [1.0, 1.0, 1.0])
	if t is Array and t.size() >= 3:
		s.tint = Color(clampf(float(t[0]), 0.0, 2.0), clampf(float(t[1]), 0.0, 2.0), clampf(float(t[2]), 0.0, 2.0), 1.0)
	else:
		w.append("tint: ожидается массив [r, g, b]")
	var dr: Variant = d.get("drift", [0.0, 0.0])
	if dr is Array and dr.size() >= 2:
		s.drift = Vector2(clampf(float(dr[0]), -60.0, 60.0), clampf(float(dr[1]), -60.0, 60.0))
	else:
		w.append("drift: ожидается массив [x, y]")
	s.wrap = Wrap.MIRROR if str(d.get("wrap", "clamp")) == "mirror" else Wrap.CLAMP
	var f: Variant = d.get("flip", [1, 1])
	if f is Array and f.size() >= 2:
		s.flip = Vector2i(-1 if float(f[0]) < 0.0 else 1, -1 if float(f[1]) < 0.0 else 1)
	var q := str(d.get("min_quality", "low"))
	if q in QUALITIES:
		s.min_quality = q
	else:
		w.append("min_quality: неизвестное значение «%s», взято low" % q)
	s.warnings = w
	return s


func to_dict() -> Dictionary:
	return {
		"texture": texture_key,
		"blend": "mix" if blend == Blend.MIX else "add",
		"parallax": parallax,
		"zoom": zoom,
		"brightness": brightness,
		"intensity": intensity,
		"tint": [tint.r, tint.g, tint.b],
		"drift": [drift.x, drift.y],
		"wrap": "mirror" if wrap == Wrap.MIRROR else "clamp",
		"flip": [flip.x, flip.y],
		"min_quality": min_quality,
	}
