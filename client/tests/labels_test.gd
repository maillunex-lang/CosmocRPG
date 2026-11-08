extends SceneTree
## Динамические подписи: исчезают при приближении камеры и вблизи корабля игрока.
const ShipView := preload("res://scripts/space/ship_view.gd")
var _f := 0
func _c(ok: bool, n: String) -> void:
	print("PASS  " if ok else "FAIL  ", n)
	if not ok:
		_f += 1
func _alpha(v, zoom: float, dist: float) -> float:
	var a := 0.0
	for i in 200:
		a = v.update_label(0.05, zoom, dist)
	return a
func _initialize() -> void:
	var other := ShipView.new()
	root.add_child(other)
	other.setup("p", "ship", "neutral", "Пилот")
	var me := ShipView.new()
	root.add_child(me)
	me.setup("me", "ship", "friendly", "Я", true)
	_c(_alpha(other, 50.0, 30.0) > 0.99, "далеко и обычный масштаб: подпись видна")
	_c(_alpha(other, 18.0, 30.0) < 0.02, "камера приближена: подпись скрыта")
	_c(_alpha(other, 50.0, 2.0) < 0.02, "вплотную к кораблю игрока: подпись скрыта")
	_c(_alpha(me, 50.0, 0.0) > 0.99, "собственная подпись не скрывается от близости")
	_c(_alpha(me, 18.0, 0.0) < 0.02, "собственная подпись скрыта при приближении")
	other.set_selected(true)
	_c(_alpha(other, 18.0, 2.0) > 0.99, "выбранная цель подписана всегда")
	quit(1 if _f > 0 else 0)
