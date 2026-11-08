extends RefCounted
## Версия установленного приложения. Единый источник: project.godot (`config/version` и `cosmoc/version_code`);
## экспортный пресет Android (`version/name`, `version/code`) обязан совпадать (проверяет тест updater_test).

static func code() -> int:
	return int(ProjectSettings.get_setting("cosmoc/version_code", 1))


static func version_name() -> String:
	return str(ProjectSettings.get_setting("application/config/version", "0.0.0"))


static func label() -> String:
	return "%s (%d)" % [version_name(), code()]
