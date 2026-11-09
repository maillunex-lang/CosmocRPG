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
