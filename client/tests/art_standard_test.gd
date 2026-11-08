extends SceneTree
## Тест художественного стандарта и ассетов космического фона (CPU, без рендеринга).
## Стандарт: глубокий тёмно-синий космос, сине-фиолетовые туманности, умеренное бирюзовое свечение.
## Запуск: godot --headless --path client --script res://tests/art_standard_test.gd

const SpaceBackground := preload("res://ui/space_bg/space_background.gd")
const LayerConfig := preload("res://ui/space_bg/space_layer_config.gd")
const Library := preload("res://ui/space_bg/background_library.gd")
const ArtStandard := preload("res://ui/space_bg/art_standard.gd")
const Placeholders := preload("res://ui/space_bg/placeholders.gd")

const MAX_FILE_BYTES := 400 * 1024       # лимит на один оптимизированный файл
const MAX_TOTAL_BYTES := 700 * 1024      # лимит на все фоны вместе (исходники в репозитории)
const MAX_SIDE := 2048                   # ни одна текстура не больше (память GPU)

var _failures := 0
var _checks := 0


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _compose(profile: Resource, cam: Vector2 = Vector2.ZERO, preset: String = "high", b: float = 1.0, i: float = 1.0) -> Image:
	return SpaceBackground.compose_cpu(profile, Vector2i(192, 108), cam, preset, b, i)


func _run() -> void:
	Library.reset()
	var ids: Array = Library.profile_ids()
	ids.sort()
	_check(ids.size() >= 3, "в каталоге не меньше трёх профилей систем (%s)" % str(ids))

	var composites := {}
	for pid in ids:
		var prof: Resource = Library.profile(pid)
		_check(prof.warnings.is_empty(), "профиль «%s»: данные без замечаний %s" % [pid, str(prof.warnings)])

		# цветовая схема профиля соответствует стандарту
		var m := ArtStandard.measure(_compose(prof))
		var v := ArtStandard.violations(m)
		_check(v.is_empty(), "стандарт «%s» (high): яркость %.3f, центр %.3f, сине-фиол. %.0f%%, бирюза %.1f%%, оттенок %.0f° %s" % [
			pid, m["mean_lum"], m["center_lum"], m["blue_violet"] * 100.0, m["turquoise"] * 100.0, m["mean_hue"], str(v)])
		for preset in ["medium", "low"]:
			var mp := ArtStandard.measure(_compose(prof, Vector2.ZERO, preset))
			_check(ArtStandard.violations(mp).is_empty(), "стандарт «%s» (%s): %s" % [pid, preset, str(ArtStandard.violations(mp))])

		# сдвиг камеры (параллакс) не нарушает стандарт и не показывает края
		for cam in [Vector2(60, 40), Vector2(-120, -80), Vector2(900, 700)]:
			var mc := ArtStandard.measure(_compose(prof, cam))
			_check(ArtStandard.violations(mc).is_empty(), "стандарт «%s», камера %s: %s" % [pid, str(cam), str(ArtStandard.violations(mc))])

		# крайние пользовательские настройки яркости и интенсивности не ломают стандарт
		var mx := ArtStandard.measure(_compose(prof, Vector2.ZERO, "high", SpaceBackground.MAX_LEVEL, SpaceBackground.MAX_LEVEL))
		_check(ArtStandard.violations(mx).is_empty(), "стандарт «%s» при максимуме настроек (%.1f/%.1f): яркость %.3f %s" % [
			pid, SpaceBackground.MAX_LEVEL, SpaceBackground.MAX_LEVEL, mx["mean_lum"], str(ArtStandard.violations(mx))])

		# база под слоями
		_check(ArtStandard.luminance(prof.base_color) <= ArtStandard.BASE_LUM_MAX and prof.base_color.b > prof.base_color.g and prof.base_color.g > prof.base_color.r,
			"«%s»: базовый цвет глубокий тёмно-синий" % pid)

		# дальние слои двигаются медленнее ближних (параллакс идёт от дальнего к ближнему)
		var mono := true
		for k in range(1, prof.layers.size()):
			if prof.layers[k].parallax < prof.layers[k - 1].parallax:
				mono = false
		_check(mono, "«%s»: параллакс возрастает от дальнего слоя к ближнему" % pid)
		composites[pid] = _compose(prof)

	# системы действительно различаются
	if composites.has("xi") and composites.has("omega") and composites.has("deep"):
		_check(_mean_diff(composites["xi"], composites["omega"]) > 0.01, "системы xi и omega визуально различаются")
		_check(_mean_diff(composites["xi"], composites["deep"]) > 0.01, "системы xi и deep визуально различаются")

	# ============ ассеты ============
	var catalog := FileAccess.get_file_as_string(Library.CATALOG_PATH)
	var data: Dictionary = JSON.parse_string(catalog)
	var textures: Dictionary = data["textures"]
	var used := {}
	for prof_d in data["profiles"]:
		for layer in prof_d["layers"]:
			used[layer["texture"]] = true
	var total := 0
	for key in textures:
		var path: String = textures[key]
		_check(FileAccess.file_exists(path), "ассет «%s»: файл есть (%s)" % [key, path])
		_check(used.has(key), "ассет «%s»: используется хотя бы одним профилем" % key)
		var f := FileAccess.open(path, FileAccess.READ)
		var bytes := f.get_length() if f != null else 0
		total += bytes
		_check(bytes > 0 and bytes <= MAX_FILE_BYTES, "ассет «%s»: размер %d КБ не больше %d КБ" % [key, bytes / 1024, MAX_FILE_BYTES / 1024])
		_check(path.ends_with(".webp"), "ассет «%s»: формат WebP" % key)
		var tex: Texture2D = Library.texture(key)
		_check(not Placeholders.is_placeholder(tex), "ассет «%s»: загружается" % key)
		_check(maxi(tex.get_width(), tex.get_height()) <= MAX_SIDE, "ассет «%s»: %dx%d не больше %d по стороне" % [key, tex.get_width(), tex.get_height(), MAX_SIDE])
		var imp := FileAccess.get_file_as_string(path + ".import")
		_check("compress/mode=2" in imp, "ассет «%s»: импорт со сжатием VRAM (экономит память GPU и APK)" % key)
		_check("mipmaps/generate=false" in imp, "ассет «%s»: без мипмапов (слои рисуются близко к 1:1)" % key)
	_check(total <= MAX_TOTAL_BYTES, "все фоны вместе: %d КБ не больше %d КБ" % [total / 1024, MAX_TOTAL_BYTES / 1024])
	for pid in ids:
		for cfg in Library.profile(pid).layers:
			_check(textures.has(cfg.texture_key), "профиль «%s»: ключ «%s» есть в реестре текстур" % [pid, cfg.texture_key])

	# каждая текстура профиля — не заглушка (иначе стандарт проверялся бы на заглушках)
	_check(Library.missing_assets().is_empty(), "при проверке стандарта заглушки не использовались")

	Library.reset()
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)


func _mean_diff(a: Image, b: Image) -> float:
	var t := 0.0
	var n := a.get_width() * a.get_height()
	for y in a.get_height():
		for x in a.get_width():
			var ca := a.get_pixel(x, y)
			var cb := b.get_pixel(x, y)
			t += (absf(ca.r - cb.r) + absf(ca.g - cb.g) + absf(ca.b - cb.b)) / 3.0
	return t / n
