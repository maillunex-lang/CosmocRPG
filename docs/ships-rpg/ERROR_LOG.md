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
Причина: Join существующего player ID сохраняет прежний корабль (world.go:245); тест в действительности остался на Стриже. Исправление: штатный w.SetShip с обязательной проверкой ошибки. Повторный race ws/cmd/server PASS (13.910s/3.307s). Неудачная попытка: повторный Join как смена корабля. Регрессия: тот же live WS тест, включая активные админские параметры/уровень/старый decoder/чужой корпус. Коммит: cb34353. Профилактика: проверять состояние fixture через публичные команды.

## ERROR-SR-005
Дата: 2026-11-09. Задача SR-091. Компонент quest/quest_test.go, существующий CI. Серьёзность Medium. Статус Resolved (PostgreSQL CI нового main подтвердил исправление).
Воспроизведение: GitHub Actions run 42604786797, job 129117865467, `go test -race -count=1 ./...` с PostgreSQL. FAIL TestPGRewardOnceAndPersistence: после restart Credits500 и активный first_blood, а тест ожидает ноль заданий.
Причина подтверждена кодом/логом: Catalog уже содержит цепочку, Service.Join выдаёт next при completed predecessor; memory TestContractFlowAndIdempotentReward уже ожидает first_blood. Старое PG-ожидание не обновлено. Между исходной main 3b89741 и предыдущими Ships RPG коммитами runtime quest не менялся.
Исправление: только PG-тест ожидает ровно first_blood с Progress0, Credits500 и отсутствие повторной награды после restart. Игровой код/SQL/production не меняются. Локально DB cases пропускаются, поэтому исправление нельзя объявить проверенным PostgreSQL до нового CI. Неудачные попытки: gh run view --log до завершения run недоступен; redirect job log через gh CLI не скачался; лог получен GitHub connector fetch_workflow_job_logs.
Файлы: server/internal/quest/quest_test.go. Регрессия: этот тест + существующий memory flow. Коммит: SR-091. Профилактика: следить за DB-веткой CI, не выдавать локальный SKIP за PASS.

Проверка ERROR-SR-005: GitHub Actions run42606462785, Go job129123615616 — success, PostgreSQL race/vet/build выполнены. Коммит исправления88a060c, проверяемый head cb34353.

## ERROR-SR-006
Дата: 2026-11-09. Задача SR-093. Компонент release workflow/.gitignore. Серьёзность Medium. Статус Resolved.
Воспроизведение в свежем checkout: `git ls-files client/assets/update_public.pem` пуст; openssl -verify client/assets/update_public.pem завершился ошибкой открытия файла. publish_release.sh использует именно этот путь.
Причина доказана: общее правило `*.pem` исключало публичный файл, а updater_test проверял совпадение файла только при его наличии. Встроенный PUBLIC_KEY_PEM в updater.gd корректен: им проверен опубликованный signed manifest. На VPS файл мог существовать вне git, но в clone отсутствовал.
Исправление: восстановлен тот же PUBLIC KEY из updater.gd, добавлено точечное исключение !client/assets/update_public.pem, тест требует наличие файла и совпадение со встроенным ключом. Ни новый ключ, ни приватный материал не создавались/не копировались.
Файлы: .gitignore, client/assets/update_public.pem, client/tests/updater_test.gd. Проверка: updater33/0, openssl Verified OK для реально опубликованного latest.json (0.3.11/code14), Godot evaluate code13→available/code14→up_to_date. Неудачная попытка: первоначальная проверка отсутствующим файлом; рабочая диагностика через публичный PEM из кода. Регрессия: updater_test обязательное наличие+тождество ключа. Коммит: ae16a86. Профилактика: публичные контракты подписи хранить в репозитории с точечным разрешением; приватные *.pem по-прежнему исключены.

