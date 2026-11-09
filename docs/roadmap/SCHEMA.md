# SCHEMA.md — схема docs/roadmap/tasks.json

```jsonc
{
  "schema": "roadmap-tasks/v1",
  "updated": "ГГГГ-ММ-ДД",
  "last_verified_sha": "короткий SHA main на момент обновления",
  "modules": ["GALAXY", "SHIPS", "COMBAT", "QUESTS", "AI_PLAYERS", "CREW",
              "ECONOMY", "CORPORATIONS", "UI_UX", "CLIENT", "SERVER", "INFRASTRUCTURE", "RPG"],
  "statuses": ["TODO", "IN_PROGRESS", "BLOCKED", "DONE", "FAILED", "DEFERRED"],
  "release_values": ["NOT_RELEASED", "IN_MAIN", "CI_VERIFIED", "STAGING", "PRODUCTION", "APK_RELEASED"],
  "tasks": [ { /* карточка */ } ]
}
```

Карточка задачи (обязательные: id, title, module, priority, status, description, updated):

| Поле | Тип | Описание |
|---|---|---|
| id | string | стабильный ID; существующие R-*/SR-*/QUEST-*/PROTO-*/K-*/E-* не менять |
| title, description | string | кратко и по существу |
| module | string | из списка modules |
| priority | string | P0…P3 |
| status | string | из списка statuses |
| release | object | {server, apk} — независимые каналы, значения из release_values |
| owner | string? | агент или «владелец»; null — свободно |
| branch | string? | рабочая ветка |
| depends_on | string[]? | ID из tasks.json или внешних реестров (ships-rpg, R-индекс) |
| spec | string? | путь к полному ТЗ |
| acceptance | string? | критерии готовности |
| components | string[]? | затрагиваемые компоненты |
| decisions | string[]? | связанные решения (номера из DECISIONS.md) |
| errors | string[]? | связанные ERROR-ID |
| tests | string[]? | тесты-предохранители |
| commits | string[]? | ключевые SHA |

Неизвестные поля не выдумывать. `DONE` без подтверждённых тестов запрещён. Прогресс по задачам ≠ готовность игры.
