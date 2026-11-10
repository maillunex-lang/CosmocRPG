extends SceneTree
const Gateway := preload("res://ui/gateway/server_gateway.gd")
const Intent := preload("res://ui/gateway/purchase_intent.gd")
var failures := 0
func check(ok: bool, label: String) -> void:
	if not ok:
		failures += 1
		print("FAIL ", label)
class CaptureClient:
	extends Node
	var messages: Array = []
	func send_message(kind: String, payload: Dictionary = {}) -> String:
		messages.append({"kind":kind,"payload":payload.duplicate(true)})
		return str(messages.size())

func _initialize() -> void:
	_run.call_deferred()

func _run() -> void:
	var owner := "purchase-test-" + Crypto.new().generate_random_bytes(8).hex_encode()
	var a := Intent.new()
	check(a.configure("ws://test-a", owner), "configure")
	var first: Dictionary = a.prepare("iron_ore", 2.0)
	check(not first.is_empty(), "durable intent")
	check(a.prepare("iron_ore", 2.0) == first, "same intent reuses key")
	check(a.prepare("gas", 1.0).is_empty(), "cannot replace unresolved purchase")
	var restarted := Intent.new()
	check(restarted.configure("ws://test-a", owner), "restart")
	check(restarted.pending() == first, "restore intent without sending")
	var other := Intent.new()
	check(other.configure("ws://test-b", owner) and other.pending().is_empty(), "server isolation")
	check(not restarted.acknowledge("wrong-key"), "foreign receipt ignored")
	check(restarted.acknowledge(str(first.get("operation_id", ""))), "receipt clears")
	var after := Intent.new()
	check(after.configure("ws://test-a", owner) and after.pending().is_empty(), "no restart duplicate")
	check(after.prepare("gas", -1.0).is_empty(), "invalid request")
	var next: Dictionary = after.prepare("gas", 1.0)
	check(not next.is_empty() and next.get("operation_id") != first.get("operation_id"), "new intent has new key")
	after.acknowledge(str(next.get("operation_id", "")))
	var gw := Gateway.new()
	root.add_child(gw)
	var capture := CaptureClient.new()
	gw.add_child(capture)
	gw._client = capture
	gw.url = "ws://gateway-fixture"
	gw.player_id = owner
	gw._joined = true
	gw.buy("iron_ore", 2.0)
	gw.buy("iron_ore", 2.0)
	check(capture.messages.size() == 2, "retry dispatch")
	var request: Dictionary = capture.messages[0]["payload"]
	check(capture.messages[1]["payload"]["operation_id"] == request["operation_id"], "gateway preserves operation ID")
	gw._on_message("error", {"code":"purchase_unavailable", "operation_id":request["operation_id"]}, "1")
	check(not gw._purchase_intent.pending().is_empty(), "unknown response keeps intent")
	var second_gw := Gateway.new()
	root.add_child(second_gw)
	var second_capture := CaptureClient.new()
	second_gw.add_child(second_capture)
	second_gw._client = second_capture
	second_gw.url = gw.url
	second_gw._on_message("welcome", {"player_id":owner,"name":"Пилот"}, "2")
	var buys := 0
	var queries := 0
	for sent: Dictionary in second_capture.messages:
		if sent["kind"] == "buy": buys += 1
		if sent["kind"] == "purchase_status": queries += 1
	check(buys == 0 and queries == 1, "reconnect queries status without blind purchase")
	second_gw._on_message("purchase_status", {"found":false,"operation_id":request["operation_id"]}, "3")
	check(second_gw._purchase_intent.pending().is_empty(), "known rollback clears intent")
	gw._purchase_intent.acknowledge(str(request["operation_id"]))
	gw.queue_free()
	second_gw.queue_free()
	print("Итого: purchase intent, провалов %d" % failures)
	quit(0 if failures == 0 else 1)
