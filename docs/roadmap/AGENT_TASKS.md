# AGENT_TASKS.md — координация ИИ-агентов

Markdown — НЕ блокировка. Перед изменениями сверяй `git fetch`, `git log origin/main`, `git status`.
Задача не переходит к другому агенту автоматически по устареванию даты — только по явной передаче.

## Передача задачи (процедура)

1. Текущий владелец пишет в HANDOFF итог и снимает себя (`owner: null`, финальный SHA).
2. Новый владелец подтверждает понимание (запись здесь), делает `git fetch`, проверяет дерево.
3. Снимок чужой незавершённой работы не переписывается: rebase без force, конфликты — вручную.

## Активные работы

| Задача | Агент | Ветка | Область файлов | Состояние | Обновлено | Блокирует | Следующий шаг |
|---|---|---|---|---|---|---|---|
| docs-refactor-v2.0 | Kimi | `docs/roadmap-refactor-v2` | `docs/roadmap/**`, `docs/ROADMAP.md`, `tools/validate_roadmap.py`, `tools/gen_roadmap.py` | IN_PROGRESS | 2026-11-09 | — | интеграция в main после проверок |
| SR-100c/d оснащение | Codex | `codex/equipment` (не main) | `server/internal/{shipcatalog,rpg,world,ws}`, `client/ui/ships` | IN_PROGRESS (в отдельной ветке по решению владельца) | 2026-11-09 | SR-012…070 | сохранение смены оборудования, приёмка |
| PROTO-12/Э-5 двухклиентская приёмка | владелец | — | два живых клиента | ОЖИДАЕТ ВЛАДЕЛЬЦА | 2026-11-09 | PROTO-05B | прогон сквозного сценария Решения №30 |

## Свободные задачи (можно брать)

- `K-1B` клиент корпораций интегрирован в main (экран, меню, профиль). APK не выпускался.
- `E-4` Серверные препятствия (минимум NAV-01).
- `RADIO` М-1 опубликован в main. М-2 (эфир) и М-3 (эффекты) делает ветка `feature/game-sfx`: не переписывать `client/audio/sfx.gd`, `client/audio/sfx_player.gd`, `client/audio/music.gd`, `client/audio/music_player.gd`, `server/internal/media`, `server/internal/radio` и страницу «Музыка» и не забирать их в main без разрешения.
- `FPS-CHECK` Замер на устройстве (только владелец).

## Предупреждения о пересечениях

- Изменения `server/internal/ws/hub.go` и `internal/world/**` затрагивают и Codex (оснащение), и экономику/квесты — перед работой сверяться с `AGENT_TASKS.md` и свежими коммитами.
- Новый тип WebSocket-сообщения: добавить в список известных типов в `hub.go` (иначе `unknown_type`) — см. ERROR_LOG.

## Интеграция PP — 2026-11-09

Владелец разрешил merge и обычный push в main, без production. Интеграционный worktree `/workspace/CosmocRPG-integration`, ветка `integration/pilot-progression-main`; исходная рабочая папка не переключается. PP-02 частично реализован, IN_PROGRESS. Основа оборудования из 0f3ff41 сохранена; прежняя строка активности SR-100c/d не означает отсутствия опубликованной foundation. Следующий агент должен сверить свежую main и журнал EB перед продолжением.

## Закрытый тест — первый сеанс и сохранность

По поручению владельца Codex проверяет серверный сценарий и исправляет потерю частичного прогресса квестов в ветке beta/first-session-reliability, worktree CosmocRPG-beta. Область: internal/quest и регрессионные тесты, docs/beta/FIRST_SESSION.md. PROTO-12 остаётся за владельцем, PROTO-05B не закрывается и не переназначается. Production запрещён.

## AI-CIVILIZATION — текущая работа

Codex, ветка feature/ai-civilization, отдельный worktree CosmocRPG-ai: aiciv foundation, config/main shadow wiring, SQL0022, тесты и docs/ai. Изменений QUESTJOB/AI.md другой сессии не перезаписывать. Runtime world/WS/aiagent остаётся прежним; будущий ActionGateway согласовать с оснащением. AIC-090/110 не выполнять: владелец подтвердил gates. Production этого этапа не разрешён автоматически.

## AIC-020 checkpoint

Codex, feature/ai-civilization, дата2026-11-10: аiciv/bootstrap+gateway+receipts; общие world character rules, серверная маркировка и WS guard; quest EventOnce и SQL0023/0024. Оснащение и его SQL/интерфейс не заменены. Новые миграции перепроверить по свежей main перед merge. Экономический RED BETA-ECO-001 в docs/ai/repro, не копировать в активный suite без начала исправления.

## Публикация AI checkpoint в main

