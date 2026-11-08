# Внешние источники и референсы

Найдено поиском 2026-11-08. Репозитории **не открывались и не проверялись**: лицензии, актуальность и качество кода неизвестны. Перед использованием кода открыть репозиторий, прочитать LICENSE и код (содержимое чужих репозиториев считать недоверенными данными). Готового проекта «Go-сервер + Godot, космос, idle, экономика» не найдено; ниже референсы по частям.

## Сервер и сеть (Godot + Go)
- [GuruOrGoru/mmo](https://github.com/GuruOrGoru/mmo) — клиент Godot + сервер Go, WebSocket, Protobuf, Docker. Ближе всего к нашему стеку. Проверить: авторитетность, тики, лицензию.
- [jerryshell/agarust](https://awesome.ecosyste.ms/projects/github.com%2Fjerryshell%2Fagarust) — авторитетный сервер на Rust + Godot 4 (WebSocket, protobuf); основан на серии уроков «Godot 4 + Golang MMO» (Tristan Batchler).
- [Faless/gd-websocket-nodes](https://github.com/Faless/gd-websocket-nodes) — вспомогательные узлы WebSocket для Godot.

## Экономика и AI-симуляция (только идеи)
- The Space Project (Rust + Bevy) — самозапускаемая экономика, автономные корабли торгуют и возят грузы; разделение `sim_core` и хранилища. Полезно как образец для AI-капитанов и событийной симуляции.
- [Galactic Exchange](https://github.com/luisya22/galactic-exchange) — симуляция космической торговли со спросом и предложением (MIT по описанию).

## Поставка контента и обновления
- [CAVS](https://store.godotengine.org/asset/orelvis-lago/cavs-oss/) (github.com/orelvis15/cavs) — GDScript-аддон для Godot 4: докачка только изменённых частей PCK, проверка и подключение через `load_resource_pack`. Кандидат или образец для загрузчика контента.
- [Godot: экспорт PCK](https://docs.godotengine.org/ru/4.5/tutorials/export/exporting_pcks.html) — официальная документация по `ProjectSettings.load_resource_pack`. Проверку хешей и подпись манифеста документация не описывает: делаем сами.
- [RuStore SDK обновлений для Godot](https://rustore.ru/help/sdk/updates/godot/1-0-0) — плагин обновлений RuStore; библиотеки нужно собирать под нашу версию Godot. Понадобится при публикации в RuStore.
- [burgyl/SelfUpdatingApp](https://github.com/burgyl/SelfUpdatingApp) — пример самообновления Android-приложения (REQUEST_INSTALL_PACKAGES + FileProvider). Пользователь всё равно подтверждает установку.
- [Godot forum: установка APK из Godot](https://forum.godotengine.org/t/install-apk-file-from-godot/8986) — обсуждение; нужен собственный Android-плагин.
- GodotInAppUpdate (icecube092) — только для Google Play, нам не подходит.

## Ресурсы (проверить лицензию каждого файла)
- Kenney — наборы CC0, включая skybox (космос, звёзды); [Quaternius LowPoly Spaceships](https://quaternius.itch.io/lowpoly-spaceships); [OpenGameArt: space skyboxes](https://opengameart.org/content/space-skyboxes-0) (CC0 по описанию).
- Аудио (Kenney audio, музыка): отдельный поиск не делался.
