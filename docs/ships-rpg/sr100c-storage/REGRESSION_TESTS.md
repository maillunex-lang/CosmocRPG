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
