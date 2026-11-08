---
name: godot-gdscript
description: >
  Типизированный GDScript для Godot 4.7 в 2D-клиенте Уралии.
  Use when writing a script, signal, @export, @onready, await, or Godot 4 node lifecycle.
---

# GDScript для Godot 4.7

Клиент 2D (`AnimatedSprite2D`, 720×1280), не 3D.
Статическая типизация. `class_name` там, где тип переиспользуется.

`_process` для визуала, `_physics_process` почти не нужен: боя на физике нет.
Не вызывать `get_node` по длинному пути каждый кадр, кэшировать через `@onready`.
Сигнал вместо прямой ссылки на чужую ноду.

Боевые числа в нодах не хранить. Исход считает движок на сервере, клиент только проигрывает события.

Не использовать API Godot 3: `yield`, `KinematicBody`, `move_and_slide` с аргументом.
Версию редактора сверять (`godot --version`, сейчас 4.7.2), не выдумывать методы, при сомнении проверять headless-запуском.
Текст интерфейса — только через ключи локализации.

Проверка: `godot --headless --path client` и `.venv/bin/pytest -q`.

Выход: скрипт, на какую ноду вешать, какие поля `@export`.

<!-- Источник: Ufacity godot-gdscript. 3D и физика убраны, добавлены 2D и локализация. -->
