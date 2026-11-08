extends RefCounted
## Художественный стандарт космического фона (docs/ui/ART_STANDARD.md): измерение и проверка.
## Стандарт: глубокий тёмно-синий космос, сине-фиолетовые туманности, умеренное бирюзовое свечение,
## тёмный центр для читаемости кораблей. Пороги выведены из эталонных изображений владельца.

# --- пороги ---
const MEAN_LUM_MAX := 0.24          # фон не должен быть ярким в целом
const MEAN_LUM_MIN := 0.04          # и не чёрным
const CENTER_LUM_MAX := 0.10        # центр (40% x 40%) тёмный: корабли и подписи читаются
const BLUE_VIOLET_MIN := 0.85       # доля сине-фиолетовых (оттенок 200–300°) среди насыщенных пикселей
const TURQUOISE_MAX := 0.08         # доля ярких бирюзовых пикселей (165–200°, S>0.4, V>0.5): «умеренное свечение»
const MEAN_HUE_MIN := 205.0         # средний цвет: глубокий синий
const MEAN_HUE_MAX := 255.0
const BASE_LUM_MAX := 0.03          # базовый цвет под слоями почти чёрный


static func luminance(c: Color) -> float:
	return 0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b


## Измерение изображения. Для скорости изображение уменьшается до ~384 px по ширине.
static func measure(source: Image) -> Dictionary:
	var img := source.duplicate() as Image
	if img.is_compressed():
		img.decompress()
	if img.get_width() > 384:
		img.resize(384, int(round(384.0 * img.get_height() / img.get_width())), Image.INTERPOLATE_BILINEAR)
	var w := img.get_width()
	var h := img.get_height()
	var total := 0.0
	var center_total := 0.0
	var center_n := 0
	var sat_n := 0
	var bv_n := 0
	var turq_n := 0
	var sum := Vector3.ZERO
	for y in h:
		for x in w:
			var c := img.get_pixel(x, y)
			var l := luminance(c)
			total += l
			sum += Vector3(c.r, c.g, c.b)
			if x >= int(w * 0.3) and x < int(w * 0.7) and y >= int(h * 0.3) and y < int(h * 0.7):
				center_total += l
				center_n += 1
			var mx := maxf(c.r, maxf(c.g, c.b))
			var s := c.s
			var hue := c.h * 360.0
			if s > 0.25 and mx > 0.12:
				sat_n += 1
				if hue >= 200.0 and hue < 300.0:
					bv_n += 1
			if hue >= 165.0 and hue < 200.0 and s > 0.4 and mx > 0.5:
				turq_n += 1
	var n := float(w * h)
	var mean := Color(sum.x / n, sum.y / n, sum.z / n)
	return {
		"mean_lum": total / n,
		"center_lum": center_total / maxf(1.0, center_n),
		"blue_violet": float(bv_n) / maxf(1.0, sat_n),
		"turquoise": float(turq_n) / n,
		"mean_hue": mean.h * 360.0,
		"mean_color": mean,
	}


## Список нарушений стандарта (пусто, если всё в норме).
static func violations(m: Dictionary) -> PackedStringArray:
	var v := PackedStringArray()
	if m["mean_lum"] > MEAN_LUM_MAX:
		v.append("средняя яркость %.3f > %.2f" % [m["mean_lum"], MEAN_LUM_MAX])
	if m["mean_lum"] < MEAN_LUM_MIN:
		v.append("средняя яркость %.3f < %.2f (почти чёрный)" % [m["mean_lum"], MEAN_LUM_MIN])
	if m["center_lum"] > CENTER_LUM_MAX:
		v.append("яркость центра %.3f > %.2f" % [m["center_lum"], CENTER_LUM_MAX])
	if m["blue_violet"] < BLUE_VIOLET_MIN:
		v.append("сине-фиолетовых %.0f%% < %.0f%%" % [m["blue_violet"] * 100.0, BLUE_VIOLET_MIN * 100.0])
	if m["turquoise"] > TURQUOISE_MAX:
		v.append("яркой бирюзы %.1f%% > %.0f%%" % [m["turquoise"] * 100.0, TURQUOISE_MAX * 100.0])
	if m["mean_hue"] < MEAN_HUE_MIN or m["mean_hue"] > MEAN_HUE_MAX:
		v.append("средний оттенок %.0f° вне [%.0f, %.0f]" % [m["mean_hue"], MEAN_HUE_MIN, MEAN_HUE_MAX])
	return v
