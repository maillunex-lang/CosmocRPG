extends Control
## Экран «Космос» (решение UI-01): живой 2.5D-мир (2D-фон + 3D-корабли) под компактным HUD.
## Данные берёт из шлюза: Session.gateway или, если его нет, явный демо-шлюз (с плашкой «Демо»).
## Камера: ui/camera (свайп вращает, щипок масштабирует, тап выбирает цель). Движение кораблей
## интерполируется по снимкам состояния (без «шагов» при 10 снимках в секунду).

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const UiSettings := preload("res://ui/ui_settings.gd")
const ShipView := preload("res://scripts/space/ship_view.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")
const BackgroundLibrary := preload("res://ui/space_bg/background_library.gd")
const FrameStats := preload("res://ui/debug/frame_stats.gd")
const CameraConfig := preload("res://ui/camera/space_camera_config.gd")
const CameraGestures := preload("res://ui/camera/camera_gestures.gd")

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
	_hud.move_transform = screen_to_world_move
	_rig.mode_changed.connect(func(_m: int) -> void: _hud.set_camera_mode_name(_rig.mode_name()))
	_hud.set_camera_mode_name(_rig.mode_name())
	_hud.modal_opened.connect(gestures.cancel)  # окно подтверждения прерывает жест камеры
	stats.mark("HUD", float(Time.get_ticks_msec() - t_ready))
	_hud.exit_requested.connect(_on_exit)
	gateway.state_updated.connect(_on_state)
	var t_state := Time.get_ticks_msec()
	_on_state(gateway.snapshot())
	stats.mark("мир+фон", float(Time.get_ticks_msec() - t_state))


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
	# фон: сдвиг только от перемещения корабля в экранных осях; вращение, наклон и масштаб камеры фон не трогают
	var prof: Resource = _bg.profile
	if prof != null:
		_bg.set_camera(_rig.parallax_px() / prof.px_per_unit)


func ship_view(id: String) -> Node3D:
	return _ships.get(id)


func _on_state(state: Dictionary) -> void:
	# фон зависит от системы: текстуры и параметры берутся из профиля, который задают данные
	var bg_id: String = str(state.get("system", {}).get("background", ""))
	if bg_id != _bg_id:
		_bg_id = bg_id
		_bg.apply_profile(BackgroundLibrary.profile(bg_id))
	var now := _now()
	# новый снимок только при новом игровом времени; состояние без времени считается новым снимком каждый раз
	var sim_t: float = float(state.get("time", now))
	var new_snapshot := sim_t != _last_sim_time
	_last_sim_time = sim_t
	var seen := {}
	var ship: Dictionary = state.get("ship", {})
	if not ship.is_empty():
		_sync_ship(ME, "ship", "friendly", str(ship.get("name", "")), ship["pos"], ship["dir"], true, sim_t, now, new_snapshot)
		seen[ME] = true
	for o in state.get("objects", []):
		_sync_ship(str(o["id"]), str(o["kind"]), str(o["stance"]), str(o["name"]), o["pos"], o["dir"], false, sim_t, now, new_snapshot)
		seen[o["id"]] = true
	for id in _ships.keys():
		if not seen.has(id):
			_ships[id].queue_free()
			_ships.erase(id)
	var target := str(state.get("target_id", ""))
	for id in _ships:
		_ships[id].set_selected(id == target)


func _sync_ship(id: String, kind: String, stance: String, display_name: String, pos: Vector2, dir: Vector2,
		is_me: bool, sim_t: float, now: float, new_snapshot: bool) -> void:
	var v: Node3D = _ships.get(id)
	if v == null:
		v = ShipView.new()
		_ships_root.add_child(v)
		v.setup(id, kind, stance, display_name, is_me)
		v.buffer.delay = camera_config.snapshot_delay
		_ships[id] = v
		v.set_world_position(pos)  # первое появление: сразу на месте
		v.set_direction(dir)
		if is_me:
			_last_me_pos = pos
			var first := Vector3(pos.x, 0.0, pos.y)
			_rig.snap_to(first)
		new_snapshot = true
	if new_snapshot:
		v.push_snapshot(sim_t, now, pos, dir)


## Управление относительно камеры: вектор джойстика (вверх на экране = вперёд для камеры) в мировые оси.
func screen_to_world_move(v: Vector2) -> Vector2:
	var axes: Dictionary = _rig.screen_axes()
	var r: Vector3 = axes["right"]
	var f: Vector3 = axes["forward"]
	var up := -v.y
	return Vector2(r.x * v.x + f.x * up, r.z * v.x + f.z * up).limit_length(1.0)


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
