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

## 2026-11-09 — SR-100a, реальная интеграция паспорта в ангар
- `go test -race -count=1 ./internal/ws ./cmd/server`: сначала неверная fixture смены hull дала FAIL (ERROR-SR-004), затем исправление на SetShip — PASS: ws13.910s, cmd/server3.307s.
- `COSMOC_TEST_GODOT=godot go test -race -count=1 -v ./internal/ws -run '^TestPassportGodotClient$'`: PASS, 3.591s; Godot подключился к настоящему ephemeral httptest WebSocket server, получил state/ships/passport через ServerGateway и показал его в игровом ангаре. Память, localhost, без БД/production. Сценарий добавлен в tools/test_client.sh для CI4.7.2.
- `COSMOC_TEST_GODOT=godot go test -race -count=1 -json ./...`: PASS, 20 пакетов, 308 test/subtest PASS, 62 SKIP (БД), 0 FAIL; 3 пакета без тестов. Live Godot case также PASS в этом полном запуске.
- Godot4.6.3 `--headless --path client --import`: PASS, SCRIPT ERROR/ERROR отсутствуют. Импортер удалил 4.7-поля в двух glb.import; эти сгенерированные изменения просмотрены и восстановлены, в коммит не входят.
- `--script res://tests/hangar_test.gd`: 28/0; `fleet_shop_test.gd`:53/0; `updater_test.gd`:31/0 (настоящий manifest key/openssl branch SKIP); `apk_installer_test.gd`:9/0.
- `go vet ./...`, `go build -o /workspace/scratch/cosmoc-server-r63 ./cmd/server`, gofmt и git diff --check: PASS. Подписанный APK не собирался/не публиковался, Android установка не проверена. Локальный Godot4.6.3 отличается от CI4.7.2; окончательный CI будет записан отдельно.

Совместимость: активные админские HP/energy/armor/boost и масштабирование уровня присутствуют в passport.base и не меняются запросом; world остаётся12 hulls; старый decoder читает прежний fleet; provider nil/другой корпус/старый ответ не показывают секцию; шлюз не прикрепляет паспорт чужому hull; неизвестный cap не подменяется hardcoded30.

## 2026-11-09 — удалённая проверка нового main и живой канал обновлений
- main cb34353 успешно опубликован. CI https://github.com/maillunex-lang/CosmocRPG/actions/runs/42606462785: Go job129123615616 **success** (включая PostgreSQL race и сборку); admin и Docker success, client ещё выполняется. ERROR-SR-005 Resolved.
- Прямой gh CLI начал отвечать401 Bad credentials при позднем опросе; получение jobs через GitHub connector работает. Причина401 не установлена, никаких токенов/секретов в отчёт не копируется.
- READ-ONLY GET https://rikagro.ru/download/latest.json + latest.json.sig: опубликована0.3.11/code14, min1, size39268009, published2026-11-09T07:54:07Z. Подпись **Verified OK** прежним публичным ключом (из embedded client constant; затем восстановленным файлом). Godot Updater.evaluate с настоящим signed manifest: installed13→available, installed14→up_to_date. APK не скачивался/не устанавливался/не публиковался.
- ERROR-SR-006: первая openssl-проверка выявила отсутствие public PEM в checkout. Восстановлен тот же публичный ключ и точечное .gitignore исключение; `godot --headless --path client --script res://tests/updater_test.gd` **33 проверок/0 провалов**. Реальный private-key/openssl-sign branch по-прежнему SKIP, подписанный production manifest проверен публичным ключом независимо.

## Итог CI игрового шага cb34353
Run42606462785: **все4 jobs success** (сервер Go/PostgreSQL, Godot4.7.2, admin, Docker). Новый live-сценарий в client job129123615478 выполнился11:09:14–11:09:19Z: `== Ships RPG: настоящий локальный WebSocket → ангар Стрижа (без БД)` → `ok .../internal/ws 0.769s`. PostgreSQL CI подтвердил исправление88a060c (ERROR-SR-005 Resolved).
Ограничение зелёного CI: render-тесты логируют ERROR о VK_KHR_surface/ERR_CANT_OPEN и3 freed lambda captures, хотя завершаются44/0 и12/0. Baseline client job129117865136 до интеграции имел те же2 Vulkan/3 lambda сообщения. Зафиксирован открытый ERROR-SR-007; реальную чистую графическую/Android приёмку не заявлять.
Дополнительный релизный commit ae16a86: тот же public key, .gitignore exception и updater presence-test; локально33/0 и production manifest Verified OK. Следующий main push инициирует отдельный CI для этих малых изменений/финальной документации; результат не приравнивать автоматически к завершённому run для cb34353.

