extends SceneTree
## Invoked by TestCorporationGodotClient with an ephemeral local Go server.
const Gateway := preload("res://ui/gateway/server_gateway.gd")
const Identity := preload("res://ui/player_identity.gd")
const Session := preload("res://ui/session.gd")

var failed := false

func _initialize() -> void:
	_run.call_deferred()

func check(ok: bool, text: String) -> void:
	if not ok:
		failed = true
	print("PASS  " if ok else "FAIL  ", text)

func wait_for(condition: Callable) -> bool:
	var end := Time.get_ticks_msec() + 8000
	while Time.get_ticks_msec() < end:
		if condition.call():
			return true
		await process_frame
	return false

func connect_pilot(token: String, url: String) -> Node:
	Identity.set_override(token)
	var gateway := Gateway.new()
	root.add_child(gateway)
	gateway.connect_to_server(url)
	var ready: bool = await wait_for(func() -> bool: return gateway.corporation_snapshot().get("known", false) and not gateway.snapshot().is_empty())
	check(ready, "пилот вошёл, сведения о членстве получены")
	return gateway

func _run() -> void:
	var url := OS.get_environment("COSMOC_TEST_WS_URL")
	if not url.begins_with("ws://127.0.0.1:"):
		push_error("Corporation integration requires an ephemeral localhost server")
		quit(1)
		return
	var owner = await connect_pilot("corporation-owner-local-test", url)
	Session.gateway = owner
	var space: Control = load("res://scenes/space.tscn").instantiate()
	space.auto_exit = false
	root.add_child(space)
	await process_frame
	space.get_node("%Hud").get_node("%CorporationMenuButton").pressed.emit()
	var screen: Control = space.get_node("CorporationLayer/CorporationOverlay")
	await wait_for(func() -> bool: return not owner.corporation_busy())
	check(screen._empty.visible, "меню открыло экран без членства")
	screen._name_input.text = "Local Test Fleet"
	screen._tag_input.text = "lt"
	screen._update_actions()
	screen._create.pressed.emit()
	screen._confirm._close(true)
	check(await wait_for(func() -> bool: return not owner.corporation_busy() and not owner.corporation_snapshot().get("corp", {}).is_empty()), "создание подтверждено сервером")
	check(screen._corp_title.text == "[LT] Local Test Fleet", "название и тег дошли до экрана")
	check(await wait_for(func() -> bool: return int(owner.snapshot().get("credits", -1)) == 10000), "взнос отражён в балансе")
	var profile: Control = space.open_profile()
	check(profile.find_child("CorporationValue", true, false).text == "[LT] Local Test Fleet", "профиль показывает серверный тег")
	await wait_for(func() -> bool: return not owner.corporation_busy())

	var member = await connect_pilot("corporation-member-local-test", url)
	member.corporation_join("not-found")
	await wait_for(func() -> bool: return not member.corporation_busy())
	check(member.corporation_snapshot().get("corp", {}).is_empty(), "неудачное вступление не создаёт членство")
	member.corporation_join("lt")
	check(await wait_for(func() -> bool: return not member.corporation_busy() and not member.corporation_snapshot().get("corp", {}).is_empty()), "второй пилот вступил по тегу")
	owner.corporation_info()
	await wait_for(func() -> bool: return not owner.corporation_busy())
	check(screen._members.get_child_count() == 2, "обновление показывает обоих участников")
	member.close()
	member.queue_free()
	await process_frame
	member = await connect_pilot("corporation-member-local-test", url)
	check(member.corporation_snapshot().get("corp", {}).get("tag", "") == "LT", "членство восстановлено при новом подключении")
	member.corporation_leave()
	check(await wait_for(func() -> bool: return not member.corporation_busy() and member.corporation_snapshot().get("corp", {}).is_empty()), "второй пилот вышел")
	owner.corporation_info()
	await wait_for(func() -> bool: return not owner.corporation_busy())
	screen._leave.pressed.emit()
	screen._confirm._close(true)
	check(await wait_for(func() -> bool: return not owner.corporation_busy() and screen._empty.visible), "основатель вышел через подтверждение")
	check(profile.find_child("CorporationValue", true, false).text == "нет", "открытый профиль обновляется после выхода")
	var exited := [false]
	space.exit_requested.connect(func() -> void: exited[0] = true)
	screen._name_input.text = "Another Fleet"
	screen._tag_input.text = "AF"
	screen._update_actions()
	screen._create.pressed.emit()
	space.notification(Node.NOTIFICATION_WM_GO_BACK_REQUEST)
	check(not screen._confirm.is_open() and not exited[0], "Android Назад сначала отменяет подтверждение")
	space.notification(Node.NOTIFICATION_WM_GO_BACK_REQUEST)
	await process_frame
	check(space.get_node_or_null("CorporationLayer") == null and not exited[0], "Android Назад закрывает корпорацию без выхода из игры")
	owner.close()
	member.close()
	member.queue_free()
	Session.gateway = null
	Identity.set_override("")
	space.queue_free()
	await process_frame
	await process_frame
	await create_timer(4.5).timeout
	if not failed:
		print("CORPORATION_ONLINE_PASS")
	quit(1 if failed else 0)
