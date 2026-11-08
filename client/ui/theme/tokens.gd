extends RefCounted
## Токены оформления «Premium Hybrid Sci-Fi» (решение UI-11). Единственное место,
## где заданы цвета и размеры интерфейса. Размеры логические: база 1280x720, масштаб даёт stretch.

# --- Палитра (утверждена владельцем) ---
const BG := Color("#081421")          # фон
const SURFACE := Color("#14283C")     # поверхности
const ACCENT := Color("#42C6E5")      # основной акцент
const ACCENT_2 := Color("#9575E8")    # вторичный акцент
const RARE := Color("#F3C46B")        # редкие награды
const DANGER := Color("#F16C79")      # опасность

# --- Производные (не входят в утверждённую палитру, подобраны к ней) ---
const SURFACE_RAISED := Color("#1D3854")  # наведение/нажатие на поверхности
const SURFACE_SUNKEN := Color("#0C1B2C")  # поля ввода, углублённые области
const TEXT := Color("#E8F2FA")            # основной текст
const TEXT_MUTED := Color("#93A9BF")      # вторичный текст
const SUCCESS := Color("#5FD6A0")         # успех/«всё хорошо»

const PANEL_ALPHA := 0.86     # полупрозрачность панелей
const BORDER_ALPHA := 0.30    # прозрачность рамки (цвет ACCENT)

# --- Отступы и скругления ---
const SPACE_XS := 4
const SPACE_SM := 8
const SPACE_MD := 12
const SPACE_LG := 16
const SPACE_XL := 24
const RADIUS := 10
const BORDER_WIDTH := 1

# --- Размеры нажимаемых элементов и шрифтов ---
const TOUCH_MIN := 56         # минимальная высота кнопки, логических px (не меньше 48 dp)
const FONT_SMALL := 16
const FONT_BODY := 20
const FONT_BUTTON := 22
const FONT_H2 := 26
const FONT_TITLE := 34

# --- Анимации (решение UI-11: открытие 150–250 мс) ---
const ANIM_OPEN_MS := 200


## Цвет в виде "#RRGGBB" для BBCode.
static func hex(c: Color) -> String:
	return "#" + c.to_html(false)


## Относительная яркость по WCAG 2.x.
static func luminance(c: Color) -> float:
	var f := func(v: float) -> float:
		return v / 12.92 if v <= 0.03928 else pow((v + 0.055) / 1.055, 2.4)
	return 0.2126 * f.call(c.r) + 0.7152 * f.call(c.g) + 0.0722 * f.call(c.b)


## Коэффициент контраста двух цветов по WCAG (от 1 до 21).
static func contrast(a: Color, b: Color) -> float:
	var la := luminance(a)
	var lb := luminance(b)
	var hi := maxf(la, lb)
	var lo := minf(la, lb)
	return (hi + 0.05) / (lo + 0.05)