## 2026-11-09 — SR-100b1, starter ENG/M01
- База remote main a19f7c0: CI run42608041551 completed/success (получен через GitHub connector workflow runs GET). Предыдущий release/public-key checkpoint подтверждён.
- `go test -race -count=1 ./internal/world ./internal/ws` после core implementation: PASS (world1.663s/ws14.137s).
- `go test -race -count=1 ./internal/world ./internal/shipcatalog` с новыми negative/compatibility tests: PASS, world1.664s/shipcatalog11.267s.
- Проверено: starter ID стабилен/единственный и привязан к owner; M01 совпадает с текущим изменённым админом boost по Energy/CD/Duration/SpeedMul/VFX; два alias используют только cooldown key boost и не обходят CD в обе стороны. Отсутствующий/чужой/поддельный instance/prototype/slot/unequipped engine и ранг0/6 отвергаются без списания/событий/движения. Death/no_energy и прежний boost других hulls сохранены, M01 другим hulls не открыт. Prototype/slot/M01 сверены с валидированным паспортным blueprint.
- `COSMOC_TEST_GODOT=godot go test -race -count=1 -v ./internal/ws -run 'TestM01OverWebSocket|TestPassportGodotClient'`: PASS4.660s; live WS M01 вызван, legacy boost получает shared cooldown. Сквозной Godot client→ангар→starter→M01 тоже PASS; клиентская часть фиксируется SR-100b2.

## 2026-11-09 — SR-100b2, клиент/полная проверка
- `COSMOC_TEST_GODOT=godot go test -race -count=1 -json ./...` — exit0,20 пакетов PASS,322 test/subtest PASS,62 SKIP (локальная PostgreSQL не настроена),0 FAIL;3 пакета без тестов. Real Godot case исполнялся, не SKIP.
- Godot4.6.3 `--headless --path client --script res://tests/hangar_test.gd` —33/0: engine shown отдельно от draft plan, CatalogID M01/disabled reason через gateway, HUD unavailable без энергии/CD как причины, old server fallback boost/enabled:true.
- `--script res://tests/hud_test.gd` —161/0 (существующие layout/бой/управление/состояния).
- Расширенный live `TestPassportGodotClient`: настоящий local ephemeral WS→gateway→ангар показывает owned/equipped engine; gateway выбирает M01 и отправляет команду, state подтверждает boosting. SQL/production не используется, child закрывается тестом.
- `go vet ./...`, `go build -o /workspace/scratch/cosmoc-server-m01 ./cmd/server`, gofmt/diff-check PASS. Бинарник не запускался. APK не собирался, Mobile/Android визуальная приёмка не заявляется.
Core server commit21aaa0b; клиентский SHA d465afe. Remote CI нового main будет проверен отдельно; прежняя база a19f7c0 уже имеет CI42608041551 success.

Push сначала отклонён non-fast-forward: другой ИИ добавил landing commit545032a (2 строки site/index.html). Выполнен обычный merge c9a3391 без конфликтов/force; server/client/tools относительно протестированного d465afe не изменились. `python3 tools/check_site.py`: CSP PASS,14 ссылок. Повторный push main c9a3391 PASS. CI42616666345 запущен, первые admin/Docker jobs success, Go/Godot ожидаются. Это ожидаемая защита Git от потери чужой работы, не программная регрессия.

## SR-100b: полный remote CI и toast lifecycle
CI https://github.com/maillunex-lang/CosmocRPG/actions/runs/42616666345 для main c9a3391: все4 jobs **success** (Go/PostgreSQL job129158113492, Godot4.7.2 job129158113111, admin, Docker). Это remote подтверждение gameplay шага21aaa0b+d465afe и сохранённого concurrent landing545032a.
После первоначального локального HUD161/0 найден один raw Lambda capture ERROR. Baseline a19f7c0 в отдельном worktree:161/0, без ERROR в одном прогоне; lifecycle busy timer ожидание2.2s тоже не воспроизвело. Затем toast+screen close+3.7s дал детерминированный FAIL live test: child exit0 с3 rawERROR, Go guard ловит их. ERROR-SR-008.
После connect(p.queue_free) вместо toast lambda:
- `COSMOC_TEST_GODOT=godot go test -race -count=1 -v ./internal/ws -run '^TestPassportGodotClient$'` — PASS9.034s (включает настоящий M01 cast, закрытие сцены, истечение toast таймера; ни ERROR, ни SCRIPT ERROR).
- `godot --headless --path client --script res://tests/hud_test.gd` —161/0, ERROR/SCRIPT ERROR отсутствуют.
- Временный baseline worktree удалён, общий imported cache не удалялся/не менял tracked assets.
Это дополнительный локальный lifecycle fix после зелёного CI c9a3391; новый push проверит его отдельно. Новый полный Go run не нужен: production Go/механика модулей не менялись, обновлены3 lifecycle/test файла и документы.

