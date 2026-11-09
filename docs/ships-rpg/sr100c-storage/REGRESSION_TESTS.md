# Проверки SR-100c storage
Дата2026-11-09. Ветка codex/sr100c-storage, база0b77fea.

## Фактические запуски
- `go test -race -count=1 ./internal/equipment` без DB URL: memory PASS1.028s, PG cases SKIP. Это не проверка SQL.
- Первый запуск с реальным отдельным PostgreSQL16-alpine: FAIL до SQL, duplicate migration19 (CST-ERROR-001). Перенумерована только наша draft миграция в0020.
- Следующий PG suite: миграции/legacy сохранения/flow/CAS/rollback/autosave PASS, constraint fixture и owner cascade FAIL (CST-ERROR-002/003). Исправлены fixture и deferred composite FK.
- После исправления: equipment PASS5.798s, db PASS4.216s.
- Финальная targeted проверка после дополнительных concurrent read/extra slots/direct-delete cases: `COSMOC_TEST_DATABASE_URL=<изолированный localhost> go test -race -count=1 -json -timeout 180s ./internal/equipment ./internal/db` — **23 test/subtest PASS,0 SKIP,0 FAIL**, equipment6.441s/db3.978s.
- `git diff --check`, gofmt новых файлов — PASS. Общие Go race/vet/build ещё запускаются.

DB: собственный временный контейнер postgres:16-alpine (образ sha256:aa6cb33cf4c19c765cefcf48d96176258800870a9a1e930ad126200f9d825139), published только127.0.0.1 с динамическим портом, tmpfs data, RAM512MiB/CPU2; public тестовые credentials, не VPS. Каждый тест получает отдельную временную схему через существующий testdb.Pool, schema public не изменяется. Контейнер/данные после сессии удалить. Сырой URL/порты ephemeral не переносить в production-конфиг.

## Проверенные свойства
1. Новая schema добавляет только3 таблицы, не переписывает players/XP/ships и сама не выдаёт предметы. Native instance ID совпадает с прежним world snapshot.
2. Ensure идемпотентен и не восстанавливает снятый двигатель; Load ничего не выдаёт; неизвестный владелец не получает предмет.
3. CAS допускает одного победителя из16 запросов с одинаковой ревизией; read видит согласованные fitted/revision при изменении; повторный bootstrap не делает duplicate grant.
4. SQL composite ownership/ship FK, unique instance и slot key запрещают чужой item/cross-hull/double fit. FK deferred до commit, чтобы завершались обе owner cascade ветви; fitted item нельзя удалить отдельно.
5. Внедрённая SQL-ошибка после удаления slot откатывает slot и revision; старый world.Store.Save не перетирает equipment, оно хранится отдельно.
6. Неподдержанные/повреждённые loadout не нормализуются молча и не теряют чужие slots; error возвращает нулевой snapshot.

Клиент/рендер не менялись/не проверялись: ERROR-SR-007 выполняет Kimi. Store пока не подключён к runtime/RPC, док/бой/масса/мощность всех модулей/авторизация команды и согласование с world — следующий интеграционный шаг.

## Итог общего прогона
`go test -race -p 4 -count=1 -json -timeout300s ./...` с реальным PostgreSQL завершился exit1: 425 событий PASS (включая пакеты/подтесты), один упавший тест TestPGStoreCorporationFlow и FAIL его пакета. Ошибка pgstore_test.go:27: SQLSTATE42P08, несовместимые типы параметра$1. На чистом архиве базы0b77fea этот же тест воспроизвёл ту же ошибку (exit1); это не регрессия equipment. См. CST-ERROR-004. Все новые SQL cases выполнены.
5 SKIP-событий:3 пакета без тестов и2 теста (существующий AmbientFollowFromAnotherCaptain; PassportGodotClient без внешнего Godot). Это не пропуски SQL equipment.
`go vet ./...` и `go build -o /workspace/scratch/cosmoc-equipment-storage ./cmd/server` — PASS. Сервер не запускался, production не затронут.

