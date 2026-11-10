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
## Серверные интерактивные объекты живых локаций: рисуются процедурными заглушками (позже модели из Meshy AI).
const OBJECT_KINDS := ["deposit", "anomaly", "derelict", "signal"]
const RING_RADIUS := {"deposit": 3.4, "anomaly": 3.6, "derelict": 3.4, "signal": 1.6}
const RESOURCE_COLORS := {"iron_ore": Color("#b08968"), "crystals": Color("#7fe3ff"), "gas": Color("#9be37f"), "scrap": Color("#9aa4ad")}
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ShipDef := preload("res://ui/ships/ship_def.gd")
const SnapshotBuffer := preload("res://scripts/space/snapshot_buffer.gd")
const SfxRules := preload("res://audio/sfx.gd")

var object_id := ""
var kind := ""
var buffer = SnapshotBuffer.new()   # снимки состояния для плавного движения
var world_pos := Vector2.ZERO        # текущая отображаемая позиция (x, y мира)
var heading := Vector2(1, 0)         # текущее отображаемое направление

var _ring: MeshInstance3D
## false: подпись рисует экранный слой (ui/nameplates.gd), 3D-текст скрыт. По умолчанию true (тесты и режимы без слоя).
var use_3d_label := true
var _label: Label3D
var _label_alpha := 1.0
var _label_base_y := 1.8    # исходная высота подписи над объектом (задаётся в setup)
var _label_lift := 0.0      # временное вертикальное смещение от стекинга (мир. ед.)
var me := false
var visual: Node3D        # визуальный адаптер корабля (ShipVisual); у станции нет
var _model_evicted := false
var hidden_since := 0
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


func setup(id: String, kind_: String, stance: String, display_name: String, is_me: bool = false, info: Dictionary = {}) -> void:
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
	elif kind_ in OBJECT_KINDS:
		_build_world_object(kind_, id, display_name, info)
	else:
		# облик по умолчанию: у разных объектов чередуются два облика «Стрижа» (сервер присылает настоящий через set_ship)
		set_ship(str(info.get("ship", info.get("type", ShipDef.DEFAULT_ID))),
			str(info.get("skin", "")))
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
	var r := 4.2 if kind_ == "station" else (RING_RADIUS.get(kind_, 1.9) as float)
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
	_label.text = display_name + (" · врем." if kind_ in OBJECT_KINDS else "")
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
	if kind_ in OBJECT_KINDS:
		_label.position = Vector3(0, 2.6, 0)
	_label_base_y = _label.position.y
	add_child(_label)


