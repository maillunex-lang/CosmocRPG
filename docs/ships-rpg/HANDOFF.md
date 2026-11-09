# Актуальный HANDOFF — SR-100c, 2026-11-09

## Состояние и координация
Пользователь разрешил1→2→3: подключить сохранение при входе, смену у станции и интерфейс, затем сверить/объединить main. Codex работает в `codex/sr100c-storage`, worktree `/workspace/worktrees/cosmoc-sr100c-storage`. Root `/workspace/CosmocRPG` (main) не переключался и не менялся. Kimi делает ERROR-SR-007 в main; не закрывать его ошибку/не подхватывать незавершённую ветку. Старые указания «Codex непосредственно main» ниже заменены этим разрешением.
SR-100c-1/2/3 DONE по узким критериям native engine; код и общие журналы опубликованы в main d9669c9 без force. Remote CI отдельно pending, не success. Последний code fix53d08c3 сохраняет learned rank при refresh. Main при интеграции включала чужие QUESTJOB changes; merge74f9cfb и последующие49f1909/d9669c9 сохранили их без конфликтов, номер0020 свободен. Сохранено и чужое удаление NEW_STORY_CAMPAIGN.md(26300aa), добавление FinalQuest.md85abfdd. Перед push повторить fetch/merge, только non-force. Production требует отдельного разрешения. Сборка APK при необходимости разрешена пользователем; выпуск сейчас ограничен отсутствием прежнего Android-окружения/ключей, новые ключи не создавать.

## Коммиты и поведение
- 38385f2: migration0020 adds player_module_items/player_ship_loadouts/player_ship_loadout_slots; owner+ship FK, unique grant/instance/slot, deferred ownership FK корректно поддерживает owner cascade. Никаких destructive migrations/перезаписи XP.
- 6cec61f: equipment PG/memory Ensure/Load/SetEngine, idempotent bootstrap, transactional revision CAS, согласованные snapshot, запрет повреждённой/чужой/неподдержанной сборки; старый autosave не пишет эти таблицы.
- 01bc109: server main включает PG equipment store. До Join загружается сохранённый engine; ошибка закрывает вход до создания/замены сессии. Для других hull Load без grant, Strizh Ensure только первый раз. Runtime-only Record fields не сохраняются old Store.Save; перенос между мирами/корпусами/перезаходом сохраняет состояние. Старая ревизия не затирает новую в live ship.
- ac029a8: equipment_set/get; payload revision+equipped, owner из текущей joined session. Только Strizh ENG-S-START/tech-1. Живой неподвижный корабль у основной станции(radius20); no autopilot/mining/active boost/attack и10s после входящего/исходящего урона. Combat grace переносится при jump. Проверка/DB/apply сериализованы; SQL error перечитывает authoritative state, неизвестный outcome закрывает boost до recovery. Снятие сохраняет learned rank1/CD/энергию, запрещает обе M01/boost aliases.
- 7407d43: ангар показывает кнопку и server reason, меняет отображение только по state; открытый ангар обновляется. Старый payload/демо скрывает неподдерживаемые controls; HUD availability использует прежний server-owned путь. Новый настоящий PG/WS/Godot scenario.
- 9573379: test-only исправление прежней corp fixture (отдельный32-byte token hash); production corp SQL не менялся.

## Фактические проверки
Реальный PostgreSQL16 только в собственном local container/tmpfs; каждый test получает отдельную схему. Initial23 SQL теста/подтеста PASS без skip; bootstrap world/equipment/db race PASS; runtime targeted4 packages race+PG PASS(4.137s/5.085s/1.407s/3.803s). Corp race+PG PASS1.523s. Godot4.6.3 import0 ERROR; hangar39/0; HUD161/0; live equipment PG/Godot PASS10.426s: снять→disabled M01→reconnect→refit→M01, конечный SQL revision2; raw ERROR отвергаются. Vet/build PASS после74f9cfb, bash-n клиентского test runner PASS. Полный последовательный Go+PG+Godot race suite до final rank fix:431 test/subtest/22 пакета PASS,1 ambient SKIP,3 пакета без тестов,0 FAIL. Final review нашёл ERROR-SR-010 (rank reset); red→green, после fix все3 затронутых пакета целиком:157 test/subtest PASS,0 SKIP/FAIL,оба Godot harness выполнены. Vet/build после fix PASS. Remote CI code head d9669c9: https://github.com/maillunex-lang/CosmocRPG/actions/runs/42650859949 — pending на момент проверки. Docs push/параллельные main commits могут отменить этот run; смотреть последний main CI, не приравнивать локальные PASS к remote success.
Все первоначальные FAIL сохранены в ERROR_LOG/REGRESSION_TESTS. ERROR-SR-009/CST004 Resolved: corp fixture TEXT/BYTEA. CST005:2 прежних account WS теста timed out при параллельном запуске, причина не установлена; отдельный повтор PASS14.597s, deadlines/криптография не менялись. Итог общего suite дописывается после завершения. ERROR-SR-010 Resolved: runtime refresh сохраняет прежний learned rank0/4, cold initializer остаётсяrank1; новый regression проверяет energy/CD. ERROR-SR-007 остаётся Kimi. Graphical/Vulkan/Android smoke не выполнялись в этом этапе.

