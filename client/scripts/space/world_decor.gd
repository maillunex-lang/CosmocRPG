extends Node3D
## Клиентский декор живых локаций: планеты и луны, кольца зон, астероиды и обломки по чанкам (MultiMesh, пулинг),
## пыль, кометы, дальние корабли. Сервер декор не хранит и не синхронизирует: он строится по раскладке и seed, одинаково у всех.
## Серверные интерактивные объекты (месторождения, враги, станции, точки исследования) рисует ShipView по данным состояния.
## Все модели здесь процедурные заглушки (позже заменяются моделями из Meshy AI). Количество объектов ограничено пресетом качества.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const PlanetShader := preload("res://scripts/space/planet.gdshader")

const CHUNK := 50.0            # размер чанка декора, ед. мира
const HUB_FREE := 70.0         # вокруг главной станции декор не ставится: место для полётов и боёв
const PLANE_Y := -6.0          # декор лежит под плоскостью кораблей
const ZONE_COLORS := {"pve": Color("#F16C79"), "mining": Color("#F3C46B"), "explore": Color("#9575E8"), "trade": Color("#42C6E5")}

var layout: Dictionary = {}
var visual: Dictionary = {}
var preset_name := "medium"
var preset: Dictionary = {}
var chunks: Dictionary = {}          # Vector2i -> Node3D (видимые чанки)
var _pool: Array = []                # освобождённые чанки для повторного использования
var _rock_mesh: Mesh
var _debris_mesh: Mesh
var _rock_mat: StandardMaterial3D
var _debris_mat: StandardMaterial3D
var _statics: Node3D
var _dust: CPUParticles3D
var _comet: Node3D
var _comet_t := 0.0
var _comet_rng := RandomNumberGenerator.new()
var _lane_ships: Array = []
var _t := 0.0
var _last_chunk := Vector2i(1 << 30, 1 << 30)
var _blockers: Array = []            # [Vector3(x, y, radius)]: планеты, станции, зоны безопасности
var built := false


## Строит декор по раскладке и визуальным параметрам сервера. Повторный вызов пересобирает.
func build(lay: Dictionary, vis: Dictionary, preset_id: String) -> void:
	clear()
	layout = lay
	visual = vis
	var presets: Dictionary = vis.get("presets", {})
	preset_name = preset_id if presets.has(preset_id) else "medium"
	preset = presets.get(preset_name, {"chunk_radius": 1, "asteroids_per_chunk": 3, "debris_per_chunk": 1, "dust_particles": 0, "comets": false, "ships": 0})
	_make_assets()
	_statics = Node3D.new()
	_statics.name = "Statics"
	add_child(_statics)
	var decor: Dictionary = vis.get("decor", {})
	if bool(decor.get("planets", true)):
		for b in lay.get("bodies", []):
			if str(b.get("kind", "")) == "rock":
				_add_rock(b)
			else:
				_add_body(b)
	for z in lay.get("zones", []):
		_add_zone(z)
	for b in lay.get("bodies", []):
		_blockers.append(Vector3(_pos(b)[0], _pos(b)[1], float(b.get("radius", 0.0)) + 3.0))
	for s in lay.get("stations", []):
		_blockers.append(Vector3(_pos(s)[0], _pos(s)[1], float(s.get("safe_radius", 0.0))))
	if int(preset.get("dust_particles", 0)) > 0:
		_make_dust()
	_comet_rng.seed = int(lay.get("seed", 1)) ^ 0x5bd1e995
	_comet_t = _comet_rng.randf_range(8.0, 25.0)
	for i in int(round(float(preset.get("ships", 0)) * float(decor.get("ships", 1.0)))):
		_add_lane_ship(i)
	built = true


func clear() -> void:
	for c in get_children():
		c.queue_free()
	chunks.clear()
	_pool.clear()
	_blockers.clear()
	_lane_ships.clear()
	_dust = null
	_comet = null
	_last_chunk = Vector2i(1 << 30, 1 << 30)
	built = false


