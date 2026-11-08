extends SceneTree
## Декор живых локаций: детерминизм, лимиты пресетов, свободное пространство у станции, пулинг чанков, очистка, скорость.
## Раскладка берётся с сервера (файл из COSMOC_LAYOUT_JSON), если задана; иначе используется встроенная проверочная.

const WorldDecor := preload("res://scripts/space/world_decor.gd")

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


func _data() -> Dictionary:
	var p := OS.get_environment("COSMOC_LAYOUT_JSON")
	if p != "" and FileAccess.file_exists(p):
		var d: Variant = JSON.parse_string(FileAccess.get_file_as_string(p))
		if d is Dictionary:
			return d
	return {"layout": {"seed": 77, "radius": 400, "hub": [-12, 14],
			"bodies": [{"id": "pl-1", "name": "Планета", "kind": "planet", "style": "gas", "pos": [-110, -285], "radius": 40, "ring": true, "color": [0.4, 0.5, 0.8]},
				{"id": "mn-1", "name": "Луна", "kind": "moon", "style": "rocky", "pos": [-150, -330], "radius": 7, "color": [0.6, 0.6, 0.6]}],
			"stations": [{"id": "st1", "name": "Рубеж", "pos": [-12, 14], "safe_radius": 40}],
			"zones": [{"id": "z-belt-1", "type": "mining", "name": "Пояс", "pos": [-20, -157], "radius": 67}, {"id": "z-pve-1", "type": "pve", "name": "Пираты", "pos": [200, -150], "radius": 100}],
			"lanes": []},
		"visual": {"decor": {"asteroids": 1.0, "debris": 1.0, "comets": 1.0, "ships": 1.0, "planets": true},
			"presets": {"low": {"chunk_radius": 1, "asteroids_per_chunk": 3, "debris_per_chunk": 1, "dust_particles": 0, "comets": false, "ships": 0},
				"medium": {"chunk_radius": 2, "asteroids_per_chunk": 5, "debris_per_chunk": 2, "dust_particles": 24, "comets": true, "ships": 2},
				"high": {"chunk_radius": 2, "asteroids_per_chunk": 8, "debris_per_chunk": 3, "dust_particles": 48, "comets": true, "ships": 4}}}}


func _positions(d: Node3D) -> Array:
	var out: Array = []
	for k in d.chunks:
		for inst in (d.chunks[k] as Node3D).get_children():
			for p in (inst.get_meta("spots", PackedVector2Array()) as PackedVector2Array):
				out.append(p)
	return out


func _run() -> void:
	var data := _data()
	var lay: Dictionary = data["layout"]
	var vis: Dictionary = data["visual"]
	var hub := Vector2(float(lay["hub"][0]), float(lay["hub"][1]))
	# --- лимиты пресетов ---
	var counts := {}
	for preset in ["low", "medium", "high"]:
		var d: Node3D = WorldDecor.new()
		root.add_child(d)
		d.build(lay, vis, preset)
		d.update(hub, 0.016)
		var r: int = int(vis["presets"][preset]["chunk_radius"])
		_c(d.visible_chunk_count() == (2 * r + 1) * (2 * r + 1), "%s: видимых чанков %d" % [preset, d.visible_chunk_count()])
		var cap: int = d.visible_chunk_count() * (int(vis["presets"][preset]["asteroids_per_chunk"]) + int(vis["presets"][preset]["debris_per_chunk"])) * 3
		counts[preset] = d.instance_count()
		_c(counts[preset] <= cap, "%s: объектов декора %d не больше лимита %d" % [preset, counts[preset], cap])
		d.queue_free()
		await process_frame
	_c(counts["low"] < counts["high"], "пресет low легче high (%d < %d)" % [counts["low"], counts["high"]])
	# --- свободное пространство у станции ---
	var d2: Node3D = WorldDecor.new()
	root.add_child(d2)
	d2.build(lay, vis, "high")
	d2.update(hub, 0.016)
	var near := 0
	for p in _positions(d2):
		if (p as Vector2).distance_to(hub) < 70.0:
			near += 1
	_c(near == 0, "в радиусе 70 вокруг главной станции декора нет")
	_c(d2.get_node("Statics").get_child_count() >= (lay["bodies"] as Array).size() + (lay["zones"] as Array).size(), "планеты, луны и кольца зон построены")
	# --- детерминизм ---
	var first := _positions(d2)
	first.sort_custom(func(a: Vector2, b: Vector2) -> bool: return a.x < b.x or (a.x == b.x and a.y < b.y))
	var d3: Node3D = WorldDecor.new()
	root.add_child(d3)
	d3.build(lay, vis, "high")
	d3.update(hub, 0.016)
	var second := _positions(d3)
	second.sort_custom(func(a: Vector2, b: Vector2) -> bool: return a.x < b.x or (a.x == b.x and a.y < b.y))
	_c(first == second and first.size() > 0, "одинаковая раскладка даёт одинаковый декор (%d объектов)" % first.size())
	d3.queue_free()
	# --- пулинг: полёт через много чанков ---
	var t0 := Time.get_ticks_usec()
	var worst := 0.0
	for i in 60:
		var t1 := Time.get_ticks_usec()
		d2.update(hub + Vector2(i * 20.0, i * 9.0), 0.05)
		worst = maxf(worst, float(Time.get_ticks_usec() - t1) / 1000.0)
	await process_frame
	var nodes := 0
	for c in d2.get_children():
		if c.get_class() == "Node3D" and c.name != &"Statics":
			nodes += 1
	var r_high: int = int(vis["presets"]["high"]["chunk_radius"])
	var live: int = (2 * r_high + 1) * (2 * r_high + 1)
	_c(d2.visible_chunk_count() == live, "после полёта видимых чанков столько же (%d)" % live)
	_c(nodes <= live + 12, "чанки переиспользуются, узлов %d (видимых %d)" % [nodes, live])
	_c(worst < 40.0, "самое долгое обновление при смене чанков %.1f мс (лимит 40)" % worst)
	print("  60 обновлений заняли %.1f мс" % (float(Time.get_ticks_usec() - t0) / 1000.0))
	# --- очистка ---
	d2.clear()
	await process_frame
	_c(d2.visible_chunk_count() == 0 and not d2.built, "очистка снимает чанки")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
