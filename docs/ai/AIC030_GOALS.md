# AIC-030 — цели по типажу и классу

## Выбор цели

По профессии из AI Studio (`aiagent.BehaviorFor`):

| Поведение | Профессии (примеры) | Цель | Цепочка |
|------------|----------------------|------|--------|
| patrol | наёмник, страж, охотник | `farm_credits` | find → approach → attack → (retreat) |
| miner | шахтёр, сборщик утиля | `mine_and_sell` | find deposit → approach → mine → sell |
| trader | торговец, грузоперевозчик | `trade_run` | пока hold у станции (цепочка в разработке) |
| scout | разведчик, исследователь | `explore` | find point → approach → next |
| support / другие | дипломат, медик, неизвестные | `idle_dock` | стоит у станции, не бродит |

## Код

- `goals_by_type.go` — `GoalForProfession` / `GoalForProfile`.
- `goals.go` — цепочки для боя, добычи, исследования; остальные держат позицию.
- `economy.go` — `IssueMine` / `IssueMineStop` / `IssueSell`.
- В `main.go` цель берётся из профиля первого капитана Studio.

Агент не летает без цели. При низком корпусе любая цепочка прерывается отступлением.
