# HANDOFF ветки SR-100c, 2026-11-09

## Состояние и координация
Пользователь разрешил1→2→3: подключить сохранение при входе, смену у станции и интерфейс, затем сверить/объединить main. Codex работает в `codex/sr100c-storage`, worktree `/workspace/worktrees/cosmoc-sr100c-storage`. Root `/workspace/CosmocRPG` (main) не переключался и не менялся. Kimi делает ERROR-SR-007 в main; не закрывать его ошибку/не подхватывать незавершённую ветку. Старые указания «Codex непосредственно main» ниже заменены этим разрешением.
SR-100c-1/2 DONE локально; SR-100c-3 IN_PROGRESS: последние проверки/публикация. Main до публикации содержит чужие новые документы QUESTJOB; merge74f9cfb сохранил их без конфликтов, миграция0020 свободна. Перед push повторить fetch/merge, только non-force. Production и выпуск APK не разрешены этой интеграцией.

## Коммиты и поведение
- 38385f2: migration0020 adds player_module_items/player_ship_loadouts/player_ship_loadout_slots; owner+ship FK, unique grant/instance/slot, deferred ownership FK корректно поддерживает owner cascade. Никаких destructive migrations/перезаписи XP.
- 6cec61f: equipment PG/memory Ensure/Load/SetEngine, idempotent bootstrap, transactional revision CAS, согласованные snapshot, запрет повреждённой/чужой/неподдержанной сборки; старый autosave не пишет эти таблицы.
- 01bc109: server main включает PG equipment store. До Join загружается сохранённый engine; ошибка закрывает вход до создания/замены сессии. Для других hull Load без grant, Strizh Ensure только первый раз. Runtime-only Record fields не сохраняются old Store.Save; перенос между мирами/корпусами/перезаходом сохраняет состояние. Старая ревизия не затирает новую в live ship.
- ac029a8: equipment_set/get; payload revision+equipped, owner из текущей joined session. Только Strizh ENG-S-START/tech-1. Живой неподвижный корабль у основной станции(radius20); no autopilot/mining/active boost/attack и10s после входящего/исходящего урона. Combat grace переносится при jump. Проверка/DB/apply сериализованы; SQL error перечитывает authoritative state, неизвестный outcome закрывает boost до recovery. Снятие сохраняет learned rank1/CD/энергию, запрещает обе M01/boost aliases.
- 7407d43: ангар показывает кнопку и server reason, меняет отображение только по state; открытый ангар обновляется. Старый payload/демо скрывает неподдерживаемые controls; HUD availability использует прежний server-owned путь. Новый настоящий PG/WS/Godot scenario.
- 9573379: test-only исправление прежней corp fixture (отдельный32-byte token hash); production corp SQL не менялся.

## Фактические проверки
Реальный PostgreSQL16 только в собственном local container/tmpfs; каждый test получает отдельную схему. Initial23 SQL теста/подтеста PASS без skip; bootstrap world/equipment/db race PASS; runtime targeted4 packages race+PG PASS(4.137s/5.085s/1.407s/3.803s). Corp race+PG PASS1.523s. Godot4.6.3 import0 ERROR; hangar39/0; HUD161/0; live equipment PG/Godot PASS10.426s: снять→disabled M01→reconnect→refit→M01, конечный SQL revision2; raw ERROR отвергаются. Vet/build PASS после74f9cfb, bash-n клиентского test runner PASS. Полный последовательный Go+PG+Godot race suite ещё идёт, не объявлять весь CI зелёным до результата.
Все первоначальные FAIL сохранены в ERROR_LOG/REGRESSION_TESTS. ERROR-SR-009/CST004 Resolved: corp fixture TEXT/BYTEA. CST005:2 прежних account WS теста timed out при параллельном запуске, причина не установлена; отдельный повтор PASS14.597s, deadlines/криптография не менялись. Итог общего suite дописывается после завершения. ERROR-SR-007 остаётся Kimi. Graphical/Vulkan/Android smoke не выполнялись в этом этапе.

## Известные ограничения и неготовое
Это native-engine slice SR-100c, не общий inventory/масса/мощность/arbitrary modules/3 builds/все72 навыка. Новых stat modifiers поверх legacy base нет. Основная станция — настоящий station object, не вся safe-zone. Radius20/grace10 — initial alpha параметры, не завершённая балансировка.
DB callback держит mutex системы до write2s+recovery1s: строгая атомарность без TOCTOU, но медленная DB может задержать систему до3s (и текущий tick Router). До роста нагрузки нужен неблокирующий coordinator/queue/reservation и нагрузочная приёмка. Повтор SetEngine после неоднозначного COMMIT нельзя трактовать как rollback — сначала read.
APK release требует прежние app ID/signing keystore, SDK/AAR и manifest key; в cloud secrets bindings отсутствуют. Ключи не генерировать/не просить в чат. Новый APK/manifest не публиковались. Реальное обновление Android и production server — SR-100d с отдельным разрешением. Старые клиенты совместимы, новые controls требуют нового клиента.

## Следующие три шага
1. Проверить полный локальный suite и последний remote head/CI. Сохранить фактические PASS/FAIL/SKIP и run URL. Не маскировать отмену CI чужим docs-push как успешный прогон этого head.
2. Завершить публикацию проверенной интеграции в main без force, сохранив все чужие QUESTJOB/другие изменения. Root main checkout не обновлять пока он может принадлежать Kimi; следующему сеансу самостоятельно fetch и согласовать рабочую копию. Сверять migration numbers до любого будущего добавления.
3. SR-100d: графическая/Android приёмка вместе с результатами ERROR-SR-007, latency coordinator/load test перед масштабированием; подготовка release APK по RELEASE_PROCESS.md при доступных прежних ключах. Production только после разрешения.

## Возобновление и файлы
Сначала читать главную docs/ROADMAP.md, ships-rpg/{ROADMAP,ERROR_LOG,REGRESSION_TESTS,HANDOFF,DECISIONS}.md и sr100c-storage journals. Последний SHA — `git log -1`, поскольку этот документ сам входит в очередной docs commit. Paths: server/internal/equipment/*, world/{starter_engine,starter_loadout,equipment,world,combat,store}.go, ws/{equipment*,hub}.go, cmd/server/main.go, db/migrations/0020*, corp/pgstore_test.go; client/ui/ships/hangar.gd, ui/gateway/server_gateway.gd, scripts/space/space_screen.gd, tests/{hangar,equipment_online}_test.gd; tools/test_client.sh (новый PG scenario, render sections untouched).
```bash
cd /workspace/worktrees/cosmoc-sr100c-storage
git status --short --branch
git fetch origin main
git log -8 --oneline
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
# COSMOC_TEST_DATABASE_URL должен указывать только на отдельную тестовую БД.
COSMOC_TEST_GODOT=godot go test -race -p 1 -count=1 -timeout=300s ./...
go vet ./...
go build ./cmd/server
```
Системный /usr/bin/go — не Go toolchain. DB test schemas убираются testdb.Pool; контейнер cosmoc-sr100c-runtime-20261109 принадлежит только этой сессии и должен быть удалён в конце. Сырые логи /workspace/scratch/sr100c-* ephemeral; итог хранить в git, тестовый URL не превращать в production конфиг.

Главная карта — ../ROADMAP.md, этот журнал ей подчинён. Детали первоначальных storage failures сохранены в ERROR_LOG.md.