static func _pos(d: Dictionary) -> Array:
	var p: Array = d.get("pos", [0, 0])
	return [float(p[0]), float(p[1])]


func _make_assets() -> void:
	var rock := SphereMesh.new()
	rock.radius = 1.0
	rock.height = 2.0
	rock.radial_segments = 6
	rock.rings = 3
	_rock_mesh = rock
	var deb := BoxMesh.new()
	deb.size = Vector3(1.0, 0.25, 0.6)
	_debris_mesh = deb
	_rock_mat = StandardMaterial3D.new()
	_rock_mat.albedo_color = Color(0.36, 0.33, 0.32)
	_rock_mat.roughness = 1.0
	_rock_mat.vertex_color_use_as_albedo = true
	_debris_mat = StandardMaterial3D.new()
	_debris_mat.albedo_color = Color(0.45, 0.5, 0.55)
	_debris_mat.metallic = 0.6
	_debris_mat.roughness = 0.5
	_debris_mat.vertex_color_use_as_albedo = true


func _add_rock(b: Dictionary) -> void:
	var r := float(b.get("radius", 6.0))
	var p := _pos(b)
	var mi := MeshInstance3D.new()
	var rock := SphereMesh.new()
	rock.radius = r
	rock.height = r * 2.2
	rock.radial_segments = 7
	rock.rings = 4
	mi.mesh = rock
	mi.material_override = _rock_mat
	mi.position = Vector3(p[0], PLANE_Y + r * 0.25, p[1])
	mi.rotation = Vector3(0.35, float(hash(str(b.get("id", ""))) % 628) / 100.0, 0.2)
	_statics.add_child(mi)


func _add_body(b: Dictionary) -> void:
	var r := float(b.get("radius", 10.0))
	var p := _pos(b)
	var root := Node3D.new()
	root.name = str(b.get("id", "body"))
	root.position = Vector3(p[0], PLANE_Y - r, p[1])
	var mi := MeshInstance3D.new()
	var sph := SphereMesh.new()
	sph.radius = r
	sph.height = r * 2.0
	sph.radial_segments = 32 if preset_name != "low" else 16
	sph.rings = 16 if preset_name != "low" else 8
	mi.mesh = sph
	var mat := ShaderMaterial.new()
	mat.shader = PlanetShader
	var c: Array = b.get("color", [0.4, 0.55, 0.8])
	mat.set_shader_parameter("base_color", Vector3(float(c[0]), float(c[1]), float(c[2])))
	mat.set_shader_parameter("style", {"rocky": 0, "gas": 1, "ice": 2}.get(str(b.get("style", "rocky")), 0))
	mat.set_shader_parameter("seed", float(hash(str(b.get("id", ""))) % 1000) / 100.0)
	mi.material_override = mat
	root.add_child(mi)
	if bool(b.get("ring", false)):
		var ring := MeshInstance3D.new()
		var t := TorusMesh.new()
		t.inner_radius = r * 1.35
		t.outer_radius = r * 1.85
		t.rings = 48 if preset_name != "low" else 24
		t.ring_segments = 4
		ring.mesh = t
		ring.scale = Vector3(1, 0.03, 1)
		var rm := StandardMaterial3D.new()
		rm.albedo_color = Color(0.85, 0.8, 0.7, 0.45)
		rm.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
		rm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		ring.material_override = rm
		root.add_child(ring)
	var lab := Label3D.new()
	lab.text = "%s · временная модель" % str(b.get("name", ""))
	lab.font = UiTheme.make_font(500)
	lab.font_size = 34
	lab.fixed_size = true
	lab.pixel_size = 0.0011
	lab.modulate = Color(Tokens.TEXT, 0.7)
	lab.outline_size = 8
	lab.outline_modulate = Tokens.BG
	lab.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	lab.no_depth_test = true
	lab.position = Vector3(0, r + 8.0, 0)
	root.add_child(lab)
	_statics.add_child(root)


