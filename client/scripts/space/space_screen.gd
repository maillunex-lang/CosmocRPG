extends Control
## Экран «Космос» (решение UI-01): живой 2.5D-мир (2D-фон + 3D-корабли) под компактным HUD.
## Данные берёт из шлюза: Session.gateway или, если его нет, явный демо-шлюз (с плашкой «Демо»).
## Камера: ui/camera (свайп вращает, щипок масштабирует, тап выбирает цель). Движение кораблей
## интерполируется по снимкам состояния (без «шагов» при 10 снимках в секунду).

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const ShipView := preload("res://scripts/space/ship_view.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")
const BackgroundLibrary := preload("res://ui/space_bg/background_library.gd")
const FrameStats := preload("res://ui/debug/frame_stats.gd")
const CameraConfig := preload("res://ui/camera/space_camera_config.gd")
const CameraGestures := preload("res://ui/camera/camera_gestures.gd")
const LoadingOverlay := preload("res://ui/loading_overlay.gd")
const Hangar := preload("res://ui/ships/hangar.gd")
const ShopScreen := preload("res://ui/shop_screen.gd")
const CorporationScreen := preload("res://ui/corporation_screen.gd")
const ProfileScreen := preload("res://ui/profile_screen.gd")
const StarMapScreen := preload("res://ui/star_map_screen.gd")
const StationScreen := preload("res://ui/station_screen.gd")
const ChatOverlay := preload("res://ui/chat_overlay.gd")
const MapScreen := preload("res://ui/map_screen.gd")
const QuestLog := preload("res://ui/quest_log.gd")
const SettingsScreen := preload("res://ui/settings_screen.gd")
const DialogueCard := preload("res://ui/dialogue_card.gd")
const Nameplates := preload("res://ui/nameplates.gd")
const Tokens := preload("res://ui/theme/tokens.gd")
const WorldDecor := preload("res://scripts/space/world_decor.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const SfxRules := preload("res://audio/sfx.gd")

signal exit_requested

## true: «Выход» возвращает на экран подключения; в тестах выключается.
@export var auto_exit := true
@export var pick_radius := 90.0   # радиус выбора объекта касанием, логических px

const ME := "me"

var gateway: UiGateway
var camera_config: Resource
var gestures: RefCounted
## Источник времени (секунды); в тестах подменяется для детерминизма.
var time_source: Callable = Callable()
var stats: CanvasLayer   # отладочная панель производительности

var _ships: Dictionary = {}       # id -> ShipView
var _bg_id := ""
var _last_sim_time := -1.0
var _ship_speed := 0.0
var _last_me_pos := Vector2.ZERO
var _plates: Control   # экранные подписи объектов
var _decor: Node3D   # декор живых локаций (строится по раскладке сервера)

@onready var _hud: Control = %Hud
@onready var _bg: Control = %SpaceBackground
@onready var _ships_root: Node3D = %Ships
@onready var _rig: Node3D = %CameraRig
@onready var _camera: Camera3D = %Camera3D


## Ближайший к точке касания объект в пределах радиуса. points: id -> позиция на экране.
static func nearest_id(points: Dictionary, pos: Vector2, radius: float) -> String:
	var best := ""
	var best_d := radius
	for id in points:
		var d := (points[id] as Vector2).distance_to(pos)
		if d <= best_d:
			best_d = d
			best = id
	return best


func _now() -> float:
	if time_source.is_valid():
		return float(time_source.call())
	return float(Time.get_ticks_usec()) / 1000000.0


func _ready() -> void:
	var t_ready := Time.get_ticks_msec()
	stats = FrameStats.new()
	add_child(stats)
	stats.attach_to(_hud.get_node("%DebugSlot"))
	var show_loading := Session.opened_msec >= 0   # только при настоящем входе в мир (в тестах экрана загрузки нет)
	if Session.opened_msec >= 0:
		# от нажатия кнопки до начала _ready: загрузка и создание сцены, HUD и текстур
		stats.mark("сцена", float(t_ready - Session.opened_msec))
		Session.opened_msec = -1
	theme = UiTheme.build()

	# камера: конфигурация из данных, автоцентрирование можно включить пользовательской настройкой
	camera_config = CameraConfig.load_default()
	for w in camera_config.warnings:
		push_warning("Камера: " + w)
	if UiSettings.get_bool("camera_auto_center", camera_config.auto_center_enabled):
		camera_config.auto_center_enabled = true
	_rig.apply_config(camera_config, true)
	gestures = CameraGestures.new(camera_config.pinch_sensitivity)
	gestures.orbited.connect(func(d: Vector2) -> void: _rig.orbit(d.x, d.y))
	gestures.pinched.connect(func(f: float) -> void: _rig.zoom_by(f))
	gestures.tapped.connect(select_at)

	gateway = Session.gateway if Session.gateway != null else DemoGateway.new()
	if gateway.get_parent() == null:
		add_child(gateway)
	_hud.bind(gateway)
	_plates = Nameplates.new()
	_plates.name = "Nameplates"
	add_child(_plates)
	move_child(_plates, _hud.get_index())   # под интерфейсом, над миром
	_hud.camera_mode_requested.connect(_rig.cycle_mode)
	_hud.world_tapped.connect(select_at)
	_hud.hangar_requested.connect(open_hangar)
	_hud.shop_requested.connect(open_shop)
	_hud.map_requested.connect(open_map)
	_hud.quest_log_requested.connect(open_quest_log)
	_hud.settings_requested.connect(open_settings)
	_hud.profile_requested.connect(open_profile)
	_hud.corporation_requested.connect(open_corporation)
	_hud.star_map_requested.connect(open_star_map)
	_hud.station_requested.connect(open_station)
	_hud.chat_requested.connect(func() -> void: open_chat())
	_hud.move_transform = screen_to_world_move
	_rig.mode_changed.connect(func(_m: int) -> void: _hud.set_camera_mode_name(_rig.mode_name()))
	_hud.set_camera_mode_name(_rig.mode_name())
	_hud.modal_opened.connect(gestures.cancel)  # окно подтверждения прерывает жест камеры
	stats.mark("HUD", float(Time.get_ticks_msec() - t_ready))
	_hud.exit_requested.connect(_on_exit)
	gateway.state_updated.connect(_on_state)
	gateway.combat_event.connect(_on_combat_event)
	gateway.state_updated.connect(_on_state_for_overlays)
	var talk := DialogueCard.new()
	talk.name = "DialogueCard"
	add_child(talk)
	talk.bind(gateway)
	var t_state := Time.get_ticks_msec()
	_on_state(gateway.snapshot())
	stats.mark("мир+фон", float(Time.get_ticks_msec() - t_state))
	if show_loading:
		var ov := LoadingOverlay.new()
		add_child(ov)
		ov.finished.connect(func(sec: float) -> void: stats.mark("прогрев", sec * 1000.0))


func _process(delta: float) -> void:
	var now := _now()
	for id in _ships:
		_ships[id].update_interp(now)
	var me: Node3D = _ships.get(ME)
	if me != null:
		var p: Vector2 = me.world_pos
		if delta > 0.0:
			_ship_speed = lerpf(_ship_speed, p.distance_to(_last_me_pos) / delta, clampf(delta * 8.0, 0.0, 1.0))
		_last_me_pos = p
		_rig.viewport_height = size.y
		_rig.update(delta, Vector3(p.x, 0.0, p.y), me.heading, _ship_speed)
		if _decor != null:
			_decor.update(p, delta)
	_update_ship_labels(delta, me)
	# фон: сдвиг только от перемещения корабля в экранных осях; вращение, наклон и масштаб камеры фон не трогают
	var prof: Resource = _bg.profile
	if prof != null:
		_bg.set_camera(_rig.parallax_px() / prof.px_per_unit)


## Подписи объектов: экранный слой (ui/nameplates.gd). Показываются цель, свой корабль, ближайшие враги и игроки, станции,
## находки рядом; не больше MAX_PLATES, без пересечений; дальние бледнее. Принципы MMO: лимит, приоритет цели, затухание по расстоянию.
func _update_ship_labels(_delta: float, me: Node3D) -> void:
	var cam: Camera3D = _rig.camera
	if cam == null or _plates == null:
		return
	var area := Rect2(Vector2(8, 70), _plates.size - Vector2(16, 150)) if _plates.size.x > 100.0 else Rect2(0, 0, 1280, 720)
	var zoom: float = _rig.zoom_size()
	var me_pos: Vector2 = me.world_pos if me != null else Vector2.ZERO
	var close_view := zoom < 26.0   # камера вплотную: подписываются только цель и свой корабль
	var items: Array = []
	for id in _ships:
		var v: Node3D = _ships[id]
		v.use_3d_label = false
		v.update_label(0.0, zoom, 0.0)
		if cam.is_position_behind(v._label.global_position):
			continue
		var anchor: Vector2 = v.label_screen_pos(cam)
		var d: float = v.world_pos.distance_to(me_pos)
		var kind: String = v.kind
		var sel: bool = v.is_selected()
		var prio := -1.0
		var maxd := 1.0
		var col: Color = Tokens.TEXT
		if sel:
			prio = 1000.0
		elif v.me:
			prio = 900.0
		elif close_view:
			continue
		elif kind == "hostile":
			if not bool(v.get_meta("dead", false)):
				prio = 500.0 - d
				maxd = 140.0
				col = Tokens.DANGER.lightened(0.35)
		elif kind == "player" or kind == "ai" or kind == "convoy" or kind == "patrol":
			prio = 450.0 - d
			maxd = 160.0
			col = Tokens.ACCENT.lightened(0.3)
		elif kind == "station":
			prio = 300.0 - d * 0.2
			maxd = 260.0
		elif kind in ["deposit", "anomaly", "derelict", "signal"]:
			prio = 200.0 - d
			maxd = 70.0
			col = Tokens.RARE.lightened(0.2)
		if prio < 0.0 or (not sel and not v.me and d > maxd):
			continue
		var alpha := 1.0 if (sel or v.me) else clampf(1.0 - 0.6 * d / maxd, 0.4, 1.0)
		items.append({"id": id, "text": v._label.text, "anchor": anchor, "priority": prio, "force": sel or v.me, "alpha": alpha, "color": col, "big": sel or v.me, "icon": "bot" if kind == "ai" else ""})
	_plates.update_plates(items, area)


func ship_view(id: String) -> Node3D:
	return _ships.get(id)


## Декор строится один раз, когда сервер прислал раскладку системы (в демо раскладки нет: остаётся прежний космос).
func _ensure_decor() -> void:
	if _decor != null or not ("layout" in gateway):
		return
	var lay: Dictionary = gateway.layout
	if lay.is_empty():
		return
	_decor = WorldDecor.new()
	_decor.name = "WorldDecor"
	_ships_root.get_parent().add_child(_decor)
	_decor.build(lay, gateway.visual, UiSettings.graphics_preset())


func _on_state(state: Dictionary) -> void:
	# фон зависит от системы: текстуры и параметры берутся из профиля, который задают данные
	var bg_id: String = str(state.get("system", {}).get("background", ""))
	if bg_id != _bg_id:
		_bg_id = bg_id
		_bg.apply_profile(BackgroundLibrary.profile(bg_id))
	_ensure_decor()
	var now := _now()
	# новый снимок только при новом игровом времени; состояние без времени считается новым снимком каждый раз
	var sim_t: float = float(state.get("time", now))
	var new_snapshot := sim_t != _last_sim_time
	_last_sim_time = sim_t
	var seen := {}
	var ship: Dictionary = state.get("ship", {})
	if not ship.is_empty():
		_sync_ship(ME, "ship", "friendly", str(ship.get("name", "")), ship["pos"], ship["dir"], true, sim_t, now, new_snapshot, ship)
		seen[ME] = true
	for o in state.get("objects", []):
		_sync_ship(str(o["id"]), str(o["kind"]), str(o["stance"]), str(o["name"]), o["pos"], o["dir"], false, sim_t, now, new_snapshot, o)
		seen[o["id"]] = true
	for id in _ships.keys():
		if not seen.has(id):
			_ships[id].queue_free()
			_ships.erase(id)
	var target := str(state.get("target_id", ""))
	for id in _ships:
		_ships[id].set_selected(id == target)


func _sync_ship(id: String, kind: String, stance: String, display_name: String, pos: Vector2, dir: Vector2,
		is_me: bool, sim_t: float, now: float, new_snapshot: bool, info: Dictionary = {}) -> void:
	var v: Node3D = _ships.get(id)
	if v == null:
		v = ShipView.new()
		_ships_root.add_child(v)
		v.setup(id, kind, stance, display_name, is_me, info)
		v.buffer.delay = camera_config.snapshot_delay
		_ships[id] = v
		v.set_world_position(pos)  # первое появление: сразу на месте
		v.set_direction(dir)
		if is_me:
			_last_me_pos = pos
			var first := Vector3(pos.x, 0.0, pos.y)
			_rig.snap_to(first)
		new_snapshot = true
	if v.visual != null:
		# облик и тип корабля приходят от сервера; характеристики у всех обликов одинаковые
		if str(info.get("skin", "")) != "":
			v.set_ship(str(info.get("ship", info.get("type", ""))), str(info["skin"]))
		v.boosting = bool(info.get("boosting", false))
		var dead := bool(info.get("dead", false))
		v.set_meta("dead", dead)
		if not dead and not v.visual.alive:
			v.visual.respawn()
	if new_snapshot:
		v.push_snapshot(sim_t, now, pos, dir)


## Управление относительно камеры: вектор джойстика (вверх на экране = вперёд для камеры) в мировые оси.
func screen_to_world_move(v: Vector2) -> Vector2:
	var axes: Dictionary = _rig.screen_axes()
	var r: Vector3 = axes["right"]
	var f: Vector3 = axes["forward"]
	var up := -v.y
	return Vector2(r.x * v.x + f.x * up, r.z * v.x + f.z * up).limit_length(1.0)


## Карта системы поверх сцены: раскладка берётся у сервера (в демо — пустая карта малой системы).
func open_map() -> Control:
	var existing := get_node_or_null("MapLayer/MapOverlay")
	if existing != null:
		return existing
	gestures.cancel()
	var m: Control = MapScreen.new()
	m.name = "MapOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "MapLayer"
	add_child(layer)
	layer.add_child(m)
	m.theme = theme
	var lay: Dictionary = gateway.layout if "layout" in gateway else {}
	m.set_data(lay, _map_state())
	var on_state := func(st: Dictionary) -> void:
		if is_instance_valid(m):
			m.set_data(lay, _map_state_from(st))
	gateway.state_updated.connect(on_state)
	m.target_selected.connect(func(id: String) -> void:
		if not id.begins_with("gate:"):
			gateway.set_target(id))
	m.autopilot_requested.connect(func(id: String) -> void:
		if id.begins_with("gate:"):
			_fly_gate(id.trim_prefix("gate:"))
		else:
			gateway.set_target(id)
			gateway.set_autopilot(true)
		layer.queue_free())
	m.waypoint_requested.connect(func(pos: Vector2) -> void:
		gateway.set_waypoint(pos)
		layer.queue_free())
	m.closed.connect(func() -> void: layer.queue_free())
	m.tree_exiting.connect(func() -> void: gateway.state_updated.disconnect(on_state))
	return m


func _map_state() -> Dictionary:
	return _map_state_from(gateway.snapshot())


func _map_state_from(st: Dictionary) -> Dictionary:
	var copy := st.duplicate()
	if gateway.has_method("jump_gates"):
		copy["gates"] = gateway.jump_gates()
	return copy


## Журнал заданий поверх космоса: текст и автопилот к месту выполнения.
func open_quest_log() -> Control:
	var existing := get_node_or_null("QuestLogLayer/QuestLogOverlay")
	if existing != null:
		return existing
	gestures.cancel()
	var log: Control = QuestLog.new()
	log.name = "QuestLogOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "QuestLogLayer"
	add_child(layer)
	layer.add_child(log)
	log.theme = theme
	log.set_quests(gateway.snapshot().get("quests", []))
	log.fly_requested.connect(func(q: Dictionary) -> void:
		_fly_to_quest(q)
		layer.queue_free())
	log.closed.connect(func() -> void: layer.queue_free())
	return log


func _fly_gate(system_id: String) -> bool:
	for g in gateway.jump_gates():
		if str(g.get("to", "")) != system_id:
			continue
		var pos: Array = g.get("pos", [0, 0])
		gateway.set_waypoint(Vector2(float(pos[0]), float(pos[1])))
		return true
	return false


func _fly_to_quest(q: Dictionary) -> void:
	var snap: Dictionary = gateway.snapshot()
	var cur := str(snap.get("system", {}).get("id", ""))
	var need := str(q.get("system", ""))
	if need != "" and cur != "" and need != cur:
		if not _fly_gate(need) and _hud != null:
			_hud.show_toast("Ворота к системе задания не найдены")
		return
	if str(q.get("kind", "")) == "jump":
		if not _fly_gate(str(q.get("target", ""))) and _hud != null:
			_hud.show_toast("Ворота к системе задания не найдены")
		return
	var id := _quest_object_id(q, snap)
	if id != "":
		gateway.set_target(id)
		gateway.set_autopilot(true)
		return
	if _hud != null:
		_hud.show_toast("Цель задания сейчас не видна")


func _quest_object_id(q: Dictionary, snap: Dictionary) -> String:
	var target := str(q.get("target", ""))
	var marker := str(q.get("marker", ""))
	var kind := str(q.get("kind", ""))
	var me := Vector2.ZERO
	var ship: Variant = snap.get("ship", {})
	if ship is Dictionary and (ship as Dictionary).get("pos") is Vector2:
		me = (ship as Dictionary)["pos"]
	var best := ""
	var best_d := 1.0e12
	for raw in snap.get("objects", []):
		if typeof(raw) != TYPE_DICTIONARY:
			continue
		var o: Dictionary = raw
		if bool(o.get("dead", false)):
			continue
		var id := str(o.get("id", ""))
		var okind := str(o.get("kind", ""))
		if target != "" and id == target:
			return id
		var hit := false
		if marker != "" and okind == marker and (target == "" or target == okind or target == id):
			hit = true
		elif kind == "kill" and okind == "hostile" and target == "":
			hit = true
		elif kind == "mine" and okind == "deposit" and (target == "" or str(o.get("resource", "")) == target):
			hit = true
		elif kind == "visit" and (okind == target or (marker != "" and okind == marker and (target == "" or target == okind or target == id))):
			hit = true
		elif kind in ["sell", "deliver", "talk", "report"] and okind == "station":
			hit = true
		elif kind == "reach" and okind == "station" and (target == "" or target == "station"):
			hit = true
		if not hit:
			continue
		var pos: Variant = o.get("pos", Vector2.ZERO)
		var d := 0.0
		if pos is Vector2:
			d = me.distance_to(pos)
		if d < best_d:
			best_d = d
			best = id
	return best


## Окно чата поверх игры (R-61): полупрозрачное, наполовину или на весь экран; под ним видна игра и работает управление.
func open_chat(m: int = ChatOverlay.Mode.HALF) -> Control:
	var existing := get_node_or_null("ChatLayer/ChatOverlay")
	if existing != null:
		return existing
	if _hud == null or _hud.chat_panel == null or not _hud.chat_panel.enabled:
		_hud.show_toast("Чат доступен только при игре на сервере")
		return null
	var layer := CanvasLayer.new()
	layer.layer = 35
	layer.name = "ChatLayer"
	add_child(layer)
	var ov: Control = ChatOverlay.new()
	ov.name = "ChatOverlay"
	layer.add_child(ov)
	ov.theme = theme
	ov.open(_hud.chat_panel, m)
	ov.closed.connect(func() -> void: layer.queue_free())
	return ov


func close_chat() -> bool:
	var ov := get_node_or_null("ChatLayer/ChatOverlay")
	if ov == null:
		return false
	ov.close_overlay()
	return true


## Настройки поверх сцены: автоответ на атаку уходит серверу, панель FPS применяется сразу.
func open_settings() -> Control:
	var existing := get_node_or_null("SettingsLayer/SettingsOverlay")
	if existing != null:
		return existing
	gestures.cancel()
	var st: Control = SettingsScreen.new()
	st.name = "SettingsOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "SettingsLayer"
	add_child(layer)
	layer.add_child(st)
	st.theme = theme
	st.option_changed.connect(func(opt: String, v: bool) -> void:
		if opt == "auto_retaliate":
			gateway.set_option(opt, v)
		elif opt == FrameStats.SETTING_KEY:
			stats.set_shown(v))
	st.closed.connect(func() -> void: layer.queue_free())
	return st


var _hangar_wait_ship := ""   # корабль, смену на который мы ждём от сервера (ангар откроется заново с ним)


## Магазин поверх сцены. Каталог и результат покупок приходят от сервера; в демо магазин недоступен.
func open_shop() -> Control:
	var existing := get_node_or_null("ShopLayer/ShopOverlay")
	if existing != null:
		return existing
	if not gateway.has_shop():
		_hud.show_toast("Магазин доступен только при игре на сервере")
		return null
	gestures.cancel()
	var sh: Control = ShopScreen.new()
	sh.name = "ShopOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "ShopLayer"
	add_child(layer)
	layer.add_child(sh)
	sh.theme = theme
	sh.set_credits(int(gateway.snapshot().get("credits", -1)))
	var on_data := func(d: Dictionary) -> void:
		if is_instance_valid(sh):
			sh.set_data(d, int(gateway.snapshot().get("credits", -1)))
	var on_fail := func(t: String) -> void:
		if is_instance_valid(sh):
			sh.show_error(t)
	gateway.shop_updated.connect(on_data)
	gateway.shop_failed.connect(on_fail)
	sh.buy_requested.connect(func(id: String) -> void: gateway.shop_buy(id))
	sh.equip_requested.connect(func(slot: String, id: String) -> void: gateway.shop_equip(slot, id))
	sh.closed.connect(func() -> void:
		gateway.shop_updated.disconnect(on_data)
		gateway.shop_failed.disconnect(on_fail)
		layer.queue_free())
	gateway.shop_open()
	return sh


## Экран станции (Э-2): ремонт, покупка ресурсов, улучшение.
func open_station() -> Control:
	var existing := get_node_or_null("StationLayer/StationOverlay")
	if existing != null:
		return existing
	gestures.cancel()
	var st: Control = StationScreen.new()
	st.name = "StationOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "StationLayer"
	add_child(layer)
	layer.add_child(st)
	st.theme = theme
	(st as StationScreen).bind(gateway)
	st.closed.connect(func() -> void: layer.queue_free())
	return st


## Звёздная карта (Э-1): системы, фракции, переходы.
func open_star_map() -> Control:
	var existing := get_node_or_null("StarMapLayer/StarMapOverlay")
	if existing != null:
		return existing
	gestures.cancel()
	var sm: Control = StarMapScreen.new()
	sm.name = "StarMapOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "StarMapLayer"
	add_child(layer)
	layer.add_child(sm)
	sm.theme = theme
	(sm as StarMapScreen).bind(gateway)
	sm.closed.connect(func() -> void: layer.queue_free())
	return sm


## Профиль пилота: аватарка, титул, VIP, подписка, кредиты. Срок подписки подтягивается из магазина.
func open_profile() -> Control:
	var existing := get_node_or_null("ProfileLayer/ProfileOverlay")
	if existing != null:
		return existing
	gestures.cancel()
	var p: Control = ProfileScreen.new()
	p.name = "ProfileOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "ProfileLayer"
	add_child(layer)
	layer.add_child(p)
	p.theme = theme
	p.set_data(gateway.snapshot())
	p.set_shop_available(gateway.has_shop())
	p.set_corporation(gateway.corporation_snapshot())
	gateway.corporation_updated.connect(p.set_corporation)
	if gateway.has_corporations():
		gateway.corporation_info()
	var on_shop := func(d: Dictionary) -> void:
		if is_instance_valid(p):
			p.set_data(gateway.snapshot(), d.get("state", {}))
	gateway.shop_updated.connect(on_shop)
	var on_pilot := func(pl: Dictionary) -> void:
		if is_instance_valid(p):
			p.set_pilot(pl)
	gateway.pilot_updated.connect(on_pilot)
	p.attribute_requested.connect(func(attr: String) -> void: gateway.pilot_spend(attr, 1))
	if gateway.has_pilot():
		gateway.pilot_open()
	if gateway.has_shop():
		gateway.shop_open()   # состояние магазина: срок подписки
	p.shop_requested.connect(func() -> void:
		layer.queue_free()
		open_shop())
	p.closed.connect(func() -> void:
		gateway.shop_updated.disconnect(on_shop)
		gateway.pilot_updated.disconnect(on_pilot)
		layer.queue_free())
	return p


## Баланс в открытом магазине и повторное открытие ангара после смены корабля.
func _on_state_for_overlays(state: Dictionary) -> void:
	var shop := get_node_or_null("ShopLayer/ShopOverlay")
	if shop != null and state.has("credits"):
		shop.set_credits(int(state["credits"]))
	var hangar := get_node_or_null("HangarLayer/HangarOverlay")
	if hangar != null and str(state.get("ship", {}).get("type", "")) == hangar.ship_id:
		hangar.set_data(state.get("ship", {}))
	if _hangar_wait_ship != "" and str(state.get("ship", {}).get("type", "")) == _hangar_wait_ship:
		_hangar_wait_ship = ""
		var old := get_node_or_null("HangarLayer")
		if old != null:
			old.queue_free()
			await get_tree().process_frame
		open_hangar()


## Ангар поверх сцены (соединение с сервером остаётся). Выбор облика и специализации уходит на сервер.
func open_hangar() -> Control:
	if get_node_or_null("HangarLayer/HangarOverlay") != null:
		return get_node_or_null("HangarLayer/HangarOverlay")
	gestures.cancel()
	var h: Control = Hangar.new()
	h.name = "HangarOverlay"
	var own: Dictionary = gateway.snapshot().get("ship", {})
	if str(own.get("type", "")) != "":
		h.ship_id = str(own["type"])   # ангар показывает тот корабль, на котором летит игрок
	if str(own.get("skin", "")) != "":
		h.skin = str(own["skin"])
	if str(own.get("spec", "")) != "":
		h.spec = str(own["spec"])
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "HangarLayer"
	add_child(layer)
	layer.add_child(h)
	h.theme = theme
	var ship: Dictionary = gateway.snapshot().get("ship", {})
	h.set_data(ship)
	h.skin_selected.connect(func(k: String) -> void:
		if gateway.has_method("set_skin"):
			gateway.set_skin(k)
		var me: Node3D = _ships.get(ME)
		if me != null:
			me.set_ship(str(ship.get("type", "FTR-001")), k))
	h.spec_selected.connect(func(k: String) -> void:
		if gateway.has_method("set_spec"):
			gateway.set_spec(k))
	h.closed.connect(func() -> void:
		_hangar_wait_ship = ""
		layer.queue_free())
	h.engine_selected.connect(func(equipped: bool, revision: int) -> void:
		if gateway.has_method("set_starter_engine"):
			gateway.set_starter_engine(equipped, revision))
	h.equipment_refresh_requested.connect(func() -> void:
		if gateway.has_method("refresh_equipment"):
			gateway.refresh_equipment())
	h.ship_selected.connect(func(id: String) -> void:
		_hangar_wait_ship = id
		gateway.set_ship(id))
	var on_ships := func(list: Array) -> void:
		if is_instance_valid(h):
			h.set_fleet(list)
	gateway.ships_updated.connect(on_ships)
	h.tree_exiting.connect(func() -> void: gateway.ships_updated.disconnect(on_ships))
	gateway.request_ships()
	return h


## События боя от сервера (или демо) → эффекты. Решения о попаданиях и уроне уже приняты источником данных.
func _on_combat_event(e: Dictionary) -> void:
	var from_id := str(e.get("from", ""))
	var to_id := str(e.get("to", ""))
	var from_v: Node3D = _ships.get(_alias(from_id))
	var to_v: Node3D = _ships.get(_alias(to_id))
	match str(e.get("type", "")):
		"fire":
			if from_v != null:
				from_v.play("Fire", to_v.global_position if to_v != null else null)
		"hit_shield":
			if to_v != null:
				to_v.play("HitShield")
		"hit_hull":
			if to_v != null:
				to_v.play("HitHull")
		"death":
			if to_v != null:
				to_v.play("Death")
		"respawn":
			if to_v != null:
				to_v.play("Respawn")
		"boost":
			if from_v != null:
				from_v.play("Boost")
		"intercept":
			if from_v != null:
				from_v.play("Boost")
				from_v.play("Fire", to_v.global_position if to_v != null else null)
		"scan":
			if from_v != null:
				from_v.play("Scan", float(e.get("radius", 20.0)) / 2.0)
	var role := SfxRules.cue_for(str(e.get("type", "")), from_id, to_id, _self_ids())
	if role != "":
		var sfx := get_node_or_null("/root/Sfx")
		if sfx != null and sfx.has_method("play"):
			sfx.play(role)


func _self_ids() -> Array:
	var ids: Array = ["me"]
	if gateway != null and gateway.has_method("own_id"):
		var oid := str(gateway.own_id())
		if oid != "":
			ids.append(oid)
	return ids


## Идентификатор игрока в событиях сервера — публичный id; свой корабль в сцене хранится под ME.
func _alias(id: String) -> String:
	if id == "me" or (gateway != null and gateway.has_method("own_id") and id == gateway.own_id()):
		return ME
	return id


## Положения объектов на экране (для выбора касанием).
func screen_points() -> Dictionary:
	var pts := {}
	for id in _ships:
		if id == ME:
			continue
		var p3: Vector3 = (_ships[id] as Node3D).global_position
		if not _camera.is_position_behind(p3):
			pts[id] = _camera.unproject_position(p3)
	return pts


# --- ввод ---
# Начало жеста: только события, которые интерфейс не забрал (UI в приоритете: джойстик, кнопки, панели).
# Продолжение и завершение уже начатого жеста принимаются до интерфейса (_input): палец, начавший жест на
# свободной области, остаётся за камерой, даже если проходит над кнопкой или отпускается над панелью.

func _unhandled_input(event: InputEvent) -> void:
	var ms := Time.get_ticks_msec()
	if event is InputEventScreenTouch:
		if event.pressed:
			gestures.touch(event.index, true, event.position, ms)
	elif event is InputEventMouseButton and event.device != InputEvent.DEVICE_ID_EMULATION:
		match event.button_index:
			MOUSE_BUTTON_LEFT:
				if event.pressed:
					gestures.touch(CameraGestures.MOUSE_LEFT, true, event.position, ms)
			MOUSE_BUTTON_RIGHT:
				if event.pressed:
					gestures.touch(CameraGestures.MOUSE_RIGHT, true, event.position, ms)
			MOUSE_BUTTON_WHEEL_UP:
				if event.pressed:
					gestures.wheel(1, camera_config.wheel_zoom_step)
			MOUSE_BUTTON_WHEEL_DOWN:
				if event.pressed:
					gestures.wheel(-1, camera_config.wheel_zoom_step)
	elif event.is_action_pressed("ui_cancel"):
		if not close_corporation() and not close_chat():
			_on_exit()


func _input(event: InputEvent) -> void:
	if gestures == null:
		return
	var ms := Time.get_ticks_msec()
	if event is InputEventScreenTouch:
		if not event.pressed and gestures.has_touch(event.index):
			gestures.touch(event.index, false, event.position, ms)
	elif event is InputEventScreenDrag:
		if gestures.has_touch(event.index):
			gestures.drag(event.index, event.position)
	elif event is InputEventMouseButton and event.device != InputEvent.DEVICE_ID_EMULATION:
		if not event.pressed:
			var idx := CameraGestures.MOUSE_LEFT if event.button_index == MOUSE_BUTTON_LEFT else (CameraGestures.MOUSE_RIGHT if event.button_index == MOUSE_BUTTON_RIGHT else -1000)
			if idx != -1000 and gestures.has_touch(idx):
				gestures.touch(idx, false, event.position, ms)
	elif event is InputEventMouseMotion and event.device != InputEvent.DEVICE_ID_EMULATION:
		if event.button_mask & MOUSE_BUTTON_MASK_LEFT and gestures.has_touch(CameraGestures.MOUSE_LEFT):
			gestures.drag(CameraGestures.MOUSE_LEFT, event.position)
		if event.button_mask & MOUSE_BUTTON_MASK_RIGHT and gestures.has_touch(CameraGestures.MOUSE_RIGHT):
			gestures.drag(CameraGestures.MOUSE_RIGHT, event.position)


## Выбор цели касанием в точке экрана. Пустое место снимает цель, но не во время атаки или добычи:
## случайное касание карты не должно прерывать бой и сбор ресурсов (прекращают их кнопки или выбор другой цели).
func select_at(pos: Vector2) -> void:
	var id := nearest_id(screen_points(), pos, pick_radius)
	if id == "":
		var st: Dictionary = gateway.snapshot()
		if bool(st.get("attacking", false)) or str((st.get("ship", {}) as Dictionary).get("mining", "")) != "":
			return
	gateway.set_target(id)


func _notification(what: int) -> void:
	if what == NOTIFICATION_WM_GO_BACK_REQUEST:  # кнопка «Назад» на Android
		if not close_corporation() and not close_chat():   # сначала закрываем открытый экран корпорации или чат
			_on_exit()
	elif what == NOTIFICATION_APPLICATION_FOCUS_OUT or what == NOTIFICATION_WM_WINDOW_FOCUS_OUT:
		if gestures != null:
			gestures.cancel()  # потеря фокуса завершает активный жест без скачков


func _on_exit() -> void:
	exit_requested.emit()
	if auto_exit:
		Session.gateway = null
		get_tree().change_scene_to_file("res://scenes/connect.tscn")


func open_corporation() -> Control:
	var existing := get_node_or_null("CorporationLayer/CorporationOverlay")
	if existing != null:
		return existing
	if not gateway.has_corporations():
		_hud.show_toast("Корпорации доступны только при игре на сервере")
		return null
	gestures.cancel()
	var screen := CorporationScreen.new()
	screen.name = "CorporationOverlay"
	var layer := CanvasLayer.new()
	layer.layer = 40
	layer.name = "CorporationLayer"
	add_child(layer)
	layer.add_child(screen)
	screen.theme = theme
	screen.bind(gateway)
	screen.closed.connect(func() -> void: layer.queue_free())
	return screen


func close_corporation() -> bool:
	var screen := get_node_or_null("CorporationLayer/CorporationOverlay")
	if screen == null:
		return false
	screen._close_screen()
	return true
