# Передача Ships RPG v2.1
Дата: 2026-11-09. Ветка: `codex/ships-rpg-foundation`. Исходная контрольная точка этой задачи: `79df286`.

## Текущая стадия
**SR-011 DONE**: полный проверяемый каталог 15 моделей / 12 классов / 30 специализаций. SR-011a (контракт/schema/валидатор) — `6a657db`. SR-011b (авторские данные и реальные compatibility/coverage-тесты) — `699e8b5` (последний SHA реализации). Финальный docs-коммит с этой передачей смотреть через `git log -1 --oneline`. Весь этап 1 **IN_PROGRESS**, SR-012/013/014 остаются TODO. Прежние SR-000/010/015/014a DONE; caps 100/50 уже реализованы и подтверждены владельцем, шесть узлов + усиление ульты на 20 согласованы, но деревья ещё не созданы.

## Что реализовано
- `server/internal/shipcatalog/passport.go`: PassportCatalog/ShipPassport, compiler JSON Schema с существующей SchemaJSON для base, семантические проверки ID/спек/слотов/категорий/размеров/бюджетов/старта/геометрии; DefaultPassportCatalog; полное сравнение старых base через CheckLegacyPassportBases.
- `data/rpg_passports.schema.json`: все поля полного паспорта обязательны; unknown fields отклоняются; диапазоны 72 skill ID, размеры S/M/L/XL, группы слотов, предварительность новых групп полей и новых base. Регистрирует внешний `ship-patch-v1.json` из SchemaJSON(), поэтому базовые диапазоны не дублируются.
- `data/rpg_passports.json`: все 15 паспортов. base 12 старых — полный неизменённый снимок ShipSpec, новые три base — предварительные. Всё новое явно помечено preliminary_fields.
- `passport_test.go`: synthetic negative/schema/roundtrip/concurrent tests; `passport_data_test.go`: реальные 15/12/30, полное legacy equality, 15 разных силуэтов, все weapon/engine mounts, hull permissions для всех 72 ID, нет неявной публикации, нет shared mutable state, точные бюджеты и их независимость от combat energy.
- `docs/ships-rpg/PASSPORTS.md`: человекочитаемая таблица, единицы, предварительность, расчёты/ограничения; SR-ADR-003 описывает архитектуру. Обновлены главные ROADMAP/HANDOFF и профильные AUDIT/ROADMAP/DECISIONS/ERROR_LOG/REGRESSION_TESTS/HANDOFF.

## Реальные проверки
- Полный `go test -race -count=1 -json ./...` — exit 0: 20 пакетов PASS, 304 test/subtest PASS, 62 SKIP, 0 FAIL; 3 пакета без тестов.
- После финального усиления required base schema и добавления отрицательного теста роли: `go test -race -count=1 -json ./internal/shipcatalog` — exit 0, 8.392s, 68 PASS, 1 SKIP.
- `go vet ./...`, `go build -o /workspace/scratch/cosmoc-server-sr011 ./cmd/server`, gofmt, git diff --check — PASS (exit 0), включая повтор после усиления schema. Бинарник не запускался.
- Тестовая PostgreSQL URL не настроена: DB cases пропущены. Godot/Android/production не проверялись: код БД/клиента не менялся.
- ERROR-SR-003: документ корректно отклонялся schema uniqueItems, но тест ошибочно ожидал текст Go-проверки. Исправлено в 6a657db. Все ERROR-SR-001/002/003 Resolved, открытых ошибок на этой точке нет. Не стирать исторические неудачные прогоны.

## Границы и важные решения
Каталог v2 пока не подключён к world/Service/Store/клиенту. Runtime по-прежнему 12 моделей; три новых корабля ещё нельзя выбрать в игре. Не публиковать документ через старую структуру world.Doc: новые поля потеряются. Активные админские версии PostgreSQL могут отличаться от seed; будущая миграция должна сохранить их значения, не заменить встроенным snapshot.

base включает прежние local specs, а specializations — новые global IDs: например, Ястреб пока хранит interceptor/scout в base, целевые ветки hunter/assault в новом поле. Это намеренное сохранение совместимости, не уже выполненная миграция. pulse остаётся автоатакой и не становится W01 автоматически. boost/intercept/scan при будущих alias/migration должны сохранить per-ship параметры.

Стартовые модули — self-contained **предлагаемые blueprints**, не предметы игрока и не финальный общий каталог. Семейства — классификация; в SR-013 вынести в общий модульный справочник, добавить полные ограничения/редкости и доказать все unlock-пути. Текущая проверка hull permissions всех 72 ID не доказывает доступность их через модули/изучение. В SR-030 нужны владение, сохранённые билды и атомарные транзакции.

Новые сопротивления и стартовые модули не меняют старые боевые статы; не складывать их повторно с base. MW размещения отдельно от энергии. dimensions_m = [длина,ширина,высота], visual [X,Y,Z] метры, -Z вперёд/+Y вверх, collision box полуразмеры [ширина/2,высота/2,длина/2]. Игровые скорость/ускорение/дальность остаются в world units. Placeholder IDs/силуэты/крепления — данные; сами процедурные meshes относятся к SR-050, GLB Стрижа сохранены.

## Следующие три шага
1. SR-012: декомпозировать контракт 72 навыков/30 ульт на малые проверяемые семейства, сначала IN_PROGRESS; ввести строгие типы целей/эффектов, JSON Schema, refs и предварительные формулы. Не объявлять библиотеку готовой по одним ID.
2. Реализовать первое закончённое семейство данных и тесты с per-rank параметрами/развилками/PvP; сохранить старые pulse/boost/intercept/scan через явные compatibility tests. Runtime effect registry — отдельный SR-020, не исполнение строк JSON.
3. SR-013: общий каталог модулей/36 пассивов с rpg.Compute, полный граф module → abilities и достижимость всех 72. Затем SR-014: шесть узлов 1/4/7/10/14/18 + усиление ульты 20.

## Возобновление
```bash
cd /workspace/CosmocRPG
git status --short --branch
git log -8 --oneline
cat docs/ships-rpg/ROADMAP.md docs/ships-rpg/ERROR_LOG.md docs/ships-rpg/HANDOFF.md
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
go version
go test -race -count=1 ./internal/shipcatalog
go vet ./...
```
Go1.27.3 установлен локально с проверкой официального SHA-256; go.mod/CI не менялись. /usr/bin/go — другая программа, при потере scratch установить официальный toolchain заново, сперва проверив версию/хеш.

Вне Git: toolchain, генератор черновых данных (итоговый JSON — авторитетный артефакт), логи `ships-rpg-passport-contract*.log`, `ships-rpg-passports-data-tests.jsonl`, `ships-rpg-sr011-all-tests.jsonl`, `ships-rpg-passports-final-tests.jsonl` в /workspace/scratch. Они могут исчезнуть при смене окружения; сводка сохранена в REGRESSION_TESTS. Ни серверов, ни контейнеров эта задача не запускала. Коммиты локальные, push/PR/deploy не выполнялись. **Другие проекты VPS не трогать; production-деплой без отдельного разрешения запрещён.**

Рабочее дерево после финального docs-коммита должно быть чистым. Для review: `git diff 79df286..699e8b5 --stat`; сначала читать PASSPORTS.md (сводная таблица и ограничения), затем passport.go/schema и тесты. Документ содержит все поля §5; готовность SR-011 не означает готовность всего этапа 1 или runtime-интеграции.
