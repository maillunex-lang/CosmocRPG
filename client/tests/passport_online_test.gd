extends SceneTree
## Live client integration against an ephemeral memory-backed Go server.
## Invoked by ws.TestPassportGodotClient; never uses production or a database.

const Gateway := preload("res://ui/gateway/server_gateway.gd")
const Session := preload("res://ui/session.gd")

var _failed := false

func _initialize() -> void:
	_run.call_deferred()

func _run() -> void:
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if not url.begins_with("ws://127.0.0.1:"):
		push_error("The passport integration test requires an ephemeral localhost server")
		quit(1)
		return
	var gateway := Gateway.new()
	root.add_child(gateway)
	Session.gateway = gateway
	gateway.connect_to_server(url)
	var deadline := Time.get_ticks_msec() + 10000
	while Time.get_ticks_msec() < deadline and gateway.snapshot().get("ship", {}).is_empty():
		await process_frame
	if gateway.snapshot().get("ship", {}).is_empty():
		push_error("No ship state received from local server")
		quit(1)
		return
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	root.add_child(screen)
	await process_frame
	var hangar: Control = screen.open_hangar()
	deadline = Time.get_ticks_msec() + 10000
	while Time.get_ticks_msec() < deadline and hangar._passport.is_empty():
		await process_frame
	var base: Dictionary = hangar._passport.get("base", {})
	var live: Dictionary = gateway.snapshot().get("ship", {})
	if str(hangar._passport.get("id", "")) != "FTR-001" or not hangar._passport_info.visible:
		push_error("Server passport did not reach the hangar")
		_failed = true
	if int(base.get("hull", -1)) != int(live.get("hull_max", -2)) or int(base.get("energy", -1)) != int(live.get("energy_max", -2)):
		push_error("Passport overwrote or lost authoritative base stats")
		_failed = true
	if hangar._max_level != 50 or not hangar._passport_info.text.contains("предварительно"):
		push_error("Server level cap or draft budget label missing")
		_failed = true
	var modules: Array = gateway.snapshot().get("ship", {}).get("modules", [])
	if modules.size() != 1 or not bool(modules[0].get("owned", false)) or not bool(modules[0].get("equipped", false)) or not hangar._module_info.text.contains("M01"):
		push_error("Bound engine did not reach the hangar")
		_failed = true
	if gateway._ability_ids.is_empty() or gateway._ability_ids[0] != "M01":
		push_error("M01 alias was lost in the gateway")
		_failed = true
	gateway.use_skill(0)
	deadline = Time.get_ticks_msec() + 5000
	while Time.get_ticks_msec() < deadline and not bool(gateway.snapshot().get("ship", {}).get("boosting", false)):
		await process_frame
	if not bool(gateway.snapshot().get("ship", {}).get("boosting", false)):
		push_error("M01 did not execute via the real client command")
		_failed = true
	Session.gateway = null
	if not _failed:
		print("PASSPORT_ONLINE_PASS: real WebSocket -> Strizh hangar -> bound engine -> M01 cast")
	quit(1 if _failed else 0)
