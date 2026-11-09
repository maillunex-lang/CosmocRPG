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
