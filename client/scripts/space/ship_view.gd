extends Node3D
## 3D-представление корабля или станции. ВРЕМЕННЫЕ модели-заглушки из примитивов:
## настоящие модели придут через менеджер ресурсов (см. docs/ARCHITECTURE.md).
## Мировые координаты (x, y) отображаются в 3D как (x, 0, y). Нос корабля смотрит в -Z.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const LABEL_ZOOM_HIDE := 26.0   # размер обзора, при котором подписи полностью скрыты
const LABEL_ZOOM_SHOW := 40.0   # и при котором полностью видны
const LABEL_NEAR_HIDE := 5.0    # расстояние до корабля игрока (ед. мира): скрыта
const LABEL_NEAR_SHOW := 11.0   # видна
const BASE_SPEED := 14.0        # обычная скорость, ед/с: по ней считается тяга двигателей
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ShipDef := preload("res://ui/ships/ship_def.gd")
const SnapshotBuffer := preload("res://scripts/space/snapshot_buffer.gd")

var object_id := ""
var kind := ""
var buffer = SnapshotBuffer.new()   # снимки состояния для плавного движения
var world_pos := Vector2.ZERO        # текущая отображаемая позиция (x, y мира)
var heading := Vector2(1, 0)         # текущее отображаемое направление

var _ring: MeshInstance3D
var _label: Label3D
var _label_alpha := 1.0
var me := false
var visual: Node3D        # визуальный адаптер корабля (ShipVisual); у станции нет
var ship_type := ""
var skin := ""
var boosting := false
var _last_t := -1.0
var _last_pos := Vector2.ZERO
var _last_dir := Vector2.ZERO
var _turn := 0.0
var _speed_frac := 0.0


static func color_for(kind_: String, stance: String, is_me: bool) -> Color:
	if is_me:
		return Tokens.ACCENT
	if kind_ == "station":
		return Tokens.SURFACE_RAISED.lightened(0.25)
	if stance == "hostile":
		return Tokens.DANGER
	return Tokens.ACCENT_2


func setup(id: String, kind_: String, stance: String, display_name: String, is_me: bool = false) -> void:
	object_id = id
	kind = kind_
	scale = Vector3.ONE * 1.6  # заглушки крупнее: при широком обзоре иначе почти не видны
	me = is_me
	var col := color_for(kind_, stance, is_me)
	var mat := StandardMaterial3D.new()
	mat.albedo_color = col
	mat.metallic = 0.3
	mat.roughness = 0.5
	mat.emission_enabled = true
	mat.emission = col
	mat.emission_energy_multiplier = 0.25
	if kind_ == "station":
		_add_box(Vector3(6.0, 1.2, 6.0), Vector3.ZERO, mat)
		_add_box(Vector3(9.0, 0.2, 1.0), Vector3(0, 0.3, 0), mat)
		_add_box(Vector3(1.0, 0.2, 9.0), Vector3(0, 0.3, 0), mat)
	else:
		# облик по умолчанию: у разных объектов чередуются два облика «Стрижа» (сервер присылает настоящий через set_ship)
		set_ship(ShipDef.DEFAULT_ID, str(ShipDef.skin_ids()[int(hash(id)) % 2]))
		var glow := MeshInstance3D.new()
		var sph := SphereMesh.new()
		sph.radius = 0.1
		sph.height = 0.2
		glow.mesh = sph
		var gm := StandardMaterial3D.new()
		gm.albedo_color = col
		gm.emission_enabled = true
		gm.emission = col
		gm.emission_energy_multiplier = 2.0
		glow.material_override = gm
		glow.position = Vector3(0, 0.45, 0.0)   # маркер принадлежности над кабиной
		add_child(glow)

	_ring = MeshInstance3D.new()
	var torus := TorusMesh.new()
	var r := 4.2 if kind_ == "station" else 1.9
	torus.inner_radius = r
	torus.outer_radius = r + 0.12
	_ring.mesh = torus
	var rm := StandardMaterial3D.new()
	rm.albedo_color = Tokens.RARE
	rm.emission_enabled = true
	rm.emission = Tokens.RARE
	rm.emission_energy_multiplier = 1.5
	rm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	_ring.material_override = rm
	_ring.visible = false
	add_child(_ring)

	_label = Label3D.new()
	_label.text = display_name
	_label.font = UiTheme.make_font(600)
	_label.font_size = 40
	_label.fixed_size = true       # постоянный размер на экране при любом масштабе камеры
	_label.pixel_size = 0.0012
	_label.outline_size = 10
	_label.outline_modulate = Tokens.BG
	_label.modulate = Tokens.TEXT
	_label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_label.no_depth_test = true
	_label.position = Vector3(0, 1.8, 0) if kind_ != "station" else Vector3(0, 2.5, 0)
	add_child(_label)


