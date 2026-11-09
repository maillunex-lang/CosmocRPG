# Передача Ships RPG v2.1
Дата: 2026-11-09. Ветка: codex/ships-rpg-foundation. База: 3b89741.
Стадия: SR-000 и SR-010 DONE; этап 1 в целом IN_PROGRESS, полные паспорта/умения/модули ещё TODO.
Прочитать CODEX_MASTER_PROMPT.md полностью, затем ROADMAP.md, ERROR_LOG.md, REGRESSION_TESTS.md и DECISIONS.md.

Сделано: аудит кода/данных/БД/UI/CI, интеграция главной карты, ADR, журнал ошибок. Новые runtime-механики пока не включены.
Тесты: первичный Go-прогон не состоялся (ERROR-SR-001, теперь Resolved); официальный Go1.27.3 установлен локально, shipcatalog race PASS. Полный серверный прогон и vet уточняются в REGRESSION_TESTS.
Открыто: ERROR-SR-002; согласование caps/размеров деревьев (вопрос задан через async). Ответа не предполагать.
Изменены: docs/ROADMAP.md, docs/HANDOFF.md, docs/ships-rpg/{AUDIT,ROADMAP,ERROR_LOG,REGRESSION_TESTS,DECISIONS,HANDOFF}.md. История изменений — основная docs/HANDOFF.md; известные ограничения — ERROR_LOG и раздел рисков AUDIT (без дублирующих журналов).
Коммит аудита: 8c7b620; текущий блок SR-010 — следующий коммит (его SHA смотреть через git log -- server/internal/shipcatalog/identity.go).

Следующие три шага:
1. SR-011: начать полные паспорта, используя новый identity registry; сохранить старые runtime-ключи специализаций до явной миграции.
2. SR-012: спроектировать контракты эффектов и реализовать первое завершённое семейство навыков с предварительными числами.
3. SR-013/014: модули/достижимость/пассивы; перед caps/деревьями проверить ответ владельца. Не объявлять этап DONE по одним счётчикам.

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
