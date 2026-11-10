extends SceneTree
## Эффекты М-3: отбор библиотеки, роли, пауза между звуками, чей выстрел слышен. Сеть не нужна.

const Lib := preload("res://audio/sfx.gd")

var _f := 0
var _n := 0


func _initialize() -> void:
	_run.call_deferred()


func _c(ok: bool, name: String) -> void:
	_n += 1
	if ok:
		print("PASS  ", name)
	else:
		_f += 1
		print("FAIL  ", name)


func _track(kind: String, title: String, sha: String, ext: String, tags: String, enabled: bool = true, url: String = "") -> Dictionary:
	var path := "/media/files/%s.%s" % [sha, ext]
	return {
		"kind": kind, "title": title, "enabled": enabled, "ext": ext, "sha256": sha,
		"tags": tags, "url": path if url == "" else url, "size": 4,
	}


func _run() -> void:
	var ui := "ab".repeat(32)
	var late := "ff".repeat(32)
	var early := "01".repeat(32)
	var bad := "zz".repeat(32)
	var catalog := {"tracks": [
		_track("music", "Фон", ui, "mp3", "ui"),
		_track("sfx", "Клик", ui, "mp3", "UI, menu"),
		_track("sfx", "Выкл", late, "ogg", "weapon", false),
		_track("sfx", "Чужой", ui, "mp3", "weapon", true, "https://evil.example/a.mp3"),
		_track("sfx", "Поздний выстрел", late, "ogg", "weapon"),
		_track("sfx", "Ранний выстрел", early, "mp3", "weapon;hit"),
		_track("sfx", "Двигатель", ui, "ogg", "engine"),
		_track("sfx", "Пустой тег", early, "mp3", ""),
		_track("sfx", "Opus", early, "opus", "ui"),
		_track("sfx", "Битый хэш", bad, "mp3", "ui"),
	]}
	var bucket: Dictionary = Lib.cues(catalog)
	_c(Lib.pick(bucket, "ui")["title"] == "Клик", "кнопка: включённый sfx с тегом ui")
	_c(Lib.pick(bucket, "engine")["title"] == "Двигатель", "двигатель по тегу")
	_c(Lib.pick(bucket, "weapon")["title"] == "Ранний выстрел", "из нескольких выстрелов берётся меньшая сумма")
	_c(Lib.pick(bucket, "hit")["sha256"] == early and Lib.pick(bucket, "hit")["url"] == "/media/files/%s.mp3" % early, "попадание и безопасный путь")
	_c(Lib.roles_in("Weapon, UI ui").size() == 2, "теги без регистра и повторов")
	_c(not Lib.engine_active(0.08, false) and Lib.engine_active(0.09, false) and Lib.engine_active(0.0, true), "двигатель слышен в ходе и на форсаже")
	_c(Lib.engine_pitch(0.0, false) >= 0.75 and Lib.engine_pitch(9.0, true) <= 1.45, "тон двигателя в границах")
	_c(Lib.allow(1000, -1, 45) and not Lib.allow(1000, 980, 45) and Lib.allow(1025, 980, 45), "пауза между одинаковыми звуками")
	var me := ["me", "pilot-1"]
	_c(Lib.cue_for("fire", "pilot-1", "npc", me) == "weapon", "свой выстрел")
	_c(Lib.cue_for("intercept", "me", "npc", me) == "weapon", "свой перехват звучит как выстрел")
	_c(Lib.cue_for("fire", "ai-captain", "npc", me) == "", "чужой и ИИ-выстрел без звука")
	_c(Lib.cue_for("hit_hull", "npc", "me", me) == "hit" and Lib.cue_for("hit_shield", "me", "npc", me) == "", "попадание только по себе")
	_c(Lib.cue_for("death", "npc", "me", me) == "" and Lib.cue_for("fire", "", "me", me) == "", "смерть и пустой источник молчат")
	print("Итого: проверок %d, провалов %d" % [_n, _f])
	quit(1 if _f > 0 else 0)
