# SR-100c: изолированное хранение оборудования

Подзадачи существующей docs/ships-rpg/ROADMAP.md, не новая главная карта.
База: 0b77fea, ветка codex/sr100c-storage; worktree /workspace/worktrees/cosmoc-sr100c-storage.
Kimi занимается ERROR-SR-007 в main. Не менять клиент, рендер, HUD, workflow CI и общие журналы до согласованного объединения. Публикация этой ветки не означает merge/production.

| ID | Приоритет | Зависимости | Описание / критерии DONE | Статус | Даты | Файлы | Тесты | SHA / ERROR |
|---|---|---|---|---|---|---|---|---|
| C-ST-01 | P0 | SR-100b | Дополнительные таблицы owned items/loadouts/slots; стабильный native ID; ownership/unique FK, old tables unchanged | DONE | 2026-11-09 / 2026-11-09 | migration0020; equipment types/store | real PG migration+legacy data+constraints PASS | 38385f2 / CST-ERROR-001,003 |
| C-ST-02 | P0 | C-ST-01 | PG + memory: Ensure once, Load read-only, Set engine CAS; no reset unequipped, no partial writes | DONE | 2026-11-09 / 2026-11-09 | equipment/*.go | backend parity, concurrency, rollback, stale autosave, PG | 6cec61f / CST-ERROR-004 |
| C-ST-03 | P0 | C-ST-02 | Проверки/документация/feature branch для безопасного merge с Kimi, без включения RPC | DONE | 2026-11-09 / 2026-11-09 | этот task journal | targeted race+PG PASS; vet/build PASS; общий suite: baseline corp FAIL | — / — |

После интеграции: coordinator загрузки до world.Join и сохранённый atomic setter с проверкой настоящего дока/боя, затем интерфейс. Пока этот backend не является серверной командой экипировки и не включён в игру.

## Продолжение по разрешению пользователя: 1 → 2 → 3
| ID | Приоритет | Зависимости | Критерии DONE | Статус | Даты | Файлы / проверки |
|---|---|---|---|---|---|---|
| C-RT-01 | P0 | C-ST-01…03 | Загрузка до Join; reconnect/hull/jump сохраняют снятый двигатель; fail closed | DONE | 2026-11-09 / 2026-11-09 | world/ws/main; real PG + WS |
| C-RT-02 | P0 | C-RT-01 | Session-owned CAS команда только у станции вне боя; rollback/reconcile; UI/reasons | TODO | — / — | world/ws/client; PG/WS/Godot |
| C-RT-03 | P0 | C-RT-02 | Сверка актуального main/Kimi, миграций, тестов, объединение и общие журналы | TODO | — / — | main + roadmap/HANDOFF; CI |
Пользователь разрешил последовательность и последующее объединение. Работа ведётся в feature branch; production не разрешён. ERROR-SR-007 остаётся за Kimi.
