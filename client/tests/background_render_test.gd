extends SceneTree
## Проверка РЕАЛЬНОЙ отрисовки фона шейдерами (нужен рендеринг: xvfb) и сравнение с эталонной
## программной сборкой (формулы шейдеров на CPU), а также с художественным стандартом.
## Запуск: tools/test_client.sh (или xvfb-run godot --path client --script res://tests/background_render_test.gd)

const SpaceBackground := preload("res://ui/space_bg/space_background.gd")
const Library := preload("res://ui/space_bg/background_library.gd")
const ArtStandard := preload("res://ui/space_bg/art_standard.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

var _failures := 0
var _checks := 0
var _bg: Control


func _initialize() -> void:
	_run.call_deferred()


func _check(cond: bool, name: String) -> void:
	_checks += 1
	if cond:
		print("PASS  ", name)
	else:
		_failures += 1
		print("FAIL  ", name)


func _grab() -> Image:
	await process_frame
	await process_frame
	await RenderingServer.frame_post_draw
	var img := root.get_texture().get_image()
	img.convert(Image.FORMAT_RGBF)
	return img


func _mean_abs_diff(a: Image, b: Image) -> float:
	var small_a := a.duplicate() as Image
	var small_b := b.duplicate() as Image
	small_a.resize(160, 90, Image.INTERPOLATE_BILINEAR)
	small_b.resize(160, 90, Image.INTERPOLATE_BILINEAR)
	var total := 0.0
	for y in 90:
		for x in 160:
			var ca := small_a.get_pixel(x, y)
			var cb := small_b.get_pixel(x, y)
			total += (absf(ca.r - cb.r) + absf(ca.g - cb.g) + absf(ca.b - cb.b)) / 3.0
	return total / (160.0 * 90.0)


func _setup(profile_id: String) -> void:
	if _bg != null:
		_bg.queue_free()
		await process_frame
	_bg = SpaceBackground.new()
	root.add_child(_bg)
	_bg.apply_profile(Library.profile(profile_id))
	_bg.set_camera(Vector2.ZERO)


func _run() -> void:
	UiSettings.override("reduce_motion", true)   # без самодвижения: кадры воспроизводимы
	UiSettings.override("graphics_preset", "high")
	var size := Vector2i(root.size)
	print("окно: ", size)

	for pid in Library.profile_ids():
		await _setup(pid)
		var shot := await _grab()
		var m := ArtStandard.measure(shot)
		var v := ArtStandard.violations(m)
		_check(v.is_empty(), "стандарт, отрисовка «%s»: ср. яркость %.3f, центр %.3f, сине-фиол. %.0f%%, бирюза %.1f%%, оттенок %.0f° %s" % [
			pid, m["mean_lum"], m["center_lum"], m["blue_violet"] * 100.0, m["turquoise"] * 100.0, m["mean_hue"], str(v)])
		# совпадение с эталонной сборкой (CPU, те же формулы)
		var ref := SpaceBackground.compose_cpu(Library.profile(pid), Vector2i(320, 180))
		var diff := _mean_abs_diff(shot, ref)
		_check(diff < 0.03, "«%s»: отрисовка шейдером совпадает с эталонной сборкой (средняя разница %.4f)" % [pid, diff])

	# яркость и интенсивность реально меняют картинку
	await _setup("xi")
	var normal := await _grab()
	var mn := ArtStandard.measure(normal)
	_bg.set_levels(0.0, 1.0)
	var dark := ArtStandard.measure(await _grab())
	_check(dark["mean_lum"] < mn["mean_lum"] * 0.35, "яркость 0: фон почти погашен (%.3f против %.3f)" % [dark["mean_lum"], mn["mean_lum"]])
	_bg.set_levels(1.5, 1.0)
	var bright := ArtStandard.measure(await _grab())
	_check(bright["mean_lum"] > mn["mean_lum"], "яркость 1.5: ярче нормы")
	_bg.set_levels(1.0, 0.0)
	var flat := ArtStandard.measure(await _grab())
	_check(flat["mean_lum"] < 0.06, "интенсивность 0: остаётся только база (%.3f)" % flat["mean_lum"])
	_bg.set_levels(1.0, 1.0)

	# параллакс двигает картинку: камера сдвинута, кадр отличается
	var before := await _grab()
	_bg.set_camera(Vector2(40, 25))
	var moved := await _grab()
	_check(_mean_abs_diff(before, moved) > 0.003, "параллакс: при сдвиге камеры картинка меняется")

	# заглушки при отсутствии текстур тоже в палитре и без падений
	Library.reset()
	Library.load_catalog()
	Library.register_profile({"id": "tech", "name": "Тест", "layers": [
		{"texture": "no_stars", "blend": "add", "wrap": "mirror"}, {"texture": "no_neb", "blend": "add"}, {"texture": "no_gas", "blend": "mix", "intensity": 0.5}]})
	await _setup("tech")
	var ph := ArtStandard.measure(await _grab())
	_check(_bg.missing_assets().size() == 3, "недостающие текстуры перечислены: %s" % str(_bg.missing_assets()))
	_check(ph["mean_lum"] < ArtStandard.MEAN_LUM_MAX and ph["mean_lum"] > 0.0, "заглушки: фон не пустой и не яркий (%.3f)" % ph["mean_lum"])

	UiSettings.override("reduce_motion", null)
	UiSettings.override("graphics_preset", null)
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
