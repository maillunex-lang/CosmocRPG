---
name: codex-ci-debugger
description: Диагностировать сбои существующего CI Godot, pytest и Android без замены pipeline. Use when this workflow is relevant to the Uralia Godot game.
---
# codex-ci-debugger: Уралия

## Порядок работы
1. Прочитай `docs/CI.md`, `.github/workflows/ci.yml`, `tools/ci_local.sh` и точный упавший GitHub Actions run.
2. В Уралии существуют jobs `structure`, `tests`, `godot` и ручной `apk`. Не создавай вторую CI-систему.
3. Выдели первое подтверждённое место падения: JSON/schema, pytest, Godot headless import, Android templates, export preset, APK size или подпись.
4. Учти двухпроходный импорт Godot и локальные ассеты `_local`, которые не хранятся в Git. Пропущенные тесты отмечай SKIPPED, не PASS.
5. Правь минимальную причину, не ослабляй проверки, не раскрывай секреты и не запускай production deploy.
6. После фикса приложи commit SHA, URL run, статус каждого job; если запуск невозможен, напиши NOT RUN.

## Общие ограничения
Работать только в `maillunex-lang/ufa5` по согласованной задаче. Сверять утверждённые решения с кодом, не переносить 3D MMORPG-механики из AnimeAutoMMORPG. Не трогать Claude MCP, Codex MCP, VPS, другие репозитории, занятые порты, production, секреты и платежи. Никаких внешних зависимостей или исполняемых хуков без аудита и разрешения. Отчёт о файлах, реальных проверках, рисках и следующем шаге.

## Происхождение
Оригинальная адаптация рабочих принципов из собственного репозитория `maillunex-lang/AnimeAutoMMORPG/.agents/skills/codex-ci-debugger/SKILL.md`; инструкции переписаны под существующую архитектуру Уралии. Сторонний код не добавлен.