Коммиты SR-100b:21aaa0b server, d465afe client, c9a3391 merged main (remote CI42616666345 success); a1c5c9e дополнительный lifecycle fix (локальная red/green regression PASS). Последующий docs-checkpoint не меняет executable code.

## SR-007 (2026-11-09, Kimi Code) — render-стек CI

Фактические команды и результаты (VPS, после `sudo apt-get install mesa-vulkan-drivers vulkan-tools`):
- `vulkaninfo --summary` → GPU0 llvmpipe, apiVersion 1.4.318 ✅
- `xvfb-run -a godot --path client --resolution 1280x720 --audio-driver Dummy --script res://tests/background_render_test.gd` → `RENDERER mobile / vulkan`, Итого 12/0, **0 строк ERROR** ✅
- `... camera_render_test.gd` → `RENDERER mobile / vulkan`, Итого 44/0, **0 строк ERROR** ✅
- До установки драйвера воспроизводились `Required Vulkan instance extension VK_KHR_surface not found` + ALSA `ERR_CANT_OPEN`.

Регрессионные крючки:
- `tools/test_client.sh`: полные логи render-тестов во временные файлы; любой `^ERROR|SCRIPT ERROR` → FAIL даже при exit 0; запуск с `--audio-driver Dummy`.
- `camera_render_test.gd` / `background_render_test.gd`: `_check_renderer()` — метод `mobile` и не-null `RenderingServer.get_rendering_device()` (Vulkan); иначе FAIL `_failures += 1`.
- CI: `.github/workflows/ci.yml` клиентский job ставит `mesa-vulkan-drivers libvulkan1 vulkan-tools`.

Граница: проверка desktop/CI; Android-визуальная приёмка — у владельца.

## SR-100c: storage → bootstrap → dock setter → ангар
2026-11-09. Коммиты38385f2/6cec61f(schema/store),01bc109(bootstrap),ac029a8(setter/reconcile),7407d43(client/livePG),9573379(corp fixture), merge main74f9cfb. [Полная история запусков](sr100c-storage/REGRESSION_TESTS.md).
- Реальный PostgreSQL16 в отдельном локальном tmpfs контейнере; каждый test — собственная schema.23 initial targeted cases PASS/0 SKIP/FAIL: legacy XP, unique/ownership/cross-hull FK, cascade, rollback, CAS16, concurrent snapshots, old autosave.
- Bootstrap realPG world/equipment/db race PASS3.152s/5.638s/2.972s; targeted WS PASS3.035s. Reconnect и fresh server запрещают обе boost/M01 aliases для снятого двигателя; hull/jump/stale-bootstrap проверены.
- Runtime targeted race с реальным PG4 пакета PASS4.137s/5.085s/1.407s/3.803s. Unsafe gate tests, recent outgoing/incoming combat и transfer, CAS, rollback/lost commit response/failed reload+recovery, movement serialization. Missing revision/equipped и чужие owner/item selectors отклоняются.
- Godot4.6.3 import PASS/0 ERROR; hangar39/0; HUD161/0; оба лога без raw ERROR. Старый payload скрывает новые controls.
- `COSMOC_TEST_GODOT=godot COSMOC_TEST_DATABASE_URL=<isolated localhost> go test -race -count=1 -v ./internal/ws -run '^TestEquipmentGodotClientPostgres$'` PASS10.426s. Реальный клиент: кнопка снять→SQL revision1→disabled M01→reconnect→состояниеfalse→кнопка установить→revision2→успешный M01; конечное SQL состояние проверяет Go. Harness проверяет raw ERROR и lifecycle после закрытия. Новый сценарий добавлен в tools/test_client.sh при наличии PG_URL/COSMOC_TEST_DATABASE_URL (CI уже имеет PostgreSQL service); renderer шаги не менялись.
- Corp baseline fix9573379: race+realPG PASS1.523s. Первоначальные FAIL и два account timeout не скрыты, см. ERROR_LOG. Account recheck отдельно PASS14.597s, deadlines не ослаблены.
- `go vet ./...` и `go build -o /workspace/scratch/cosmoc-sr100c-final ./cmd/server` PASS после merge74f9cfb. `bash -n tools/test_client.sh` PASS. Первый final-suite setup не запустил пакеты из-за ошибочного написания timeout-флага; исправлено на `-timeout=300s`, окончательный результат ниже.
- Полный последовательный Go+PG+Godot race suite сейчас проверяется; CI — после публикации. Android/Vulkan/release/APK/production не заявлены готовыми. ERROR-SR-007 остаётся Kimi.

