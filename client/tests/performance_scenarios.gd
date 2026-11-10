extends SceneTree
## Deterministic client-only replay. -- <scenario 0..13> <frames> <commit SHA>
const Session := preload("res://ui/session.gd")
const Demo := preload("res://ui/gateway/demo_gateway.gd")
const DecorFixture := preload("res://tests/world_decor_test.gd")
var screen: Control
var gateway: Node

func _initialize() -> void:
	_run.call_deferred()

func _run() -> void:
	var args := OS.get_cmdline_user_args()
	var scenario := int(args[0]) if args.size() > 0 else 0
	var frames := int(args[1]) if args.size() > 1 else 1800
	var sha := args[2] if args.size() > 2 else ""
	if scenario < 0 or scenario > 13 or frames < 1:
		quit(2)
		return
	root.size = Vector2i(1280, 720)
	gateway = Demo.new()
	gateway.auto_tick = false
	gateway._objects.clear()
	var count: int = [0, 0, 10, 50, 100, 300, 50, 50, 50, 10, 0, 10, 100, 10][scenario]
	var rng := RandomNumberGenerator.new()
	rng.seed = 77
	for i in count:
		var pos := Vector2(rng.randf_range(-28, 28), rng.randf_range(-18, 18))
		if scenario == 12 and i % 2 == 0:
			pos += Vector2(500, 500)
		gateway._objects[str(i)] = {"id": str(i), "kind": "player" if i % 2 == 0 else "ai",
			"stance": "neutral", "name": "Fixture", "ship": "FTR-001", "skin": "a",
			"pos": pos, "dir": Vector2.RIGHT}
	Session.gateway = gateway
	screen = load("res://scenes/space.tscn").instantiate()
	root.add_child(screen)
	screen.auto_exit = false
	screen.get_node("%CameraRig").set_view(0, 60, 50)
	if scenario in [1, 11]:
		var fixture := DecorFixture.new()
		var data: Dictionary = fixture._data()
		fixture.free()
		var decor := preload("res://scripts/space/world_decor.gd").new()
		screen.get_node("%Ships").get_parent().add_child(decor)
		decor.build(data.layout, data.visual, "medium")
		screen._decor = decor
	if scenario == 10:
		screen.open_hangar()
	for frame in frames:
		gateway._t = frame / 60.0
		if scenario == 6:
			for object in gateway._objects.values():
				object.pos += Vector2(0.02, 0)
		if scenario == 11:
			gateway._ship.pos = Vector2(frame * 0.4, frame * 0.1)
		if frame % 6 == 0:
			gateway.state_updated.emit(gateway.snapshot())
		if scenario in [7, 8] and frame % 10 == 0:
			for id in gateway._objects:
				gateway.combat_event.emit({"type": "fire" if scenario == 7 else "hit_hull", "from": id, "to": id})
		if scenario == 9 and frame % 6 == 0:
			for object in gateway._objects.values():
				object.skin = "a" if frame % 12 == 0 else "b"
		if scenario == 13 and frame % 120 == 0:
			gateway.demo_next_system()
		await process_frame
	if sha != "":
		print("EXPORT ", screen.stats.export_json("user://performance-%d.json" % scenario, sha, scenario))
	screen.queue_free()
	await process_frame
	Session.gateway = null
	quit()
