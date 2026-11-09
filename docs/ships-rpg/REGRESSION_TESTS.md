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

Итог SR-014a: 120 test/subtest PASS, 11 SKIP, 0 FAIL; пакеты rpg/pilot/shipxp/world/ws — PASS. `go vet ./...` и `go build -o /workspace/scratch/cosmoc-server ./cmd/server` — PASS (exit 0); бинарник не запускался. `git diff --check` — PASS.

Коммиты проверки: SR-000 8c7b620, SR-010 8ce0106, SR-015 0e7cbb3, SR-014a 788d951.

## 2026-11-09 — SR-011a, контракт полных паспортов
- `go test -race -count=1 ./internal/shipcatalog`: первый контракт PASS (6.526s), после усиления проверок один тест FAIL из-за неверного ожидания слоя валидации (ERROR-SR-003).
- После исправления `go test -race -count=1 ./internal/shipcatalog`: PASS, `ok .../shipcatalog 7.007s`. `go vet ./internal/shipcatalog`: PASS.
- Проверены schema/типы/roundtrip, 39 отрицательных изменений паспорта, version/count/дубли моделей и visuals, обязательная предварительность нового base, неизвестные поля/JSON, конкурентная загрузка, обнаружение дрейфа старых энергии и CD.
- Паспорта в этих тестах синтетические. Авторские 15 паспортов, все реальные старые числа и семейства модулей проверяются отдельной SR-011b. Реальная БД, Godot и production не затронуты.

## 2026-11-09 — SR-011b, 15 полных паспортов
- `go test -race -count=1 -json ./internal/shipcatalog` — PASS, пакет 7.754s, 67 test/subtest PASS, 1 SKIP (PostgreSQL).
- `go test -race -count=1 -json ./...` — PASS (exit 0): 20 пакетов с тестами PASS; 304 test/subtest PASS, 62 SKIP, 0 FAIL; ещё 3 пакета без тестов. Примеры времени: shipcatalog 9.941s, world 1.613s, ws 13.300s, aistudio 42.647s. PostgreSQL cases пропущены штатно через testdb; это не доказательство проверки SQL.
- Затем schema усилена: для полного паспорта обязательны все поля base (включая role, размеры/системы/регенерацию/temporary), хотя старый Ship_Patch_Pack_v1 допускает частичные значения. Добавлен отрицательный тест пустой роли. Повторный `go test -race -count=1 -json ./internal/shipcatalog` — PASS (exit 0), 8.392s, 68 test/subtest PASS, 1 SKIP.
- `go vet ./...` и `go build -o /workspace/scratch/cosmoc-server-sr011 ./cmd/server` — PASS (exit 0), включая повтор после усиления schema. `gofmt` и `git diff --check` — PASS. Бинарник не запускался.

Проверены все 15 моделей / 12 классов / 30 специализаций, полное равенство 12 исходных ShipSpec (включая все abilities), отдельная предварительность 3 новых base, 15 разных силуэтов, крепления под все оружейные слоты/двигатели, все стартовые комплекты, 72 допустимых skill ID хотя бы у одного корпуса, отсутствие неявной публикации (world.New по-прежнему содержит 12 моделей), независимые копии вложенных данных, сериализация без потери полей и границы массы/MW отдельно от боевого energy.
Новые ошибки SR-011b не обнаружены; ERROR-SR-003 исправлена в 6a657db. Godot, реальная БД и production не менялись/не проверялись. Модульное открытие всех 72 навыков и реализация эффектов ещё не проверяются, это следующие задачи.

SHA текущей задачи: `6a657db` — SR-011a, `699e8b5` — SR-011b/завершение SR-011. Финальный gofmt/vet/build после последней schema — PASS.

## 2026-11-09 — R-63/SR-090/091, публикация и CI
`git fetch origin main`: remote 3b89741, divergence 0/8; `git push origin HEAD:main` — PASS, main fast-forward до 75931d4 (накопленные тестированные Ships RPG + новый план). Автодеплоя нет. CI https://github.com/maillunex-lang/CosmocRPG/actions/runs/42604786797: Go vet/gofmt PASS, PostgreSQL race FAIL на прежнем TestPGRewardOnceAndPersistence (ERROR-SR-005). admin/Docker PASS; client ещё выполнялся на момент проверки. PostgreSQL shipcatalog/shipxp/pilot/world/ws в этом run PASS — это первая фактическая удалённая DB-проверка накопленных изменений. Не объявлять весь CI зелёным.
Исправление SR-091 затрагивает только тест квестовой цепочки. Следующая push/CI должна подтвердить его с БД.
