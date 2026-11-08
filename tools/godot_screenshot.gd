extends SceneTree
# Скриншот сцены. Переменные: SCENE (res://...), SHOT (путь png),
# INSETS="левый,верхний,правый,нижний" — имитация выреза камеры (px окна), если на сцене есть узел SafeArea.
func _initialize():
	var s = load(OS.get_environment("SCENE") if OS.get_environment("SCENE") != "" else "res://scenes/main.tscn").instantiate()
	root.add_child(s)
	var ins := OS.get_environment("INSETS")
	if ins != "":
		var p := ins.split(",")
		var sa = s.find_child("SafeArea", true, false)
		if sa != null and p.size() == 4:
			sa.set_debug_insets(int(p[0]), int(p[1]), int(p[2]), int(p[3]))
	await create_timer(0.5).timeout
	await RenderingServer.frame_post_draw
	var img = root.get_texture().get_image()
	img.save_png(OS.get_environment("SHOT"))
	print("SHOT_SAVED ", img.get_size())
	quit()