## ERROR-SR-007
Дата: 2026-11-09. Компонент: существующие Godot camera/background render тесты в CI. Серьёзность Medium. Статус **Resolved** (Kimi Code, 2026-11-09). Задача: регрессионный долг SR-100d/SR-070 (не новый паспортный путь).
Воспроизведение: client job129123615478 в run42606462785: при camera/background появляются `Required Vulkan instance extension VK_KHR_surface not found`, `ERR_CANT_OPEN`; в camera ещё3 `Lambda capture at index0 was freed`. Тесты возвращают44/0 и12/0, job зелёный.
Ожидалось: реальная отрисовка без runtime ERROR. Фактически: CI job success при этих сообщениях. Причина отсутствия extension/источник lambda не установлены; доказаны сами сообщения и отсутствие их влияния на exit status. Ошибки не связаны доказательно с новым паспортом: baseline run42604786797 client job129117865136 имеет те же2 Vulkan и3 freed-capture сообщения до SR-100a.
Исправление (Kimi Code): первопричина подтверждена — в CI и на VPS был установлен libvulkan1 (loader) без ICD-драйвера; установлен mesa-vulkan-drivers (lavapipe llvmpipe), Godot подтверждает «Vulkan 1.4 — Forward Mobile — llvmpipe». ALSA-ERROR (ERR_CANT_OPEN от audio_driver_alsa) убран легально: render-тесты запускаются с --audio-driver Dummy (звук тестами не проверяется, рендер не затронут). Усиление: tools/test_client.sh сохраняет полные логи render-тестов и проваливает прогон при любом «ERROR|SCRIPT ERROR», даже при exit 0; camera_render_test/background_render_test подтверждают стек метод=mobile и RenderingServer.get_rendering_device()!=null (Vulkan). Renderer НЕ менялся (Mobile утверждён), ERROR не подавляются, fallback на OpenGL теперь проваливает тест. Ограничение: это проверка desktop/CI; Android-приёмка отрисовки у владельца обязательна. Неудачные попытки: лог running job недоступен(BlobNotFound), после завершения получены оба лога.
Файлы для исследования: tools/test_client.sh, client/tests/camera_render_test.gd, background_render_test.gd. Регрессия/коммит исправления: отсутствуют. Профилактика: отличать success job от чистоты runtime-лога; Android визуальная приёмка ещё обязательна. Новый live WS→Godot→ангар прошёл отдельно0.769s до render этапа.

SR-100b1 (2026-11-09): новых ошибок не выявлено. Проверены ожидаемые отказы module_unavailable/ability_not_learned без изменений энергии/CD/событий; это negative cases, не фактические дефекты. ERROR-SR-007 остаётся открытым и относится к прежнему CI render стенду.

SR-100b2 (2026-11-09): новых ошибок не выявлено. Полный локальный race20 пакетов/322 PASS/62 PG SKIP, client hangar33/0 и HUD161/0, живой Godot→M01 cast PASS. ERROR-SR-007 (прежний render CI) остаётся Open. Отсутствие сохранённой смены оборудования в этом шаге явно ограничено отсутствием такого RPC, а не временным сбросом доступной пользователю настройки.

## ERROR-SR-008 (связан с симптомами ERROR-SR-007)
Дата2026-11-09. Задача SR-100b3. Компонент client/ui/components/alert_bar.gd. Серьёзность Medium. Статус Resolved (локальная детерминированная регрессия).
Наблюдение: hud_test161/0, но один raw ERROR Lambda capture at index0 was freed. Базовый a19f7c0 в отдельном worktree с тем же импортированным asset cache за один прогон161/0 не дал ERROR: одиночное сравнение не доказывало ни причину, ни отсутствие прежнего дефекта.
Воспроизведение: в реальном WS→Godot тесте создать toast, закрыть экран, дождаться TOAST_SECONDS+0.2. При усилении Go guard на любой ERROR: получены3 Lambda capture ERROR при child exit0/PASSPORT_ONLINE_PASS; Go regression test корректно FAIL.
Причина подтверждена red/green: toast timeout lambda захватывает Node p. Godot проверяет освобождённый capture до входа в тело, поэтому внутренний is_instance_valid(p) не предотвращает ERROR после освобождения панели/экрана/лимита уведомлений. Код alert_bar.gd до этого не менялся нашим SR-100b.
Неудачные попытки: ожидание BUSY_TIMEOUT после уничтожения HUD не воспроизвело ошибку; предположение о busy timer отвергнуто, этот production timer не переписывался. Исправление: connect(p.queue_free) — bound method автоматически отключается при удалении receiver, без захвата свободного Node лямбдой.
Файлы: alert_bar.gd, passport_online_test.gd, ws/passport_test.go. Регрессия: live real-client test теперь ждёт toast lifetime после закрытия сцены и отклоняет любой raw ERROR, а не только SCRIPT ERROR. Проверка после исправления: live race PASS9.034s, HUD161/0, в обоих логах нет ERROR/SCRIPT ERROR. Коммит: a1c5c9e. Профилактика: для таймеров использовать callable с lifecycle receiver; зелёный exit0 не достаточен при runtime ERROR. ERROR-SR-007 остаётся Open по Vulkan/прочим неустановленным render причинам; не объявлять весь renderer исправленным.

