# AIC-030 — минимальный CivilizationDirector

## Сделано

- `server/internal/aiciv/director.go`:
  - `NewDirector(res, maxActive, log)`
  - `Tick(ctx, candidates)` — резервирует до `maxActive` незарезервированных капитанов.
  - `Run(ctx, interval, candidatesFn)` — цикл.
  - `ReleaseAll`.
- В `main.go` под флагом `COSMOC_AI_CIVILIZATION_GATEWAY` запускается Director.
- `candidatesFn` берёт реальные captain ID из активной версии AI Studio (`StudioSource.Profiles`).
- Старый Manager смотрит на `Reservations` и пропускает зарезервированных (не спавнит их как FSM).

## Что ещё не сделано

- Открытие Gateway на зарезервированного агента и вызов `IssueCombat` / других команд.
- Выбор целей через Utility/GOAP.
- Снимок мира (ближайшие hostile NPC).
- Квоты по регионам и снятие видимых при 0 игроков.
- Восстановление после рестарта.
- Резервация пока только предотвращает двойной spawn; сам агент ещё не заходит в мир как player.

Флаги по умолчанию выключены. При включённом gateway Director начнёт резервировать до 8 капитанов из Studio; старый Manager их не спавнит.
