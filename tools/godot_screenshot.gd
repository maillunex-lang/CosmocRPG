extends SceneTree
func _initialize():
	var s = load(OS.get_environment("SCENE") if OS.get_environment("SCENE") != "" else "res://scenes/main.tscn").instantiate()
	root.add_child(s)
	await create_timer(0.5).timeout
	await RenderingServer.frame_post_draw
	var img = root.get_texture().get_image()
	img.save_png(OS.get_environment("SHOT"))
	print("SHOT_SAVED ", img.get_size())
	quit()
