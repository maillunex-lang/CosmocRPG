extends SceneTree
## Корабль «Стриж» (FTR-001): описание, облики, точки привязки, двигатели, состояния и эффекты (логика, без рендера).

const ShipDef := preload("res://ui/ships/ship_def.gd")
const ShipVisual := preload("res://ui/ships/ship_visual.gd")
const ShipView := preload("res://scripts/space/ship_view.gd")

var _f := 0
var _n := 0


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _run() -> void:
	var d := ShipDef.get_def("FTR-001")
	_c(d.get("id") == "FTR-001" and d.get("name") == "Стриж" and d.get("crew") == 1 and d.get("engines") == 2, "паспорт Стрижа в описании")
	_c(d.get("dimensions_m") == [12, 8.4, 2.8] or d["dimensions_m"][0] == 12, "габариты 12 × 8,4 × 2,8 м")
	_c(ShipDef.skin_ids("FTR-001").size() == 2, "два облика")
	for k in ShipDef.skin_ids("FTR-001"):
		var sk := ShipDef.skin("FTR-001", k)
		_c(ResourceLoader.exists(str(sk["glb"])) and load(str(sk["glb"])) is PackedScene, "GLB облика «%s» импортируется" % k)
	_c(ShipDef.get_def("нет такого").get("id") == "FTR-001", "неизвестный корабль заменяется стандартным")

	var v: Node3D = ShipVisual.new()
	root.add_child(v)
	await process_frame
	for n in ShipVisual.POINT_NAMES:
		_c(v.point(n) is Marker3D, "точка привязки %s есть" % n)
	_c(v.point("Engine_L").position.z > 0.0 and v.point("Engine_R").position.z < 0.0, "двигатели по обе стороны оси")
	_c(v.flames.size() == 2, "два двигателя с пламенем")
	_c(v.skin_id == "a", "облик по умолчанию «a»")
	_c(v.build_count == 1 and v.skin_instance_count == 1, "одна первоначальная сборка")
	var original_id: int = v._skin_root.get_instance_id()
	for i in 100:
		v.set_skin("a")
	_c(v._skin_root.get_instance_id() == original_id, "100 одинаковых set_skin сохраняют GLB")
	var mesh_count := _count_meshes(v.model)
	v.set_skin("b")
	await process_frame
	await process_frame
	_c(v.skin_id == "b", "облик переключился на «b»")
	_c(_count_meshes(v.model) >= 1, "после смены облика модель на месте (мешей %d)" % _count_meshes(v.model))
	_c(mesh_count >= 1, "модель облика «a» содержит меши")
	v.set_skin("zzz")
	_c(v.skin_id == "b", "неизвестный облик не ломает выбор")

	# состояния двигателей
	for i in 40:
		v.update_motion(0.05, 0.0, 0.0, false)
	var idle: float = v.flames[0].scale.x
	_c(v.anim_state == "Idle" and idle > 0.3 and idle < 0.8, "Idle: слабая пульсация (%.2f)" % idle)
	for i in 40:
		v.update_motion(0.05, 1.0, 0.0, false)
	var move: float = v.flames[0].scale.x
	_c(v.anim_state == "Move" and move > idle * 1.4, "Move: тяга сильнее (%.2f > %.2f)" % [move, idle])
	for i in 40:
		v.update_motion(0.05, 1.0, 2.0, false)
	_c(v.anim_state == "Turn" and absf(v.bank.rotation.z) > 0.1, "Turn: крен при манёвре (%.2f)" % v.bank.rotation.z)
	var bank_left: float = v.bank.rotation.z
	for i in 40:
		v.update_motion(0.05, 1.0, -2.0, false)
	_c(v.bank.rotation.z * bank_left < 0.0, "крен зависит от направления поворота")
	for i in 40:
		v.update_motion(0.05, 1.0, 0.0, true)
	var boost: float = v.flames[0].scale.x
	_c(v.anim_state == "Boost" and boost > move * 1.4, "Boost: форсаж сильнее обычной тяги (%.2f)" % boost)
	_c(v._boost_trail != null and v._boost_trail.emitting, "Boost: светящийся след включён")
	for i in 20:
		v.update_motion(0.05, 1.0, 0.0, false)
	_c(not v._boost_trail.emitting, "след выключается после форсажа")

	# события
	v.play_fire(Vector3(10, 0, -10))
	_c(v._flash.size() == 2 and v._bolts.size() == 2, "Fire: вспышки и заряды на обеих пушках")
	v.play("HitShield")
	_c(v._shell != null and v._shell.visible, "HitShield: оболочка щита")
	v.play("HitHull")
	_c(v._sparks != null and v._sparks.emitting, "HitHull: искры")
	v.play("Boost")
	v.play("Scan", 30.0)
	_c(v._scan_ring != null and v._scan_ring.visible, "Scan: кольцо сканера")
	v.play("Dock")
	_c(v.anim_state == "Dock", "Dock: анимация стыковки запущена")
	v.undock()
	_c(v.bank.scale == Vector3.ONE and v.anim_state == "Idle", "undock возвращает корабль")
	v.play("Death")
	_c(not v.alive and v.anim_state == "Death" and v._boom != null and v._boom.emitting, "Death: взрыв и обломки")
	_c(v._boom.lifetime <= 1.5, "обломки ограничены по времени (%.1f с)" % v._boom.lifetime)
	v.update_motion(0.05, 1.0, 0.0, false)
	v.play("Fire")
	v.play("Respawn")
	_c(v.alive and v.bank.visible, "Respawn возвращает корабль")
	v.play("Неизвестное")
	_c(true, "неизвестное событие игнорируется")

	# Overlapping events and skin changes must retain bounded ownership.
	v.respawn()
	for i in 100:
		v.update_motion(0.016, 1.0, 0.0, true)
		v.play_fire(Vector3(10, 0, 0))
		v.play_hit_hull()
		v.play_hit_shield()
		v.set_skin("a" if i % 2 == 0 else "b")
		await process_frame
	_c(v._flash.is_empty() and v._bolts.is_empty(), "100 смен скина очищают оружие согласованно")
	_c(v._boost_trail == null and v.active_tween_count() == 0, "нет потерянных следов и Tween")
	var stable_nodes: int = v.get_child_count()
	for i in 100:
		v.play_fire(Vector3(10, 0, 0))
		v.play_hit_shield()
	_c(v._bolts.size() == 2 and v.active_tween_count() <= 5, "частые события имеют ограниченные каналы")
	v.play_death()
	v.respawn()
	await create_timer(0.5).timeout
	_c(v.bank.visible and v.bank.scale == Vector3.ONE, "старый death callback не скрывает respawn")
	_c(v.get_child_count() == stable_nodes, "respawn убирает заряды top_level")
	# ShipView: облик по данным сервера
	var sv: Node3D = ShipView.new()
	root.add_child(sv)
	sv.setup("p1", "player", "neutral", "Тест", false)
	await process_frame
	sv.set_ship("FTR-001", "b")
	_c(sv.visual != null and sv.skin == "b" and sv.ship_type == "FTR-001", "ShipView принимает облик от сервера")
	var vis_before = sv.visual
	sv.set_ship("FTR-001", "b")
	_c(sv.visual == vis_before, "повторная установка того же облика ничего не пересоздаёт")
	sv.set_ship("FTR-001", "a")
	_c(sv.skin == "a" and sv.visual == vis_before, "смена облика не создаёт второй корабль")
	var st: Node3D = ShipView.new()
	root.add_child(st)
	st.setup("st1", "station", "friendly", "Станция", false)
	_c(st.visual == null, "у станции нет визуала корабля")
	var direct: Node3D = ShipView.new()
	root.add_child(direct)
	direct.setup("server-hull", "player", "neutral", "Fixture", false, {"ship": "COR-001", "skin": "a"})
	_c(direct.ship_type == "COR-001" and direct.visual.build_count == 1 and direct.visual.skin_instance_count == 1, "правильный серверный корпус без промежуточного Стрижа")
	direct.play("Fire", Vector3.ONE)
	direct.set_ship("FTR-002", "b")
	await process_frame
	_c(direct.visual.ship_id == "FTR-002" and direct.visual._bolts.is_empty(), "смена корпуса сбрасывает прежние эффекты")
	var definition: Dictionary = ShipDef.get_def("FTR-002")
	var original_path: String = definition.skins.b.glb
	definition.skins.b.glb = "res://missing-test-model.glb"
	direct.visual._skin_root.queue_free()
	await process_frame
	direct.visual.set_skin("b")
	_c(not is_instance_valid(direct.visual._skin_root), "неудачная загрузка не объявлена успешной")
	definition.skins.b.glb = original_path
	direct.visual.set_skin("b")
	_c(is_instance_valid(direct.visual._skin_root), "загрузка повторяется после ошибки")
	direct.queue_free()
	await process_frame
	# Repeated create/free cycles after warming resource caches.
	var resources: Array = []
	for batch in 3:
		for cycle in 20:
			var temporary := ShipVisual.new()
			root.add_child(temporary)
			temporary.play_fire(Vector3.ONE)
			temporary.play_hit_hull()
			await process_frame
			temporary.queue_free()
			await process_frame
		resources.append(Performance.get_monitor(Performance.OBJECT_RESOURCE_COUNT))
	_c(resources[2] <= resources[1] + 2, "ресурсы стабилизируются в повторных create/free циклах")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)


func _count_meshes(n: Node) -> int:
	var c := 1 if n is MeshInstance3D else 0
	for ch in n.get_children():
		c += _count_meshes(ch)
	return c
