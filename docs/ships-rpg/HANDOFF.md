# HANDOFF — SR-100c-4, 2026-11-09

## Текущий этап
Ветка `codex/sr100c-coordinator`, worktree `/workspace/worktrees/cosmoc-sr100c-coordinator`, базаb945efb. Пользователь подтвердил следующий этап: убрать DB stalls из общего мира/сессий, проверить параллельность, обновить журналы и безопасно объединить.4a/4b DONE,4c IN_PROGRESS до подтверждения публикации; local validation завершена. Root `/workspace/CosmocRPG` (main) и прежний worktree storage не переключались/не менялись. Kimi делает ERROR-SR-007 в main, эту ошибку не закрывать и незавершённые ветки не подхватывать. Production не разрешён; сборка APK при необходимости разрешена, но прежнее Android SDK/AAR/signing окружение не подключено.

## Готовые коммиты
- **bc0aed9** — world.ChangeStarter резервирует только actor под коротким mutex, вызывает store вне lock, применяет authoritative outcome под коротким lock. Одна lease на actor; проверка совпадения Player instance и lease перед apply. SQL CAS/recovery не менялись. На panic release/fail closed. Снимки и simulation не ждут IO.
- **aa7f950** — per-owner cancellable gate для WS gameplay/finishJoin/leave. Handler.mu только для краткого active map access, ни ожидание gate, ни storage не держат его. Новый session того же public ID ждёт commit/recovery до bootstrap/замены; stale old commands проверяют active session; old leave не отключает successor. Refcount включает holder/waiters, cancel удаляет waiter, idle entries удаляются; idempotent unlock.
- **a5bfd2e** — optional equipment.pending → ангар «Сохраняем оснащение…», control disabled. Ability snapshots тоже disabled/equipment_busy до завершения. Старые payload/клиенты совместимы. Нового RPC/protocol version/миграции нет.

## Инварианты
Док/бой/expected revision проверяются при принятии операции. На pending запрещены действия этого actor: move/autopilot/cast/attack/mining/spec/hull/transfer/remove/reload/вторая смена. World.Step удерживает у станции только его movement; другие actors/ticks/snapshots продолжаются. Входящий урон, автоответ и смерть НЕ задерживаются/не откатываются. Урон после принятия не отменяет принятую смену; она может завершиться после hit, включая смерть. Это явное admission rule, не обещание проверки combat на сетевом COMMIT. Engine-only slice не даёт healing/stat boosts при refit.
Grace cleanup ждёт resolution, потом обычное удаление сохраняет committed revision. Router проверяет lease+gate distance атомарно при remove, pending Leave не теряет byPlayer binding. Нет временного таймера, который снимет lease раньше завершения IO и разрешит позднему callback перезаписать следующий actor. Production PG callback имеет прежние deadlines write2s/recovery1s; зависший сторонний callback, игнорирующий deadline, удержит только своего actor/owner, не общий world/Handler mutex. Для общего arbitrary inventory нужны отдельные дальнейшие правила, этот этап их не реализует.
Cold init/rank1 и legacy energy/CD не изменены; runtime refresh сохраняет learned rank. Старый autosave не пишет equipment tables. Не трактовать COMMIT error как rollback: authoritative Load, unknown outcome закрывает boost до equipment_get/reconnect. Старые ограничения ≤3s общей остановки из HANDOFFb945efb устранены в этой ветке; обычное время IO для самого actor осталось.

