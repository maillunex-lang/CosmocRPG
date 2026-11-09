# C-RT-02 завершён
Server bootstrap01bc109; server dock setter — предыдущий commit в git log; client controls и настоящий PG/Godot regression — этот commit. Targeted server race/PG PASS, hangar39/0, HUD161/0, live PG→Godot PASS. Следующий C-RT-03: fetch/merge latest main, исправить отдельно baseline corp fixture, последовательный общий suite/vet/build и сверка новых shared journals. ERROR-SR-007 не менять; production/APK не публиковать. Риск slowDB mutex latency≤3s сохранён в ADR005/API. Старые разделы ниже — история, не актуальный TODO runtime.

---

# Продолжение C-RT-01 (2026-11-09)
Пользователь разрешил1→2→3: bootstrap, dock-only setter/UI, затем сверка и merge main. C-RT-01 DONE: runtime/main подключены к PG store; tests PG world/equipment/db PASS и targeted WS PASS. Полный параллельный suite имеет timeout account harness(CST-ERROR-005), нужна повторная проверка. Сейчас C-RT-02 TODO. Ветка прежняя, main не менялся. Собственный контейнер cosmoc-sr100c-runtime-20261109 временно нужен для следующих тестов, удалить в конце сессии. Не публиковать production/APK без ключей/разрешения. Подробности новых решений и фактических запусков в DECISIONS/REGRESSION_TESTS.

---

# HANDOFF — SR-100c storage, 2026-11-09

Ветка `codex/sr100c-storage`, worktree `/workspace/worktrees/cosmoc-sr100c-storage`, база0b77fea. Main checkout `/workspace/CosmocRPG` не переключать: Kimi занимается ERROR-SR-007. В main не объединяли, production не развёртывали. Общие журналы, клиент, HUD, CI не меняли. При объединении обязательно перенести ссылку на этот task journal в главную ROADMAP/HANDOFF/ERROR_LOG/REGRESSION_TESTS.

## Готово
C-ST-01/02/03 завершены в пределах storage slice. Коммиты38385f2(schema/interface) и6cec61f(PG/memory stores+tests). Миграция0020 добавляет только3 таблицы; не переписывает players/XP. Native ID совпадает с прежним world starter. Ensure выдаёт начальную связку один раз, не надевает снятый двигатель при reconnect. Load read-only. SetEngine использует CAS revision и SQL-транзакцию; один победитель при16 конкурентных запросах. Старый autosave не пишет эти таблицы. Composite ownership/ship FK запрещает чужой предмет; deferred проверка допускает owner cascade, но запрещает удалить fitted item отдельно. Неизвестные/повреждённые slots отвергаются без потери данных.

## Проверки и ограничения
Реальный PostgreSQL16:23 targeted теста/подтеста PASS,0 SKIP/FAIL. Race, rollback после промежуточного DELETE, concurrent snapshot, owner cascade, theft/double fit, legacy XP проверены. Vet/build PASS. Общий Go+PG race suite exit1: единственный падающий тест TestPGStoreCorporationFlow, SQLSTATE42P08 в существующей fixture. Воспроизведён на чистом архиве базы0b77fea; CST-ERROR-004 OPEN. Остальные результаты и предыдущие исправленные ошибки описаны в локальных REGRESSION_TESTS/ERROR_LOG. Общий suite нельзя объявлять зелёным.
Собственный временный PostgreSQL контейнер удалён после проверок, данные были tmpfs. Production DB/сервер/APK не использовались. Клиентский тест Godot здесь не запускался: это зона Kimi.

## Что ещё не сделано
Store не подключён к runtime, world.Join, WS/RPC или UI. Снятие двигателя в игре пока не включено. Это не завершение всего SR-100c. Отсутствуют серверная авторизация команды, проверка настоящего dock/combat, budget массы/мощности, reconciliation committed DB→world. Новый StarterStore не принимает произвольных item/prototype/slot. Не использовать его как публичный equip API.
При неоднозначном сетевом сбое COMMIT будущий coordinator должен перечитать authoritative loadout: ошибка ответа не гарантирует откат уже принятой транзакции. Нельзя возвращать world в старое состояние вслепую.

## Следующие шаги
1. После завершения Kimi fetch origin main, проверить его HANDOFF, изменения API и уникальность номера0020 (0019 уже занята корпорациями). На последней проверке origin/main=c274922, изменения после базы только QUESTJOB/GearScore.md и HiddenMap.md. Нашу draft миграцию можно перенумеровать до применения, опубликованные/применённые чужие миграции не менять.
2. Согласованно объединить эту feature branch, перенести статусы в общую roadmap и журналы. Перед merge повторить targeted SQL tests и проверить общий CI; baseline corp FAIL не скрывать. Не мержить автоматически одновременно с Kimi.
3. Отдельным небольшим этапом реализовать coordinator bootstrap до world.Join и dock-only atomic setter с актуальными battle/state checks, затем WS и UI. Обязательно проверить reconnect/rollback/stale autosave до включения игрокам. Production deployment требует разрешения пользователя.

## Повторение проверок
Нужен настоящий PostgreSQL и COSMOC_TEST_DATABASE_URL на отдельную тестовую БД (testdb создаёт/удаляет отдельные схемы). В server: `go test -race -count=1 ./internal/equipment ./internal/db`, затем `go vet ./...` и `go build ./cmd/server`. В текущем окружении Go находится в /workspace/scratch/toolchain/go/bin (системный /usr/bin/go не toolchain). Полные локальные JSON logs находятся в /workspace/scratch/equipment-storage-final-targeted.jsonl и equipment-storage-all-go-pg.jsonl; они ephemeral, итог сохранён в git.

## C-RT-02a checkpoint
C-RT-01 commit01bc109. Server setter/reconcile+tests готовы, client changes остаются в worktree и будут отдельным коммитом. Targeted realPG/race PASS; документы API и ADR005 описывают exact gates и latency limitation. Новые тесты equipment_godot_test.go требуют COSMOC_TEST_GODOT и отдельную DB. Следом client commit, затем latest main merge/full checks.
