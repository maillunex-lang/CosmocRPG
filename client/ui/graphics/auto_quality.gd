extends RefCounted
class_name AutoQualityController
const ThermalGuard := preload("res://ui/graphics/thermal_guard.gd")

const PROFILES := ["low", "medium", "high"]
const SCALES := [0.5, 0.67, 0.75, 0.85, 1.0]
const WINDOW_SECONDS := 0.75
const DOWN_WINDOWS := 2
const UP_WINDOWS := 4

var enabled := true
var profile := "medium"
var render_scale := 1.0
var _elapsed := 0.0
var _frame_sum := 0.0
var _frames := 0
var _slow_windows := 0
var _fast_windows := 0
var _target_fps := 60.0

func _init(initial_profile: String = "medium", initial_scale: float = 1.0, target_fps: int = 60) -> void:
	profile = initial_profile if initial_profile in PROFILES else "medium"
	render_scale = _nearest_scale(initial_scale)
	_target_fps = 30.0 if target_fps == 30 else 60.0

func set_target_fps(value: int) -> void:
	_target_fps = 30.0 if value == 30 else 60.0

func update(frame_ms: float, thermal_level: String, delta: float) -> Dictionary:
	if not enabled:
		return {}
	_elapsed += maxf(delta, 0.0)
	_frame_sum += maxf(frame_ms, 0.0)
	_frames += 1
	if _elapsed < WINDOW_SECONDS:
		return {}
	var average_ms := _frame_sum / maxf(float(_frames), 1.0)
	_elapsed = 0.0
	_frame_sum = 0.0
	_frames = 0
	var budget_ms := 1000.0 / _target_fps
	var hot := ThermalGuard.is_hot(thermal_level)
	var cool := ThermalGuard.is_cool(thermal_level)
	if hot or average_ms > budget_ms + 5.0:
		_slow_windows += 1
		_fast_windows = 0
	else:
		_slow_windows = 0
		if cool and average_ms < budget_ms - 5.0:
			_fast_windows += 1
		else:
			_fast_windows = 0
	if hot or _slow_windows >= DOWN_WINDOWS:
		_slow_windows = 0
		_fast_windows = 0
		return _step(-1)
	if _fast_windows >= UP_WINDOWS:
		_slow_windows = 0
		_fast_windows = 0
		return _step(1)
	return {}

func _step(direction: int) -> Dictionary:
	var old_profile := profile
	var old_scale := render_scale
	var scale_index := SCALES.find(render_scale)
	if scale_index < 0:
		scale_index = 4
	var next_scale_index := clampi(scale_index + direction, 0, SCALES.size() - 1)
	if direction < 0 and next_scale_index == scale_index and profile != "low":
		profile = PROFILES[maxi(PROFILES.find(profile) - 1, 0)]
	elif direction > 0 and next_scale_index == scale_index and profile != "high":
		profile = PROFILES[mini(PROFILES.find(profile) + 1, PROFILES.size() - 1)]
	render_scale = SCALES[next_scale_index]
	if old_profile == profile and is_equal_approx(old_scale, render_scale):
		return {}
	return {"graphics_preset": profile, "render_scale": render_scale, "old_preset": old_profile, "old_scale": old_scale}

func _nearest_scale(value: float) -> float:
	var best := SCALES[0]
	var distance := absf(value - best)
	for candidate in SCALES:
		if absf(value - candidate) < distance:
			best = candidate
			distance = absf(value - candidate)
	return best
