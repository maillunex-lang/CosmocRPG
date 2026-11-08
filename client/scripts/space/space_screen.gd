extends Control
## Экран «Космос» (решение UI-01): живой 2.5D-мир (2D-звёзды + 3D-корабли) под компактным HUD.
## Данные берёт из шлюза: Session.gateway или, если его нет, явный демо-шлюз (с плашкой «Демо»).

const Session := preload("res://ui/session.gd")
const DemoGateway := preload("res://ui/gateway/demo_gateway.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")
const ShipView := preload("res://scripts/space/ship_view.gd")
const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

signal exit_requested

## true: «Выход» возвращает на экран подключения; в тестах выключается.
@export var auto_exit := true
@export var pick_radius := 90.0   # радиус выбора объекта касанием, логических px
@export var camera_follow := 6.0  # скорость сглаживания камеры

const ME := "me"
## Смещение камеры в мировых единицах: корабль виден в центре свободной области между левой и правой колонками HUD.
const CAMERA_SHIFT_X := 6.0

var gateway: UiGateway
var _ships: Dictionary = {}       # id -> ShipView
var _cam_goal := Vector3.ZERO

@onready var _hud: Control = %Hud
@onready var _stars: Node2D = %Starfield
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


func _ready() -> void:
	theme = UiTheme.build()
	_camera.position = Vector3(0, 28, 20)
	_camera.look_at(_rig.global_position)
	gateway = Session.gateway if Session.gateway != null else DemoGateway.new()
	if gateway.get_parent() == null:
		add_child(gateway)
	_hud.bind(gateway)
	_hud.exit_requested.connect(_on_exit)
	gateway.state_updated.connect(_on_state)
	_on_state(gateway.snapshot())


func _process(delta: float) -> void:
	_rig.position = _rig.position.lerp(_cam_goal, clampf(delta * camera_follow, 0.0, 1.0))
	_stars.set_scroll(Vector2(_rig.position.x, _rig.position.z))


func ship_view(id: String) -> Node3D:
	return _ships.get(id)


func _on_state(state: Dictionary) -> void:
	var seen := {}
	var ship: Dictionary = state.get("ship", {})
	if not ship.is_empty():
		_sync_ship(ME, "ship", "friendly", str(ship.get("name", "")), ship["pos"], ship["dir"], true)
		seen[ME] = true
		_cam_goal = Vector3(ship["pos"].x + CAMERA_SHIFT_X, 0.0, ship["pos"].y)
	for o in state.get("objects", []):
		_sync_ship(str(o["id"]), str(o["kind"]), str(o["stance"]), str(o["name"]), o["pos"], o["dir"], false)
		seen[o["id"]] = true
	for id in _ships.keys():
		if not seen.has(id):
			_ships[id].queue_free()
			_ships.erase(id)
	var target := str(state.get("target_id", ""))
	for id in _ships:
		_ships[id].set_selected(id == target)


func _sync_ship(id: String, kind: String, stance: String, display_name: String, pos: Vector2, dir: Vector2, is_me: bool) -> void:
	var v: Node3D = _ships.get(id)
	if v == null:
		v = ShipView.new()
		_ships_root.add_child(v)
		v.setup(id, kind, stance, display_name, is_me)
		_ships[id] = v
	v.set_world_position(pos)
	v.set_direction(dir)


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


func _unhandled_input(event: InputEvent) -> void:
	var pos := Vector2.ZERO
	if event is InputEventScreenTouch and event.pressed:
		pos = event.position
	elif event is InputEventMouseButton and event.pressed and event.button_index == MOUSE_BUTTON_LEFT \
			and event.device != InputEvent.DEVICE_ID_EMULATION:
		pos = event.position
	elif event.is_action_pressed("ui_cancel"):
		_on_exit()
		return
	else:
		return
	select_at(pos)


## Выбор цели касанием в точке экрана (пустое место снимает цель).
func select_at(pos: Vector2) -> void:
	gateway.set_target(nearest_id(screen_points(), pos, pick_radius))


func _notification(what: int) -> void:
	if what == NOTIFICATION_WM_GO_BACK_REQUEST:  # кнопка «Назад» на Android
		_on_exit()


func _on_exit() -> void:
	exit_requested.emit()
	if auto_exit:
		Session.gateway = null
		get_tree().change_scene_to_file("res://scenes/connect.tscn")
