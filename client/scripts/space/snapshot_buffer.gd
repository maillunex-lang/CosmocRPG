extends RefCounted
## Буфер снимков состояния для интерполяции движения (клиентская сторона). Сервер (и демо-шлюз)
## присылает позиции дискретно, обычно 10–20 раз в секунду; без интерполяции корабль «шагает».
## Отображение идёт с небольшой задержкой (DELAY): между двумя известными снимками позиция
## плавно интерполируется. Это только визуальное сглаживание: игровое состояние не меняется.

const SIZE := 8
const MAX_EXTRAPOLATION := 0.25   # с: как долго можно продолжать движение при отсутствии новых снимков
const TELEPORT := 40.0            # ед.: больший скачок считается телепортом (без «пролёта» между точками)
const OFFSET_SMOOTH := 0.12

var delay := 0.12

var _t: Array[float] = []         # игровое время снимка, с
var _pos: Array[Vector2] = []
var _dir: Array[Vector2] = []
var _offset := 0.0                # локальное время минус игровое (сглаженное)
var _have_offset := false


func _init(delay_s: float = 0.12) -> void:
	delay = delay_s


func count() -> int:
	return _t.size()


func clear() -> void:
	_t.clear()
	_pos.clear()
	_dir.clear()
	_have_offset = false


## Добавить снимок. sim_t — игровое время, local_t — локальное время получения. Повтор и более старое время игнорируются.
func push(sim_t: float, local_t: float, pos: Vector2, dir: Vector2) -> bool:
	if not _t.is_empty() and sim_t + 0.5 < _t[_t.size() - 1]:
		clear()
	elif not _t.is_empty() and sim_t <= _t[_t.size() - 1] + 1e-6:
		return false
	if not _pos.is_empty() and pos.distance_to(_pos[_pos.size() - 1]) > TELEPORT:
		clear()
	var sample := local_t - sim_t
	if not _have_offset or absf(sample - _offset) > 0.5:
		_offset = sample
		_have_offset = true
	else:
		_offset += (sample - _offset) * OFFSET_SMOOTH
	_t.append(sim_t)
	_pos.append(pos)
	_dir.append(dir if dir.length() > 0.001 else Vector2(1, 0))
	if _t.size() > SIZE:
		_t.pop_front()
		_pos.pop_front()
		_dir.pop_front()
	return true


## Позиция и направление для локального времени local_now: { valid, pos, dir }.
func sample(local_now: float) -> Dictionary:
	var n := _t.size()
	if n == 0:
		return {"valid": false, "pos": Vector2.ZERO, "dir": Vector2(1, 0)}
	if n == 1:
		return {"valid": true, "pos": _pos[0], "dir": _dir[0]}
	var rt := local_now - _offset - delay
	if rt <= _t[0]:
		return {"valid": true, "pos": _pos[0], "dir": _dir[0]}
	var last := n - 1
	if rt >= _t[last]:
		# новых снимков ещё нет: продолжаем движение по последней скорости, но не дольше предела
		var dt: float = minf(rt - _t[last], MAX_EXTRAPOLATION)
		var seg := _t[last] - _t[last - 1]
		var vel := (_pos[last] - _pos[last - 1]) / maxf(seg, 0.001)
		return {"valid": true, "pos": _pos[last] + vel * dt, "dir": _dir[last]}
	for i in range(n - 1):
		if rt >= _t[i] and rt <= _t[i + 1]:
			var k := (rt - _t[i]) / maxf(_t[i + 1] - _t[i], 0.0001)
			var a := _dir[i].angle()
			var b := _dir[i + 1].angle()
			return {"valid": true, "pos": _pos[i].lerp(_pos[i + 1], k), "dir": Vector2.from_angle(lerp_angle(a, b, k))}
	return {"valid": true, "pos": _pos[last], "dir": _dir[last]}
