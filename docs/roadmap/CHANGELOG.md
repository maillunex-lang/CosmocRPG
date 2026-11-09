# CHANGELOG.md — изменения системы дорожной карты

## v2.0 (2026-11-09, Kimi Code, ветка docs/roadmap-refactor-v2)

- ROADMAP.md превращён из монолита (~450 строк) в дашборд (~200 строк): паспорт, сводка, приоритеты, направления, блокеры, навигация, чек-лист «Перед публикацией» (сохранён маркер DEFAULT_VISIBLE для теста frame_stats_test.gd).
- Разделы переехали без потерь: требования и решения → `REQUIREMENTS_INDEX.md`; очередь → `NOW.md`; доменные блоки → `modules/*.md` (12 файлов + COMBAT + CREW + RPG).
- Создан машинный реестр `tasks.json` (33 операционные задачи; поля status и release.server/release.apk разделены).
- Добавлены: `AI_START_HERE.md`, `AGENT_TASKS.md`, `BLOCKERS.md`, `HISTORY.md`, `SCHEMA.md`; `STATUS.md` и `DEPENDENCIES.md` генерируются.
- Инструменты: `tools/validate_roadmap.py` (проверки, read-only), `tools/gen_roadmap.py` (сводки из tasks.json, единый источник).
- Специализированные документы (ships-rpg/, QUESTJOB/, ui/, design/) не переписывались — остаются подробными источниками, синхронизация через ID во внешних реестрах валидатора.
