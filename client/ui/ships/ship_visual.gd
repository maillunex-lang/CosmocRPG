extends Node3D
## Визуальный адаптер корабля (сцена-обёртка над GLB, оригинальные файлы не правятся): облик, точки привязки,
## двигатели и эффекты. Без скелета: анимации задаются кодом (твины и параметры).
## Состояния: Idle, Move, Turn, Boost; события: Fire, HitShield, HitHull, Death, Dock.
## Игровую логику не содержит: показывает то, что решил сервер.

const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVfx := preload("res://ui/ships/ship_vfx.gd")

const POINT_NAMES := ["Engine_L", "Engine_R", "Weapon_L", "Weapon_R", "Missile_01", "Hit_Center", "Camera_Focus", "Dock_Point"]

var build_count := 0
var skin_instance_count := 0
var _loaded_ship := ""
var ship_id := ShipDef.DEFAULT_ID
var skin_id := ""
var bank: Node3D       # наклон при манёвре (крен)
var model: Node3D      # ориентация и масштаб GLB
var points: Dictionary = {}   # имя -> Marker3D
var flames: Array = []
var anim_state := "Idle"
var alive := true

var _skin_root: Node
var _flame_color := Color(0.35, 0.9, 1.0)
var _t := 0.0
var _thrust := 0.0       # сглаженная тяга 0..2
var _bank := 0.0
var _boost_trail: CPUParticles3D
var _move_trail: CPUParticles3D
var _flash: Array = []
var _bolts: Array = []
var _shell: MeshInstance3D
var _sparks: CPUParticles3D
var _boom: CPUParticles3D
var _boom_flash: MeshInstance3D
var _scan_ring: MeshInstance3D
var _skill_ring: MeshInstance3D
var _skill_flash: MeshInstance3D
var _skill_sparks: CPUParticles3D
var _skill_bolt: MeshInstance3D
var _dock_tween: Tween


func _ready() -> void:
	if model == null:
		build(ship_id, skin_id)


## Строит иерархию: Bank > Model (ориентация, масштаб) > GLB + точки + двигатели. Повторный вызов пересобирает.
func build(sid: String, skin: String = "") -> void:
	build_count += 1
	ship_id = sid if ShipDef.PATHS.has(sid) else ShipDef.DEFAULT_ID
	_skin_root = null
	_loaded_ship = ""
	for c in get_children():
		c.queue_free()
	points.clear()
	flames.clear()
	_flash.clear()
	_bolts.clear()
	_shell = null
	_sparks = null
	_boom = null
	_boom_flash = null
	_scan_ring = null
	_skill_ring = null
	_skill_flash = null
	_skill_sparks = null
	_skill_bolt = null
	_boost_trail = null
	_move_trail = null
	var def := ShipDef.get_def(ship_id)
	bank = Node3D.new()
	bank.name = "Bank"
	add_child(bank)
	model = Node3D.new()
	model.name = "Model"
	model.rotation_degrees.y = -90.0   # нос GLB смотрит в -X: поворачиваем в локальный -Z
	model.scale = Vector3.ONE * float(def.get("visual_scale", 2.2))
	bank.add_child(model)
	var pts: Dictionary = def.get("points", {})
	for n in POINT_NAMES:
		var m := Marker3D.new()
		m.name = n
		var v: Array = pts.get(n, [0, 0, 0])
		m.position = Vector3(v[0], v[1], v[2])
		model.add_child(m)
		points[n] = m
	set_skin(skin if skin != "" else str(ShipDef.skin_ids(ship_id)[0]))
	for n in ["Engine_L", "Engine_R"]:
		var f := ShipVfx.flame(_flame_color)
		f.position = Vector3(0.2, 0, 0)
		f.name = "Flame"
		(points[n] as Marker3D).add_child(f)
		flames.append(f)


