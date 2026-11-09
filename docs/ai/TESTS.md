# AI civilization — foundation: фактические проверки

База 588fde6; ветка feature/ai-civilization. Полная реализация AI.md НЕ завершена.

- Полный `go test -race -count=1 -json ./...`, COSMOC_TEST_DATABASE_URL указывает только на свой временный PostgreSQL16, loopback+tmpfs: exit0. События: {('skip', 'package'): 3, ('pass', 'test'): 497, ('pass', 'package'): 24, ('skip', 'test'): 3}.
- 3 test SKIP: один AmbientFollowFromAnotherCaptain (допустимая вариативность) и два Godot harness (COSMOC_TEST_GODOT не задан). 3 package SKIP — без тестов. Новые PostgreSQL-тесты выполнены, не пропущены.
- Профильные aiciv/config/db/cmd-server: race+PG PASS, включая сохранение личности, CAS привязки, уникальность player, дедуп/TTL/retention128, раздельную память, отсутствие игроков после shadow sync, отмену context, стоимость/глубину/бюджеты GOAP и отказ неизвестных/запрещённых действий.
- `go vet ./...`, `go build ./...`, gofmt, diff --check PASS.
- validate_roadmap.py: 54 задачи PASS; source anchors11 проверены; номера SQL1…22 уникальны.

Не выполнялись: замена активных AI, создание новых player, исполнение команд, новая речь, canary, нагрузка активных100/200, графическая приёмка, admin UI acceptance, production. Planner utility weights — технические начальные параметры, не утверждённый игровой баланс; в live принятии решений не используются.

Миграция0022 additive; rollback foundation = оставить флаг COSMOC_AI_CIVILIZATION_SHADOW выключенным, сохранить таблицы/данные. Старый Manager/Dialogue не удалены. Полный откат будущих активных AI и сохранность имущества должны быть проверены до AIC-100. Не удалять сохранённые записи ради отката.