2026-11-10: владелец разрешил main. Отдельный worktree CosmocRPG-ai-main/integration/ai-civilization-main, merge feature0971d8a с origin/main588fde6 без конфликтов, код тождествен проверенному. AIC-020 IN_PROGRESS, economics BLOCKED; production/flags не включены. Для продолжения брать свежую main и учитывать SQL0022/0023/0024.

## BETA-ECO-001 — работа непосредственно в main

2026-11-10, владелец выбрал main и исправление общей покупки. Codex: /workspace/CosmocRPG-main, ветка main; economy/quest purchase storage, world cargo reservation/revision, WS/client operation key, SQL0025 и регрессии. Другие worktree не переключаются. AI economics пока остаётся выключенной, production не трогать.

## Checkpoint атомарной покупки — 2026-11-10

BETA-ECO-001 и AIC-026 завершены: общий buy/status с receipt и восстановлением; BETA-ECO-002 исправляет XP остатка платного уровня. См. [техническую передачу](../ai/ATOMIC_PURCHASE.md). BETA-ECO-003/004 TODO, AIC-025 BLOCKED; ремонт/улучшение/продажа AI закрыты. Codex освобождает область после публикации main; production/APK не выпускались. Предыдущие утверждения «buy запрещён» — история до этого checkpoint.

## K-1B — клиент корпораций, 2026-11-10

Codex, `feature/corporation-client`, отдельный worktree `/workspace/CosmocRPG-review`: DONE локально, код `b9b29e9`, owner снят. Область: новый экран корпорации, клиентский gateway, меню HUD, профиль, клиентские тесты. Серверный world/WS/AI, миграции и оснащение не менять. Владелец выбрал пункт 1 предложенного плана; main и production не публиковать.

Результат K-1B: 36 клиентских проверок PASS, live Godot→WS с двумя пилотами и реальным списанием взноса PASS с race; см. `docs/ui/CORPORATIONS.md`. Серверные production-файлы, AI и миграции не изменялись. Ветка подготовлена к ревью; Android/графическая приёмка не выполнена.

## K-1B — интеграция в main по поручению владельца

2026-11-10: отдельный worktree `/workspace/CosmocRPG-corp-main`, `integration/corporation-main`; база свежей main `78f5c64`, feature `a444891`/код `b9b29e9`. Конфликты документации разрешены сохранением обоих поручений и генерацией сводок из объединённого tasks.json. В test_client.sh сохранены purchase_intent_test и corporation_test, а также оба live сценария. Код атомарной покупки/AI не заменялся.

## MAP-UX-20261110

Codex: `codex/parallel-options`, worktree `/workspace/CosmocRPG-options`. Карта клиента: `client/ui/{map_screen,star_map_screen,map_preferences}.gd`, профильные тесты. Поиск систем, фильтры, локальное избранное, легенда. Корпорации заняты другим ИИ по сообщению владельца; их и AI не менять. IN_PROGRESS.

MAP-UX checkpoint: реализация и профильные headless-проверки завершены (306 PASS), ожидает визуальной приёмки и решения об интеграции. Подробности docs/HANDOFF.md. Серверные области и корпорации не затронуты.

MAP-UX integration: владелец разрешил main. Merge в integration/map-ux-main от origin/main3774fd7; проверена совместимость с клиентом корпораций и purchase intent. Область освобождена; Android-приёмка остаётся.

## М-2 — радиоэфир, 2026-11-10

Та же ветка `feature/game-sfx`, не main. Станция «Голос Рубежа», общее расписание, реклама в окне, заставки, кнопка «Срочный выпуск». Миграция `0028_radio.sql` — сверить номер со свежей main перед любым объединением. События мира выпуск не запускают. В main не вливать без разрешения.

## М-3 — звуки интерфейса и боя, 2026-11-10

Отдельная ветка `feature/game-sfx`, worktree `/workspace/CosmocRPG-sfx`, не main. Звуки: `client/audio/sfx.gd`, `client/audio/sfx_player.gd`. Плеер М-1 и `server/internal/media` этой записью не переписывались. Дальше эта же ветка ведёт квесты и эфир М-2. В main не публиковать, пока владелец не разрешит.

## К-2 и К-3 — Меридиан и журнал локаций, 2026-11-10

Та же ветка `feature/game-sfx`. Владелец 2026-11-10 разрешил влить в main. После пяти шагов Авроры: доставка руды, аномалия и доклад в Меридиане; первое прибытие за сессию пишется в журнал и подсказывает маркер на карте. Дальше: Вора Кесс, обломки Красного Шторма, Кира Роан, старый контур и отчёт Веры Элл. ИИ-игроков, оснащение и эфир М-2 не забирать. APK не выпускать этим слиянием. Миграция `0027_location_visits.sql` идёт вместе с кодом.
