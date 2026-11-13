extends RefCounted
class_name ThermalGuard

static func normalize(value: String) -> String:
	var v := value.strip_edges().to_lower()
	if v in ["cool", "none", "light", "moderate", "severe", "critical", "unknown"]:
		return v
	return "unknown"

static func read() -> String:
	var env := OS.get_environment("COSMOC_THERMAL_LEVEL")
	return normalize(env) if env != "" else "unknown"

static func is_hot(level: String) -> bool:
	return normalize(level) in ["severe", "critical"]

static func is_cool(level: String) -> bool:
	return normalize(level) in ["cool", "none"]
