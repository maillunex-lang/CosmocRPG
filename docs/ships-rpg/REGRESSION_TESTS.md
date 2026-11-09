# Фактические тесты Ships RPG

## 2026-11-09 — SR-000
- `go test -race -count=1 ./...` из server: FAIL до запуска тестов, `Go: Unknown option: test` (ERROR-SR-001).
- `godot --version`: 4.6.3.stable.official.7d41c59c4; CI использует 4.7.2. Клиентские сценарии не запускались, клиент не изменён.
- PostgreSQL URL в окружении не настроен; DB-тесты нельзя считать выполненными при skip.

Результаты после восстановления Go и каждой следующей задачи добавляются ниже. Не заменять эти записи ожидаемыми результатами.

## 2026-11-09 — SR-010
- Восстановлен `go version go1.27.3 linux/amd64` в /workspace/scratch/toolchain/go. SHA-256 сверён с официальными метаданными go.dev. Это версия локальной проверки, не смена go.mod (1.22.2) или CI.
- `PATH=/workspace/scratch/toolchain/go/bin:$PATH go test -race -count=1 ./internal/shipcatalog` из server: PASS, `ok .../shipcatalog 3.800s`.
- Новые тесты: 15/12/30, сохранение всех 12 старых ID, наличие трёх новых моделей, независимость возвращаемого реестра, отсутствие изменения legacy runtime-документа; 11 отрицательных изменений контракта + malformed/null/trailing JSON/неизвестное поле. Проверяются schema и межсущностные ссылки.
- `git diff --check`: PASS. Реестр не подключён к runtime, полные паспорта/умения/прогрессия пока TODO.

## 2026-11-09 — SR-015, воспроизведение и общая проверка
- После восстановления toolchain: `go test -race -count=1 ./...` — PASS, 20 пакетов с тестами, 3 без тестов. Это серверная проверка без подключённой PostgreSQL, DB cases пропускаются через testdb.Pool.
- `go vet ./...` — PASS (exit 0).
- До исправления: `go test -count=1 ./internal/world -run TestUnsupportedCatalogAbilityFailsAtomically` — FAIL, `custom: got <nil>, want ErrUnknown`. Ошибка ERROR-SR-002 воспроизведена.
- Первая команда исправления использовала неверный cwd, FileNotFoundError; правка повторена с правильным путём. Промежуточный JSON-прогон не используется как итог, поскольку начался до исправления.
- Финальный запуск после правки: `go test -race -count=1 -json ./...`, PASS (exit 0): 253 test/subtest событий PASS, 62 SKIP, 0 FAIL; 20 пакетов PASS, 3 без тестов. Регрессионный TestUnsupportedCatalogAbilityFailsAtomically — PASS. Повторный go vet и gofmt — PASS.

## 2026-11-09 — SR-014a, подтверждённые caps 100/50
Владелец ответил «Да, применить новые требования». Изменён только MaxLevel (пилот 100, корабль 50); Base/Growth прежние.
`go test -race -count=1 -json ./internal/rpg ./internal/pilot ./internal/shipxp ./internal/world ./internal/ws` — PASS (exit 0), все 5 пакетов.
TestExtendedCurvesPreserveLegacyXP проверяет все прежние пороги 1–60/1–30 и XPForNext ниже старого cap, продолжение после прежнего cap, новые caps. Существующие service-тесты проверяют идемпотентность начислений, отдельный XP каждого корабля, world clamp — 50.
Интеграционные PostgreSQL cases пропущены через testdb; миграции не менялись и не выполнялись. Клиент не менялся: max_level приходит с сервера; fallback 60 в profile_screen для старых/неполных данных оставлен совместимым. Godot/Android/production не проверялись.
