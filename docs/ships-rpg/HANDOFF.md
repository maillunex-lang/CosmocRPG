# Текущая работа SR-100b (2026-11-09)
SR-100b1 DONE: server-bound starter ENG/M01, shared legacy boost CD/parameters, отрицательные ownership/equipment/rank/atomicity tests. SR-100b2 IN_PROGRESS: client presentation/cast и полный прогон. ADR006 обязателен: пока фиксированный неотчуждаемый штатный starter, без RPC смены и без mutable inventory; SQL/док/сохранение — SR-100c. Детали новых тестов в REGRESSION_TESTS. База main a19f7c0 имеет CI42608041551 success. Предыдущая передача ниже — история SR-100a; актуальная будет переписана после полного завершения SR-100b.

---

# HANDOFF — Ships RPG / R-63
Дата: 2026-11-09. Локальная ветка `codex/ships-rpg-foundation`. Пользователь разрешил публиковать проверенную работу/планы в **remote main**. Production-деплой — только с отдельного разрешения.

## Сделано и опубликовано
- Прежний фундамент SR-000/010/011/015/014a в main:15 draft паспортов, schema, identities15/12/30, отказ неизвестных runtime abilities, caps100/50. В runtime всё ещё12 игровых моделей.
- `75931d4`: R-63 и переход к сквозной интеграции на Стриже, изучен механизм APK Claude. SR-090 DONE.
- `88a060c`: test-only исправление старого PG ожидания после первого квеста; теперь first_blood/Progress0/Credits500 и без повторной награды. Runtime quest/SQL не менялся. SR-091 DONE, проверен PG CI.
- `cb34353`: SR-100a DONE — настоящий серверный паспорт текущего Стрижа в игровом ангаре. **Все4 jobs CI run42606462785 success** (Go с PostgreSQL, Godot4.7.2, admin, Docker). Новый live-сценарий Go WS→Godot gateway→hangar PASS0.769s в Godot CI.
- `ae16a86`: SR-093 DONE — тот же PUBLIC update PEM сохранён в git с точечным исключением .gitignore; обязательный presence/equality test. Локально updater33/0; production signed manifest Verified OK. Эта фиксация и финальная документация публикуются следующим push; новый CI отделять от завершённого CI для cb34353.
- Точный SHA финальной передачи: `git log -1 --oneline` после её docs-коммита. Перед началом сверить origin/main: другие ИИ теперь видят работу и могут менять код. Не force push.

## Что делает SR-100a
`server/cmd/server/main.go` загружает проверенный PassportCatalog и передаёт optional provider в ws.Options. Существующий ships response получает optional passport/ship_max_level; новые команды/версия протокола не требуются. Метаданные выдаются только текущему FTR-001; passport.base берётся из World.SpecInfo игрока, включая активные админские правки и масштабирование уровня. SetCatalog не вызывается, новые draft корабли/статы не публикуются. Provider nil отключает расширение в альтернативной конфигурации.
`client/ui/gateway/server_gateway.gd` передаёт optional passport по старому ships_updated; чужой hull/старый ответ не получают секцию. `client/ui/ships/hangar.gd` показывает серверное описание/base и предварительный план mass/MW/module budget/slots отдельно от живых статов. Sidebar прокручивается, cap приходит с сервера вместо30; старый сервер показывает уровень без выдуманного max.
**Старые навыки ещё не зависят от новых модулей.** Никакие starter blueprints пока не являются принадлежащими игроку предметами, новых бонусов к base нет.