## ERROR-SR-009 — существующий corporation PG fixture
Дата2026-11-09; SR-100c-3/интеграционная регрессия; компонент corp/pgstore_test.go:27; Medium; Resolved. Связан CST-ERROR-004.
Воспроизведение: общий Go race suite с настоящим PostgreSQL падает TestPGStoreCorporationFlow: SQLSTATE42P08 inconsistent types deduced for parameter$1. То же подтверждено на чистом git archive0b77fea без equipment изменений. Ожидалось создание test players и проверка полного corp flow; фактически fixture не доходила до Create.
Доказанная причина: один$1 использован для public_id(TEXT) и token_hash(BYTEA), причём последний требует32 байта. Исправление9573379: отдельные параметры id/hash/name,32-byte world.TokenHash(id). Production corporation SQL не менялся. Регрессия: тот же полный flow (create/exists/join/members/leave), race+realPG PASS1.523s. Неудачные попытки/история в CST-ERROR-004; исходный общий suite не объявлялся зелёным. Профилактика: fixtures используют реальные schema types/хеши и прогоняются с PG, не только SKIP.

### Ошибки изолированной SR-100c ветки
[CST-ERROR-001…005](sr100c-storage/ERROR_LOG.md): duplicate draft migration19→20, FK test-fixture precedence, deferred ownership FK для owner cascade — Resolved/реальные SQL тесты. CST-ERROR-005: в широком параллельном запуске2 прежних account WS теста получили5s context timeout; повтор отдельно PASS14.597s без изменения production/deadlines. Причина не установлена; последовательный общий прогон фиксируется в REGRESSION_TESTS. ERROR-SR-007 не изменён: им занимается Kimi.

## ERROR-SR-010 — обновление оснащения переписывало изученный ранг
Дата2026-11-09; SR-100c-2/final review; Medium; Resolved (fix commit см. git log: preserve learned rank). Воспроизведение: новый TestEquipmentDoesNotRewriteLearnedRankOrCombatResources на valid server rank0/4, energy37 и cooldown100 вызывает ChangeStarter/RestoreStarter/live Join. До исправления FAIL: rank=1 want=0, energy37/cd100 сохранены. Причина доказана: runtime refresh переиспользовал cold initStarterEngine, который выдаёт начальный ранг1. Это не должно заново изучать отсутствующий навык или стирать обученный ранг4.
Исправление: отдельный refreshStarterEngine сохраняет прежний rank; используется только при runtime смене/reload/live reconnect, cold initShip остаётся начальной выдачейrank1. Модульная availability остаётся независимой. Файлы world/{starter_engine,starter_loadout,equipment,world,equipment_test}.go. До публикации повторяются world/equipment/ws race+realPG+Godot; фактический результат ниже/в REGRESSION_TESTS. Профилактика: тестировать learned state отдельно от наличия модуля, в том числе rank0 и4; не считать подтверждение одного startup rank1 достаточным.

Проверка ERROR-SR-010 после fix:157 test/subtest PASS,3 затронутых пакета целиком с race+PG+Godot,0 SKIP/FAIL. Vet/build PASS.

## ERROR-SR-011 — IO оснащения блокирует общий мир и сессии
Дата2026-11-09; SR-100c-4; Medium; Investigating (world часть исправлена, WS часть впереди). Прежнее ограничение ADR008 подтверждено red test TestEquipmentSlowStoreDoesNotBlockSimulation: callback удерживается channel barrier, параллельно move другого игрока+incoming damage+Step+Snapshot не завершаются за1s. После release тест завершается FAIL без leaked goroutines. Доказанная причина: ChangeStarter держал world.mu на SQL callback, equipmentMsg держит Handler.mu на DB/restore; Tick после Step тоже ждёт Handler.mu.
Core исправление: per-actor equipmentLease, callback вне world lock, brief validation/apply, reserve gates movement/autopilot/cast/attack/mining/hull/spec/transfer/remove/reload; world snapshots/tick/other actors и входящий урон продолжаются. Нормальная WS замена сессии будет ждать per-owner gate в следующем этапе. Grace cleanup ждёт resolution; unexpected panic освобождает lease/fail closed, без сброса rank/CD. Router transfer проверяет reservation+gate distance в одном world critical section. Урон после принятия не откатывается и не отменяет уже committed operation: accepted change ordered before hit, не immunity.
Проверка: red→green slow-store; полный `go test -race -count=1 ./internal/world` PASS1.787s, guards/panic/disconnect/transfer tests. Старый test, ожидавший блокирующий mutex, заменён явными assertions нового pending contract. Файлы world/equipment*, world/world.go/router/combat/fleet/living/starter_loadout. WS global lock ещё требует исправления; не считать весь ERROR resolved до проверки.

### ERROR-SR-011 — WS часть исправлена, Resolved локально
Дополнительный red TestEquipmentSlowStoreDoesNotBlockOtherSessions FAIL1.03s подтвердил, что Handler.mu задерживал Tick/join при свободном world.mu. Исправление: cancellable reference-counted per-owner gate для gameplay/finishJoin/leave; активная сессия проверяется под кратким Handler.mu, IO вне global locks. Новый join ждёт завершения принятой gear операции, затем читает committed loadout; old leave не отключает новую сессию. Idle entries удаляются, unlock idempotent, cancelled wait не запускает работу. Нет новых миграций/изменения production.
Проверки: slowIO WS green PASS2.441s; targeted WS+реальный PostgreSQL PASS6.553s. Новый PG test держит row FOR UPDATE, пока другой игрок шлёт move/ships, Tick и третий join успешно завершаются; затем подтверждён revision+1. Memory WS replacement ждёт принятого commit и продолжает CAS сrev1→2. Gate cancellation/isolation/cleanup проверены. Повтор world+WS targeted с PG PASS1.047s/5.793s. Full server/PG/Godot впереди; local Resolved не означает production deployment/CI success.

