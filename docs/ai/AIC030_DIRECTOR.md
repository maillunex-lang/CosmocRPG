# AIC-030 — минимальный CivilizationDirector

## Сделано

- `server/internal/aiciv/director.go`:
  - `NewDirector(res, maxActive, log)`
  - `Tick(ctx, candidates)` — резервирует до `maxActive` незарезервированных капитанов.
  - `Run(ctx, interval, candidatesFn)` — цикл.
  - `ReleaseAll`.
- В `main.go` под флагом `COSMOC_AI_CIVILIZATION_GATEWAY` запускается Director с пустым `candidatesFn` (безопасно, ничего не резервирует, пока не подключён список Studio).
- Старый Manager уже смотрит на `Reservations` и пропускает зарезервированных.

## Что ещё не сделано

- Реальный `candidatesFn` из AI Studio (список captain ID).
- Открытие Gateway на зарезервированного агента и вызов `IssueCombat` / других команд.
- Выбор целей через Utility/GOAP.
- Снимок мира (ближайшие hostile NPC).
- Квоты по регионам и снятие видимых при 0 игроков.
- Восстановление после рестарта.

Флаги по умолчанию выключены. Director не открывает сессий и не делает действий, пока не подключён список и Gateway.
