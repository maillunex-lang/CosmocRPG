# Решения ветки SR-100c storage

## CST-ADR-001 — отдельный storage slice (2026-11-09)
Контекст: Kimi работает над ERROR-SR-007 в main; Codex авторизован вести независимый backend этап в feature branch.
Варианты: менять world/WS/client одновременно; сначала подготовить и проверить отдельный store.
Выбор: пакет equipment, новая0020 schema и только wrapper прежней starter identity в новом world файле; не менять существующие world/WS/клиентские файлы, CI, общие roadmap/error/handoff. Ветка от0b77fea в отдельном worktree. До merge общий статус хранить в этой папке; итог перенести ссылкой в главную карту при интеграции.
Последствия: нет функциональной кнопки смены/нового RPC/изменения боевого поведения, production не затронут. Это C-ST-01…03 внутри SR-100c, не весь SR-100c DONE.

## CST-ADR-002 — owned item и loadout revision (2026-11-09)
Контекст: существующий native engine имеет детерминированный owner-bound ID; bootstrap/reconnect не должны создавать новый item или возвращать снятый.
Выбор: module items уникальны по owner/ship/grant_key и instance; loadout по owner/ship; slots по owner/ship/slot и unique instance. Composite FK запрещает чужого owner/ship. Grant и начальная экипировка только при первом создании loadout в одной транзакции. Старый world.starterEngineID переиспользуется через exported wrapper, алгоритм не меняется.
Последствия: в текущем store управляется только bound native Strizh engine; arbitrary prototypes/slots/trade/transfer не принимаются. Таблицы поддерживают несколько slots/items, но этот narrow service отвергает неподдержанную fitted сборку вместо удаления/перезаписи. Общая inventory/каталог/масса/мощность не объявляются реализованными.

## CST-ADR-003 — CAS/atomicity/старый autosave (2026-11-09)
Выбор: SetEngine блокирует loadout row, проверяет expected revision, затем меняет slot и revision одной транзакцией. No-op на актуальной ревизии не повышает её. Старый autosave не пишет эти отдельные таблицы. Load читает item/slot/revision одним SQL statement. Memory backend повторяет те же правила.
Последствия: устаревший клиент должен перечитать state после ErrConflict. Ошибка SQL до commit не оставляет частичного slot/revision; при неоднозначной сетевой ошибке COMMIT будущий coordinator обязан перечитать авторитетный state, не считать автоматически, что commit не случился.
Дальше: перед публичным setter нужны серверная авторизация из session, настоящий dock и combat checks, coordinated world update/rollback, загрузка до выдачи способностей и перенос state между мирами. Store сам не является границей player authorization и не проверяет dock/battle.

## CST-ADR-004 — authoritative bootstrap и runtime transport
По новому разрешению пользователя1→2→3 прежнее ограничение ADR001 на world/WS/client снимается поэтапно; ERROR-SR-007 остаётся Kimi. StarterState alias world.StarterLoadout устраняет import cycle. PG bootstrap до Join: Ensure только когда активный hull Strizh/legacy fallback, для других hull Load без выдачи. Runtime-only Record fields переносят состояние между мирами; старые Save их не пишут. PersistentEquipment закрывает boost при fallback без загруженного state; nil Equipment сохраняет прежние тестовые/legacy harnesses. На live reconnect ресурсы не пересоздаются, старая ревизия не перетирает новую, равная конфликтующая отвергается. Store error не допускает корабль в мир и не заменяет действующую сессию.
