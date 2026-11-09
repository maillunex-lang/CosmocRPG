# Ошибки SR-100c storage
Отдельный журнал ветки для снижения конфликтов с работой Kimi. При merge перенести ссылки/итог в общий ERROR_LOG.
Фактических ошибок на старте нет. Известный ERROR-SR-007 принадлежит Kimi, его статус здесь не изменять.

## CST-ERROR-001
Дата2026-11-09. Задача C-ST-01. Компонент migration numbering. Серьёзность Medium. Статус Resolved (повторный PG прогон должен подтвердить остальные проверки).
Воспроизведение: первый реальный `go test -race ./internal/equipment` с новой миграцией0019_equipment_storage.sql. Ожидалось применение новой schema, фактически все PG cases FAIL до выполнения SQL: `дублирующийся номер миграции19`. Memory cases PASS.
Доказанная причина: база ветки0b77fea уже содержит0019_corporations.sql, а первоначальная сверка списка выполнялась в прежнем main checkout3914404 и пропустила эту миграцию.
Исправление: только наша ещё не применённая миграция переименована в0020_equipment_storage.sql, обновлён migration-test и план. Чужая0019_corporations.sql не менялась. Никаких repair/delete production операций не было. Неудачная попытка: выбор0019 до проверки полного списка в новой рабочей копии. Проверка: повторный real PostgreSQL migration/store suite. Коммит будет записан после фиксации. Профилактика: проверять номера в конкретном worktree и повторно перед merge; не перенумеровывать уже применённые опубликованные миграции.

## CST-ERROR-002
Дата2026-11-09. Задача C-ST-02, тест ограничений. Серьёзность Low. Статус Resolved (повторная проверка ниже).
Первый SQL suite после исправления номера ожидал23503(FK) при попытке чужой экипировки, но получил23505(unique instance). Причина: чужой предмет уже был установлен, и unique-индекс корректно срабатывал раньше ownership FK. Исправление: до попыток theft/cross-hull снять именно Alice starter; Bob оставить установленным для отдельной проверки double fit. Файл equipment/pg_test.go. Это уточнение fixture, не ослабление SQL ограничений.

## CST-ERROR-003
Дата2026-11-09. Задача C-ST-01, FK deletion lifecycle. Серьёзность Medium. Статус Resolved.
Воспроизведение: TestPGDeletingOwnerCascadesEquipmentOnly, DELETE owner в тестовой схеме. Ожидалось каскадное удаление принадлежащих ему loadout/item/slots без влияния на другого owner. Фактически23503: slot→item FK проверялся до завершения обеих owner cascade ветвей.
Причина доказана SQL/test: две родительские таблицы каскадируют от players, а FK slot→item был immediate NO ACTION. Исправление в draft0020: сделать этот composite FK DEFERRABLE INITIALLY DEFERRED. Ownership/ship-связь сохраняется и проверяется при commit, прямое удаление fitted item без удаления slot по-прежнему отклоняется. Остальные UNIQUE не ослаблены. Не использовать удаление чужих таблиц/ручной repair. Файлы migration0020/equipment/pg_test.go; регрессия owner cascade + raw theft/double fit + transaction rollback. Коммит после проверки.

Проверка всех CST-ERROR-001/002/003: повторный `go test -race -count=1 -json ./internal/equipment ./internal/db` на реальном PostgreSQL16 —23 PASS,0 SKIP/FAIL. Включены owner cascade, запрет прямого удаления fitted item, theft FK отдельно от unique double-fit, rollback после промежуточного DELETE. Исправления миграции применялись только в новых временных testdb-схемах, production не затрагивался.

## CST-ERROR-004
Дата2026-11-09. Компонент существующий corp/pgstore_test.go:27. Статус OPEN, вне области equipment. Общий Go+PG прогон: TestPGStoreCorporationFlow падает с SQLSTATE42P08 inconsistent types deduced for parameter$1. Fixture использует один$1 для public_id и token_hash разных SQL типов. Подтверждено на чистом git archive0b77fea без equipment изменений: `go test -count=1 -run '^TestPGStoreCorporationFlow$' ./internal/corp` с тем же отдельным PostgreSQL также FAIL. Чужой corp-код не изменялся. Следующему ответственному разделить параметры fixture и проверить дальнейшие assertions. Не относится к ERROR-SR-007 Kimi.
Schema исправления CST-ERROR-001/003 зафиксированы38385f2; исправленная fixture CST-ERROR-002 входит в следующий equipment store commit.