func _add_zone(z: Dictionary) -> void:
	var type := str(z.get("type", ""))
	var col: Color = ZONE_COLORS.get(type, Tokens.TEXT_MUTED)
	var p := _pos(z)
	var r := float(z.get("radius", 10.0))
	var root := Node3D.new()
	root.name = str(z.get("id", "zone"))
	root.position = Vector3(p[0], PLANE_Y + 5.0, p[1])
	var ring := MeshInstance3D.new()
	var t := TorusMesh.new()
	t.inner_radius = r - 0.35
	t.outer_radius = r
	t.rings = 64 if preset_name != "low" else 32
	t.ring_segments = 3
	ring.mesh = t
	ring.scale = Vector3(1, 0.05, 1)
	var m := StandardMaterial3D.new()
	m.albedo_color = Color(col, 0.38)
	m.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	m.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	ring.material_override = m
	root.add_child(ring)
	var lab := Label3D.new()
	lab.text = str(z.get("name", ""))
	lab.font = UiTheme.make_font(600)
	lab.font_size = 30
	lab.fixed_size = true
	lab.pixel_size = 0.001
	lab.modulate = Color(col, 0.85)
	lab.outline_size = 8
	lab.outline_modulate = Tokens.BG
	lab.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	lab.no_depth_test = true
	lab.position = Vector3(0, 0, -r)
	root.add_child(lab)
	_statics.add_child(root)


func _make_dust() -> void:
	_dust = CPUParticles3D.new()
	_dust.amount = int(preset.get("dust_particles", 24)) * 2
	_dust.lifetime = 6.0
	_dust.preprocess = 6.0
	_dust.emission_shape = CPUParticles3D.EMISSION_SHAPE_BOX
	_dust.emission_box_extents = Vector3(60, 2, 60)
	_dust.direction = Vector3(1, 0, 0.3)
	_dust.spread = 25.0
	_dust.initial_velocity_min = 0.6
	_dust.initial_velocity_max = 1.6
	_dust.gravity = Vector3.ZERO
	var q := QuadMesh.new()
	q.size = Vector2(0.18, 0.18)
	_dust.mesh = q
	var dm := StandardMaterial3D.new()
	dm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	dm.albedo_color = Color(0.75, 0.85, 1.0, 0.45)
	dm.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	dm.billboard_mode = BaseMaterial3D.BILLBOARD_ENABLED
	_dust.material_override = dm
	_dust.position.y = PLANE_Y + 4.0
	_dust.name = "Dust"
	add_child(_dust)


func _add_lane_ship(i: int) -> void:
	var lanes: Array = layout.get("lanes", [])
	var st: Dictionary = {}
	for s in layout.get("stations", []):
		st[str(s.get("id", ""))] = s
	var a := Vector2(float(layout.get("hub", [0, 0])[0]), float(layout.get("hub", [0, 0])[1]))
	var b := a + Vector2(160, 40)
	if lanes.size() > 0 and st.has(str(lanes[0].get("a", ""))) and st.has(str(lanes[0].get("b", ""))):
		var pa := _pos(st[str(lanes[0]["a"])])
		var pb := _pos(st[str(lanes[0]["b"])])
		a = Vector2(pa[0], pa[1])
		b = Vector2(pb[0], pb[1])
	var n := MeshInstance3D.new()
	var box := BoxMesh.new()
	box.size = Vector3(1.6, 0.4, 0.7)
	n.mesh = box
	var m := StandardMaterial3D.new()
	m.albedo_color = Color(0.5, 0.55, 0.6)
	m.emission_enabled = true
	m.emission = Color(0.4, 0.8, 1.0)
	m.emission_energy_multiplier = 0.6
	n.material_override = m
	n.name = "LaneShip%d" % i
	add_child(n)
	_lane_ships.append({"node": n, "a": a, "b": b, "phase": float(i) / maxf(1.0, float(preset.get("ships", 1))), "speed": 0.012 + 0.004 * float(i % 3)})