## Фактические проверки
Source head a5bfd2e. `COSMOC_TEST_GODOT=godot COSMOC_TEST_DATABASE_URL=<отдельный local PG> go test -race -p 1 -count=1 -json -timeout=300s ./...` **exit0:441 test/subtest PASS,22 packages PASS,0 FAIL;1 прежний AmbientFollowFromAnotherCaptain SKIP,3 пакета без тестов**. Ни один SQL или live Godot case не пропущен. Это локальный полный результат, не remote CI success.
Red→green: World slowIO1.00s FAIL из-за world.mu → PASS; WS slowIO1.03s FAIL из-за Handler.mu → PASS. Full world race PASS1.787s. Targeted WS+PG PASS6.553s; повтор core/ws1.047s/5.793s. Fatal-damage acceptance1.087s: commit не воскрешает игрока. Panic cleanup, grace, router leave/jump, duplicate lease, owner gate cancel/isolation/idle cleanup, reconnect→committed revision→следующийCAS проверены. Реальный PG row FOR UPDATE удерживался, пока B move/ships + Tick + C join завершались, затем A commit подтвердилrev+1.
Godot4.6.3 import exit0/0 ERROR; hangar40/0 без raw ERROR. Полный Go suite выполнил оба live Godot harness (passport и PostgreSQL unequip/reconnect/refit/M01). Vet/build PASS. UI import changes2 glb.import восстановлены,3 generated UID не включены. Renderer/Vulkan/Android не тестировались: ERROR-SR-007 у Kimi. APK/manifest/production не публиковались, version/code не менялись.
ERROR-SR-011 Resolved локально (bc0aed9/aa7f950, UIa5bfd2e). История красных запусков и причина сохранены в ERROR_LOG/REGRESSION_TESTS. Старые ERROR009/010 остаются Resolved; старый timeoutCST005 не повторился, deadlines не ослаблялись.

## Файлы и окружение
World: equipment.go, equipment_async_test.go, equipment_test.go (старый mutex-expectation test заменён новым contract), world.go, router.go, combat.go, fleet.go, living.go, starter_loadout.go. WS: player_operation{,_test}.go, equipment.go, hub.go, equipment_async{,_pg}_test.go. Client: ui/ships/hangar.gd, tests/hangar_test.gd. Документы: API, главные ROADMAP/HANDOFF, профильные ROADMAP/ERROR_LOG/REGRESSION_TESTS/DECISIONS/HANDOFF, UI_CHANGELOG.
Go находится в `/workspace/scratch/toolchain/go/bin`; системный /usr/bin/go не toolchain. PostgreSQL16-alpine только собственный контейнер `cosmoc-equipment-coordinator-20261109`, tmpfs512MiB/2CPU, published127.0.0.1; testdb создаёт/удаляет отдельные schemas. Контейнер удалить после проверок. Никакой VPS/чужой DB не использовались. Сырые логи `/workspace/scratch/coordinator-*` ephemeral, итоги сохранены в git.

## Следующие три шага
1. Fetch main, проверить изменённые paths/новые migration numbers/чужие commits; merge только опубликованную main без force. До9b02d69 менялся только QUESTJOB/FinalQuest.md, Go source послеa5bfd2e не менялся. Завершить4c publication checkpoint и отметить actual CI run; не называть cancelled/pending success.
2. SR-100d: игровая/графическая/Android приёмка и результат Kimi ERROR-SR-007. Новый APK делать с прежним app ID/keystore/manifest key через environment bindings, ключи в чат не просить/не генерировать замену. Production только после отдельного разрешения.
3. Расширение module families/слотов/builds по общей roadmap после приёмки; полный inventory/budgets/72/30/36 не DONE. Проверить нагрузку перед массовым расширением, actor isolation не заменяет общий load test.

## Команды возобновления
```bash
cd /workspace/worktrees/cosmoc-sr100c-coordinator
git status --short --branch
git fetch origin main
git log -8 --oneline
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
# COSMOC_TEST_DATABASE_URL только для своей отдельной тестовой БД.
COSMOC_TEST_GODOT=godot go test -race -p 1 -count=1 -timeout=300s ./...
go vet ./...
go build ./cmd/server
```
Читать docs/ROADMAP.md и все профильные журналы перед продолжением. Последний SHA смотреть git log, поскольку HANDOFF входит в следующий docs commit. История SR-100c storage/bootstrap остаётся в git и sr100c-storage/HANDOFF.md; актуальные решения в SR-ADR009 заменяют старый world-mutex callback.