## Известные ограничения и неготовое
Это native-engine slice SR-100c, не общий inventory/масса/мощность/arbitrary modules/3 builds/все72 навыка. Новых stat modifiers поверх legacy base нет. Основная станция — настоящий station object, не вся safe-zone. Radius20/grace10 — initial alpha параметры, не завершённая балансировка.
DB callback держит mutex системы до write2s+recovery1s: строгая атомарность без TOCTOU, но медленная DB может задержать систему до3s (и текущий tick Router). До роста нагрузки нужен неблокирующий coordinator/queue/reservation и нагрузочная приёмка. Повтор SetEngine после неоднозначного COMMIT нельзя трактовать как rollback — сначала read.
APK release требует прежние app ID/signing keystore, SDK/AAR и manifest key; в cloud secrets bindings отсутствуют. Ключи не генерировать/не просить в чат. Новый APK/manifest не публиковались. Реальное обновление Android и production server — SR-100d с отдельным разрешением. Старые клиенты совместимы, новые controls требуют нового клиента.

## Следующие три шага
1. Проверить CI последнего main head и результат Kimi ERROR-SR-007. Предыдущие PASS431/157 — фактические локальные проверки, не утверждение об успешной Vulkan/Android приёмке. Не менять renderer ради скрытия ERROR.
2. SR-100d: игровая/графическая/Android приёмка, подготовка нового APK по RELEASE_PROCESS.md после подключения прежних SDK/AAR/app keystore/manifest key через окружение. Версия/code и каналы не изменялись. Без production deployment до отдельного разрешения.
3. До расширения нагрузки убрать DB latency из общего world mutex через coordinator/queue/reservation, затем расширять модульный сценарий по основной roadmap/утверждённым QUESTJOB решениям. Полный arbitrary inventory/билды/72/30/36 не DONE.

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
Системный /usr/bin/go — не Go toolchain. DB test schemas убираются testdb.Pool; собственный контейнер cosmoc-sr100c-runtime-20261109 уже остановлен и удалён после всех проверок. Для следующего прогона нужен новый отдельный PG, прежний URL/порт не использовать. Сырые логи /workspace/scratch/sr100c-* ephemeral; итог хранить в git, тестовый URL не превращать в production конфиг.

---

# Исторические checkpoints до SR-100c

# Актуальный порядок работы (R-64, 2026-11-09)
**Рабочая ветка Codex теперь main**, синхронизирована с origin/main; прежнее имя codex/ships-rpg-foundation ниже относится к истории выполненного SR-100b. Kimi делает свои патчи в отдельной ветке и сам добавляет их в main после завершения текущей работы Codex. Не подхватывать незавершённую ветку Kimi, не force push; перед публикацией сверять актуальную origin/main и сохранять чужие изменения.
База переключения8ffa713; этот координационный коммит не меняет игровой код. Следующая задача разработки — **SR-100c (TODO)**: сохранение изменяемой экипировки, док вне боя, атомарный rollback и повторный вход. Production/APK без новых разрешений/готового signing-окружения не публиковать. CI базы8ffa713 (run42618339165) на момент переключения ещё in_progress; не приравнивать к завершённому CI gameplay c9a3391.

---

# Последняя контрольная точка SR-100b (2026-11-09)
В main опубликованы **21aaa0b**(server),**d465afe**(client),merge **c9a3391** с сохранением чужого landing545032a. CI42616666345 для c9a3391 — все4 jobs success (Go/PostgreSQL, Godot4.7.2, admin, Docker). Дополнительный **a1c5c9e** исправляет доказанный toast lifetime defect (ERROR-SR-008); локальный real-client race9.034s и HUD161/0 без runtime ERROR, его новый CI проверяется отдельно.
SR-100b DONE; следующий SR-100c TODO: сохранённая изменяемая экипировка, atomic save/rollback, док вне боя, reconnect/hull/system transitions. Сейчас starter fixed/non-transferable, кнопки снятия/установки нет. APK/production/SQL в этой сессии не публиковались/не менялись.
Последний SHA реализации a1c5c9e; финальный docs-checkpoint смотреть git log -1. Все подробности/команды/ограничения ниже. ERROR-SR-007 остаётся Open по Vulkan/неподтверждённым render проблемам.