## Заглушки месторождений, аномалий, заброшенных станций и сигналов. Цвет месторождения задаёт ресурс; есть мягкое свечение и пульсация.
func _build_world_object(kind_: String, id: String, display_name: String, info: Dictionary = {}) -> void:
	match kind_:
		"deposit":
			var rkey := str(info.get("resource", "")) if str(info.get("resource", "")) != "" else _resource_of(display_name)
			var col: Color = RESOURCE_COLORS.get(rkey, Color("#b08968"))
			var rng := RandomNumberGenerator.new()
			rng.seed = hash(id)
			for i in 5:
				var m := MeshInstance3D.new()
				var sph := SphereMesh.new()
				sph.radius = rng.randf_range(0.5, 1.1)
				sph.height = sph.radius * 2.0
				sph.radial_segments = 6
				sph.rings = 3
				m.mesh = sph
				var mat := StandardMaterial3D.new()
				mat.albedo_color = col.darkened(0.35)
				mat.emission_enabled = true
				mat.emission = col
				mat.emission_energy_multiplier = 1.4
				mat.roughness = 0.8
				m.material_override = mat
				m.position = Vector3(rng.randf_range(-1.6, 1.6), rng.randf_range(0.0, 0.4), rng.randf_range(-1.6, 1.6))
				add_child(m)
			# подсвеченный диск под месторождением: его видно среди декоративных камней
			var disc := MeshInstance3D.new()
			var dt := TorusMesh.new()
			dt.inner_radius = 2.1
			dt.outer_radius = 2.5
			disc.mesh = dt
			disc.scale = Vector3(1, 0.08, 1)
			var dmat := StandardMaterial3D.new()
			dmat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
			dmat.albedo_color = Color(col, 0.75)
			dmat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
			disc.material_override = dmat
			disc.position.y = -0.2
			add_child(disc)
		"anomaly":
			var t := MeshInstance3D.new()
			var tm := TorusMesh.new()
			tm.inner_radius = 2.2
			tm.outer_radius = 2.6
			t.mesh = tm
			var am := StandardMaterial3D.new()
			am.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
			am.albedo_color = Color(Tokens.ACCENT_2, 0.8)
			am.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
			t.material_override = am
			t.name = "AnomalyRing"
			add_child(t)
			var core := MeshInstance3D.new()
			var cs := SphereMesh.new()
			cs.radius = 0.8
			cs.height = 1.6
			core.mesh = cs
			var cm := StandardMaterial3D.new()
			cm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
			cm.albedo_color = Color(0.8, 0.7, 1.0)
			core.material_override = cm
			add_child(core)
		"derelict":
			var dm := StandardMaterial3D.new()
			dm.albedo_color = Color(0.25, 0.28, 0.32)
			dm.metallic = 0.5
			_add_box(Vector3(4.0, 0.8, 2.0), Vector3.ZERO, dm)
			_add_box(Vector3(1.0, 0.6, 5.0), Vector3(-0.6, 0.1, 0.4), dm)
			_add_box(Vector3(2.4, 0.3, 0.6), Vector3(0.8, 0.5, -1.4), dm)
		"signal":
			var b := MeshInstance3D.new()
			var bs := SphereMesh.new()
			bs.radius = 0.45
			bs.height = 0.9
			b.mesh = bs
			var bm := StandardMaterial3D.new()
			bm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
			bm.albedo_color = Tokens.ACCENT
			b.material_override = bm
			b.name = "Beacon"
			add_child(b)


## Ресурс по подписи месторождения (сервер присылает «Месторождение (Железная руда)»; точный ключ приходит в resource).
func _resource_of(display_name: String) -> String:
	for pair in [["Железная руда", "iron_ore"], ["Кристаллы", "crystals"], ["Газ", "gas"], ["Лом", "scrap"]]:
		if display_name.contains(pair[0]):
			return pair[1]
	return ""


## Корабль и облик (по данным сервера). Повторный вызов с тем же обликом ничего не делает.
func set_ship(type_id: String, skin_id: String) -> void:
	if type_id == "" or not ShipDef.PATHS.has(type_id):
		type_id = ShipDef.DEFAULT_ID
	if skin_id == "":
		skin_id = str(ShipDef.skin_ids(type_id)[0])
	if not ShipDef.get_def(type_id).get("skins", {}).has(skin_id):
		return
	if _model_evicted:
		ship_type = type_id
		skin = skin_id
		return
	if visual != null and ship_type == type_id and skin == skin_id and is_instance_valid(visual._skin_root) and not visual._skin_root.is_queued_for_deletion():
		return
	if visual == null or ship_type != type_id:
		if visual != null:
			visual.queue_free()
		visual = ShipVisual.new()
		visual.ship_id = type_id
		visual.skin_id = skin_id
		add_child(visual)
	else:
		visual.set_skin(skin_id)
	ship_type = type_id
	skin = visual.skin_id


