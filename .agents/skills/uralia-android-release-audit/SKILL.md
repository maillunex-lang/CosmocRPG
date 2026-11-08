---
name: uralia-android-release-audit
description: Audit the existing Uralia Godot 4.7.2 Android Debug workflow, APK <=95 MB, resource manifests, signing hygiene and reproducible build evidence without publishing.
---
# Аудит Android-релиза Уралии

## Сначала изучи
- `docs/CI.md`, `docs/ENVIRONMENT.md`, `docs/PROGRESS.md`, `client/project.godot`, `client/export_presets.cfg`.
- `.github/workflows/ci.yml`, `tools/ci_local.sh`, `tools/check_apk_size.py`, `tools/asset_manifest.py`, `schemas/asset_manifest.schema.json` (если присутствуют).
- Актуальные правила `.gitignore`, `docs/ASSET_LICENSES.md`, `art_src/LICENSES.md`.

## Текущие проектные ограничения
1. Godot 4.7.2 указан в проектных документах, но фактическую версию и шаблоны сверяй при запуске; Android-клиент вертикальный, 720×1280, package `org.uralia.game` в описанном debug-процессе.
2. Максимальный стартовый APK: 95 МБ. Основные ресурсы должны скачиваться после установки, а не незаметно раздувать APK.
3. Существующий CI уже включает `structure`, `tests`, `godot` и ручное задание `apk`. Не создавай второй pipeline без обоснования. Документированный ранее debug APK 28.32 МБ — исторический результат, а не гарантия текущего размера.
4. Отдельно различай работающий asset manifest и ещё не реализованные функции клиентской докачки, атомарной активации, кэша и ручной проверки обновления. Не ставь PASS без исполняемого теста.
5. В репозиторий нельзя добавлять APK/AAB, ключи подписи, `.env`, секреты и локальные настройки. Release signing не заменять временным debug-ключом.

## Процедура
1. Проверь актуальный HEAD, экспортный пресет, ABI/minSdk, CI-конфигурацию, размер и хэш существующего артефакта, если доступен.
2. Выполни read-only аудит и доступные тесты. Не запускай ручную APK-сборку, если она не согласована в рамках задачи.
3. Если сборка разрешена, используй существующий изолированный GitHub Actions workflow и фиксируй commit SHA, Godot version, artifact, SHA256, APK size, `apksigner verify` и фактические тесты.
4. Проверь ресурсные манифесты и отсутствие секретов в git/logs; не публикуй, не деплой и не меняй VPS.
5. Выдай отчёт PASS/FAIL/NOT TESTED по каждому пункту, список изменений и запись в существующем журнале передачи задач.

## Безопасность
Не изменяй Claude MCP, Codex MCP, занятые порты, production-сервисы, сторонние VPS-проекты и текущую политику ветки `main`. Малые коммиты, без force push.