## Видимые чанки вокруг камеры (pos — положение камеры в мире), анимации, кометы. Вызывать каждый кадр.
func update(cam_pos: Vector2, delta: float) -> void:
	if not built:
		return
	_t += delta
	var c := Vector2i(int(floor(cam_pos.x / CHUNK)), int(floor(cam_pos.y / CHUNK)))
	if c != _last_chunk:
		_last_chunk = c
		_sync_chunks(c)
	if _dust != null:
		_dust.position.x = cam_pos.x
		_dust.position.z = cam_pos.y
	for s in _lane_ships:
		var ph := fposmod(float(s["phase"]) + _t * float(s["speed"]), 2.0)
		var f := ph if ph <= 1.0 else 2.0 - ph
		var pos: Vector2 = (s["a"] as Vector2).lerp(s["b"], f)
		var n := s["node"] as Node3D
		n.position = Vector3(pos.x, PLANE_Y + 2.0, pos.y)
		var d: Vector2 = ((s["b"] as Vector2) - (s["a"] as Vector2)).normalized() * (1.0 if ph <= 1.0 else -1.0)
		n.rotation.y = atan2(-d.y, d.x)
	if bool(preset.get("comets", false)):
		_update_comet(cam_pos, delta)


func visible_chunk_count() -> int:
	return chunks.size()


func instance_count() -> int:
	var n := 0
	for k in chunks:
		for mm in (chunks[k] as Node3D).get_children():
			n += (mm as MultiMeshInstance3D).multimesh.instance_count
	return n


func _sync_chunks(center: Vector2i) -> void:
	var r := int(preset.get("chunk_radius", 1))
	var want := {}
	for dx in range(-r, r + 1):
		for dy in range(-r, r + 1):
			want[Vector2i(center.x + dx, center.y + dy)] = true
	for k in chunks.keys():
		if not want.has(k):
			var node: Node3D = chunks[k]
			node.visible = false
			_pool.append(node)
			chunks.erase(k)
	for k in want:
		if not chunks.has(k):
			chunks[k] = _fill_chunk(k)


func _zone_factor(p: Vector2) -> float:
	var f := 1.0
	for z in layout.get("zones", []):
		var zp := _pos(z)
		if Vector2(zp[0], zp[1]).distance_to(p) <= float(z.get("radius", 0.0)):
			if str(z.get("type", "")) == "mining":
				f = maxf(f, 2.5)
			elif str(z.get("type", "")) == "trade":
				f = minf(f, 0.5)
	return f


func _blocked(p: Vector2) -> bool:
	var hub: Array = layout.get("hub", [0, 0])
	if Vector2(float(hub[0]), float(hub[1])).distance_to(p) < HUB_FREE:
		return true
	for b in _blockers:
		if Vector2(b.x, b.y).distance_to(p) < b.z:
			return true
	return false


## Чанк строится детерминированно: seed раскладки и координаты чанка определяют каждый камень.
func _fill_chunk(k: Vector2i) -> Node3D:
	var node: Node3D
	if _pool.size() > 0:
		node = _pool.pop_back()
		node.visible = true
		for c in node.get_children():
			c.queue_free()
	else:
		node = Node3D.new()
		add_child(node)
	var rng := RandomNumberGenerator.new()
	rng.seed = int(layout.get("seed", 1)) ^ (k.x * 73856093) ^ (k.y * 19349663)
	var origin := Vector2(k.x * CHUNK, k.y * CHUNK)
	var decor: Dictionary = visual.get("decor", {})
	var zf := _zone_factor(origin + Vector2(CHUNK, CHUNK) * 0.5)
	var na := int(round(float(preset.get("asteroids_per_chunk", 3)) * float(decor.get("asteroids", 1.0)) * zf))
	var nd := int(round(float(preset.get("debris_per_chunk", 1)) * float(decor.get("debris", 1.0)) * zf))
	_populate(node, _rock_mesh, _rock_mat, na, origin, rng, 0.5, 1.5, true)
	_populate(node, _debris_mesh, _debris_mat, nd, origin, rng, 0.7, 1.8, false)
	return node


