# Журнал ошибок Ships RPG

## ERROR-SR-001
Дата: 2026-11-09. Задача SR-000. Компонент: окружение. Серьёзность: Medium. Статус: Resolved.
Воспроизведение: `cd server && go test -race -count=1 ./...`.
Ожидалось: запуск Go-тестов. Фактически: exit 1, `Go: Unknown option: test`; `go version` даёт `Go: Unknown option: version`.
Причина: PATH разрешает `/usr/bin/go`, не Go toolchain; /usr/local/go/bin/go отсутствует.
Неудачные попытки: первоначальный запуск базовых тестов. Исправление: toolchain в `/workspace/scratch/toolchain/go`, SHA-256 официального архива проверяется перед распаковкой; использовать явный PATH.
Файлы проекта не требуют исправления. Проверка/регрессия: `go version` → go1.27.3; race-прогон shipcatalog PASS (3.800s), полный прогон записывается отдельно. Коммит: документационный SR-000. Профилактика: проверять версию инструмента до тестов.

## ERROR-SR-002
Дата: 2026-11-09. Задача: SR-015 (выделена из SR-020). Компонент: world/combat.go. Серьёзность: Medium. Статус: Resolved.
Воспроизведение: `go test -count=1 ./internal/world -run TestUnsupportedCatalogAbilityFailsAtomically` после добавления теста, до исправления. Тест добавляет abilities.custom в каталог игрока, применяет через SetCatalog и вызывает UseAbility.
Ожидалось: ErrUnknown, без списания энергии/CD/событий/перемещения. Фактически: `custom: got <nil>, want ErrUnknown`, exit 1.
Первопричина доказана тестом и кодом: available принимал любой существующий ключ; switch в UseAbility без default возвращал nil без обработчика.
Неудачные попытки: первый запуск регрессионного теста закономерно красный; первая команда редактирования указала server/... из cwd server и завершилась FileNotFoundError, код тогда не изменился. Путь исправлен.
Исправление: available сначала допускает только реализованные boost/intercept/scan; pulse остаётся отдельной автоатакой. Неизвестные/неподдержанные ключи отвергаются до проверок ресурсов. Обработчик mine не затронут: добыча остаётся отдельной командой.
Файлы: server/internal/world/combat.go, combat_test.go. Регрессия: TestUnsupportedCatalogAbilityFailsAtomically; полный итог race/vet — REGRESSION_TESTS.md. Коммит: 0e7cbb3.
Профилактика: SR-020 должен заменить whitelist строгим общим реестром обработчиков; добавление записи JSON не означает исполненную механику. Тест оставить.

SR-014a (2026-11-09): новых ошибок не обнаружено, race пяти связанных пакетов PASS; изменение согласовано владельцем, не является регрессией прежних caps.

## ERROR-SR-003
Дата: 2026-11-09. Задача: SR-011a. Компонент: passport_test.go. Серьёзность: Low. Статус: Resolved.
Воспроизведение: `go test -race -count=1 ./internal/shipcatalog` после добавления case duplicate_legacy_spec. Ожидалось: отказ валидатора, тест PASS. Фактически: документ верно отклонён JSON Schema по uniqueItems, но тест ожидал текст более поздней семантической проверки и дал FAIL (`unexpected error ... items at 0 and 2 are equal`).
Причина доказана выводом: SchemaJSON() уже проверяет uniqueItems, поэтому семантический этап не достигается. Неудачная попытка: избыточная повторная проверка уникальности в Go. Исправление: тест проверяет факт отказа schema, дублирующая Go-проверка удалена; более поздние межсущностные проверки остаются.
Файлы: shipcatalog/passport.go, passport_test.go. Проверка: повторный race PASS, `ok .../shipcatalog 7.007s`. Регрессия: TestPassportRejectsInvalidData/duplicate_legacy_spec. Коммит: 6a657db. Профилактика: не связывать тесты общего отказа с конкретным слоем, если ошибку корректно ловит schema.

SR-011b (2026-11-09): новых ошибок не выявлено. Полный Go race PASS (20 пакетов, 304 test/subtest PASS, 62 SKIP), финальная проверка shipcatalog после усиления schema PASS (68 PASS/1 SKIP). Открытых ERROR-ID на текущей контрольной точке нет. Ограничения: runtime содержит 12 моделей, новые паспорта не опубликованы; module/effect refs и SQL/Godot — следующие этапы, не скрытые FAIL.

## ERROR-SR-004
Дата: 2026-11-09. Задача SR-100a. Компонент ws/passport_test.go. Серьёзность Low. Статус Resolved.
Воспроизведение: race ws, TestShipsPassportUsesActiveScaledBaseAndPreservesLegacyFleet; тест вызывает повторный world.Join с другим ship type. Ожидалось отсутствие паспорта для TMP-MINE; получено `passport enabled on another hull`.
Причина: Join существующего player ID сохраняет прежний корабль (world.go:245); тест в действительности остался на Стриже. Исправление: штатный w.SetShip с обязательной проверкой ошибки. Повторный race ws/cmd/server PASS (13.910s/3.307s). Неудачная попытка: повторный Join как смена корабля. Регрессия: тот же live WS тест, включая активные админские параметры/уровень/старый decoder/чужой корпус. Коммит: SR-100a после фиксации. Профилактика: проверять состояние fixture через публичные команды.

## ERROR-SR-005
Дата: 2026-11-09. Задача SR-091. Компонент quest/quest_test.go, существующий CI. Серьёзность Medium. Статус Investigating (исправление теста подготовлено, PostgreSQL-проверка ожидается).
Воспроизведение: GitHub Actions run 42604786797, job 129117865467, `go test -race -count=1 ./...` с PostgreSQL. FAIL TestPGRewardOnceAndPersistence: после restart Credits500 и активный first_blood, а тест ожидает ноль заданий.
Причина подтверждена кодом/логом: Catalog уже содержит цепочку, Service.Join выдаёт next при completed predecessor; memory TestContractFlowAndIdempotentReward уже ожидает first_blood. Старое PG-ожидание не обновлено. Между исходной main 3b89741 и предыдущими Ships RPG коммитами runtime quest не менялся.
Исправление: только PG-тест ожидает ровно first_blood с Progress0, Credits500 и отсутствие повторной награды после restart. Игровой код/SQL/production не меняются. Локально DB cases пропускаются, поэтому исправление нельзя объявить проверенным PostgreSQL до нового CI. Неудачные попытки: gh run view --log до завершения run недоступен; redirect job log через gh CLI не скачался; лог получен GitHub connector fetch_workflow_job_logs.
Файлы: server/internal/quest/quest_test.go. Регрессия: этот тест + существующий memory flow. Коммит: SR-091. Профилактика: следить за DB-веткой CI, не выдавать локальный SKIP за PASS.
