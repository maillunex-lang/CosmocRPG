# CosmocRPG Release Candidate QA

Дата: 2026-11-14  
Цель: Beta 0.2 → Release Candidate

## Пройдено

- Server: `go test ./...`
- Godot editor/headless parse check
- Safe Area
- Layout and HUD layout
- Graphics settings and shader effects
- Frame stats
- Server gateway: 12/12
- Corporation UI: 36/36
- Map and star map
- Background content and assets
- Quest hints, briefing, dialogue rules
- APK export and Android v2 signature verification
- Public HTTPS APK route

## Ограничения тестовой среды

- `background_render_test.gd` и `camera_render_test.gd` требуют Vulkan; headless Termalin запускает OpenGL3, поэтому эти проверки нельзя считать renderer-проверкой устройства.
- `performance_scenarios.gd` не завершает headless-процесс самостоятельно; его нельзя использовать как автоматический pass/fail без отдельного длительного runner.
- Финальная проверка FPS, нагрева и батареи должна выполняться на физическом Android-устройстве.

## Безопасность диагностики

- `GET /diagnostics` отдаёт только версию, время генерации и список безопасных маршрутов.
- Токены, пароли и секреты исключаются из observability ring buffer.
- Debug overlay остаётся скрытым по умолчанию.

## Rollback

1. Вернуть предыдущий APK из опубликованного HTTPS-архива.
2. Для GitHub использовать предыдущий commit `559757c35134586b1231c7b0fb57514a7581fe03`.
3. Миграции 0030/0031 применять отдельно после резервной копии базы.
4. Рабочие сервисы не перезапускать во время отката без отдельного deployment-задания.
