extends RefCounted
## Временные технические заглушки для слоёв фона. Используются, когда текстура системы
## отсутствует или не загрузилась. Узнаваемы (тонкая техническая сетка), но в палитре игры.
## Каждая заглушка помечена метаданными: placeholder = true, role = звёзды/туманность/газ.

const W := 256
const H := 144
const GRID := 32


static func make(role: String) -> ImageTexture:
	var rng := RandomNumberGenerator.new()
	rng.seed = hash(role)
	var img := Image.create_empty(W, H, false, Image.FORMAT_RGBA8)
	for y in H:
		for x in W:
			var c := _pixel(role, x, y)
			if x % GRID == 0 or y % GRID == 0:
				c = Color(minf(c.r + 0.05, 1.0), minf(c.g + 0.10, 1.0), minf(c.b + 0.12, 1.0), maxf(c.a, 0.35) if role != "stars" else 0.25)
			img.set_pixel(x, y, c)
	if role == "stars":
		for i in 90:
			var p := Vector2i(rng.randi_range(0, W - 1), rng.randi_range(0, H - 1))
			var v := rng.randf_range(0.5, 1.0)
			img.set_pixelv(p, Color(v, v, 1.0, 1.0))
	var tex := ImageTexture.create_from_image(img)
	tex.set_meta("placeholder", true)
	tex.set_meta("role", role)
	tex.resource_name = "placeholder:" + role
	return tex


static func _pixel(role: String, x: int, y: int) -> Color:
	var u := float(x) / W
	var v := float(y) / H
	match role:
		"nebula":
			# тёмный центр, сине-фиолетовые края
			var d := clampf(Vector2(u - 0.5, v - 0.5).length() * 1.6, 0.0, 1.0)
			var violet := 0.5 + 0.5 * sin(u * 6.0 + v * 3.0)
			return Color(0.05 + 0.15 * violet * d, 0.08 * d, 0.30 * d + 0.05, 1.0)
		"gas":
			var a := maxf(0.0, sin(u * 9.0) * sin(v * 5.0 + 1.0)) * clampf(Vector2(u - 0.5, v - 0.5).length() * 1.8, 0.0, 1.0)
			return Color(0.25, 0.65, 0.85, a * 0.5)
		_:
			return Color(0.0, 0.0, 0.0, 1.0)  # звёзды: чёрный фон, точки добавляются отдельно


static func is_placeholder(tex: Texture2D) -> bool:
	return tex != null and tex.has_meta("placeholder")
