# Общая атомарная покупка — BETA-ECO-001 / AIC-026

Дата: 2026-11-10. Источник: [AI.md §8](../../QUESTJOB/AI.md#8-экономика-антиэксплойты-и-безопасность).
Реализовано и проверено локально, публикация в main разрешена; production/APK не выпущены.

## Контракт и ответственность

- `buy`: resource, amount, operation_id (UTF-8, 1–128 байт). Сервер проверяет док, живого персонажа, товар, количество, цену, вместимость и деньги. Цена берётся сервером. Денежные балансы не передаются клиентом.
- Ключ принадлежит игроку; receipt сравнивает resource+amount. Повтор того же intent возвращает исходный расход и актуальные деньги/груз. Другой intent с тем же ключом — purchase_conflict. Изменение цены не меняет результат уже выполненного intent.
- `purchase_status` принимает те же поля, только читает. Найденный receipt возвращает ok/action=buy; отсутствие — purchase_status/found=false. Не запускает покупку. Старый клиент без ключа получает session-local fallback, поэтому не имеет гарантии retry через reconnect.
- Новый Godot Gateway сохраняет один незавершённый intent на endpoint+пилота в user://; без токенов. Сначала запись, затем отправка. Reconnect автоматически запрашивает только статус. Потерянный ACK не вызывает новую покупку; purchase_unavailable сохраняет intent.
- AI gateway использует action_id как operation_id, отвергает другую привязку. Buy/status разрешены людям и AI через общий обработчик. Repair/sell/ship_upgrade для новых AI по-прежнему закрыты. Gateway/director flags по умолчанию выключены.

## SQL и конкурентность

SQL0025 добавляет players.cargo_revision и resource_purchase_receipts. Старые балансы/XP/груз не меняются, receipt не создаются задним числом. Номер миграции проверяется по свежей main перед публикацией.

quest.PGStore использует тот же players.credits: row lock, проверка receipt отдельным statement после lock (READ COMMITTED barrier), проверка ревизии/вместимости/денег, UPDATE credits+hold+revision и INSERT receipt одной транзакцией. Ошибка INSERT откатывает UPDATE. MemStore повторяет правила на общем wallet lock, без отдельного AI-кошелька.

World резервирует только actor и освобождает общий mutex до IO. Pending/uncertain блокирует его изменяющие команды/добычу, но не урон, чужие движения и снимки. Pending autosave и quest evaluation не ждут SQL под глобальным simulation tick. Cargo revision защищает оплаченный груз от старого autosave; новые mining/sale ревизии сохраняются обычным путём.

Неизвестный COMMIT не объявляется успехом. Обработчик выполняет независимую короткую сверку; при недоступной БД actor остаётся uncertain, cargo не сохраняется поверх неизвестного результата. Status/reconnect читает authoritative cargo с row-lock barrier, восстанавливает подтверждённое состояние, затем снимает запрет. Это гарантия для одного авторитетного World на игрока: горизонтальные независимые in-memory симуляции одним изменением не реализованы. Общий durable world→quest outbox остаётся будущей работой.

## Проверки и результаты

- Полный серверный `go test -race -p 1 -count=1 ./...` с отдельной PostgreSQL16: 519 test/subtest PASS, 3 SKIP (ambient и Godot без env), 24 пакета PASS; последующие изменения проверены затронутыми пакетами отдельно.
- Финальные aiciv/ws с PG+настоящим Godot: 74 test/subtest PASS, 0 SKIP/FAIL. Включены native equipment и purchase online harness.
- Финальные economy/db/quest/world с PG/race PASS после добавления числовых edge cases и legacy SQL fixture.
- Offline purchase_intent_test: persistence, endpoint isolation, retry key, foreign ACK, unknown outcome, reconnect отправляет status и ноль buy.
- PG fault injection: ошибка receipt INSERT после debit (rollback), lost commit acknowledgement, недоступная сверка, concurrent same-key calls, stale worker/save, последующая добыча. Настоящий SQL row lock не останавливает другого пилота/общий tick; buyer получает урон.
- TestShipUpgradeUsesRemainingXP воспроизвёл вторую ошибку: при XP100 покупка уровня списывала1000 без уровня. Исправлена арифметика оставшегося XP (ToNext), green: XP150/уровень2. Это не транзакционная защита upgrade.

Для повторения: Go 1.24+, COSMOC_TEST_DATABASE_URL только отдельной тестовой БД; COSMOC_TEST_GODOT — путь к Godot. На cgroup с 2 CPU: GOMAXPROCS=2, -p 1. Production-БД не использовать. Red-артефакт docs/ai/repro/full_hold_buy_test.go.txt исторический; активный green regression — server/internal/ws/purchase_regression_test.go.

## Следующий агент

1. BETA-ECO-003: общий атомарный ремонт, включая HP/damage/recovery.
2. BETA-ECO-004: общий атомарный ship upgrade, включая XP и wallet.
3. Отдельно проверить sell перед снятием AIC-025; не считать наличие buy receipt доказательством остальных handlers.
4. Затем AIC-030 director/квоты/замена старой FSM, без включения AI-003/011 и отсутствующих подсистем.

Не включать production flags, не назначать недоделанным задачам DONE. Миграция не применяется к production этим этапом.
