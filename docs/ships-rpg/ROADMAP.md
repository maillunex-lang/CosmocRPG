# Дорожная карта Ships RPG v2.1

Подчинена [главной карте](../ROADMAP.md), источник R-62. Дата: 2026-11-09. DONE здесь означает локально проверенные критерии, не production. Этап 1 разбит на SR-010…014; сам этап остаётся IN_PROGRESS до завершения всех блоков.

| ID | Приоритет | Зависимости | Описание | Критерии DONE | Статус | Дата начала/окончания | Файлы | Проверки | Коммит | ERROR-ID |
|---|---|---|---|---|---|---|---|---|---|---|
| SR-000 | P0 | — | Аудит, главная карта, журналы, ADR | Аудит перечисляет реализацию/риски/пути; ссылки и журналы существуют | DONE | 2026-11-09 / 2026-11-09 | docs/ships-rpg/*.md; docs/ROADMAP.md; docs/HANDOFF.md | Чтение кода/CI; git diff --check | 8c7b620 | ERROR-SR-001 |
| SR-010 | P0 | SR-000 | Реестр идентичностей + JSON Schema | 15 моделей/12 классов/30 специализаций; ссылки, уникальность, старые ID проверены | DONE | 2026-11-09 / 2026-11-09 | shipcatalog/data; identity*.go | Schema + negative + legacy tests | 8ce0106 | — |
| SR-011 | P0 | SR-010 | Полные паспорта и slot rules | Все поля §5, старые числа сохранены; новые предварительные | DONE | 2026-11-09 / 2026-11-09 | shipcatalog/passport*.go; data/rpg_passports*.json; PASSPORTS.md | schema/roundtrip/refs/legacy; полный race + vet/build | 6a657db + 699e8b5 | ERROR-SR-003 |
| SR-012 | P0 | SR-010 | Контракты эффектов/умений + 72 навыка/30 ульт | Полные §7 формулы, ранги, ветвления, цели, VFX; 102 уникальные механики/контракты проверены | TODO | — / — | shipcatalog; schemas | Счётчики, refs, negative, PvP coefficients | — | — |
| SR-013 | P0 | SR-011; SR-012 | 36 пассивов и стартовые модули | Модификаторы rpg.Compute; все 72 навыка достижимы через модули/спеки | TODO | — / — | rpg; shipcatalog | Caps, источники, достижимость | — | — |
| SR-014 | P1 | SR-010; SR-012 | Деревья специализаций и их прогрессия | 30 деревьев: 6 узлов 1/4/7/10/14/18, альтернативы, усиление ульты 20; данные/ранги проверены | TODO | — / — | rpg; pilot; shipxp | Совместимость XP, узлы/ранги | — | — |
| SR-020 | P0 | SR-011; SR-012 | Этап 2: effect registry и совместимость | Все effect types §11, атомарность, legacy pulse/boost/intercept/scan | TODO | — / — | world; rpg | Ошибки без списания; caps/стек; legacy | — | ERROR-SR-002 |
| SR-030 | P1 | SR-013; SR-020 | Этап 3: предметы, экипировка и три билда | Масса/мощность/слоты/владение; смена в доке; транзакция без дюпа | TODO | — / — | shipcatalog; ws; db | Rollback, concurrent, stale build | — | — |
| SR-040 | P1 | SR-014; SR-030 | Этап 4: XP, исследования, таланты, БД | Добавочные миграции; старый XP сохранён; сброс идемпотентен | TODO | — / — | rpg; pilot; shipxp; db | Копия/синтетическая БД, reset, idempotency | — | — |
| SR-050 | P1 | SR-030; SR-040 | Этап 5: ангар/HUD/процедурные силуэты | 4+ульта+автоатака; причины недоступности; независимые visuals | TODO | — / — | client/ui/ships; hud; gateway | Godot import/UI/visual regression | — | — |
| SR-060 | P1 | SR-020; SR-040 | Этап 6: NPC/PvE/PvP/экономика | Общие серверные правила, антифарм заряда, отдельные PvP-коэффициенты | TODO | — / — | world; economy | NPC parity, PvP protection, антифарм | — | — |
| SR-070 | P1 | SR-050; SR-060 | Этап 7: регрессия и нагрузка | Каталоги 15/12/30/72/30/36; старые сейвы; race/DB/Godot/нагрузка | TODO | — / — | tests; CI; docs | Полный CI + нагрузочный отчёт | — | — |

## Независимая подготовка совместимости

| ID | Приоритет | Зависимости | Описание | Критерии DONE | Статус | Дата | Файлы | Тесты | Коммит | ERROR-ID |
|---|---|---|---|---|---|---|---|---|---|---|
| SR-015 | P0 | SR-000 | Отклонять неизвестный runtime ability до effect registry | ErrUnknown, без списания энергии/CD/эффектов; legacy работают | DONE | 2026-11-09 / 2026-11-09 | world/combat.go; combat_test.go | Новый отрицательный тест + полный server race | 0e7cbb3 | ERROR-SR-002 |

SR-015 — минимальное исправление найденной в аудите ошибки; не заменяет SR-020 и не включает новые эффекты.

## Согласованная прогрессия

| ID | Приоритет | Зависимости | Описание | Критерии DONE | Статус | Дата | Файлы | Тесты | Коммит | ERROR-ID |
|---|---|---|---|---|---|---|---|---|---|---|
| SR-014a | P0 | SR-000; подтверждение владельца 2026-11-09 | Caps пилота 100 и корабля 50 без изменения XP | Старые пороги XP совпадают, прежние cap продолжаются, новый cap и clamp работают | DONE | 2026-11-09 / 2026-11-09 | rpg/progression.go; pilot/shipxp tests; world shiplevel tests | race rpg/pilot/shipxp/world/ws + vet/build | 788d951 | — |

SR-014a не закрывает SR-014: данные/ранги/таланты специализаций ещё не реализованы.

## Декомпозиция SR-011

| ID | Приоритет | Зависимости | Описание / критерии | Статус | Даты | Файлы | Тесты | SHA / ERROR |
|---|---|---|---|---|---|---|---|---|
| SR-011a | P0 | SR-010 | Типы паспортов, schema, ссылки, бюджеты и стартовые комплекты; отрицательные тесты | DONE | 2026-11-09 / 2026-11-09 | shipcatalog/passport*.go; data/rpg_passports.schema.json | Schema/семантика/roundtrip; race PASS 7.007s | 6a657db / ERROR-SR-003 |
| SR-011b | P0 | SR-011a | 15 паспортов, сохранение 12 legacy ShipSpec, новые числа предварительные, спецификация единиц | DONE | 2026-11-09 / 2026-11-09 | shipcatalog/data/rpg_passports.json; PASSPORTS.md | 15/12/30, legacy equality, комплекты; race PASS | 699e8b5 / — |

## R-63: сквозная интеграция в игру (приоритет над прежним линейным порядком)
Дата: 2026-11-09. Пользователь подтвердил поэтапное подключение и публикацию прогресса в main. SR-012…070 сохраняют требования, но более не блокируют первый узкий сценарий.

| ID | Приоритет | Зависимости | Результат / критерии DONE | Статус | Даты | Файлы | Проверки | SHA / ERROR |
|---|---|---|---|---|---|---|---|---|
| SR-090 | P0 | SR-011 | Опубликовать накопленные проверенные изменения и новый порядок в main; изучить APK Claude | DONE | 2026-11-09 / 2026-11-09 | docs; существующая ветка | main fast-forward push PASS | 75931d4 / — |
| SR-100a | P0 | SR-011 | Серверный паспорт Стрижа в ангаре; активные base/stats не подменяются, старые клиенты совместимы | DONE | 2026-11-09 / 2026-11-09 | shipcatalog; ws; gateway; hangar | live Go WS + Godot PASS; полный race PASS; hangar28/0 | cb34353 / ERROR-SR-004 |
| SR-100b | P0 | SR-100a | Один стартовый модуль → один навык (ENG/M01); сервер проверяет доступность, энергия/CD legacy сохранены | DONE | 2026-11-09 / 2026-11-09 | world; ws; shipcatalog | negative atomicity; shared alias/CD; live client cast; full race322/0 | 21aaa0b + d465afe / — |
| SR-100c | P0 | SR-100b | Сохранение/повторный вход, смена только в доке вне боя, ангар/HUD и причины недоступности | DONE | 2026-11-09 / 2026-11-09 | db; world; ws; client | full Go+PG+Godot431 PASS; final affected157 PASS; main published, remote CI pending | d9669c9 +53d08c3 / ERROR-SR-009,010 Resolved |
| SR-100d | P0 | SR-100a…c | Игровая приёмка первого сценария, release APK при клиентских изменениях, отдельный серверный выпуск | TODO | — / — | tests; release docs | Сквозной клиент+сервер, прежняя подпись APK, signed manifest; prod по разрешению | — / — |

После этого расширять модули/умения/корабли по тем же проверенным путям, параллельно заполняя SR-012/013. Четыре свободных слота, ульта и остальные требования не отменены. Не выдавать изолированный паспорт за законченный модульный сценарий.

| SR-091 | P0 | SR-090 | Восстановить PostgreSQL CI: устаревшее ожидание пустого журнала после первого задания; только тест, игровая логика не меняется | DONE | 2026-11-09 / 2026-11-09 | quest/quest_test.go | remote Go/PG CI success run42606462785 | 88a060c / ERROR-SR-005 |

SR-100a DONE означает код и локальные сквозные тесты; CI следующего push проверяется отдельно. Production сервер/APK не выпущены. Для нового клиента потребуется прежнее Android release-окружение: SDK/AAR и ключи подписи; это ограничение выпуска, не готовый APK. SR-100b/c остаются TODO, старые навыки ещё не зависят от новых blueprints.

| SR-093 | P0 | SR-090 | Восстановить тот же public update key в fresh clone и сделать его обязательным тестом | DONE | 2026-11-09 / 2026-11-09 | .gitignore; assets/update_public.pem; updater_test.gd | updater33/0; public manifest Verified OK | ae16a86 / ERROR-SR-006 |

### Контрольная точка main (после SR-100a)
- Игровой код cb34353: весь CI run42606462785 success, включая PostgreSQL и новый live Godot scenario.
- Релизная воспроизводимость ae16a86: тот же публичный PEM в git, updater33/0, живой signed manifest проверен.
- Открыт ERROR-SR-007: прежние runtime ошибки render-тестов при зелёном CI; baseline подтверждён, исследовать в SR-100d/SR-070.
- Production/APK не выпущены, модульная доступность/сохранение ещё TODO. Следующий шаг SR-100b.

## SR-100b: этапы стартового двигателя
| ID | Приоритет | Зависимости | Критерии DONE | Статус | Даты | Файлы | Проверки | SHA / ERROR |
|---|---|---|---|---|---|---|---|---|
| SR-100b1 | P0 | SR-100a | Привязанный starter instance ENG, M01/boost единый ресурсный путь, запрет без ownership/equipment/rank, прочие hulls без миграции | DONE | 2026-11-09 / 2026-11-09 | world/starter_engine*.go; combat/ship/world; ws; passport_data_test | race world/shipcatalog; live WS alias, negative atomicity | 21aaa0b / — |
| SR-100b2 | P0 | SR-100b1 | Client gateway/HUD/ангар показывают starter и availability, вызывают M01; старый payload совместим | DONE | 2026-11-09 / 2026-11-09 | client gateway/hud/hangar/tests | hangar33/0; HUD161/0; real client cast PASS | d465afe / — |

Фиксированный starter entitlement штатного двигателя не является свободным инвентарём. В SR-100b нет RPC снятия/установки: сохранённая изменяемая экипировка и условия дока/боя — SR-100c. Отрицательные тесты отсутствия модуля проверяют серверную целостность/доступность, не кнопку, которой ещё нет.

Публикация SR-100b: main c9a3391 (merge после параллельного landing commit545032a); без force, чужой site/index.html сохранён. CI https://github.com/maillunex-lang/CosmocRPG/actions/runs/42616666345 проверяет этот head; все4 jobs success.

| SR-100b3 | P1 | SR-100b2 | Уведомление переживает закрытие экрана без freed lambda ERROR; live harness отвергает raw ERROR при exit0 | DONE | 2026-11-09 / 2026-11-09 | alert_bar.gd; passport_online_test.gd; ws/passport_test.go | red→green real-client race9.034s, HUD161/0 без ERROR | a1c5c9e / ERROR-SR-008 |

CI c9a3391 для полного gameplay шага — success. Дополнительный lifecycle fix проверен локально отдельно и вызывает новый CI после push; Vulkan ERROR-SR-007 остаётся открытым.

## SR-100c: сохранённый штатный двигатель, порядок1→2→3
Координация обновлена пользователем: Kimi делает ERROR-SR-007 в main; Codex ведёт отдельную ветку и по разрешению объединяет этот этап. Исторические записи TODO/«без RPC» выше относятся к SR-100b и предыдущим checkpoints.

| ID | Приоритет | Зависимости | Критерии DONE | Статус | Даты | Файлы / проверки | SHA / ERROR |
|---|---|---|---|---|---|---|---|
| SR-100c-ST | P0 | SR-100b | Owned item/loadout/slot schema, idempotent grant, CAS/rollback, old XP/save intact | DONE | 2026-11-09 / 2026-11-09 | migration0020; equipment;23 realPG targeted PASS | 38385f2/6cec61f / CST-ERROR-001…003 Resolved |
| SR-100c-1 | P0 | SR-100c-ST | Загрузка до Join, fail closed, live reconnect/restart/hull/jump без reset | DONE | 2026-11-09 / 2026-11-09 | main/world/ws; realPG+race PASS | 01bc109 / — |
| SR-100c-2 | P0 | SR-100c-1 | Session-owned dock-only CAS setter; combat/death/movement; reconcile; ангар/HUD reasons | DONE | 2026-11-09 / 2026-11-09 | world/ws/client; race+realPG; hangar39/0 HUD161/0 livePGGodot PASS | ac029a8/7407d43 / — |
| SR-100c-3 | P0 | SR-100c-2 | Latest main/migrations checked, full checks, shared roadmap/HANDOFF, non-force publication | DONE | 2026-11-09 / 2026-11-09 | latest main merged; local checks PASS; non-force push d9669c9; remote CI pending | d9669c9 / ERROR-SR-009,010 Resolved |

Точный [task journal](sr100c-storage/ROADMAP.md), [ошибки](sr100c-storage/ERROR_LOG.md), [тесты](sr100c-storage/REGRESSION_TESTS.md), [решения](sr100c-storage/DECISIONS.md). Не объявлять SR-030/весь master prompt DONE: arbitrary modules/билды/budgets и72/30/36 ещё впереди. SR-100d — Android/graphical/release приёмка; production не развёрнут. В main должен попасть код и документы, не секреты/новый release key.

Публикация подтверждена: main85abfdd→d9669c9, без force. Первая попытка non-FF была корректно отклонена из-за чужого FinalQuest commit85abfdd; обычный merge сохранил его. Последние remote merges затрагивали только QUESTJOB: сохранено и чужое удаление NEW_STORY_CAMPAIGN.md(26300aa), файл не восстанавливался самовольно. Код не менялся после final affected157 PASS/vet/build. CI code head: [42650859949](https://github.com/maillunex-lang/CosmocRPG/actions/runs/42650859949), pending на момент проверки; финальный docs push/чужие docs могут отменить этот run — проверять последний [main CI](https://github.com/maillunex-lang/CosmocRPG/actions?query=branch%3Amain). DONE относится к проверенной реализации/интеграции; remote CI не объявлен success, SR-100d остаётся TODO.

## SR-100c-4: неблокирующее сохранение (ветка codex/sr100c-coordinator)
Пользователь подтвердил этот следующий этап. Базаb945efb, отдельный worktree, Kimi ERROR-SR-007 не менять. Проверенный этап будет опубликован с обновлёнными журналами; production не разрешён.
| ID | P | Зависимости | Критерии DONE | Статус | Даты | Проверки / SHA |
|---|---|---|---|---|---|---|
| SR-100c-4a | P0 | SR-100c | Per-player reservation, callback вне world mutex; tick/snapshot/другие игроки продолжают работу; actor actions guarded; no invulnerability | DONE | 2026-11-09 / 2026-11-09 | blocked-store red→green; полный world race PASS1.787s / этот core commit |
| SR-100c-4b | P0 | SR-100c-4a | Per-owner session gate вместо global Handler mutex на IO; reconnect/leave/CAS/recovery безопасны | DONE | 2026-11-09 / 2026-11-09 | slowIO red→green; PG row-lock+reconnect/gate cleanup race PASS6.553s / этот WS commit |
| SR-100c-4c | P0 | SR-100c-4b | PG/Godot/server checks, журналы/HANDOFF, безопасная интеграция | DONE | 2026-11-09 / 2026-11-09 | full441/22 PASS, hangar40/0, vet/build PASS; main5b71ca7, remote CI pending |

SR-100c-4c UI substep DONE локально: pending-label + hangar40/0, веточный commit. Server code commitsbc0aed9/aa7f950, vet/build PASS. Сам4c остаётся IN_PROGRESS до полного прогона/интеграции. Нет нового APK/production публикации.

SR-100c-4 validation завершена: полный Go+PG+Godot race suite441 test/subtest PASS/22 пакета,1 старый ambient skip,0 FAIL; hangar40/0; vet/build PASS.4c IN_PROGRESS до подтверждения публикации. Source a5bfd2e, main проверена до9b02d69 (только QUESTJOB/FinalQuest.md). Новых миграций нет; прежняя0020 не менялась.

SR-100c-4c DONE: main7db4176→5b71ca7 и origin/codex/sr100c-coordinator опубликованы обычным non-force push. Свежая main меняла только FinalQuest.md; все чужие изменения сохранены. Source послеa5bfd2e не менялся, поэтому docs-only merge не требовал нового полного прогона. CI code head [42656434874](https://github.com/maillunex-lang/CosmocRPG/actions/runs/42656434874) pending на момент проверки, не success; следующие docs/main commits могут заменить/отменить run. Свой PG container удалён, root main worktree не тронут. Следующий SR-100d; ERROR-SR-007 остаётся Kimi, APK/production не выпущены.

## Новое поручение: пункты4→5, отдельная ветка, main только после подтверждения
База5089cd8, ветка codex/equipment-builds-progression, worktree /workspace/worktrees/cosmoc-equipment-builds. Пользователь запретил публикацию в main до готового результата и отдельного вопроса. Ранние разрешения main не применять к этой работе. Production также не разрешён. Kimi ERROR-SR-007 не затрагивать.
| ID | P | Зависимости | Проверяемый результат | Статус | Даты | Тесты/SHA |
|---|---|---|---|---|---|---|
| EB-000 | P0 | SR-100c-4 | Аудит и декомпозиция4/5, текущие legacy contracts сохранены | DONE | 2026-11-09 / 2026-11-09 | read-only audit; этот docs commit |
| EB-010 | P0 | EB-000 | Общий каталог modules/hulls; owner/slots/size/class/levels/unique/mass/reactor validation; данные/refs | IN_PROGRESS | 2026-11-09 / — | pure/negative/catalog refs |
| EB-020 | P0 | EB-010 | Additive PG state, memory parity, atomic equipment/research/engineering transactions+реальная валюта; legacy engine сохраняется | TODO | — / — | realPG/race/rollback/ownership/reconnect |
| EB-030 | P0 | EB-020 | Runtime coordinator/WS + ангар inventory/slots/budgets/reasons | TODO | — / — | slowIO/PG/WS/Godot |
| EB-040 | P0 | EB-030 | Три билда, четыре active slots+separate ultimate/autoattack; atomic dock-only switch/stale skill prevention | TODO | — / — | rollback/CAS/disabled source/real client |
| EB-050 | P0 | EB-040 | Skill rank1–5; passive36; spec30 rank1–20/6 gated nodes+alternatives; rank20 ultimate enhancement; paid respec | TODO | — / — | budget/prerequisites/idempotent grants/old XP |
| EB-060 | P0 | EB-050 | Ultimate charge/profile actions/reset/anti-farm and runtime effects; UI skills/tree/builds | TODO | — / — | combat/profile/anti-exploit/Godot |
| EB-070 | P0 | EB-060 | Full regression, branch commits/journals/reviewable diff, detailed HANDOFF; спросить о main | TODO | — / — | fullPG/race/Godot/vet/build; NO MAIN PUSH |
Полная библиотека72 уникальных эффектов SR-012/020 остаётся отдельным требованием; unsupported abilities явно недоступны до реализации эффекта, нельзя объявлять их рабочими только по наличию ID. Нельзя завершать пункт5 фиктивными эффектами/пустыми ультами. При исчерпании сессии записать точный checkpoint и продолжить, не выдавать незавершённое за готовое/не спрашивать main до reviewable результата.