## Смена облика: меняется только GLB и цвет двигателей, характеристики у всех обликов одинаковые.
func set_skin(skin: String) -> void:
	if not (ShipDef.get_def(ship_id).get("skins", {}) as Dictionary).has(skin):
		return   # неизвестный облик: выбор не меняется
	if is_instance_valid(_skin_root) and not _skin_root.is_queued_for_deletion() and skin_id == skin and _loaded_ship == ship_id:
		return
	var sk := ShipDef.skin(ship_id, skin)
	var path := str(sk.get("glb", ""))
	if not ResourceLoader.exists(path):
		return
	var ps := load(path) as PackedScene
	if ps == null:
		return
	var replacement := ps.instantiate()
	if replacement == null:
		return
	if is_instance_valid(_skin_root):
		model.remove_child(_skin_root)
		_skin_root.queue_free()
	_skin_root = replacement
	model.add_child(_skin_root)
	skin_id = skin
	_loaded_ship = ship_id
	skin_instance_count += 1
	var tint: Array = sk.get("tint", [])
	if tint.size() >= 3:
		_apply_tint(_skin_root, Color(float(tint[0]), float(tint[1]), float(tint[2])))
	var fc: Array = sk.get("flame", [0.35, 0.9, 1.0])
	_flame_color = Color(fc[0], fc[1], fc[2])
	for f in flames:
		(f as MeshInstance3D).material_override = ShipVfx.glow(Color(_flame_color, 0.85), 1.4)
	_boost_trail = null
	_move_trail = null
	_flash.clear()


## Окраска заглушек: цвет умножается на текстуру корпуса (материалы копируются, оригинальный GLB не меняется).
func _apply_tint(node: Node, color: Color) -> void:
	if node is MeshInstance3D:
		var mi := node as MeshInstance3D
		if mi.mesh != null:
			for i in mi.mesh.get_surface_count():
				var m := mi.get_active_material(i)
				if m is BaseMaterial3D:
					var c := (m as BaseMaterial3D).duplicate() as BaseMaterial3D
					c.albedo_color = c.albedo_color * color
					mi.set_surface_override_material(i, c)
	for ch in node.get_children():
		_apply_tint(ch, color)


func point(n: String) -> Marker3D:
	return points.get(n)


# --- состояния ---

## Вызывать каждый кадр: speed — доля от обычной скорости (0..1+), turn — угловая скорость, рад/с (знак — направление), boosting — форсаж.
func update_motion(delta: float, speed_frac: float, turn: float, boosting: bool) -> void:
	if not alive:
		return
	_t += delta
	var target := 0.45 + clampf(speed_frac, 0.0, 1.0) * 0.6
	if boosting:
		target = 2.0
	_thrust = lerpf(_thrust, target, clampf(delta * 8.0, 0.0, 1.0))
	anim_state = "Boost" if boosting else ("Turn" if absf(turn) > 0.6 else ("Move" if speed_frac > 0.05 else "Idle"))
	var pulse := 1.0 + 0.08 * sin(_t * 9.0)
	for f in flames:
		var fm := f as MeshInstance3D
		fm.scale = Vector3(_thrust * pulse, 1.0 + (_thrust - 0.45) * 0.25, 1.0 + (_thrust - 0.45) * 0.25)
		fm.position.x = 0.2 * _thrust
	var want_bank := clampf(-turn * 0.35, -0.55, 0.55)
	_bank = lerpf(_bank, want_bank, clampf(delta * 6.0, 0.0, 1.0))
	bank.rotation.z = _bank
	if boosting:
		if _boost_trail == null:
			_boost_trail = ShipVfx.particles(_flame_color, 28, 0.55, 0.16, 0.2, Vector3.ZERO, false)
			_boost_trail.spread = 8.0
			_boost_trail.direction = Vector3(1, 0, 0)
			(points["Dock_Point"] as Marker3D).add_child(_boost_trail)
		_boost_trail.emitting = true
	elif _boost_trail != null:
		_boost_trail.emitting = false
	# лёгкий след на обычном ходу: корабль читается в движении даже без форсажа
	var moving := speed_frac > 0.25
	if moving and not boosting:
		if _move_trail == null:
			_move_trail = ShipVfx.particles(_flame_color.lightened(0.2), 10, 0.4, 0.12, 0.12, Vector3.ZERO, false)
			_move_trail.spread = 6.0
			_move_trail.direction = Vector3(1, 0, 0)
			(points["Dock_Point"] as Marker3D).add_child(_move_trail)
		_move_trail.emitting = true
	elif _move_trail != null:
		_move_trail.emitting = false