---

# HANDOFF — Ships RPG / SR-100b
Дата2026-11-09. Ветка codex/ships-rpg-foundation; изменения публикуются в remote main по разрешению владельца. База этой работы a19f7c0, CI42608041551 success. Production только по отдельному разрешению; APK signing окружение пока не подключено.

## Текущая стадия
SR-100b1 DONE, server commit **21aaa0b**. SR-100b2 DONE локально, client commit **d465afe**. Оба опубликованы в main через merge **c9a3391**; CI42616666345 завершён: **все4 jobs success**, Go с PostgreSQL и Godot4.7.2 включены. SR-100b в целом DONE по его узким критериям; **SR-100c TODO** (mutable equipment/persistence/dock/combat/atomic save), SR-100d TODO (полная игровая/графическая/Android/release приёмка). Остальные требования72/30/36, 4 slots+ульта, прогрессия/билды не отменены и не объявлены готовыми.

## Что сделано
- `world/starter_engine.go`: текущая штатная система Стрижа представлена ровно одним starterEngineItem: server OwnerID, детерминированный instance ID от owner, Prototype ENG-S-START, Slot tech-1, Equipped=true, starter learned rank1. Нельзя продавать/передавать/переставлять через RPC: их ещё нет. Это фиксированное native entitlement старого корабля, не преобразование draft JSON в произвольную выдачу вещей. После rejoin/hull roundtrip тот же ID, один экземпляр; общая inventory/SQL материализация — SR-100c.
- `world/combat.go`: M01 только FTR-001, нормализуется к boost до единого available/switch/cooldown path. Старый boost у Стрижа проверяет те же owner/instance/prototype/slot/equipped/rank; пустой/чужой/поддельный/отключённый модуль даёт module_unavailable, ранг0/6 ability_not_learned. Отказ не тратит энергию/CD/события/позицию. Active AbilityCfg админки сохраняется, pulse отдельно; другие hulls без M01, legacy boost прежний.
- Snapshot: abilities id остаётся boost, новые catalog_id M01/module_id/rank/enabled/unavailable_reason; ship.modules — server-owned view фиксированного двигателя. Не принимать эти данные обратно от клиента как права. Привязка prototype/slot/M01 сверена тестом с паспортным starter blueprint.
- Gateway выбирает CatalogID при наличии, иначе старый ID. Передаёт enabled/reason/modules; default old payload enabled:true. HUD блокирует disabled навык и показывает причину, учитывает старые energy/CD/target правила. Ангар показывает действующий двигатель/M01 отдельно от preliminary mass/power/slots.
- Expanded real-client scenario: реальный memory-backed localhost Go WS→Godot gateway→ангар→starter→M01→state boosting. Новый WS-only test проверяет, что следующий legacy boost получает cooldown.

## Проверки
Локальный Go1.27.3: `COSMOC_TEST_GODOT=godot go test -race -count=1 -json ./...` PASS20 пакетов,322 test/subtest PASS,62 SKIP(PG),0 FAIL;3 пакета без тестов. Конкретные unit: world1.664s, shipcatalog11.267s. Live WS+Godot targeted4.660s.
Godot4.6.3: hangar33/0, HUD161/0. В старом payload fallback boost/enabled:true, disabled/reason проходит через gateway/HUD, active starter виден отдельно от draft паспорта. vet/build/gofmt/diff-check PASS; бинарник /workspace/scratch/cosmoc-server-m01 не запускался. Никаких SQL миграций/production/APK в этой работе. Remote CI нового main проверяется отдельно после push.
Новых ERROR-ID не было, отрицательные тесты ownership/learned — ожидаемые отказы. ERROR-SR-007 Open: прежние Vulkan init/freed-lambda в CI render tests при зелёном job; не скрывать/не менять Mobile renderer ради лога, графическая/Android приёмка остаётся SR-100d/SR-070. Прежние ERROR001…006 Resolved.

