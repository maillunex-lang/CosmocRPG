extends SceneTree
## Экран подготовки: закрывается только по ровным кадрам, не раньше минимума и не позже максимума.
const Overlay := preload("res://ui/loading_overlay.gd")
var _f := 0
func _c(ok: bool, n: String) -> void:
	print("PASS  " if ok else "FAIL  ", n)
	if not ok:
		_f += 1
func _initialize() -> void:
	_run.call_deferred()
func _run() -> void:
	# ровные кадры: закрывается после минимума
	var a := Overlay.new()
	root.add_child(a)
	await process_frame
	for i in 60:
		a._process(1.0 / 60.0)
	_c(not a.is_done(), "до минимального времени (%.1f с) не закрывается" % Overlay.MIN_TIME)
	for i in 30:
		a._process(1.0 / 60.0)
	_c(a.is_done(), "после минимума и ровных кадров закрывается")
	var a_filter: int = a._root.mouse_filter
	# рваные кадры: держится до максимума
	var b := Overlay.new()
	root.add_child(b)
	await process_frame
	for i in 100:
		b._process(0.1)
	_c(b._t >= 5.0 and b.is_done(), "при рваных кадрах закрывается по максимуму")
	var c := Overlay.new()
	root.add_child(c)
	await process_frame
	for i in 40:
		c._process(0.1)   # 4 с рваных кадров
	_c(not c.is_done(), "рваные кадры не позволяют закрыться раньше максимума")
	_c(a_filter == Control.MOUSE_FILTER_IGNORE and c._root.mouse_filter == Control.MOUSE_FILTER_STOP, "пока идёт подготовка, касания в мир не проходят")
	quit(1 if _f > 0 else 0)