# --- события ---

func _tween_flash(mi: MeshInstance3D, scale_to: float, seconds: float) -> void:
	mi.visible = true
	mi.scale = Vector3.ONE * 0.3
	var tw := create_tween()
	tw.tween_property(mi, "scale", Vector3.ONE * scale_to, seconds * 0.6)
	tw.tween_callback(func() -> void: mi.visible = false).set_delay(seconds * 0.4)


## Fire: вспышки на обеих пушках и два заряда к цели (target — мировая позиция).
func play_fire(target: Variant = null) -> void:
	if not alive:
		return
	if _flash.is_empty():
		for n in ["Weapon_L", "Weapon_R"]:
			var f := ShipVfx.flash(_flame_color.lightened(0.3), 0.05)
			(points[n] as Marker3D).add_child(f)
			_flash.append(f)
			var b := ShipVfx.bolt(_flame_color.lightened(0.3))
			add_child(b)
			_bolts.append(b)
	for i in _flash.size():
		_tween_flash(_flash[i], 1.6, 0.1)
		if target is Vector3:
			var b: MeshInstance3D = _bolts[i]
			var from: Vector3 = (points["Weapon_L" if i == 0 else "Weapon_R"] as Marker3D).global_position
			b.top_level = true
			b.global_position = from
			b.look_at(target, Vector3.UP) if from.distance_to(target) > 0.01 else null
			b.visible = true
			var tw := create_tween()
			tw.tween_property(b, "global_position", target, 0.14)
			tw.tween_callback(func() -> void: b.visible = false)


## HitShield: сине-голубая оболочка вокруг корабля на долю секунды.
func play_hit_shield() -> void:
	if not alive:
		return
	if _shell == null:
		_shell = ShipVfx.shield_shell(0.95)
		(points["Hit_Center"] as Marker3D).add_child(_shell)
	_shell.visible = true
	_shell.scale = Vector3.ONE * 0.9
	var tw := create_tween()
	tw.tween_property(_shell, "scale", Vector3.ONE * 1.12, 0.22)
	tw.tween_callback(func() -> void: _shell.visible = false)


## HitHull: искры и короткая встряска.
func play_hit_hull() -> void:
	if not alive:
		return
	if _sparks == null:
		_sparks = ShipVfx.particles(Color(1.0, 0.7, 0.3), 14, 0.45, 0.07, 2.2, Vector3.ZERO)
		(points["Hit_Center"] as Marker3D).add_child(_sparks)
	_sparks.restart()
	_sparks.emitting = true
	var tw := create_tween()
	tw.tween_property(bank, "position:x", 0.04, 0.03)
	tw.tween_property(bank, "position:x", -0.04, 0.05)
	tw.tween_property(bank, "position:x", 0.0, 0.04)