## Ограничение starter entitlement
Нет public метода снять/поставить модуль в этой версии. Поэтому перезаход не теряет доступную пользователю настройку: весь starter пока неизменяемый. Негативные tests намеренно повреждают server-side entitlement для проверки отказа. **Не открывать toggle в следующем UI до сохранения/atomic server setter.** Ранг1 — единственный стартовый изученный ранг; полноценные ranks1…5/развилки/skill data — SR-012. Никаких новых stat modifiers от двигателя поверх legacy base нет.

## Следующие три шага
1. Проверить remote main/CI client и Go с PostgreSQL после публикации SR-100b. Обновить карты/журналы конкретным SHA/status. GitHub connector workflow runs GET может читать push runs (fetch_commit_workflow_runs wrapper фильтрует PR, не использовать для проверки push).
2. SR-100c: безопасная модель сохранённого оборудования/владения и атомарный setter до открытия UI, сохраняющий starter identity; добавить тестовую БД, reconnect/hull/system roundtrip, duplicate/idempotency, stale autosave/CAS, rollback при ошибке. Смена только в реальном доке и вне боя (не заменять dock проверкой всей safe-zone).
3. Добавить управление модулем в ангар/HUD с reasons после готового сохранения, затем сквозная приёмка/ERROR007/Android release. Нужны прежние SDK/AAR и app/manifest keys; не генерировать другой release key. Production по отдельному разрешению.

## Возобновление
```bash
cd /workspace/CosmocRPG
git status --short --branch
git fetch origin main
git log -8 --oneline
cat docs/ships-rpg/ROADMAP.md docs/ships-rpg/ERROR_LOG.md docs/ships-rpg/HANDOFF.md
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
COSMOC_TEST_GODOT=godot go test -race -count=1 ./internal/ws -run 'TestM01OverWebSocket|TestPassportGodotClient'
go test -race -count=1 ./internal/world -run 'TestStrizhStarter|TestM01|TestStarterEngine'
go vet ./...
```
Файлы: world/starter_engine{,_test}.go, world/{world,ship,combat}.go, ws/{hub,passport_test}.go, shipcatalog/passport_data_test.go; client gateway/server_gateway.gd, ships/hangar.gd, scripts/ui/hud.gd, tests/{hangar_test,passport_online_test}.gd; API и главные/профильные ROADMAP/HANDOFF/DECISIONS/ERROR_LOG/REGRESSION_TESTS.
Перед push сверить remote и чужие изменения, не force. gh CLI ранее401; пользоваться GitHub connector, секреты не печатать. Runtime cloud status ready, secrets/SDK signing bindings не настроены. Другая VPS/production не тронуты. Все ephemeral серверы закрыты тестами; scratch логи могут исчезнуть, сводка сохранена. После финальной фиксации оставить clean tree и проверить origin/main SHA.

## Параллельная main
Во время push другой ИИ добавил landing545032a (site/index.html,2 строки). Non-FF push корректно отклонён; обычный merge c9a3391 сохранил обе стороны, без force/конфликтов. Server/client/tools не изменились, повторный Go прогон только из-за site не нужен; check_site.py CSP/14 ссылок PASS. Push c9a3391 успешен. Ссылка CI: https://github.com/maillunex-lang/CosmocRPG/actions/runs/42616666345. Итог: все4 jobs success. Это завершённый CI именно gameplay c9a3391.

## Дополнительная исправленная ошибка жизненного цикла
При локальном HUD161/0 был один raw Lambda capture ERROR. Первоначальный baseline worktree a19f7c0 и отдельный busy timeout сценарий не воспроизвели его; гипотеза о busy callback отвергнута. Детерминированный toast + закрытие screen + ожидание3.7s дал3 ERROR при child exit0. Причина: alert_bar.gd lambda захватывает уже freed Node, внутренний is_instance_valid вызывается слишком поздно.
Заменён callback на p.queue_free (bound method с автоматическим disconnect при удалении panel). После исправления live race9.034s, HUD161/0; оба лога без ERROR/SCRIPT ERROR. Live regression теперь ждёт после закрытия сцены и ловит любой raw ERROR, не только SCRIPT ERROR. ERROR-SR-008 Resolved; ERROR-SR-007 остаётся Open по Vulkan/другим неподтверждённым render причинам. Изменения alert_bar/passport_online_test/ws harness фиксируются дополнительным коммитом после gameplay c9a3391; для них следующий CI может ещё выполняться. Следующий ИИ должен сверить последний head/runs. Механика модулей/production Go не менялась после322/62 и зелёного PG CI.
