extends SceneTree
## Real local Go + isolated PostgreSQL, invoked by TestEquipmentGodotClientPostgres.

const Gateway := preload("res://ui/gateway/server_gateway.gd")
const Session := preload("res://ui/session.gd")
const Identity := preload("res://ui/player_identity.gd")

var _failed := false

func _initialize() -> void:
	_run.call_deferred()

func _check(ok: bool, text: String) -> void:
	if not ok:
		_failed = true
		push_error(text)

func _wait_revision(gateway: Node, revision: int) -> bool:
	var deadline := Time.get_ticks_msec() + 8000
	while Time.get_ticks_msec() < deadline:
		if int(gateway.snapshot().get("ship", {}).get("equipment", {}).get("revision", -1)) == revision:
			return true
		await process_frame
	return false

func _run() -> void:
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	var token := OS.get_environment("COSMOC_TEST_EQUIPMENT_TOKEN")
	if not url.begins_with("ws://127.0.0.1:") or token == "":
		push_error("Requires isolated localhost server and synthetic equipment token")
		quit(1)
		return
	Identity.set_override(token)
	var gateway := Gateway.new()
	root.add_child(gateway)
	Session.gateway = gateway
	gateway.connect_to_server(url)
	_check(await _wait_revision(gateway, 0), "Initial equipment not received")
	var screen: Control = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	root.add_child(screen)
	await process_frame
	var hangar: Control = screen.open_hangar()
	await process_frame
	_check(not hangar._engine_button.disabled, "Docked engine change disabled")
	hangar._engine_button.pressed.emit()
	_check(await _wait_revision(gateway, 1), "Unequip did not commit")
	await process_frame
	_check(not bool(gateway.snapshot()["ship"]["modules"][0]["equipped"]), "Unequip not reflected")
	_check(hangar._engine_button.text.contains("Установить"), "Open hangar did not refresh")
	gateway.cast_ability("M01")
	await create_timer(1.2).timeout
	_check(not bool(gateway.snapshot().get("ship", {}).get("boosting", false)), "снятый двигатель всё ещё включает форсаж")
	# Reconnect the real client while the server retains the live ship.
	gateway.close()
	Session.gateway = null
	screen.queue_free()
	await process_frame
	gateway.queue_free()
	await process_frame
	gateway = Gateway.new()
	root.add_child(gateway)
	Session.gateway = gateway
	gateway.connect_to_server(url)
	_check(await _wait_revision(gateway, 1), "Reconnect reset equipment revision")
	_check(not bool(gateway.snapshot().get("ship", {}).get("modules", [{}])[0].get("equipped", true)), "Reconnect refitted removed engine")
	screen = load("res://scenes/space.tscn").instantiate()
	screen.auto_exit = false
	root.add_child(screen)
	await process_frame
	hangar = screen.open_hangar()
	await process_frame
	hangar._engine_button.pressed.emit()
	_check(await _wait_revision(gateway, 2), "Refit did not commit")
	await process_frame
	gateway.cast_ability("M01")
	var deadline := Time.get_ticks_msec() + 5000
	while Time.get_ticks_msec() < deadline and not bool(gateway.snapshot().get("ship", {}).get("boosting", false)):
		await process_frame
	_check(bool(gateway.snapshot().get("ship", {}).get("boosting", false)), "Refitted M01 did not execute")
	gateway.close()
	Session.gateway = null
	screen.queue_free()
	await process_frame
	await create_timer(3.7).timeout
	Identity.set_override("")
	if not _failed:
		print("EQUIPMENT_ONLINE_PASS: PostgreSQL unequip -> reconnect -> refit -> M01")
	quit(1 if _failed else 0)
