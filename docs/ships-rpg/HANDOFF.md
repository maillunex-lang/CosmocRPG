# Передача Ships RPG v2.1
Дата: 2026-11-09. Ветка: codex/ships-rpg-foundation. База: 3b89741.
Стадия: SR-000, SR-010, SR-015 и SR-014a DONE; этап 1 в целом IN_PROGRESS, полные паспорта/умения/модули ещё TODO.
Прочитать CODEX_MASTER_PROMPT.md полностью, затем ROADMAP.md, ERROR_LOG.md, REGRESSION_TESTS.md и DECISIONS.md.

Сделано: аудит кода/данных/БД/UI/CI, интеграция главной карты, ADR, журнал ошибок. Реестр пока не включён в runtime; runtime изменён только для отказа неподдержанных способностей и caps 100/50.
Тесты: первичный Go-прогон не состоялся (ERROR-SR-001, теперь Resolved); официальный Go1.27.3 установлен локально, shipcatalog race PASS. Полный race до изменения caps: 20 пакетов PASS, 253 test/subtest PASS, 62 SKIP; после caps — 5 затронутых пакетов PASS (120 PASS, 11 SKIP). vet/gofmt/build PASS; подробности в REGRESSION_TESTS.
Ошибки: ERROR-SR-001 устранена toolchain; ERROR-SR-002 воспроизведена и исправлена в SR-015, итог проверки ниже/в REGRESSION_TESTS. Владелец подтвердил caps 100/50 и 6 узлов + усиление 20: согласование закрыто, блокировка снята.
Изменены: docs/ROADMAP.md, docs/HANDOFF.md, docs/ships-rpg/{AUDIT,ROADMAP,ERROR_LOG,REGRESSION_TESTS,DECISIONS,HANDOFF}.md. История изменений — основная docs/HANDOFF.md; известные ограничения — ERROR_LOG и раздел рисков AUDIT (без дублирующих журналов).
Коммиты реализации: 8c7b620 — аудит; 8ce0106 — SR-010; 0e7cbb3 — SR-015; 788d951 — SR-014a (последний SHA реализации). Финальная передача фиксируется отдельным docs-коммитом; его SHA смотреть через `git log -1 --oneline`.

Следующие три шага:
1. SR-011: начать полные паспорта, используя новый identity registry; сохранить старые runtime-ключи специализаций до явной миграции.
2. SR-012: спроектировать контракты эффектов и реализовать первое завершённое семейство навыков с предварительными числами.
3. SR-013/014: модули/достижимость/пассивы; ответ владельца уже получен, применять 6 узлов и усиление 20. Не объявлять этап DONE по одним счётчикам.

Возобновление:
```bash
cd /workspace/CosmocRPG
git status --short --branch
git log -5 --oneline
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
go version
go test -race -count=1 ./...
```
Если окружение сменилось, путь toolchain может отсутствовать: установить официальный Go с проверкой checksum. Не использовать /usr/bin/go без проверки версии. Не поднимать production, не обращаться к VPS/его БД, не менять инфраструктуру других проектов. DB-тесты запускать только с отдельной тестовой БД/временными схемами. Push/PR в этой сессии пока не выполнялись.

SR-010: добавлены shipcatalog/identity.go, identity_test.go, data/rpg_identity{,.schema}.json. Реестр — только контракт идентичностей, не 15 полных игровых паспортов. Проверка JSON Schema использует уже имеющийся jsonschema/v6. Не включать реестр в world или PostgreSQL автоматически.

## Итог границ и оставшегося объёма
SR-000 DONE, SR-010 DONE, SR-014a DONE, SR-015 DONE (полный race/vet/gofmt PASS, DB-тесты skipped). Весь этап 1 остаётся IN_PROGRESS. Нет готовых полных паспортов новых трёх моделей, библиотеки 72 активных/30 ульт/36 пассивов, оборудования, билдов, новых миграций, специализационного XP/деревьев, клиентской ульты и новых процедурных силуэтов. Остальные этапы не объявлять завершёнными.

SR-015 меняет только отказ для неподдержанных ключей abilities: до исправления UseAbility("custom") возвращал nil без эффекта. available теперь допускает только boost/intercept/scan, pulse обслуживается автоатакой. Когда появится effect registry в SR-020, заменить этот список реестром и сохранить тест.

Файлы SR-015: world/combat.go, combat_test.go; обновлены главные ROADMAP/HANDOFF и все профильные журналы/карта. Прочие проекты и production не затронуты. Все коммиты локальные, push/PR/deploy не выполнены. Пользователь может продолжить с этой ветки.

Следующая сессия: сперва git status/log и профильные журналы. Не предполагать, что локальный scratch/toolchain и сырые логи переживут смену окружения. Сводные результаты тестов хранятся в репозитории. Для DB и Godot нужны отдельные проверки; успех Go без URL не доказывает PostgreSQL-совместимость. Изменений БД/клиента в этой сессии нет.

## Подтверждение и реализация SR-014a
Владелец 2026-11-09 подтвердил новые требования. Пилот 1–100, корабль 1–50; формулы Base/Growth, XP и мастерство сохранены. Шесть узлов специализации + усиление ульты на 20 утверждены, но ещё не реализованы. Это не блокировка — продолжать по зависимостям SR-012 → SR-014. Обновлены docs/DECISIONS.md и docs/design/RPG_SCHEMAS.md; прежние АР-2/АР-4 остаются историей.

Тесты SR-014a: race rpg/pilot/shipxp/world/ws PASS; прежние пороги и продолжение после caps проверены на синтетическом XP. Существующий игрок с запасом XP сверх прежнего cap может получить уровень выше 60/30 — ожидаемо. Ни stored XP, ни схема БД не переписываются. Расширение уровня продолжает прежний рост характеристик 3%/уровень; числа остаются предварительными.

## Полный список областей изменений
- `server/internal/shipcatalog/identity.go`, `identity_test.go`, `data/rpg_identity.json`, `data/rpg_identity.schema.json`: целевой identity-контракт и валидатор.
- `server/internal/world/combat.go`, `combat_test.go`: отказ неподдержанной способности.
- `server/internal/rpg/progression.go`, `rpg_test.go`: caps и проверка старого XP; `pilot/pilot_test.go`, `shipxp/shipxp{,_test}.go`, `world/world.go`, `world/shiplevel_test.go`: актуальные проверки/комментарии.
- Документы: главные ROADMAP/HANDOFF/DECISIONS, design/RPG_SCHEMAS, профильные AUDIT/ROADMAP/ERROR_LOG/REGRESSION_TESTS/DECISIONS/HANDOFF.

Временные артефакты вне Git: `/workspace/scratch/cosmoc-server` (собран, не запущен), toolchain, `ships-rpg-baseline*.log`, `ships-rpg-verified-tests.jsonl` (полный PASS до caps), `ships-rpg-caps-tests.jsonl` (PASS после caps). Промежуточный `ships-rpg-final-tests.jsonl` не использовать как итог: запущен во время исправления cwd. Запущенных этой сессией серверов/контейнеров нет.