ERROR-SR-011 final regression: sourcea5bfd2e полный Go/race/PG/Godot **441 test/subtest/22 пакета PASS,0 FAIL**,1 прежний ambient skip,0 SQL/live Godot skips. Actor reservation не даёт immunity: новый fatal damage test подтверждает hull0/dead остаются после accepted commit, revision1 сохраняется; panic cleanup/grace/transfer/gate cancellation проверены. Исправлениеbc0aed9/aa7f950, UIpendinga5bfd2e. Сторонние baseline timeout CST005 не повторились при последовательном прогоне, deadline не менялись. Remote CI/production не выдавать за успешные локальные проверки.

ERROR-SR-011 опубликован в main5b71ca7, без force; full441/22 PASS и pending remote CI отдельно отражены в REGRESSION_TESTS. Новых ошибок при интеграции не выявлено. ERROR-SR-007 остаётся отдельной работой Kimi; его статус не менялся.

## EB-ERROR-001 — слишком широкая уникальность electronics
Дата2026-11-09; EB-010; Low; Resolved. Первый `go test -race -json ./internal/fitting`: все4 root tests FAIL на загрузке default catalogue, `starter EWR-001: module_unique:electronics`. Доказанная причина: unique_group был задан для всей category electronics, но утверждённый EWR starter включает ELE-M-START и отдельный ELE-M-EW-START. Исправление: категория не означает уникальность; engine/reactor ограничены собственной unique group, class modules — своей, различные electronics/defense допускаются по существующим slots. Native паспорта не менялись. Проверка полного default registry + все15 starter budgets в конструкторе, негативный duplicate-engine test сохранён; final fitting race PASS5.316s. Профилактика: проверять новые ограничения на всех15 kits, не только Стриже.

## EB-ERROR-002 — сообщение Cloud «не удалось выполнить удалённый шаг»
Дата2026-11-09; окружение/UI; Low; Workaround, первопричина не установлена. Пользователь прислал screenshot с этой надписью после file edits. Проверка environment_status: ready/connected/failure=null. Git и все general*.go/0021/state.go на месте,903522e сохранён в feature branch. Код заново скомпилирован; затем general+legacy PG/race tests PASS (4.464s/8.686s/2.691s). Утрату файлов не обнаружено, причина UI-сбоя не доказана. Работа продолжена с проверенного состояния, main не тронут. После storage substep сделать отдельный backup commit в feature branch, не выдавать platform diagnostics за ошибку игрового сервера.

EB-ERROR-002 checkpoint: после сообщения Cloud файлы и коммиты проверены, origin feature branch содержит f624748. Последний локальный service suite завершился успешно (equipment10.373s/fitting4.616s/db2.870s). Причина ошибки интерфейса Cloud не установлена; успешные тесты не доказывают исправление самого Cloud.

## EB-INTEGRATION — конфликт только дорожной карты
При merge с main3026811 конфликт docs/ROADMAP.md из-за перехода другой сессии на dashboard+tasks.json. Сохранён новый dashboard, добавлена точная EB-контрольная точка; новый реестр дополнен2 задачами и перегенерирован штатным tools/gen_roadmap.py. Валидатор PASS35 tasks. Конфликтов Go/SQL нет; полный Go+realPG race PASS. Это integration conflict, не новая gameplay-ошибка.

## PP-INTEGRATION-001 — сводки ROADMAP и лимит CI

Дата: 2026-11-09; задача PP-02; подсистема документации/CI. При объединении с main 0f3ff41 конфликтовали tasks.json, ROADMAP.md, STATUS.md, DEPENDENCIES.md. Причина: независимое добавление PP и EB задач к одной базе. Исправление: сохранены карточки EB из свежей main, добавлены PP по уникальным ID, сводки перегенерированы. Проверка: validate_roadmap.py PASS, уникальность 21 номеров SQL подтверждена; код EB/SQL не менялся. Статус: конфликт устранён.

Отдельное наблюдение CI main 0f3ff41: GitHub Actions не запустил jobs, annotation сообщает failed payments/spending limit. Это подтверждённое внешнее ограничение, не доказанная ошибка игрового кода. Статус: OPEN, требует действий владельца в Billing & plans; не изменять workflow для обхода.
