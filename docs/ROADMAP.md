# Дорожная карта «Звёздных рубежей» — дашборд

> Монолит ушёл в прошлое. Это стартовая панель; полные данные — в `roadmap/`. Начало работы для ИИ: `roadmap/AI_START_HERE.md`.

## Паспорт проекта

| | |
|---|---|
| Название | Звёздные рубежи (CosmocRPG) — мобильная космическая MMO |
| Стадия | Альфа с друзьями: прототип (Решение №30) в основном выполнен, идёт наращивание механик |
| Ветка | `main` (релизы, координация в `roadmap/AGENT_TASKS.md`) |
| Обновлено | 2026-11-09 |
| Проверенный SHA | `829c36e` (см. `tasks.json → last_verified_sha`) |
| Руководство для ИИ | `docs/AI_HANDOFF_PROMPT.md` + `roadmap/AI_START_HERE.md` |
| Клиент / сервер | APK **0.3.12 (15)** опубликован; сервер production = main 0.3.12 (выкачан 2026-11-09) |

## Сводка прогресса

<!--GEN:SUMMARY-->
Всего задач в реестре: **41** · DONE **14** · IN_PROGRESS **1** · BLOCKED **1** · TODO **24** · DEFERRED **1** · FAILED **0** · ожидают решения владельца: **6** (см. `roadmap/BLOCKERS.md`)
<!--/GEN-->

Правило подсчёта: прогресс = доля DONE-задач в `tasks.json`. Это **не** готовность игры — см. `roadmap/STATUS.md`.

## Ближайшие приоритеты (NOW)

| ID | Приоритет | Задача | Ответственный | Статус | Подробности |
|---|---|---|---|---|---|
| PROTO-12 | P1 | Двухклиентская приёмка прототипа | владелец | IN_PROGRESS | `tasks.json`, Э-5 |
| PROTO-05B | P1 | Сквозная приёмка + контрольная точка | Kimi | TODO | после PROTO-12 |
| K-1B | P1 | Корпорации: клиент | свободен | TODO | `modules/CORPORATIONS.md` |
| K-2 | P1 | Квест deliver + глава 1 в Меридиане | свободен | TODO | `modules/QUESTS.md` |
| SR-100C | P0 | Оснащение: сохранение и смена в доке | Codex | IN_PROGRESS | `docs/ships-rpg/ROADMAP.md` |
| SR-100D | P0 | Приёмка оснащения + release | Codex | TODO | после SR-100C |
| E-4 | P2 | Серверные препятствия (минимум NAV-01) | свободен | TODO | `modules/GALAXY.md` |
| RADIO | P1 | Радио: плеер и библиотека | свободен | TODO | `modules/CLIENT.md` |

Полная очередь: `roadmap/NOW.md`. Свободные задачи: `roadmap/AGENT_TASKS.md`.

## Направления разработки

| Направление | Прогресс (задачи) | Статус | Подробности |
|---|---|---|---|
| Галактика (системы, локации) | 3/4 | активно | `modules/GALAXY.md` |
| Корабли и оснащение | 1/10 | активно (Codex) | `modules/SHIPS.md` → `docs/ships-rpg/` |
| Бой | 1/1 | в основном готово | `modules/COMBAT.md` |
| Квесты и сюжет | 1/3 | активно | `modules/QUESTS.md` |
| ИИ-игроки | 1/1 | готово (мониторинг) | `modules/AI_PLAYERS.md` |
| Экономика | 2/2 | готово (расширение в бэклоге) | `modules/ECONOMY.md` |
| Корпорации | 1/2 | активно | `modules/CORPORATIONS.md` |
| RPG-фундамент | 1/1 | фундамент готов, этапы по плану | `modules/RPG.md` |
| Интерфейс | 1/1 | готово (итерации по запросам) | `modules/UI_UX.md` |
| Клиент и поставка | 2/3 | активно | `modules/CLIENT.md` |
| Сервер и аккаунты | 1/2 | стабильно | `modules/SERVER.md` |
| Инфраструктура | 2/3 | стабильно | `modules/INFRASTRUCTURE.md` |
| Экипаж (CREW) | 0/0 | будущие этапы | `modules/CREW.md` |

## Блокирующие проблемы

6 вопросов ждут решения владельца + 1 мониторинг — список: `roadmap/BLOCKERS.md`. Критичных технических блокеров нет.

## Навигация

- Для ИИ: `roadmap/AI_START_HERE.md` — маршруты чтения под тип задачи.
- Очередь: `roadmap/NOW.md` · Статус: `roadmap/STATUS.md` (генер.) · Задачи: `roadmap/tasks.json` · Зависимости: `roadmap/DEPENDENCIES.md` (генер.)
- Требования и решения: `roadmap/REQUIREMENTS_INDEX.md` · Решения владельца: `docs/DECISIONS.md`
- Модули: `roadmap/modules/` (12 направлений) · Ошибки: `docs/ships-rpg/ERROR_LOG.md` · Проверки: `docs/ships-rpg/REGRESSION_TESTS.md`
- История: `docs/HANDOFF.md` · Индекс истории: `roadmap/HISTORY.md` · Журнал системы карты: `roadmap/CHANGELOG.md`
- Схема данных: `roadmap/SCHEMA.md` · Валидация: `python3 tools/validate_roadmap.py` · Генерация: `python3 tools/gen_roadmap.py`
- Операции: `docs/DEPLOYMENT.md` · Релиз: `docs/ships-rpg/RELEASE_PROCESS.md` · Безопасность: `docs/SECURITY.md`

## Перед публикацией (чек-лист)

- [ ] Отключить панель производительности по умолчанию (`DEFAULT_VISIBLE := false` в `client/ui/debug/frame_stats.gd`)
- [ ] Убрать демо-режим из пользовательских сборок
- [ ] Проверить права и маркировку ИИ-контента (docs/AI_CONTENT.md), правила RuStore
- [ ] Резервные копии release-ключа Android, БД и исходных ассетов вне VPS
- [ ] Вход в админку в настоящем браузере проверен владельцем
- [ ] Внешние бэкапы ключей и БД у владельца (вне VPS) — открытый вопрос Q-5

## Правило владельца (кратко)

Каждый новый промпт сначала фиксируется в реестре (`roadmap/REQUIREMENTS_INDEX.md`, R-NN). Противоречие — вопрос владельцу, агент не решает сам. Полная формулировка — в начале `REQUIREMENTS_INDEX.md`.
