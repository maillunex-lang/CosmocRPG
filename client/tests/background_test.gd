extends SceneTree
## Тест системы космического фона: конфигурация и проверка данных, каталог и заглушки, математика
## параллакса, узел SpaceBackground (слои, настройки, качество, самодвижение, смена системы), модуль стандарта.
## Запуск: godot --headless --path client --script res://tests/background_test.gd

const SpaceBackground := preload("res://ui/space_bg/space_background.gd")
const LayerConfig := preload("res://ui/space_bg/space_layer_config.gd")
const Profile := preload("res://ui/space_bg/space_background_profile.gd")
const Library := preload("res://ui/space_bg/background_library.gd")
const Placeholders := preload("res://ui/space_bg/placeholders.gd")
const ArtStandard := preload("res://ui/space_bg/art_standard.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

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


## Сравнение значений словаря с допуском для чисел (цвета хранятся в 32-битных числах).
func _approx_equal(a: Variant, b: Variant) -> bool:
	if a is Array and b is Array:
		if a.size() != b.size():
			return false
		for i in a.size():
			if not _approx_equal(a[i], b[i]):
				return false
		return true
	if (a is float or a is int) and (b is float or b is int):
		return is_equal_approx(float(a), float(b))
	return str(a) == str(b)


func _holder(size: Vector2i = Vector2i(1280, 720)) -> SubViewport:
	var h := SubViewport.new()
	h.size = size
	h.disable_3d = true
	root.add_child(h)
	return h


func _bg_in(h: SubViewport, profile_id: String = "xi") -> Control:
	var bg: Control = SpaceBackground.new()
	h.add_child(bg)
	bg.apply_profile(Library.profile(profile_id))
	return bg


func _run() -> void:
	UiSettings.override("reduce_motion", true)
	Library.reset()

	# ============ SpaceLayerConfig ============
	var c = LayerConfig.from_dict({"texture": "a"})
	_check(c.warnings.is_empty() and c.parallax == 0.2 and c.zoom == 1.2 and c.blend == LayerConfig.Blend.ADD and c.wrap == LayerConfig.Wrap.CLAMP, "слой: значения по умолчанию")
	c = LayerConfig.from_dict({"texture": "a", "parallax": 9.0, "zoom": 0.1, "brightness": -3, "intensity": 50, "drift": [999, -999], "min_quality": "ultra", "tint": "красный"})
	_check(c.parallax == 2.0 and c.zoom == 1.0 and c.brightness == 0.0 and c.intensity == 2.0, "слой: числа ограничиваются диапазонами")
	_check(c.drift == Vector2(60, -60), "слой: самодвижение ограничено ±60 px/с")
	_check(c.min_quality == "low" and c.warnings.size() >= 6, "слой: неверные значения дают замечания (%d)" % c.warnings.size())
	c = LayerConfig.from_dict({})
	_check(c.warnings.size() >= 1 and c.texture_key == "", "слой: без текстуры есть замечание")
	c = LayerConfig.from_dict({"texture": "a", "parallax": NAN})
	_check(c.parallax == 0.2, "слой: NaN заменяется значением по умолчанию")
	var valid := {"texture": "stars", "blend": "mix", "parallax": 0.4, "zoom": 1.5, "brightness": 0.9, "intensity": 0.6,
		"tint": [0.9, 0.8, 1.0], "drift": [3.0, -2.0], "wrap": "mirror", "flip": [-1, 1], "min_quality": "medium"}
	var rt = LayerConfig.from_dict(valid).to_dict()
	var same := true
	for k in valid:
		if not _approx_equal(valid[k], rt[k]):
			same = false
			print("  расхождение в ключе ", k, ": ", valid[k], " / ", rt[k])
	_check(same and LayerConfig.from_dict(valid).warnings.is_empty(), "слой: словарь -> слой -> словарь без потерь")
	_check(LayerConfig.from_dict({"texture": "a", "flip": [-5, 7]}).flip == Vector2i(-1, 1), "слой: отражение приводится к ±1")

	# ============ SpaceBackgroundProfile ============
	var p = Profile.from_dict({"name": "без id", "layers": []})
	_check(p.warnings.size() == 2, "профиль: нет id и слоёв — два замечания (%s)" % str(p.warnings))
	p = Profile.from_dict({"id": "t", "brightness": 9, "intensity": -1, "px_per_unit": 1000, "base_color": [2, -1, 0.5], "layers": [{"texture": "a"}, "мусор", 5]})
	_check(p.brightness == 2.0 and p.intensity == 0.0 and p.px_per_unit == 60.0, "профиль: множители ограничены")
	_check(p.base_color == Color(1, 0, 0.5, 1) and p.layers.size() == 1, "профиль: цвет ограничен, не-словари в слоях пропущены")
	p = Profile.from_dict({"id": "t", "layers": "не массив"})
	_check(p.layers.is_empty() and p.warnings.size() >= 1, "профиль: layers не массив обрабатывается")
	p = Profile.from_dict({"id": "t", "base_color": "#102030", "layers": [{"texture": "a"}]})
	_check(p.base_color.is_equal_approx(Color("#102030")), "профиль: цвет строкой #RRGGBB")
	var back = Profile.from_dict(Profile.from_dict({"id": "x", "name": "Икс", "layers": [valid]}).to_dict())
	_check(back.id == "x" and back.display_name == "Икс" and back.layers.size() == 1, "профиль: круговое преобразование")

	# ============ каталог ============
	_check(Library.load_catalog() and Library.catalog_errors().is_empty(), "каталог загружается без ошибок")
	var ids: Array = Library.profile_ids()
	_check(ids.has("xi") and ids.has("omega") and ids.has("deep"), "в каталоге есть профили xi, omega, deep (%s)" % str(ids))
	var xi = Library.profile("xi")
	_check(xi.layers.size() == 3 and xi.warnings.is_empty(), "профиль xi: 3 слоя без замечаний")
	_check(xi.layers[0].texture_key == "stars_deep" and xi.layers[0].wrap == LayerConfig.Wrap.MIRROR, "xi: звёзды первыми, зеркальный повтор")
	_check(xi.layers[2].blend == LayerConfig.Blend.MIX and is_equal_approx(xi.layers[2].intensity, 0.35), "xi: газ с прозрачностью и интенсивностью 0.35")
	_check(Library.profile("deep").layers.size() == 2, "профиль deep: два слоя (без газа)")
	_check(Library.profile("нет-такого").id == "xi" and Library.profile("").id == "xi", "неизвестный профиль даёт профиль по умолчанию")
	var t1: Texture2D = Library.texture("stars_deep", "stars")
	_check(not Placeholders.is_placeholder(t1) and t1.get_width() == 1672, "текстура звёзд загружена, не заглушка (%d px)" % t1.get_width())
	_check(Library.missing_assets().is_empty(), "при полном каталоге недостающих ассетов нет")
	var ph: Texture2D = Library.texture("нет_такой_текстуры", "gas")
	_check(Placeholders.is_placeholder(ph) and Library.missing_assets().has("нет_такой_текстуры"), "отсутствующая текстура: заглушка и запись в списке недостающих")
	Library.register_texture("нет_такой_текстуры", "res://assets/space/backgrounds/gas_turquoise.webp")
	_check(not Placeholders.is_placeholder(Library.texture("нет_такой_текстуры", "gas")) and not Library.missing_assets().has("нет_такой_текстуры"), "register_texture заменяет заглушку настоящей текстурой")
	Library.register_texture("битый_путь", "res://нет/файла.webp")
	_check(Placeholders.is_placeholder(Library.texture("битый_путь", "nebula")) and "файл не найден" in str(Library.missing_assets()["битый_путь"]), "несуществующий файл: заглушка и причина")
	Library.register_profile({"id": "custom", "name": "Новая", "layers": [{"texture": "stars_deep", "wrap": "mirror"}]})
	_check(Library.profile("custom").layers.size() == 1 and Library.profile_ids().has("custom"), "register_profile добавляет систему без пересборки")
	Library.reset()
	_check(not Library.load_catalog("res://нет/каталога.json") and Library.catalog_errors().size() == 1, "отсутствующий каталог: ошибка, без падения")
	Library.reset()
	var bad_path := "user://bad_catalog.json"
	var f := FileAccess.open(bad_path, FileAccess.WRITE)
	f.store_string("{это не json")
	f.close()
	_check(not Library.load_catalog(bad_path), "битый JSON каталога: ошибка, без падения")
	f = FileAccess.open(bad_path, FileAccess.WRITE)
	f.store_string('{"textures": [], "profiles": {}}')
	f.close()
	Library.reset()
	_check(not Library.load_catalog(bad_path) and Library.catalog_errors().size() == 2, "каталог с неверными типами: ошибки перечислены")
	Library.reset()
	_check(Library.profile("xi").layers.size() == 3, "после сброса каталог перечитывается")

	# ============ заглушки ============
	for role in ["stars", "nebula", "gas"]:
		var a: ImageTexture = Placeholders.make(role)
		var b: ImageTexture = Placeholders.make(role)
		_check(a.get_size() == Vector2(Placeholders.W, Placeholders.H) and Placeholders.is_placeholder(a), "заглушка «%s»: размер и пометка" % role)
		_check(a.get_image().get_data() == b.get_image().get_data(), "заглушка «%s»: детерминирована" % role)
	var gas_img := Placeholders.make("gas").get_image()
	var has_transparent := false
	for x in range(0, Placeholders.W, 7):
		if gas_img.get_pixel(x, 20).a < 0.05:
			has_transparent = true
	_check(has_transparent, "заглушка газа содержит прозрачные области")
	var nm := ArtStandard.measure(Placeholders.make("nebula").get_image())
	_check(nm["mean_lum"] < 0.2 and nm["center_lum"] < nm["mean_lum"], "заглушка туманности тёмная, центр темнее краёв (%.3f / %.3f)" % [nm["center_lum"], nm["mean_lum"]])

	# ============ математика параллакса ============
	var vp := Vector2(1280, 720)
	var tex := Vector2(1672, 941)
	var u0 := SpaceBackground.compute_uv(vp, tex, 1.0, 0.0, Vector2.ZERO, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)
	_check(u0["extent"].x > 0.99 and u0["extent"].y > 0.99, "zoom 1.0: текстура впритык (extent %s)" % str(u0["extent"]))
	var u1 := SpaceBackground.compute_uv(vp, tex, 1.2, 0.0, Vector2.ZERO, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)
	_check(absf(u1["extent"].x - 1.0 / 1.2) < 0.01 and absf(u1["offset"].x - (1.0 - u1["extent"].x) * 0.5) < 1e-5, "zoom 1.2: видна 1/1.2 текстуры по центру")
	var cam := Vector2(30, -20)
	var up0 := SpaceBackground.compute_uv(vp, tex, 1.2, 0.0, cam, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)
	_check(up0["offset"].is_equal_approx(u1["offset"]), "parallax 0: слой неподвижен при любом положении камеры")
	var small := Vector2(0.5, 0.5)
	var a_far: Vector2 = SpaceBackground.compute_uv(vp, tex, 1.3, 0.1, small, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)["offset"] - SpaceBackground.compute_uv(vp, tex, 1.3, 0.1, Vector2.ZERO, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)["offset"]
	var a_near: Vector2 = SpaceBackground.compute_uv(vp, tex, 1.3, 0.5, small, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)["offset"] - SpaceBackground.compute_uv(vp, tex, 1.3, 0.5, Vector2.ZERO, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)["offset"]
	_check(a_near.length() > a_far.length() * 4.0, "ближний слой (0.5) смещается заметно сильнее дальнего (0.1)")
	_check(a_near.x > 0.0 and a_near.y > 0.0, "камера вправо-вниз сдвигает видимую область вправо-вниз")
	var sample_sizes := [Vector2(1280, 720), Vector2(1600, 720), Vector2(2000, 916), Vector2(3000, 720), Vector2(1280, 960), Vector2(720, 1280)]
	var edges_ok := true
	for s in sample_sizes:
		for z in [1.0, 1.2, 1.5, 2.5]:
			for cm in [Vector2(5000, 0), Vector2(-5000, 300), Vector2(0, -9000), Vector2(1e6, 1e6)]:
				var r := SpaceBackground.compute_uv(s, tex, z, 1.0, cm, 14.4, Vector2(500, -500), LayerConfig.Wrap.CLAMP)
				var o: Vector2 = r["offset"]
				var e: Vector2 = r["extent"]
				if o.x < -1e-4 or o.y < -1e-4 or o.x + e.x > 1.0 + 1e-4 or o.y + e.y > 1.0 + 1e-4 or e.x > 1.0 + 1e-6 or e.y > 1.0 + 1e-6:
					edges_ok = false
	_check(edges_ok, "режим CLAMP: края текстуры не показываются ни при каких размерах, zoom и положении камеры")
	var um := SpaceBackground.compute_uv(vp, tex, 1.2, 1.0, Vector2(500, 0), 14.4, Vector2.ZERO, LayerConfig.Wrap.MIRROR)
	_check(um["offset"].x > 1.0, "режим MIRROR: сдвиг не ограничен (бесконечная прокрутка)")
	var ud := SpaceBackground.compute_uv(vp, tex, 1.2, 0.0, Vector2.ZERO, 14.4, Vector2(20, 0), LayerConfig.Wrap.MIRROR)
	_check(ud["offset"].x > u1["offset"].x, "самодвижение добавляется к сдвигу")
	var uz := SpaceBackground.compute_uv(Vector2.ZERO, tex, 1.2, 1.0, cam, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)
	_check(uz["offset"] == Vector2.ZERO and uz["extent"] == Vector2.ONE, "нулевой размер экрана: безопасные значения")
	_check(SpaceBackground.compute_uv(vp, tex, 0.3, 0.0, Vector2.ZERO, 14.4, Vector2.ZERO, LayerConfig.Wrap.CLAMP)["extent"].x > 0.99, "zoom меньше 1 приравнивается к 1")
	_check(SpaceBackground.soft_limit(0.0, 0.1) == 0.0 and absf(SpaceBackground.soft_limit(1000.0, 0.1) - 0.1) < 1e-5, "soft_limit: ноль и насыщение")
	_check(absf(SpaceBackground.soft_limit(-0.3, 0.1) + SpaceBackground.soft_limit(0.3, 0.1)) < 1e-6, "soft_limit: нечётная функция")
	_check(absf(SpaceBackground.soft_limit(0.001, 0.1) - 0.001) < 1e-5, "soft_limit: около нуля почти линейна")
	_check(SpaceBackground.soft_limit(5.0, 0.0) == 0.0, "soft_limit: нулевой запас даёт ноль")

	# ============ узел SpaceBackground ============
	UiSettings.override("graphics_preset", "high")
	var h := _holder()
	var bg := _bg_in(h)
	await process_frame
	await process_frame
	_check(bg.layer_count() == 3, "узел: три слоя профиля xi")
	var i0: Dictionary = bg.layer_info(0)
	_check(not Placeholders.is_placeholder(i0["texture"]) and i0["mirror"] and is_equal_approx(i0["brightness"], 1.0), "узел: слой звёзд — настоящая текстура, зеркальный повтор")
	var i2: Dictionary = bg.layer_info(2)
	_check(is_equal_approx(i2["intensity"], 0.35), "узел: интенсивность газа 0.35 из профиля")
	var base_off0: Vector2 = bg.layer_info(0)["uv_offset"]
	var base_off2: Vector2 = bg.layer_info(2)["uv_offset"]
	bg.set_camera(Vector2(10, 6))
	await process_frame
	var d0: Vector2 = bg.layer_info(0)["uv_offset"] - base_off0
	var d2: Vector2 = bg.layer_info(2)["uv_offset"] - base_off2
	_check(d0.length() > 0.0 and d2.length() > d0.length(), "узел: камера сдвигает слои, ближний сильнее дальнего")
	bg.set_levels(0.5, 0.5)
	await process_frame
	_check(is_equal_approx(bg.layer_info(0)["brightness"], 0.5) and is_equal_approx(bg.layer_info(2)["intensity"], 0.175), "узел: пользовательские яркость и интенсивность умножаются")
	bg.set_levels(99.0, -5.0)
	await process_frame
	_check(bg.user_brightness == SpaceBackground.MAX_LEVEL and bg.user_intensity == 0.0, "узел: уровни ограничены диапазоном")
	bg.set_levels(1.0, 1.0)
	_check(bg.mouse_filter == Control.MOUSE_FILTER_IGNORE and bg.get_child(1).mouse_filter == Control.MOUSE_FILTER_IGNORE, "узел: фон не перехватывает ввод")
	await process_frame
	_check(not bg._dirty, "узел: без изменений параметры шейдера не пересчитываются")
	h.size = Vector2i(2400, 720)
	await process_frame
	await process_frame
	_check(bg.layer_info(1)["uv_extent"].y < bg.layer_info(1)["uv_extent"].x or bg.layer_info(1)["uv_extent"].x > 0.99, "узел: при смене размера экрана UV пересчитывается (%s)" % str(bg.layer_info(1)["uv_extent"]))
	h.queue_free()

	# настройки пользователя читаются при создании
	UiSettings.override("bg_brightness", 0.7)
	UiSettings.override("bg_intensity", 1.2)
	h = _holder()
	bg = _bg_in(h)
	await process_frame
	_check(is_equal_approx(bg.user_brightness, 0.7) and is_equal_approx(bg.layer_info(0)["brightness"], 0.7), "узел: яркость из настроек применяется")
	_check(is_equal_approx(bg.layer_info(2)["intensity"], 0.35 * 1.2), "узел: интенсивность из настроек применяется")
	h.queue_free()
	UiSettings.override("bg_brightness", null)
	UiSettings.override("bg_intensity", null)

	# пресеты графики
	for preset in ["low", "medium", "high", "странный"]:
		UiSettings.override("graphics_preset", preset)
		h = _holder()
		bg = _bg_in(h)
		await process_frame
		var gas_on: bool = bg.layer_info(2)["enabled"]
		var stars_on: bool = bg.layer_info(0)["enabled"] and bg.layer_info(1)["enabled"]
		_check(stars_on and gas_on == (preset != "low"), "пресет «%s»: звёзды и туманность всегда, газ только не на low" % preset)
		h.queue_free()
	UiSettings.override("graphics_preset", "high")

	# самодвижение и уменьшение анимаций
	h = _holder()
	bg = _bg_in(h)
	UiSettings.override("reduce_motion", true)
	bg._process(2.0)
	_check(bg.layer_info(2)["drift_px"] == Vector2.ZERO, "уменьшение анимаций: самодвижение отключено")
	UiSettings.override("reduce_motion", false)
	bg._process(2.0)
	_check(bg.layer_info(2)["drift_px"].is_equal_approx(Vector2(6.0, 2.4)) and bg.layer_info(0)["drift_px"] == Vector2.ZERO, "самодвижение: газ дрейфует (3; 1.2) px/с, звёзды неподвижны")
	var off_a: Vector2 = bg.layer_info(2)["uv_offset"]
	bg._process(5.0)
	_check(bg.layer_info(2)["uv_offset"] != off_a, "самодвижение сдвигает слой")
	h.queue_free()
	UiSettings.override("graphics_preset", "low")
	h = _holder()
	bg = _bg_in(h)
	bg._process(3.0)
	_check(bg.layer_info(2)["drift_px"] == Vector2.ZERO, "на low скрытый слой не дрейфует (нет лишней работы)")
	h.queue_free()
	UiSettings.override("graphics_preset", "high")
	UiSettings.override("reduce_motion", true)

	# смена системы
	h = _holder()
	bg = _bg_in(h, "xi")
	await process_frame
	var n_children := bg.get_child_count()
	bg.apply_profile(Library.profile("omega"))
	await process_frame
	_check(bg.layer_count() == 3 and bg.get_child_count() == n_children, "смена системы: слои заменены без утечки узлов (%d)" % bg.get_child_count())
	var flip: Vector2 = (bg._layers[0]["mat"] as ShaderMaterial).get_shader_parameter("uv_flip")
	_check(flip == Vector2(-1, 1) and is_equal_approx(bg.layer_info(0)["brightness"], 1.25), "omega: отражение и яркость звёзд из профиля")
	bg.apply_profile(Library.profile("deep"))
	await process_frame
	_check(bg.layer_count() == 2, "deep: два слоя")
	_check(bg._base.color.is_equal_approx(Library.profile("deep").base_color), "базовый цвет берётся из профиля")
	bg.apply_profile(Profile.from_dict({"id": "пусто", "layers": []}))
	await process_frame
	_check(bg.layer_count() == 0, "профиль без слоёв: фон остаётся базовым цветом, без падения")
	bg._process(1.0)
	h.queue_free()

	# недостающие текстуры
	Library.register_profile({"id": "tech", "name": "Тест", "layers": [{"texture": "x_stars", "wrap": "mirror"}, {"texture": "x_neb"}, {"texture": "x_gas", "blend": "mix"}]})
	h = _holder()
	bg = _bg_in(h, "tech")
	await process_frame
	_check(bg.missing_assets().size() == 3 and Placeholders.is_placeholder(bg.layer_info(0)["texture"]), "недостающие текстуры: показаны заглушки, ключи перечислены")
	Library.register_texture("x_gas", "res://assets/space/backgrounds/gas_turquoise.webp")
	bg.apply_profile(Library.profile("tech"))
	_check(bg.missing_assets().size() == 2 and not Placeholders.is_placeholder(bg.layer_info(2)["texture"]), "после появления текстуры заглушка заменяется")
	h.queue_free()

	# программная сборка
	var prof = Library.profile("xi")
	var img := SpaceBackground.compose_cpu(prof, Vector2i(48, 27))
	_check(img.get_width() == 48 and img.get_height() == 27, "compose_cpu: размер изображения")
	var dark := SpaceBackground.compose_cpu(prof, Vector2i(24, 14), Vector2.ZERO, "high", 0.0, 0.0)
	var all_base := true
	for yy in 14:
		for xx in 24:
			if not dark.get_pixel(xx, yy).is_equal_approx(Color(prof.base_color.r, prof.base_color.g, prof.base_color.b)):
				all_base = false
	_check(all_base, "compose_cpu: яркость и интенсивность 0 дают только базовый цвет")
	var hi := ArtStandard.measure(SpaceBackground.compose_cpu(prof, Vector2i(96, 54), Vector2.ZERO, "high"))
	var lo := ArtStandard.measure(SpaceBackground.compose_cpu(prof, Vector2i(96, 54), Vector2.ZERO, "low"))
	_check(hi["mean_lum"] > lo["mean_lum"], "compose_cpu: пресет low без газа темнее high (%.3f < %.3f)" % [lo["mean_lum"], hi["mean_lum"]])

	# ============ модуль стандарта ============
	var solid := func(c: Color) -> Image:
		var im := Image.create_empty(32, 18, false, Image.FORMAT_RGBF)
		im.fill(c)
		return im
	var black: Dictionary = ArtStandard.measure(solid.call(Color(0, 0, 0)))
	_check("почти чёрный" in str(ArtStandard.violations(black)), "стандарт: чёрный кадр отвергается")
	var magenta: Dictionary = ArtStandard.measure(solid.call(Color(0.9, 0.1, 0.8)))
	_check(not ArtStandard.violations(magenta).is_empty(), "стандарт: яркий пурпур отвергается")
	var green: Dictionary = ArtStandard.measure(solid.call(Color(0.05, 0.3, 0.1)))
	_check("оттенок" in str(ArtStandard.violations(green)), "стандарт: зелёный кадр отвергается по оттенку")
	var navy: Dictionary = ArtStandard.measure(solid.call(Color(0.04, 0.09, 0.27)))
	_check(ArtStandard.violations(navy).is_empty(), "стандарт: глубокий синий принимается (%s)" % str(ArtStandard.violations(navy)))
	var ring := Image.create_empty(60, 34, false, Image.FORMAT_RGBF)
	ring.fill(Color(0.4, 0.4, 0.9))
	for yy in range(10, 24):
		for xx in range(18, 42):
			ring.set_pixel(xx, yy, Color(0.01, 0.01, 0.03))
	var rm: Dictionary = ArtStandard.measure(ring)
	_check(rm["center_lum"] < 0.05 and rm["mean_lum"] > rm["center_lum"], "стандарт: яркость центра измеряется отдельно")
	var turq: Dictionary = ArtStandard.measure(solid.call(Color(0.1, 0.9, 0.85)))
	_check("бирюзы" in str(ArtStandard.violations(turq)), "стандарт: избыток яркой бирюзы отвергается")

	UiSettings.override("reduce_motion", null)
	UiSettings.override("graphics_preset", null)
	Library.reset()
	print("Итого: проверок %d, провалов %d" % [_checks, _failures])
	quit(1 if _failures > 0 else 0)
