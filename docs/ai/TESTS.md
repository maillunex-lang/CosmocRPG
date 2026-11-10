# AI civilization — foundation: фактические проверки

База 588fde6; ветка feature/ai-civilization. Полная реализация AI.md НЕ завершена.

- Полный `go test -race -count=1 -json ./...`, COSMOC_TEST_DATABASE_URL указывает только на свой временный PostgreSQL16, loopback+tmpfs: exit0. События: {('skip', 'package'): 3, ('pass', 'test'): 497, ('pass', 'package'): 24, ('skip', 'test'): 3}.
- 3 test SKIP: один AmbientFollowFromAnotherCaptain (допустимая вариативность) и два Godot harness (COSMOC_TEST_GODOT не задан). 3 package SKIP — без тестов. Новые PostgreSQL-тесты выполнены, не пропущены.
- Профильные aiciv/config/db/cmd-server: race+PG PASS, включая сохранение личности, CAS привязки, уникальность player, дедуп/TTL/retention128, раздельную память, отсутствие игроков после shadow sync, отмену context, стоимость/глубину/бюджеты GOAP и отказ неизвестных/запрещённых действий.
- `go vet ./...`, `go build ./...`, gofmt, diff --check PASS.
- validate_roadmap.py: 54 задачи PASS; source anchors11 проверены; номера SQL1…22 уникальны.

Не выполнялись: замена активных AI, создание новых player, исполнение команд, новая речь, canary, нагрузка активных100/200, графическая приёмка, admin UI acceptance, production. Planner utility weights — технические начальные параметры, не утверждённый игровой баланс; в live принятии решений не используются.

Миграция0022 additive; rollback foundation = оставить флаг COSMOC_AI_CIVILIZATION_SHADOW выключенным, сохранить таблицы/данные. Старый Manager/Dialogue не удалены. Полный откат будущих активных AI и сохранность имущества должны быть проверены до AIC-100. Не удалять сохранённые записи ради отката.

## AIC-020/021…024 — 2026-11-10

Полный серверный race+свой PostgreSQL16: 506 test/subtest PASS,24 packages PASS,0FAIL;3 testSKIP (ambient/дваGodot). После последней правки guard/misconfigured-store и direct-WS-bypass профильные aiciv/config/quest/world/ws повторно PASS. Отдельно оба Godot live harness PASS (11.950s); import exit0. Vet/build exit0, gofmt/diff --check PASS. 24 уникальных номера SQL, валидатор60 tasks PASS. Новые023/024 к production не применялись.

Проверено: атомарный bootstrap при8 конкурентных вызовах, отказ резервного ника без orphan, сохранение playerID/имени, отсутствие халявных XP/денег, новая lease инвалидирует старую, expired не создаёт гостя, gateway off, AI marker/quota50humans+AI, обычная WS-добыча создаёт cargo, недоступная цель отклонена, replay не перезапускает добычу,20 конкурентных claim дают один dispatch, pending после restart не переисполняется, отказ прямого buy в обход контроллера. Ошибка промежуточного теста сравнения JSONB whitespace исправлена семантическим сравнением code, а не ослаблением игровой проверки.

Подтверждённый открытый RED вне active suite: BETA-ECO-001, при полном трюме buy отклонён, баланс1000→989. Дефект не исправлен/не скрыт: воспроизводитель docs/ai/repro/full_hold_buy_test.go.txt, AIC-025 BLOCKED. Факт зелёного штатного suite не является доказательством отсутствия этого дефекта.

Ограничения: нет общего atomic economic service/durable world outbox, нет director/canary/load/новой речи/глобальных квот. Живое прохождение кампании двумя Android-клиентами не выполнялось. Автоматического AI-spawn нет, flags off. Нельзя считать весь AI.md реализованным.