## C-RT-01 bootstrap
2026-11-09: PostgreSQL16 localhost в отдельном tmpfs контейнере. `go test -race -p 1 -count=1 ./internal/world ./internal/equipment ./internal/db` PASS3.152s/5.638s/2.972s. `go test -race -count=1 ./internal/ws -run '^TestEquipment'` с той же БД PASS3.035s: fail-closed load, live reconnect и fresh server. Проверены hull roundtrip/jump/stale bootstrap, обе alias boost/M01 запрещены после сохранённого снятия. Legacy autosave не сохраняет runtime transport.
Параллельный широкий world/equipment/ws/db запуск выявил timeout существующего TestAccountProtocolErrors: five-second client context deadline (game_test.go:571), точная причина не установлена; финальный последовательный прогон впереди. Этот запуск нельзя считать PASS. SQL targeted выше завершился успешно.

## C-RT-02a server
`go test -race -count=1 ./internal/world -run '^TestEquipment'` PASS1.257s:13 unsafe states, CAS, definite rollback, ambiguous commit/reload failure, combat transfer/expiry, serialization with movement. `go test -race -count=1 ./internal/ws -run '^TestEquipment'` с настоящим PG PASS5.205s: strict payload rejects missing/foreign selectors, persisted CAS, both ability aliases, rollback/lost reply/unavailable reload/recovery, movement rejects before SQL.
Повторный targeted `go test -race -p 1 -count=1 ./internal/world ./internal/equipment ./internal/db ./internal/ws -run 'TestEquipment|TestSavedStarter|TestStarter|TestM01|TestStrizh|TestPG|TestEquipmentMigration'` с PostgreSQL — PASS по всем4 пакетам (4.137s/5.085s/1.407s/см. scratch log). Live Godot/PG также выполнен отдельно, результат относится к следующему client commit. Общий suite повторяется после интеграции.

## C-RT-02b client
Godot4.6.3 `--headless --path client --import`: PASS,0 ERROR. Автоматические изменения2 glb.import от версии редактора восстановлены, посторонние сгенерированные UID не включены. `hangar_test.gd`:39 проверок,0 провалов,0 raw ERROR. `hud_test.gd`:161 проверка,0 провалов,0 raw ERROR. Старый payload скрывает controls; revision+equipped проходят gateway; без подтверждения UI не меняет module; combat/unknown state показывают reason/reload.
`COSMOC_TEST_GODOT=godot COSMOC_TEST_DATABASE_URL=<отдельный localhost PG> go test -race -count=1 -v ./internal/ws -run '^TestEquipmentGodotClientPostgres$'` PASS10.426s (сам test6.83s). Настоящий Godot открывает ангар, снимает двигатель через кнопку, наблюдает disabled M01, переподключается с тем же synthetic token, подтверждает сохранённое снятие, ставит обратно и вызывает M01. Go проверяет конечный SQL equipped=true/revision2. Harness отвергает любые raw ERROR и требует EQUIPMENT_ONLINE_PASS; после закрытия сцены ждёт3.7s для lifecycle callbacks. Test schemas удаляются автоматически.
APK не собирался: release Android SDK/AAR/прежние signing bindings в окружении не настроены. Ключи не генерировались, update manifest не публиковался. Следующий этап — интеграция/main/CI, а не production.

C-RT-03 baseline repair: corp race+realPG PASS1.523s после корректного32-byte fixture hash; account WS recheck PASS14.597s без изменения production/test deadlines. Targeted runtime4 пакета завершилисьPASS, WS3.803s. Полный последовательный suite ещё впереди.

Полный последовательный suite перед final rank fix:431 test/subtest PASS,22 пакета PASS,1 существующий ambient test SKIP,3 пакета без тестов,0 FAIL, exit0. Все SQL и оба Godot live harness выполнены. Code74f9cfb/docs0a1ead6. Новый regression ERROR-SR-010 сначала FAIL(rank1 вместо0), после fix повторяются world/equipment/ws с race+PG+Godot; итог записать после завершения.

После ERROR-SR-010 fix: `COSMOC_TEST_GODOT=godot COSMOC_TEST_DATABASE_URL=<isolated PG> go test -race -p 1 -count=1 -json -timeout=180s ./internal/world ./internal/equipment ./internal/ws` — **157 test/subtest PASS,3 packages PASS,0 SKIP/FAIL,exit0** (3.631s/6.075s/27.850s). Включены новый rank0/4 regression и оба живых Godot harness. `go vet ./...` и `go build ... ./cmd/server` после этого fix — PASS. Более широкий suite не повторялся: все затронутые пакеты перепроверены целиком, остальные Go файлы не менялись.