## Death: вспышка, ограниченные по времени обломки, корабль скрывается до respawn().
func play_death() -> void:
	if not alive:
		return
	alive = false
	anim_state = "Death"
	if _boom == null:
		_boom = ShipVfx.particles(Color(1.0, 0.55, 0.2), 22, 1.2, 0.12, 3.0, Vector3.ZERO)
		(points["Hit_Center"] as Marker3D).add_child(_boom)
		_boom_flash = ShipVfx.flash(Color(1.0, 0.6, 0.25), 0.6, 0.35, 1.2)
		(points["Hit_Center"] as Marker3D).add_child(_boom_flash)
	_boom.restart()
	_boom.emitting = true
	_tween_flash(_boom_flash, 2.2, 0.5)
	for f in flames:
		(f as MeshInstance3D).visible = false
	var tw := create_tween()
	tw.tween_property(bank, "scale", Vector3.ONE * 0.01, 0.35)
	tw.tween_callback(func() -> void: bank.visible = false)


func respawn() -> void:
	alive = true
	anim_state = "Idle"
	bank.visible = true
	bank.scale = Vector3.ONE
	bank.position = Vector3.ZERO
	for f in flames:
		(f as MeshInstance3D).visible = true


## Boost: короткая вспышка двигателей (устойчивое усиление задаёт update_motion с boosting).
func play_boost() -> void:
	_thrust = 2.0


## Dock: корабль плавно «втягивается» в док: уменьшается и тускнеет; undock() возвращает.
func play_dock() -> void:
	if _dock_tween != null and _dock_tween.is_valid():
		_dock_tween.kill()
	anim_state = "Dock"
	if _scan_ring == null:
		_scan_ring = ShipVfx.ring(Color(0.5, 0.9, 1.0))
		_scan_ring.rotation_degrees.x = 90
		(points["Hit_Center"] as Marker3D).add_child(_scan_ring)
	_scan_ring.visible = true
	_scan_ring.scale = Vector3.ONE * 1.6
	_dock_tween = create_tween().set_parallel(true)
	_dock_tween.tween_property(bank, "scale", Vector3.ONE * 0.35, 1.0)
	_dock_tween.tween_property(bank, "position:z", -0.6, 1.0)
	_dock_tween.tween_property(_scan_ring, "scale", Vector3.ONE * 0.5, 1.0)


func undock() -> void:
	if _dock_tween != null and _dock_tween.is_valid():
		_dock_tween.kill()
	bank.scale = Vector3.ONE
	bank.position = Vector3.ZERO
	anim_state = "Idle"
	if _scan_ring != null:
		_scan_ring.visible = false


## Кольцо сканера (радиус в мировых единицах).
func play_scan(radius: float) -> void:
	if _scan_ring == null:
		_scan_ring = ShipVfx.ring(Color(0.5, 0.9, 1.0))
		_scan_ring.rotation_degrees.x = 90
		(points["Hit_Center"] as Marker3D).add_child(_scan_ring)
	_scan_ring.visible = true
	_scan_ring.scale = Vector3.ONE * 0.5
	var tw := create_tween()
	tw.tween_property(_scan_ring, "scale", Vector3.ONE * radius * 0.5, 0.8)
	tw.tween_callback(func() -> void: _scan_ring.visible = false)


func play_burst(color: Color, radius: float) -> void:
	_pulse_ring(color, radius, 0.45)


func _skill_fx() -> bool:
	if points.is_empty() or not points.has("Hit_Center"):
		return false
	var center := points["Hit_Center"] as Marker3D
	if _skill_ring == null:
		_skill_ring = ShipVfx.ring(Color.WHITE)
		_skill_ring.rotation_degrees.x = 90
		center.add_child(_skill_ring)
		_skill_flash = ShipVfx.flash(Color.WHITE, 0.45)
		center.add_child(_skill_flash)
		_skill_sparks = ShipVfx.particles(Color(1, 0.8, 0.4), 16, 0.4, 0.08, 2.4)
		center.add_child(_skill_sparks)
		_skill_bolt = ShipVfx.bolt(Color(1, 0.85, 0.4))
		add_child(_skill_bolt)
	return true


