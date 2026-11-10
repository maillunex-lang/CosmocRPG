extends SceneTree
## Подсказка задания на карте: текст, вид точки и журнал. Сеть не нужна.

const Hints := preload("res://ui/quest_hints.gd")
const MapScreen := preload("res://ui/map_screen.gd")

var _f := 0
var _n := 0


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _run() -> void:
	_c(Hints.text({}) == "" and Hints.marker({}) == "" and Hints.prefix({}) == "", "без задания карта молчит")
	var state := {"quests": [{"id": "meridian_trace", "title": "След в пустоте", "text": "Найдите гравитационную аномалию в Меридиане.", "marker": "anomaly", "system": "meridian", "progress": 0, "goal": 1}], "locations": []}
	_c(Hints.marker(state) == "anomaly" and Hints.text(state).contains("аномалию"), "активное задание даёт текст и вид точки")
	_c(Hints.prefix(state) == Hints.text(state), "без журнала подсказка — это текст задания")
	state["locations"] = [{"id": "st1", "name": "Станция «Рубеж»", "kind": "station", "system": "meridian"}]
	_c(Hints.prefix(state).contains("Журнал локаций: 1"), "журнал добавляется к подсказке")
	var aurora := {"system": {"id": "aurora"}, "quests": [{"kind": "kill", "system": "aurora"}]}
	_c(Hints.marked(aurora, "en-1", "hostile") and not Hints.marked(aurora, "st1", "station"), "убийство отмечает врагов, не станцию")
	var reach := {"system": {"id": "aurora"}, "quests": [{"kind": "reach", "target": "st1"}]}
	_c(Hints.marked(reach, "st1", "station") and not Hints.marked(reach, "st2", "station"), "долет отмечает только нужную станцию")
	var ore := {"quests": [{"kind": "mine", "target": "iron_ore"}]}
	_c(Hints.marked(ore, "d1", "deposit", "iron_ore") and not Hints.marked(ore, "d2", "deposit", "crystals"), "добыча отмечает нужный ресурс")
	var jump := {"quests": [{"kind": "jump", "target": "meridian"}]}
	_c(Hints.marked(jump, "", "gate", "meridian") and not Hints.marked(jump, "", "gate", "redstorm"), "прыжок отмечает нужные ворота")
	var other := {"system": {"id": "aurora"}, "quests": [{"kind": "visit", "target": "anomaly", "marker": "anomaly", "system": "meridian"}]}
	_c(not Hints.marked(other, "an-1", "anomaly"), "задание другой системы не отмечается")
	var pin := {"system": {"id": "aurora"}, "quests": [{"kind": "visit", "target": "aur-az02", "marker": "anomaly", "system": "aurora"}]}
	_c(Hints.marked(pin, "aur-az02", "anomaly") and not Hints.marked(pin, "aur-az01", "anomaly"), "визит отмечает конкретную точку, не все аномалии")
	var after_brief := {"system": {"id": "aurora"}, "ship": {"pos": Vector2(-12, 14)}, "quests": [
		{"id": "ch01_q02", "kind": "visit", "target": "aur-beacon", "marker": "signal", "system": "aurora", "title": "Первый маршрут"},
	], "objects": [
		{"id": "st1", "kind": "station", "pos": Vector2(-12, 14), "dead": false},
		{"id": "aur-beacon", "kind": "signal", "pos": Vector2(400, 180), "dead": false},
		{"id": "sg-1", "kind": "signal", "pos": Vector2(-200, 40), "dead": false},
	]}
	_c(Hints.guide_quest(after_brief).get("id", "") == "ch01_q02", "после допуска ведём к первому маршруту")
	_c(Hints.object_id(after_brief, Hints.guide_quest(after_brief)) == "aur-beacon", "курс берёт учебный маяк, не любой сигнал")
	_c(Hints.can_route(after_brief), "к маяку можно включить автопилот без заранее выбранной цели")
	var sides := {"system": {"id": "aurora"}, "ship": {"pos": Vector2.ZERO}, "quests": [
		{"id": "aur_side_pirate", "kind": "kill", "title": "Чужой"},
		{"id": "ch01_q03", "kind": "visit", "target": "aur-az02", "marker": "anomaly", "system": "aurora"},
	], "objects": [{"id": "aur-az02", "kind": "anomaly", "pos": Vector2(80, 90), "dead": false}, {"id": "en-1", "kind": "hostile", "pos": Vector2(10, 10), "dead": false}]}
	_c(Hints.guide_quest(sides).get("id", "") == "ch01_q03" and Hints.object_id(sides, Hints.guide_quest(sides)) == "aur-az02", "побочный пират не перебивает главу")
	var gate_q := {"system": {"id": "aurora"}, "quests": [{"id": "first_jump", "kind": "jump", "target": "meridian", "title": "За границей"}]}
	_c(Hints.can_route(gate_q), "прыжок в Меридиан — это маршрут, даже без выбранных ворот")
	var no_ore := {"quests": [{"id": "demo_ore", "kind": "mine", "target": "iron_ore"}], "objects": [{"id": "st1", "kind": "station", "pos": Vector2.ZERO}]}
	_c(not Hints.can_route(no_ore), "без жилы автопилот к добыче не притворяется готовым")
	var lay := {"points": [{"id": "aur-beacon", "pos": [400, 180]}], "deposits": [{"id": "d1", "resource": "iron_ore", "pos": [20, 30]}]}
	_c(Hints.layout_destination(lay, {"kind": "visit", "target": "aur-beacon"}) == Vector2(400, 180), "маяк находится в раскладке, если его нет в снимке")
	var holder := SubViewport.new()
	holder.size = Vector2i(1280, 720)
	root.add_child(holder)
	var m: Control = MapScreen.new()
	holder.add_child(m)
	await process_frame
	m.set_data({"radius": 1400, "stations": [{"id": "st1", "name": "Рубеж", "pos": [0, 0]}]}, state)
	await process_frame
	_c(m._info.text.contains("аномалию") and m._info.text.contains("Журнал локаций: 1"), "пустая карта показывает задание и журнал")
	m.select("st1")
	_c(m._info.text.contains("Рубеж") and not m._info.text.contains("Журнал"), "выбранная цель не затирается заданием")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