### Полный suite перед final rank fix
`COSMOC_TEST_GODOT=godot COSMOC_TEST_DATABASE_URL=<isolated PG> go test -race -p 1 -count=1 -json -timeout=300s ./...` — **PASS exit0:431 test/subtest PASS,22 packages PASS,1 test SKIP(существующий AmbientFollowFromAnotherCaptain),3 packages без тестов,0 FAIL**. Ни один SQL/equipment/live Godot test не пропущен; оба Godot harness выполнены. Это полный прогон кода74f9cfb/документационного0a1ead6, до последующего final rank fix.
Новый red test ERROR-SR-010 подтвердил сброс learned rank; после исправления отдельно повторяются все3 затронутых пакета world/equipment/ws с PG/Godot. Не приравнивать предыдущий общий прогон к повторному полному suite после нового изменения.

После ERROR-SR-010 fix: `COSMOC_TEST_GODOT=godot COSMOC_TEST_DATABASE_URL=<isolated PG> go test -race -p 1 -count=1 -json -timeout=180s ./internal/world ./internal/equipment ./internal/ws` — **157 test/subtest PASS,3 packages PASS,0 SKIP/FAIL,exit0** (3.631s/6.075s/27.850s). Включены новый rank0/4 regression и оба живых Godot harness. `go vet ./...` и `go build ... ./cmd/server` после этого fix — PASS. Более широкий suite не повторялся: все затронутые пакеты перепроверены целиком, остальные Go файлы не менялись.

Публикация: обычный push main85abfdd→d9669c9 PASS. Перед этим non-FF безопасно отклонён, fetch+merge сохранили новый QUESTJOB/FinalQuest.md. Последние main merges были docs-only, повтор серверных тестов из-за них не требовался. CI code head [42650859949](https://github.com/maillunex-lang/CosmocRPG/actions/runs/42650859949) pending на момент проверки, не success; последующий docs push/чужие main commits могут заменить run. Root main checkout чистый и не переключался. Свой PostgreSQL tmpfs контейнер cosmoc-sr100c-runtime-20261109 остановлен/удалён после завершения всех SQL тестов. Production/APK/подписанные manifests не затронуты.

## SR-100c-4a: core reservation
Ветка codex/sr100c-coordinator/baseb945efb. `TestEquipmentSlowStoreDoesNotBlockSimulation` сначала FAIL1.00s: blocked IO блокирует tick/snapshot/other player/damage. После переноса callback вне world mutex — PASS. Полный `go test -race -count=1 ./internal/world` PASS1.787s. Дополнительно: actor-only guards, panic fail-closed cleanup, grace retention до commit, Router leave/jump не теряют reserved actor; post-commit transfer сохраняетrevision1. Это unit/race, не новый SQL прогон. WS Handler.mu пока не исправлен; новый end-to-end slowIO regression следующий.

## SR-100c-4b: owner gate, настоящий row lock
Memory-backed slowIO WS red FAIL1.03s → green PASS2.441s. `COSMOC_TEST_DATABASE_URL=<отдельный localhost PG> go test -race -count=1 ./internal/ws -run '^TestEquipment|^TestPlayerOperation'` PASS6.553s: реальный FOR UPDATE stall для A не задерживает B move/ships, Tick, C join; commit сохраняетrev+1. Конкурирующий join того же owner ждёт accepted operation, получаетunequipped rev1 и затем успешно refit rev2. Reference-counted gate cancel/other owner/idempotent release/zero idle entries проверены.
Повтор world/ws targeted с PG: PASS1.047s/5.793s. Богатый общий прогон с Godot ещё впереди; targeted run без COSMOC_TEST_GODOT пропускает live Godot и не является его проверкой. SQL cases выполнялись в новых testdb schemas внутри собственного tmpfs PG контейнера cosmoc-equipment-coordinator-20261109; удалить контейнер в конце.
