# Передача Ships RPG v2.1
Дата: 2026-11-09. Ветка: codex/ships-rpg-foundation. Предыдущая контрольная точка: 79df286.

Текущая задача: SR-011 (полные паспорта). SR-011a DONE: типы, JSON Schema на основе существующей SchemaJSON для base, семантический валидатор, стартовые бюджеты и крепления, negative/roundtrip/race-тесты. SR-011b TODO: добавить авторские 15 паспортов и проверить совпадение всех 12 legacy ShipSpec. Весь этап 1 IN_PROGRESS.

Последние проверки: race shipcatalog PASS 7.007s; vet shipcatalog PASS. ERROR-SR-003 (тест ошибочно ожидал семантический текст вместо schema uniqueItems) исправлена; ERROR-SR-001/002 также Resolved. PostgreSQL/Godot/production в этой задаче не проверялись и не менялись.

Файлы SR-011a: server/internal/shipcatalog/passport.go, passport_test.go, data/rpg_passports.schema.json; docs/ships-rpg/{ROADMAP,DECISIONS,ERROR_LOG,REGRESSION_TESTS,HANDOFF}.md; главная docs/HANDOFF.md. Schema использует ship-patch-v1.json, который компилятор регистрирует из существующей SchemaJSON().

Решение SR-ADR-003: полный документ v2 отделён от runtime Service/Store. base — точный снимок прежнего ShipSpec, не новый слой бонусов. Активные админские версии не перезаписывать. Старые local specs и новые global specialization IDs пока живут раздельно. Стартовые blueprints самодостаточные, их family — классификация, не ID будущего модуля. Реализация эффектов/достижимость unlock-путей — SR-012/013/020, владение/транзакции — SR-030. Новые числа помечены preliminary_fields.

Следующие три шага:
1. SR-011b: внести 15 паспортов, DefaultPassportCatalog и реальные legacy/coverage/geometry тесты. Черновой генератор и данные подготовлены в /workspace/scratch/create_passport_data.py и rpg_passports.json; перепроверить, не считать этот scratch готовым артефактом.
2. Прогнать schema/race/vet/build, завершить SR-011, обновить основную карту, журналы, подробный HANDOFF и SHA.
3. SR-012: типизированные контракты навыков и первое закончённое семейство; затем вся библиотека. Caps 100/50 уже реализованы, 6 узлов + усиление 20 согласованы, деревья ещё TODO.

Возобновление:
```bash
cd /workspace/CosmocRPG
git status --short --branch
git log -6 --oneline
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
go version
go test -race -count=1 ./internal/shipcatalog
```
Toolchain Go1.27.3 уже установлен локально с проверкой официального SHA-256; /usr/bin/go здесь другая программа. Scratch может исчезнуть при смене окружения. Для истории прежней сессии см. основную docs/HANDOFF.md и git show 79df286:docs/ships-rpg/HANDOFF.md. Последний SHA кода SR-011a — git log -- passport.go после коммита. Все коммиты локальные; push/PR/deploy не выполнялись. Другие проекты VPS не затрагивать, production без отдельного разрешения запрещён.