## Тесты и ошибки
Локально Go1.27.3: `COSMOC_TEST_GODOT=godot go test -race -count=1 -json ./...` PASS20 пакетов,308 test/subtest PASS,62 SKIP(PG),0 FAIL; 3 без тестов. Standalone live Godot test PASS3.591s, включён в tools/test_client.sh для CI. Локально Godot4.6.3: import PASS, hangar28/0, fleet_shop53/0, updater33/0 после public PEM, apk_installer9/0. Реальный private-key branch updater SKIP; подпись живого манифеста проверена публичным ключом отдельно. vet/build/gofmt/diff-check PASS. Автоматические изменения4.6 в glb.import просмотрены/восстановлены, не коммитились.
CI cb34353(run42606462785):4 jobs success, в том числе PostgreSQL и Godot4.7.2. ERROR-SR-001…006 Resolved.
**ERROR-SR-007 Open**: существующие camera/background render-тесты при зелёном job логируют VK_KHR_surface missing/ERR_CANT_OPEN; camera ещё3 freed lambda captures. Они возвращают44/0 и12/0, поэтому success job не доказывает чистый runtime render. Baseline run42604786797/client job129117865136 имеет те же2 Vulkan/3 lambda сообщения до нашего игрового изменения. Не скрывать ошибки и не менять Mobile renderer ради зелёного лога; исследовать стенд и lambda lifecycle при SR-100d/SR-070. Android/чистая визуальная приёмка не выполнена.
Поздний gh CLI отвечает401 Bad credentials (причина не установлена); GitHub connector fetch_workflow_run_jobs/fetch_workflow_job_logs работает. Не печатать/менять токены, использовать connector. Лог running job недоступен до завершения.

## APK / выпуск
Изучены записи Claude/Kimi, updater.gd, apk_installer.gd, publish_release.sh; подробности RELEASE_PROCESS.md. Прежние app release и RSA manifest keys обязательны. **Новый APK и production не выпущены.** В cloud есть Java/Godot4.6.3, но не настроены SDK/AAR и release credentials. Не создавать другой signing key — обновление установленной игры не примется. project.godot/export_presets остаются0.3.11/code14: не занимать номер до готового кандидата и сверки main/опубликованного manifest.
READ-ONLY проверка живого канала: latest0.3.11/code14, size39268009, published2026-11-09T07:54:07Z; openssl Verified OK тем же публичным ключом. Godot Updater.evaluate: installed13→available,14→up_to_date. Это уже опубликованная версия, **не** APK нового ангара. Публичный PEM теперь в git; private *.pem остаются исключены.

## Следующие три шага
1. SR-100b: IN_PROGRESS → один стартовый ENG→M01 на Стриже, серверная проверка доступности/энергии/CD; без модуля отказ без списания. Сохранить per-ship boost и отдельную pulse автоатаку. Не превращать draft blueprint в owned item без модели владения.
2. SR-100c: безопасная добавочная схема оборудования, тестовая БД, сохранение/повторный вход, смена только в доке вне боя, атомарность/откат; ангар и HUD с причинами недоступности. Пока TODO.
3. SR-100d: полная игровая/графическая приёмка, ERROR-SR-007, release-кандидат через прежнее Android signing окружение; потом отдельное разрешение на production. Остальные SR-012…070 сохраняют все требования и заполняются по мере расширения сквозных шагов.

## Команды и файлы
```bash
cd /workspace/CosmocRPG
git status --short --branch
git fetch origin main
git log -8 --oneline
cat docs/ships-rpg/ROADMAP.md docs/ships-rpg/ERROR_LOG.md docs/ships-rpg/HANDOFF.md
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
COSMOC_TEST_GODOT=godot go test -race -count=1 ./internal/ws -run 'TestShipsPassport|TestShipsWithoutPassport|TestPassportGodotClient'
go vet ./...
```
Файлы игрового шага: server/cmd/server/main.go, internal/ws/{hub,passport_test}.go; client/ui/{gateway/server_gateway,ships/hangar}.gd; client/tests/{hangar_test,passport_online_test}.gd; tools/test_client.sh. Релизный фикс: .gitignore, client/assets/update_public.pem, client/tests/updater_test.gd. Test-only CI fix: server/internal/quest/quest_test.go. Документы: API, главные ROADMAP/HANDOFF, ships-rpg ROADMAP/ERROR_LOG/REGRESSION_TESTS/HANDOFF/DECISIONS/RELEASE_PROCESS.
Все временные memory servers закрываются тестами; серверные бинарники в scratch не запускались, VPS/другие проекты не тронуты. Сырые логи /workspace/scratch/ships-rpg-r63-all-tests.jsonl и другие могут исчезнуть; сводка в REGRESSION_TESTS и CI-ссылка сохраняются. После финального push проверить remote SHA, рабочее дерево оставить чистым.
