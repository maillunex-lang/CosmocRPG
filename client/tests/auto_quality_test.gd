extends SceneTree

const AutoQuality := preload("res://ui/graphics/auto_quality.gd")
const ThermalGuard := preload("res://ui/graphics/thermal_guard.gd")

func _initialize() -> void:
	var result := run()
	print("PASS auto_quality" if result.ok else "FAIL auto_quality: %s" % result.failures)
	quit(0 if result.ok else 1)

func run() -> Dictionary:
	var failures: Array[String] = []
	_assert(ThermalGuard.normalize("severe") == "severe", failures, "thermal normalize")
	_assert(ThermalGuard.normalize("bad") == "unknown", failures, "thermal unknown")
	var q := AutoQuality.new("high", 1.0, 60)
	var changed := {}
	for i in 16:
		changed = q.update(30.0, "unknown", 0.1)
	_assert(q.profile != "high" or q.render_scale <= 1.0, failures, "unknown remains bounded")
	for i in 16:
		changed = q.update(30.0, "severe", 0.1)
	_assert(q.profile != "high" or q.render_scale < 1.0, failures, "severe downshift")
	var q2 := AutoQuality.new("low", 0.5, 60)
	for i in 64:
		q2.update(10.0, "unknown", 0.1)
	_assert(q2.profile == "low" and is_equal_approx(q2.render_scale, 0.5), failures, "unknown must block upshift")
	for i in 64:
		q2.update(10.0, "cool", 0.1)
	_assert(q2.render_scale > 0.5 or q2.profile != "low", failures, "cool can upshift")
	return {"ok": failures.is_empty(), "failures": failures}

func _assert(condition: bool, failures: Array[String], message: String) -> void:
	if not condition:
		failures.append(message)
