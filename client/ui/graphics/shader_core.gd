extends RefCounted
## Общие параметры визуальных шейдеров. Все эффекты читают один профиль качества.
class_name ShaderCore

const PRESETS := {
 "low": {"effect_scale": 0.35, "noise_octaves": 1, "particles": 0.45, "vignette": 0.0},
 "medium": {"effect_scale": 0.65, "noise_octaves": 2, "particles": 0.75, "vignette": 0.12},
 "high": {"effect_scale": 1.0, "noise_octaves": 3, "particles": 1.0, "vignette": 0.18},
}

static func preset(name: String) -> Dictionary:
 return PRESETS.get(name, PRESETS.medium)

static func effect_scale(name: String) -> float:
 return float(preset(name).get("effect_scale", 0.65))

static func can_animate() -> bool:
 return not bool(ProjectSettings.get_setting("application/config/use_custom_user_dir", false))