func _populate(parent: Node3D, mesh: Mesh, mat: Material, count: int, origin: Vector2, rng: RandomNumberGenerator, smin: float, smax: float, tint: bool) -> void:
	var xforms: Array = []
	var colors: Array = []
	for i in count:
		var p := origin + Vector2(rng.randf() * CHUNK, rng.randf() * CHUNK)
		var s := rng.randf_range(smin, smax)
		var rot := rng.randf() * TAU
		var tilt := rng.randf_range(-0.4, 0.4)
		var col := Color.WHITE.darkened(rng.randf_range(0.0, 0.35)) if tint else Color.WHITE
		if _blocked(p):
			continue   # свободное пространство у станции и на планетах
		var b := Basis.from_euler(Vector3(tilt, rot, 0.0)).scaled(Vector3(s, s * rng.randf_range(0.6, 1.0), s * rng.randf_range(0.8, 1.2)))
		xforms.append(Transform3D(b, Vector3(p.x, PLANE_Y + 1.0 + rng.randf_range(0.0, 2.5), p.y)))
		colors.append(col)
	if xforms.is_empty():
		return   # пустой MultiMesh рисовал бы объект в начале координат
	var mm := MultiMesh.new()
	mm.transform_format = MultiMesh.TRANSFORM_3D
	mm.use_colors = true
	mm.mesh = mesh
	mm.instance_count = xforms.size()
	for i in xforms.size():
		mm.set_instance_transform(i, xforms[i])
		mm.set_instance_color(i, colors[i])
	var inst := MultiMeshInstance3D.new()
	var spots := PackedVector2Array()
	for x in xforms:
		spots.append(Vector2((x as Transform3D).origin.x, (x as Transform3D).origin.z))
	inst.set_meta("spots", spots)   # положения (для проверок: без отрисовки значения MultiMesh не читаются)
	inst.multimesh = mm
	inst.material_override = mat
	inst.cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	parent.add_child(inst)


func _update_comet(cam_pos: Vector2, delta: float) -> void:
	_comet_t -= delta
	if _comet == null:
		if _comet_t > 0.0:
			return
		_comet = Node3D.new()
		var head := MeshInstance3D.new()
		var sph := SphereMesh.new()
		sph.radius = 0.7
		sph.height = 1.4
		head.mesh = sph
		var hm := StandardMaterial3D.new()
		hm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		hm.albedo_color = Color(0.85, 0.95, 1.0)
		head.material_override = hm
		_comet.add_child(head)
		var tail := CPUParticles3D.new()
		tail.amount = 24
		tail.lifetime = 1.6
		tail.direction = Vector3(-1, 0, 0)
		tail.spread = 6.0
		tail.initial_velocity_min = 2.0
		tail.initial_velocity_max = 4.0
		tail.gravity = Vector3.ZERO
		var q := QuadMesh.new()
		q.size = Vector2(0.5, 0.5)
		tail.mesh = q
		var tm := StandardMaterial3D.new()
		tm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		tm.albedo_color = Color(0.6, 0.85, 1.0, 0.5)
		tm.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
		tm.billboard_mode = BaseMaterial3D.BILLBOARD_ENABLED
		tail.material_override = tm
		_comet.add_child(tail)
		var ang := _comet_rng.randf() * TAU
		var dir := Vector2(cos(ang), sin(ang))
		_comet.set_meta("dir", dir)
		_comet.position = Vector3(cam_pos.x - dir.x * 90.0, PLANE_Y + 6.0, cam_pos.y - dir.y * 90.0 + _comet_rng.randf_range(-30, 30))
		_comet.rotation.y = atan2(-dir.y, dir.x)
		add_child(_comet)
	var d: Vector2 = _comet.get_meta("dir")
	_comet.position += Vector3(d.x, 0, d.y) * 28.0 * delta
	if Vector2(_comet.position.x, _comet.position.z).distance_to(cam_pos) > 130.0:
		_comet.queue_free()
		_comet = null
		_comet_t = _comet_rng.randf_range(40.0, 90.0)