## Корабль и облик (по данным сервера). Повторный вызов с тем же обликом ничего не делает.
func set_ship(type_id: String, skin_id: String) -> void:
	if type_id == "" or not ShipDef.PATHS.has(type_id):
		type_id = ShipDef.DEFAULT_ID
	if visual != null and ship_type == type_id and skin == skin_id:
		return
	if visual == null or ship_type != type_id:
		if visual != null:
			visual.queue_free()
		visual = ShipVisual.new()
		visual.ship_id = type_id
		add_child(visual)
		visual.build(type_id, skin_id)
	else:
		visual.set_skin(skin_id)
	ship_type = type_id
	skin = visual.skin_id


## Событие сервера → эффект («Fire», «HitShield», «HitHull», «Death», «Boost», «Scan», «Respawn», «Dock»).
func play(effect: String, arg: Variant = null) -> void:
	if visual != null:
		visual.play(effect, arg)


func _add_box(size: Vector3, pos: Vector3, mat: Material) -> void:
	var m := MeshInstance3D.new()
	var box := BoxMesh.new()
	box.size = size
	m.mesh = box
	m.material_override = mat
	m.position = pos
	add_child(m)


## Мировая позиция (x, y) -> 3D (x, 0, y).
## Подпись не мешает обзору: исчезает при приближении камеры и вблизи корабля игрока.
## Выбранная цель подписана всегда. Возвращает текущую непрозрачность подписи.
func update_label(delta: float, zoom_size: float, dist_to_me: float) -> float:
	var a := smoothstep(LABEL_ZOOM_HIDE, LABEL_ZOOM_SHOW, zoom_size)
	if not me and not is_selected():
		a *= smoothstep(LABEL_NEAR_HIDE, LABEL_NEAR_SHOW, dist_to_me)
	if is_selected():
		a = maxf(a, 1.0)
	_label_alpha = lerpf(_label_alpha, a, clampf(delta * 10.0, 0.0, 1.0))
	_label.visible = _label_alpha > 0.02
	_label.modulate.a = _label_alpha
	_label.outline_modulate.a = _label_alpha
	return _label_alpha


func set_world_position(p: Vector2) -> void:
	world_pos = p
	position = Vector3(p.x, 0.0, p.y)


## Новый снимок состояния от шлюза (sim_t: игровое время, local_t: локальное время получения).
func push_snapshot(sim_t: float, local_t: float, pos: Vector2, dir: Vector2) -> void:
	buffer.push(sim_t, local_t, pos, dir)


## Обновить отображаемую позицию по буферу снимков (вызывать каждый кадр).
func update_interp(local_now: float) -> void:
	var s: Dictionary = buffer.sample(local_now)
	if s["valid"]:
		set_world_position(s["pos"])
		set_direction(s["dir"])
	if visual != null:
		var dt := local_now - _last_t if _last_t >= 0.0 else 0.0
		if dt > 0.0001 and dt < 0.5:
			var spd := world_pos.distance_to(_last_pos) / dt
			_speed_frac = lerpf(_speed_frac, spd / BASE_SPEED, clampf(dt * 10.0, 0.0, 1.0))
			if _last_dir != Vector2.ZERO:
				_turn = lerpf(_turn, angle_difference(_last_dir.angle(), heading.angle()) / dt, clampf(dt * 8.0, 0.0, 1.0))
			visual.update_motion(dt, _speed_frac, -_turn, boosting)
		_last_t = local_now
		_last_pos = world_pos
		_last_dir = heading


## Направление носа в мировых координатах.
func set_direction(dir: Vector2) -> void:
	if dir.length() < 0.001:
		return
	heading = dir.normalized()
	# локальный -Z должен смотреть в (dir.x, 0, dir.y)
	rotation.y = atan2(-dir.x, -dir.y)


func set_selected(on: bool) -> void:
	if _ring != null:
		_ring.visible = on


func is_selected() -> bool:
	return _ring != null and _ring.visible
