extends RefCounted
## Фабрика эффектов корабля. Лёгкие средства для Android: материалы общие (один на цвет), частицы CPU с малым числом
## частиц и одноразовым запуском, вспышки и оболочки — меши с аддитивным материалом. Узлы создаются один раз и переиспользуются.

static var _mats: Dictionary = {}


## Аддитивный светящийся материал без освещения (общий для одинакового цвета).
static func glow(color: Color, energy: float = 1.0) -> StandardMaterial3D:
	var key := "%s|%s" % [color.to_html(), energy]
	if _mats.has(key):
		return _mats[key]
	var m := StandardMaterial3D.new()
	m.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	m.blend_mode = BaseMaterial3D.BLEND_MODE_ADD
	m.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	m.albedo_color = Color(color.r * energy, color.g * energy, color.b * energy, color.a)
	m.cull_mode = BaseMaterial3D.CULL_DISABLED
	m.no_depth_test = false
	_mats[key] = m
	return m


static func flame(color: Color) -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	var c := CylinderMesh.new()
	c.top_radius = 0.0
	c.bottom_radius = 0.07
	c.height = 0.4
	c.radial_segments = 8
	c.rings = 1
	mi.mesh = c
	mi.rotation_degrees = Vector3(0, 0, 90)   # ось вдоль +X модели (назад от сопла)
	mi.material_override = glow(Color(color, 0.85), 1.4)
	mi.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	return mi


static func flash(color: Color, radius: float, alpha: float = 0.9, energy: float = 2.0) -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	var s := SphereMesh.new()
	s.radius = radius
	s.height = radius * 2.0
	s.radial_segments = 8
	s.rings = 4
	mi.mesh = s
	mi.material_override = glow(Color(color, alpha), energy)
	mi.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	mi.visible = false
	return mi


static func bolt(color: Color) -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	var b := BoxMesh.new()
	b.size = Vector3(0.03, 0.03, 0.5)
	mi.mesh = b
	mi.material_override = glow(Color(color, 1.0), 2.5)
	mi.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	mi.visible = false
	return mi


static func shield_shell(radius: float) -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	var s := SphereMesh.new()
	s.radius = radius
	s.height = radius * 2.0
	s.radial_segments = 12
	s.rings = 6
	mi.mesh = s
	mi.material_override = glow(Color(0.3, 0.6, 1.0, 0.14), 1.1)
	mi.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	mi.visible = false
	return mi


## Одноразовые частицы (искры, обломки, след). amount небольшой: производительность на Android.
static func particles(color: Color, amount: int, lifetime: float, size: float, speed: float, gravity: Vector3 = Vector3.ZERO, one_shot: bool = true) -> CPUParticles3D:
	var p := CPUParticles3D.new()
	p.emitting = false
	p.one_shot = one_shot
	p.amount = amount
	p.lifetime = lifetime
	p.explosiveness = 0.9 if one_shot else 0.0
	p.direction = Vector3(0, 0, 0)
	p.spread = 180.0
	p.initial_velocity_min = speed * 0.4
	p.initial_velocity_max = speed
	p.gravity = gravity
	p.local_coords = false
	var q := QuadMesh.new()
	q.size = Vector2(size, size)
	p.mesh = q
	var m := glow(Color(color, 1.0), 2.0)
	p.material_override = m
	p.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	return p


static func ring(color: Color) -> MeshInstance3D:
	var mi := MeshInstance3D.new()
	var t := TorusMesh.new()
	t.inner_radius = 0.97
	t.outer_radius = 1.0
	t.rings = 24
	t.ring_segments = 4
	mi.mesh = t
	mi.material_override = glow(Color(color, 0.8), 1.5)
	mi.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	mi.visible = false
	return mi
