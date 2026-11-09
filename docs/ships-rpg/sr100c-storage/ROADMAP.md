# SR-100c: изолированное хранение оборудования

Подзадачи существующей docs/ships-rpg/ROADMAP.md, не новая главная карта.
База: 0b77fea, ветка codex/sr100c-storage; worktree /workspace/worktrees/cosmoc-sr100c-storage.
Kimi занимается ERROR-SR-007 в main. Не менять клиент, рендер, HUD, workflow CI и общие журналы до согласованного объединения. Публикация этой ветки не означает merge/production.

| ID | Приоритет | Зависимости | Описание / критерии DONE | Статус | Даты | Файлы | Тесты | SHA / ERROR |
|---|---|---|---|---|---|---|---|---|
| C-ST-01 | P0 | SR-100b | Дополнительные таблицы owned items/loadouts/slots; стабильный native ID; ownership/unique FK, old tables unchanged | DONE | 2026-11-09 / 2026-11-09 | migration0020; equipment types/store | real PG migration+legacy data+constraints PASS | текущий schema commit / CST-ERROR-001,003 |
| C-ST-02 | P0 | C-ST-01 | PG + memory: Ensure once, Load read-only, Set engine CAS; no reset unequipped, no partial writes | IN_PROGRESS | 2026-11-09 / — | equipment/*.go | backend parity, concurrency, rollback, stale autosave, PG | — / — |
| C-ST-03 | P0 | C-ST-02 | Проверки/документация/feature branch для безопасного merge с Kimi, без включения RPC | TODO | — / — | этот task journal | gofmt/vet/race/build, diff against main | — / — |

После интеграции: coordinator загрузки до world.Join и сохранённый atomic setter с проверкой настоящего дока/боя, затем интерфейс. Пока этот backend не является серверной командой экипировки и не включён в игру.