func _pulse_ring(color: Color, radius: float, seconds: float) -> void:
	if not _skill_fx():
		return
	_skill_ring.material_override = ShipVfx.glow(Color(color, 0.9), 1.8)
	_skill_ring.visible = true
	_skill_ring.scale = Vector3.ONE * 0.35
	var tw := create_tween()
	tw.tween_property(_skill_ring, "scale", Vector3.ONE * maxf(radius, 0.8), seconds)
	tw.tween_callback(func() -> void: _skill_ring.visible = false)


func _pulse_flash(color: Color, scale_to: float) -> void:
	if not _skill_fx():
		return
	_skill_flash.material_override = ShipVfx.glow(Color(color, 0.85), 2.2)
	_tween_flash(_skill_flash, scale_to, 0.28)


func _burst_sparks(color: Color) -> void:
	if not _skill_fx():
		return
	_skill_sparks.material_override = ShipVfx.glow(Color(color, 1), 2.0)
	_skill_sparks.restart()
	_skill_sparks.emitting = true


## Удар умения: отдельный снаряд, не путать с очередью автоатаки.
func play_strike(target: Variant) -> void:
	if not alive or not _skill_fx():
		return
	_pulse_flash(Color(1.0, 0.75, 0.35), 1.4)
	if target is Vector3 and points.has("Weapon_L"):
		var from: Vector3 = (points["Weapon_L"] as Marker3D).global_position
		var b := _skill_bolt
		b.material_override = ShipVfx.glow(Color(1, 0.8, 0.35, 1), 2.6)
		b.top_level = true
		b.global_position = from
		if from.distance_to(target) > 0.01:
			b.look_at(target, Vector3.UP)
		b.visible = true
		var tw := create_tween()
		tw.tween_property(b, "global_position", target, 0.1)
		tw.tween_callback(func() -> void: b.visible = false)


func play_dash(target: Variant) -> void:
	play_boost()
	play_strike(target)
	_pulse_ring(Color(0.5, 0.85, 1.0), 1.6, 0.25)


func play_nova(radius: float) -> void:
	_pulse_ring(Color(1.0, 0.42, 0.12), radius, 0.45)
	_burst_sparks(Color(1.0, 0.5, 0.15))
	_pulse_flash(Color(1.0, 0.45, 0.1), 2.4)


func play_barrier() -> void:
	_pulse_ring(Color(0.35, 0.7, 1.0), 1.8, 0.35)
	_pulse_flash(Color(0.45, 0.75, 1.0), 1.8)


func play_mend() -> void:
	_burst_sparks(Color(0.4, 1.0, 0.55))
	_pulse_ring(Color(0.45, 1.0, 0.6), 1.3, 0.4)


func play_drone_fx() -> void:
	_burst_sparks(Color(0.7, 0.85, 1.0))
	_pulse_ring(Color(0.65, 0.8, 1.0), 1.2, 0.3)


func play_fortify() -> void:
	_pulse_ring(Color(0.95, 0.75, 0.25), 1.7, 0.5)
	_pulse_flash(Color(0.95, 0.7, 0.2), 1.5)


func play_vent() -> void:
	_burst_sparks(Color(0.3, 0.95, 1.0))
	_pulse_flash(Color(0.35, 0.9, 1.0), 1.6)


## Воспроизвести событие по имени («Fire», «HitShield», …) — единая точка входа для сетевых событий.
func play(name: String, arg: Variant = null) -> void:
	match name:
		"Fire": play_fire(arg)
		"HitShield": play_hit_shield()
		"HitHull": play_hit_hull()
		"Death": play_death()
		"Boost": play_boost()
		"Dock": play_dock()
		"Scan": play_scan(float(arg) if arg != null else 20.0)
		"Respawn": respawn()
		"Strike": play_strike(arg)
		"Dash": play_dash(arg)
		"Nova": play_nova(float(arg) if arg != null else 8.0)
		"Barrier": play_barrier()
		"Mend": play_mend()
		"Drone": play_drone_fx()
		"Fortify": play_fortify()
		"Vent": play_vent()
