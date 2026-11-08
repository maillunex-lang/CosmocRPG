extends Control
## Многослойный космический фон: база + слои (звёзды, туманность, газ) с параллаксом.
## Каждый слой — полноэкранный прямоугольник с шейдером; текстуры, параллакс, яркость, интенсивность,
## оттенок и самодвижение берутся из профиля системы (SpaceBackgroundProfile) и пользовательских
## настроек (яркость/интенсивность/пресет графики/уменьшение анимаций).
##
## Использование: bg.apply_profile(BackgroundLibrary.profile("xi")); каждый кадр bg.set_camera(мировая_позиция).

const Library := preload("res://ui/space_bg/background_library.gd")
const Profile := preload("res://ui/space_bg/space_background_profile.gd")
const LayerConfig := preload("res://ui/space_bg/space_layer_config.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const SHADER_ADD := preload("res://ui/space_bg/space_layer_add.gdshader")
const SHADER_MIX := preload("res://ui/space_bg/space_layer_mix.gdshader")

const MIN_LEVEL := 0.0
const MAX_LEVEL := 1.25  # выше нельзя: при большем значении фон выходит из художественного стандарта (art_standard_test)

var profile: Resource
## Пользовательские множители (из настроек; set_levels меняет их в рантайме).
var user_brightness := 1.0
var user_intensity := 1.0

var _base: ColorRect
var _layers: Array = []   # словари: rect, cfg, tex_size, mat, drift_px, enabled
var _cam := Vector2.ZERO
var _dirty := true
var _last_size := Vector2.ZERO


static func quality_rank(q: String) -> int:
	return LayerConfig.QUALITIES.find(q)


## Мягкое ограничение сдвига: плавно стремится к ±margin, не достигая (края текстуры не показываются).
static func soft_limit(x: float, margin: float) -> float:
	if margin <= 1e-6:
		return 0.0
	return margin * tanh(x / margin)


## Положение видимой области в UV текстуры. viewport и tex в px; cam в единицах мира.
## Возвращает { offset: Vector2, extent: Vector2 }.
static func compute_uv(viewport: Vector2, tex: Vector2, zoom: float, parallax: float, cam: Vector2,
		px_per_unit: float, drift_px: Vector2, wrap: int) -> Dictionary:
	if viewport.x <= 0.0 or viewport.y <= 0.0 or tex.x <= 0.0 or tex.y <= 0.0:
		return {"offset": Vector2.ZERO, "extent": Vector2.ONE}
	var cover := maxf(viewport.x / tex.x, viewport.y / tex.y) * maxf(zoom, 1.0)  # px экрана на px текстуры
	var shown := tex * cover                      # размер текстуры на экране
	var extent := viewport / shown                # какая доля текстуры помещается на экран (<= 1)
	var margin := (Vector2.ONE - extent) * 0.5
	var shift := (cam * parallax * px_per_unit + drift_px) / shown
	if wrap == LayerConfig.Wrap.CLAMP:
		shift = Vector2(soft_limit(shift.x, margin.x), soft_limit(shift.y, margin.y))
	return {"offset": margin + shift, "extent": extent}


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_base = ColorRect.new()
	_base.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_base.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(_base)
	user_brightness = clampf(UiSettings.get_float("bg_brightness", 1.0), MIN_LEVEL, MAX_LEVEL)
	user_intensity = clampf(UiSettings.get_float("bg_intensity", 1.0), MIN_LEVEL, MAX_LEVEL)
	resized.connect(func() -> void: _dirty = true)
	if profile == null:
		apply_profile(Library.profile(""))


## Применить профиль системы: заменяет слои, текстуры и параметры.
func apply_profile(p: Resource) -> void:
	profile = p
	for l in _layers:
		l["rect"].queue_free()
	_layers.clear()
	if _base == null:
		return  # узел ещё не в дереве: слои соберутся в _ready
	_base.color = p.base_color
	var preset := UiSettings.graphics_preset()
	for cfg in p.layers:
		var role := "gas" if cfg.blend == LayerConfig.Blend.MIX else ("stars" if cfg.wrap == LayerConfig.Wrap.MIRROR else "nebula")
		var tex: Texture2D = Library.texture(cfg.texture_key, role)
		var mat := ShaderMaterial.new()
		mat.shader = SHADER_MIX if cfg.blend == LayerConfig.Blend.MIX else SHADER_ADD
		mat.set_shader_parameter("layer_texture", tex)
		mat.set_shader_parameter("tint", cfg.tint)
		mat.set_shader_parameter("mirror_wrap", cfg.wrap == LayerConfig.Wrap.MIRROR)
		mat.set_shader_parameter("uv_flip", Vector2(cfg.flip.x, cfg.flip.y))
		var rect := ColorRect.new()
		rect.mouse_filter = Control.MOUSE_FILTER_IGNORE
		rect.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
		rect.material = mat
		var enabled := quality_rank(preset) >= quality_rank(cfg.min_quality)
		rect.visible = enabled
		add_child(rect)
		_layers.append({"rect": rect, "cfg": cfg, "tex_size": tex.get_size(), "mat": mat, "drift_px": Vector2.ZERO, "enabled": enabled})
	_dirty = true
	_update()


## Положение камеры в мире (параллакс). Вызывать каждый кадр.
func set_camera(world: Vector2) -> void:
	if not world.is_equal_approx(_cam):
		_cam = world
		_dirty = true


## Пользовательские яркость и интенсивность (0..1.5).
func set_levels(brightness: float, intensity: float) -> void:
	user_brightness = clampf(brightness, MIN_LEVEL, MAX_LEVEL)
	user_intensity = clampf(intensity, MIN_LEVEL, MAX_LEVEL)
	_dirty = true


func layer_count() -> int:
	return _layers.size()


## Состояние слоя (для тестов и отладки).
func layer_info(i: int) -> Dictionary:
	var l: Dictionary = _layers[i]
	var m: ShaderMaterial = l["mat"]
	return {
		"enabled": l["enabled"], "visible": l["rect"].visible, "texture_key": l["cfg"].texture_key,
		"texture": m.get_shader_parameter("layer_texture"),
		"uv_offset": m.get_shader_parameter("uv_offset"), "uv_extent": m.get_shader_parameter("uv_extent"),
		"brightness": m.get_shader_parameter("brightness"), "intensity": m.get_shader_parameter("intensity"),
		"drift_px": l["drift_px"], "mirror": m.get_shader_parameter("mirror_wrap"),
	}


## Ключи недостающих текстур этого профиля (для них показаны заглушки).
func missing_assets() -> Array:
	var out: Array = []
	if profile == null:
		return out
	var missing := Library.missing_assets()
	for cfg in profile.layers:
		if missing.has(cfg.texture_key):
			out.append(cfg.texture_key)
	return out


func _process(delta: float) -> void:
	if profile == null or _layers.is_empty():
		return
	if not UiSettings.reduce_motion():
		for l in _layers:
			var d: Vector2 = l["cfg"].drift
			if l["enabled"] and d != Vector2.ZERO:
				l["drift_px"] += d * delta
				_dirty = true
	if size != _last_size:
		_dirty = true
	if _dirty:
		_update()


func _update() -> void:
	_dirty = false
	_last_size = size
	if profile == null or size.x <= 0.0 or size.y <= 0.0:
		return
	for l in _layers:
		var cfg = l["cfg"]
		var uv := compute_uv(size, l["tex_size"], cfg.zoom, cfg.parallax, _cam, profile.px_per_unit, l["drift_px"], cfg.wrap)
		var m: ShaderMaterial = l["mat"]
		m.set_shader_parameter("uv_offset", uv["offset"])
		m.set_shader_parameter("uv_extent", uv["extent"])
		m.set_shader_parameter("brightness", clampf(profile.brightness * cfg.brightness * user_brightness, 0.0, 3.0))
		m.set_shader_parameter("intensity", clampf(profile.intensity * cfg.intensity * user_intensity, 0.0, 3.0))


## Программная (без GPU) сборка фона профиля в изображение: для проверки стандарта, миниатюр
## и предпросмотра в админке. Формулы совпадают с шейдерами. Слои, скрытые пресетом, пропускаются.
static func compose_cpu(p: Resource, out_size: Vector2i, cam: Vector2 = Vector2.ZERO, preset: String = "high",
		user_b: float = 1.0, user_i: float = 1.0) -> Image:
	var img := Image.create_empty(out_size.x, out_size.y, false, Image.FORMAT_RGBF)
	img.fill(p.base_color)
	for cfg in p.layers:
		if quality_rank(preset) < quality_rank(cfg.min_quality):
			continue
		var role := "gas" if cfg.blend == LayerConfig.Blend.MIX else ("stars" if cfg.wrap == LayerConfig.Wrap.MIRROR else "nebula")
		var tex: Texture2D = Library.texture(cfg.texture_key, role)
		var src := tex.get_image()
		if src.is_compressed():
			src.decompress()
		if src.get_format() != Image.FORMAT_RGBAF:
			src.convert(Image.FORMAT_RGBAF)
		var tex_size := Vector2(src.get_width(), src.get_height())
		var uv := compute_uv(Vector2(out_size), tex_size, cfg.zoom, cfg.parallax, cam, p.px_per_unit, Vector2.ZERO, cfg.wrap)
		var off: Vector2 = uv["offset"]
		var ext: Vector2 = uv["extent"]
		var bright := clampf(p.brightness * cfg.brightness * user_b, 0.0, 3.0)
		var inten := clampf(p.intensity * cfg.intensity * user_i, 0.0, 3.0)
		for y in out_size.y:
			for x in out_size.x:
				var t := off + Vector2((x + 0.5) / out_size.x, (y + 0.5) / out_size.y) * ext
				if cfg.flip.x < 0:
					t.x = 1.0 - t.x
				if cfg.flip.y < 0:
					t.y = 1.0 - t.y
				if cfg.wrap == LayerConfig.Wrap.MIRROR:
					t = Vector2(1.0 - absf(1.0 - fposmod(t.x, 2.0)), 1.0 - absf(1.0 - fposmod(t.y, 2.0)))
				else:
					t = Vector2(clampf(t.x, 0.0, 1.0), clampf(t.y, 0.0, 1.0))
				var c := src.get_pixel(mini(int(t.x * tex_size.x), src.get_width() - 1), mini(int(t.y * tex_size.y), src.get_height() - 1))
				var rgb := Vector3(c.r * cfg.tint.r, c.g * cfg.tint.g, c.b * cfg.tint.b) * bright
				var dst := img.get_pixel(x, y)
				var dv := Vector3(dst.r, dst.g, dst.b)
				if cfg.blend == LayerConfig.Blend.MIX:
					var a := clampf(c.a * inten, 0.0, 1.0)
					dv = dv.lerp(rgb, a)
				else:
					dv += rgb * inten * c.a
				img.set_pixel(x, y, Color(minf(dv.x, 1.0), minf(dv.y, 1.0), minf(dv.z, 1.0)))
	return img
