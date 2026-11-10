extends SceneTree
## Настоящие Godot → Go WS → PostgreSQL; приложение не обработало первый ACK.
const Gateway := preload("res://ui/gateway/server_gateway.gd")
const Identity := preload("res://ui/player_identity.gd")
var failed := false
func check(ok: bool, label: String) -> void:
	if not ok:
		failed = true
		push_error(label)
func _initialize() -> void:
	_run.call_deferred()
func wait_for(condition: Callable) -> bool:
	var deadline := Time.get_ticks_msec() + 8000
	while Time.get_ticks_msec() < deadline:
		if condition.call(): return true
		await process_frame
	return false
func _run() -> void:
	var endpoint := OS.get_environment("COSMOC_TEST_WS_URL")
	var token := OS.get_environment("COSMOC_TEST_PURCHASE_TOKEN")
	if not endpoint.begins_with("ws://127.0.0.1:") or token.is_empty():
		push_error("Requires isolated local purchase server")
		quit(1)
		return
	Identity.set_override(token)
	var gw := Gateway.new()
	root.add_child(gw)
	gw.connect_to_server(endpoint)
	check(await wait_for(func() -> bool: return gw._joined), "join failed")
	if failed:
		gw.close()
		quit(1)
		return
	# Транспорт видит ACK, но application callback отключён: durable intent остаётся.
	gw._client.message_received.disconnect(gw._on_message)
	var replies: Array = []
	gw._client.message_received.connect(func(kind: String, payload: Dictionary, _id: String) -> void:
		if kind == "ok" and payload.get("action", "") == "buy": replies.append(payload))
	gw.buy("gas", 1.0)
	gw.buy("gas", 1.0)
	check(await wait_for(func() -> bool: return replies.size() >= 2), "buy/replay did not respond")
	check(not gw._purchase_intent.pending().is_empty(), "lost application ACK discarded intent")
	gw.close()
	gw.queue_free()
	await process_frame
	var restored := Gateway.new()
	root.add_child(restored)
	var resolved: Array = []
	restored.station_done.connect(func(payload: Dictionary) -> void:
		if payload.get("action", "") == "buy": resolved.append(payload))
	restored.connect_to_server(endpoint)
	check(await wait_for(func() -> bool: return not resolved.is_empty()), "reconnect did not resolve receipt")
	check(restored._purchase_intent.pending().is_empty(), "resolved intent retained")
	restored.close()
	restored.queue_free()
	await process_frame
	Identity.set_override("")
	if not failed: print("PURCHASE_ONLINE_PASS")
	quit(1 if failed else 0)