## Событие сервера → эффект («Fire», «HitShield», «HitHull», «Death», «Boost», «Scan», «Respawn», «Dock»).
func play(effect: String, arg: Variant = null) -> void:
	if effect in ["Death", "Respawn"]:
		set_meta("dead", effect == "Death")
	if visual != null:
		if visual.visible or effect in ["Death", "Respawn"]:
			visual.play(effect, arg)
		if not visual.visible:
			visual._reset_effects()


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
	if not use_3d_label:
		_label.visible = false
		return 0.0
	var a := smoothstep(LABEL_ZOOM_HIDE, LABEL_ZOOM_SHOW, zoom_size)
	if not me and not is_selected():
		a *= smoothstep(LABEL_NEAR_HIDE, LABEL_NEAR_SHOW, dist_to_me)
	if is_selected():
		a = maxf(a, 1.0)
	_label_alpha = lerpf(_label_alpha, a, clampf(delta * 10.0, 0.0, 1.0))
	_label.visible = _label_alpha > 0.02
	_label.modulate.a = _label_alpha
	_label.outline_modulate.a = _label_alpha
	# стекинг: смещение к целевой высоте возвращается плавно, без дёрганья
	_label.position.y = lerpf(_label.position.y, _label_base_y + _label_lift, clampf(delta * 8.0, 0.0, 1.0))
	return _label_alpha


## Подпись сейчас на экране (с учётом прозрачности). Для стекинга.
func label_on_screen() -> bool:
	return _label.visible and _label_alpha > 0.5


## Экранная позиция якоря подписи (в пикселях вьюпорта камеры). Для стекинга подписей.
func label_screen_pos(cam: Camera3D) -> Vector2:
	return cam.unproject_position(_label.global_position)


## Половина ширины текста подписи на экране, px (оценка по длине текста).
func label_half_width_px() -> float:
	return _label.text.length() * 10.0


## Вертикальное смещение подписи от стекинга (мировые единицы; 0 = исходная высота).
func set_label_lift(lift: float) -> void:
	_label_lift = lift


func set_world_position(p: Vector2) -> void:
	world_pos = p
	position = Vector3(p.x, 0.0, p.y)


## Новый снимок состояния от шлюза (sim_t: игровое время, local_t: локальное время получения).
func push_snapshot(sim_t: float, local_t: float, pos: Vector2, dir: Vector2) -> void:
	buffer.push(sim_t, local_t, pos, dir)


## Обновить отображаемую позицию по буферу снимков (вызывать каждый кадр).
func update_interp(local_now: float) -> void:
	if kind in OBJECT_KINDS:
		_pulse(local_now)
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
			if visual.visible:
				visual.update_motion(dt, _speed_frac, -_turn, boosting)
		if me:
			_engine_sound()
		_last_t = local_now
		_last_pos = world_pos
		_last_dir = heading


## Мягкая пульсация аномалии и маяка (только внешний вид).
func _pulse(t: float) -> void:
	var ring := get_node_or_null("AnomalyRing") as Node3D
	if ring != null:
		var k := 1.0 + 0.12 * sin(t * 2.0)
		ring.scale = Vector3(k, 1.0, k)
		ring.rotation.y = t * 0.6
	var b := get_node_or_null("Beacon") as Node3D
	if b != null:
		var k2 := 0.8 + 0.35 * (0.5 + 0.5 * sin(t * 3.0))
		b.scale = Vector3.ONE * k2


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


func _exit_tree() -> void:
	if me:
		_engine_sound(false)


func _engine_sound(active: bool = true) -> void:
	var sfx := get_node_or_null("/root/Sfx")
	if sfx != null and sfx.has_method("set_engine"):
		if active:
			sfx.set_engine(SfxRules.engine_active(_speed_frac, boosting), _speed_frac, boosting)
		else:
			sfx.set_engine(false, 0.0, false)


## Only presentation changes. Snapshot interpolation and target picking remain active.
func set_full_visual(enabled: bool) -> void:
	if enabled and _model_evicted:
		_model_evicted = false
		set_ship(ship_type, skin)
		if visual != null:
			visual.alive = not bool(get_meta("dead", false))
			visual.bank.visible = visual.alive
	if visual == null or visual.visible == enabled:
		return
	visual.visible = enabled
	if not enabled:
		hidden_since = Time.get_ticks_usec()
		visual._reset_effects()
	else:
		visual.bank.visible = visual.alive


## Release only the presentation; identity, buffers and authoritative state survive.
func evict_hidden_model() -> void:
	if me or is_selected() or visual == null or visual.visible:
		return
	visual._reset_effects()
	remove_child(visual)
	visual.queue_free()
	visual = null
	_model_evicted = true
