extends Node3D
## 3D-представление корабля или станции. ВРЕМЕННЫЕ модели-заглушки из примитивов:
## настоящие модели придут через менеджер ресурсов (см. docs/ARCHITECTURE.md).
## Мировые координаты (x, y) отображаются в 3D как (x, 0, y). Нос корабля смотрит в -Z.

const Tokens := preload("res://ui/theme/tokens.gd")
const UiTheme := preload("res://ui/theme/ui_theme.gd")

var object_id := ""
var kind := ""

var _ring: MeshInstance3D
var _label: Label3D


static func color_for(kind_: String, stance: String, is_me: bool) -> Color:
	if is_me:
		return Tokens.ACCENT
	if kind_ == "station":
		return Tokens.SURFACE_RAISED.lightened(0.25)
	if stance == "hostile":
		return Tokens.DANGER
	return Tokens.ACCENT_2


func setup(id: String, kind_: String, stance: String, display_name: String, is_me: bool = false) -> void:
	object_id = id
	kind = kind_
	scale = Vector3.ONE * 1.6  # заглушки крупнее: при широком обзоре иначе почти не видны
	var col := color_for(kind_, stance, is_me)
	var mat := StandardMaterial3D.new()
	mat.albedo_color = col
	mat.metallic = 0.3
	mat.roughness = 0.5
	mat.emission_enabled = true
	mat.emission = col
	mat.emission_energy_multiplier = 0.25
	if kind_ == "station":
		_add_box(Vector3(6.0, 1.2, 6.0), Vector3.ZERO, mat)
		_add_box(Vector3(9.0, 0.2, 1.0), Vector3(0, 0.3, 0), mat)
		_add_box(Vector3(1.0, 0.2, 9.0), Vector3(0, 0.3, 0), mat)
	else:
		_add_box(Vector3(0.7, 0.3, 2.2), Vector3.ZERO, mat)            # корпус
		_add_box(Vector3(2.4, 0.1, 0.8), Vector3(0, 0, 0.4), mat)      # крылья
		_add_box(Vector3(0.12, 0.5, 0.5), Vector3(0, 0.3, 0.9), mat)   # киль
		var glow := MeshInstance3D.new()
		var sph := SphereMesh.new()
		sph.radius = 0.22
		sph.height = 0.44
		glow.mesh = sph
		var gm := StandardMaterial3D.new()
		gm.albedo_color = Tokens.ACCENT
		gm.emission_enabled = true
		gm.emission = Tokens.ACCENT
		gm.emission_energy_multiplier = 2.0
		glow.material_override = gm
		glow.position = Vector3(0, 0, 1.2)
		add_child(glow)

	_ring = MeshInstance3D.new()
	var torus := TorusMesh.new()
	var r := 4.2 if kind_ == "station" else 1.9
	torus.inner_radius = r
	torus.outer_radius = r + 0.12
	_ring.mesh = torus
	var rm := StandardMaterial3D.new()
	rm.albedo_color = Tokens.RARE
	rm.emission_enabled = true
	rm.emission = Tokens.RARE
	rm.emission_energy_multiplier = 1.5
	rm.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	_ring.material_override = rm
	_ring.visible = false
	add_child(_ring)

	_label = Label3D.new()
	_label.text = display_name
	_label.font = UiTheme.make_font(600)
	_label.font_size = 40
	_label.pixel_size = 0.016
	_label.outline_size = 10
	_label.outline_modulate = Tokens.BG
	_label.modulate = Tokens.TEXT
	_label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	_label.no_depth_test = true
	_label.position = Vector3(0, 1.8, 0) if kind_ != "station" else Vector3(0, 2.5, 0)
	add_child(_label)


func _add_box(size: Vector3, pos: Vector3, mat: Material) -> void:
	var m := MeshInstance3D.new()
	var box := BoxMesh.new()
	box.size = size
	m.mesh = box
	m.material_override = mat
	m.position = pos
	add_child(m)


## Мировая позиция (x, y) -> 3D (x, 0, y).
func set_world_position(p: Vector2) -> void:
	position = Vector3(p.x, 0.0, p.y)


## Направление носа в мировых координатах.
func set_direction(dir: Vector2) -> void:
	if dir.length() < 0.001:
		return
	# локальный -Z должен смотреть в (dir.x, 0, dir.y)
	rotation.y = atan2(-dir.x, -dir.y)


func set_selected(on: bool) -> void:
	if _ring != null:
		_ring.visible = on


func is_selected() -> bool:
	return _ring != null and _ring.visible
