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
const MapScreen := preload("res://ui/map_screen.gd")
const WorldDecor := preload("res://scripts/space/world_decor.gd")
const UiSettings := preload("res://ui/ui_settings.gd")

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
	_hud.camera_mode_requested.connect(_rig.cycle_mode)
	_hud.world_tapped.connect(select_at)
	_hud.hangar_requested.connect(open_hangar)
	_hud.shop_requested.connect(open_shop)
	_hud.map_requested.connect(open_map)
	_hud.move_transform = screen_to_world_move
	_rig.mode_changed.connect(func(_m: int) -> void: _hud.set_camera_mode_name(_rig.mode_name()))
	_hud.set_camera_mode_name(_rig.mode_name())
	_hud.modal_opened.connect(gestures.cancel)  # окно подтверждения прерывает жест камеры
	stats.mark("HUD", float(Time.get_ticks_msec() - t_ready))
	_hud.exit_requested.connect(_on_exit)
	gateway.state_updated.connect(_on_state)
	gateway.combat_event.connect(_on_combat_event)
	gateway.state_updated.connect(_on_state_for_overlays)
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
	# фон: сдвиг только от перемещения корабля в экранных осях; вращение, наклон и масштаб камеры фон не трогают
	var prof: Resource = _bg.profile
	if prof != null:
		_bg.set_camera(_rig.parallax_px() / prof.px_per_unit)


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
	m.set_data(lay, gateway.snapshot())
	var on_state := func(st: Dictionary) -> void:
		if is_instance_valid(m):
			m.set_data(lay, st)
	gateway.state_updated.connect(on_state)
	m.target_selected.connect(func(id: String) -> void: gateway.set_target(id))
	m.autopilot_requested.connect(func(id: String) -> void:
		gateway.set_target(id)
		gateway.set_autopilot(true)
		layer.queue_free())
	m.closed.connect(func() -> void: layer.queue_free())
	m.tree_exiting.connect(func() -> void: gateway.state_updated.disconnect(on_state))
	return m


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


## Баланс в открытом магазине и повторное открытие ангара после смены корабля.
func _on_state_for_overlays(state: Dictionary) -> void:
	var shop := get_node_or_null("ShopLayer/ShopOverlay")
	if shop != null and state.has("credits"):
		shop.set_credits(int(state["credits"]))
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


## Выбор цели касанием в точке экрана (пустое место снимает цель).
func select_at(pos: Vector2) -> void:
	gateway.set_target(nearest_id(screen_points(), pos, pick_radius))


func _notification(what: int) -> void:
	if what == NOTIFICATION_WM_GO_BACK_REQUEST:  # кнопка «Назад» на Android
		_on_exit()
	elif what == NOTIFICATION_APPLICATION_FOCUS_OUT or what == NOTIFICATION_WM_WINDOW_FOCUS_OUT:
		if gestures != null:
			gestures.cancel()  # потеря фокуса завершает активный жест без скачков


func _on_exit() -> void:
	exit_requested.emit()
	if auto_exit:
		Session.gateway = null
		get_tree().change_scene_to_file("res://scenes/connect.tscn")
