# Обновление статуса (2026-11-09, после публикации SR-100a)
main содержит cb34353: read-only паспорт Стрижа в игровом ангаре; 88a060c: PG-test цепочки. CI42606462785: Go/PG job129123615616 success, admin/Docker success; Godot job129123615478 ещё выполняется. ERROR-SR-005 Resolved. Новый APK/production не выпущены.
SR-093 добавляет тот же public update PEM в git и тест его наличия (33/0): *.pem раньше исключал его из clone. Production signed manifest0.3.11/code14 Verified OK, updater installed13→available/14→up_to_date. Приватных ключей/SDK в cloud нет, сборка обновляемого APK остаётся ограничением выпуска.
При позднем опросе gh CLI получил401; GitHub connector fetch_workflow_run_jobs работает, причина401 не установлена. Не логиниться чужими токенами и не печатать секреты; пользоваться connector. Следующий шаг — проверить финальный Godot CI для cb34353, затем SR-100b/c. Исторические ожидания CI ниже заменены этим статусом.

---

# Передача R-63: поэтапная игровая интеграция
Дата: 2026-11-09. Ветка локально codex/ships-rpg-foundation; публикация в remote main разрешена пользователем.

## Текущая контрольная точка
SR-090 DONE: все прежние проверенные Ships RPG коммиты и новый план опубликованы в main fast-forward до75931d4. SR-100a DONE локально: настоящий сервер → клиентский шлюз → игровой ангар Стрижа. Текущий коммит SR-100a публикуется после тестов; его SHA и результат CI будут добавлены в финальной передаче. SR-091: 88a060c — исправлено только устаревшее ожидание PG-теста quest, remote PostgreSQL CI ещё требуется.

SR-100b/c/d — TODO. **Модуль не управляет доступностью навыка, сохранение оборудования/билды/док/ульта ещё не реализованы. Production/APK не выпущены.** Каталог по-прежнему12 игровых hulls, новые три — только draft данные.

## Изменения SR-100a
- server/cmd/server/main.go: валидированный PassportCatalog при старте, без SetCatalog/публикации моделей.
- server/internal/ws/hub.go: optional provider и passport/ship_max_level в существующем ships response, только для текущего FTR-001. base берётся из текущего World.SpecInfo: активные правки админа и level scaling сохраняются. Нет новых команд/версии протокола/миграций.
- client/ui/gateway/server_gateway.gd: optional расширение передаётся по ships_updated в текущую строку hull; старые ответы поддержаны.
- client/ui/ships/hangar.gd: description/предварительные mass/MW/module budget/slot counts, server base для параметров; cap берётся из ответа, не30. Sidebar прокручивается. Старый ответ/чужой hull очищает секцию.
- ws/passport_test.go: реальный WS, активные админские поля/уровень/старый decoder/другой hull/provider nil; TestPassportGodotClient запускает настоящий локальный memory-backed httptest server и Godot клиент.
- client/tests/passport_online_test.gd и расширенный hangar_test.gd; tools/test_client.sh запускает live scenario в Godot CI.
- docs/API.md, главные и профильные карты/журналы; RELEASE_PROCESS.md описывает механизм Claude.

## Реальные проверки
Локально Go1.27.3: полный `COSMOC_TEST_GODOT=godot go test -race -count=1 -json ./...` PASS:20 пакетов,308 PASS,62 SKIP (PostgreSQL),0 FAIL; 3 пакета без тестов. Standalone live Godot test PASS3.591s. Godot4.6.3: import PASS, hangar28/0, fleet_shop53/0, updater31/0, apk_installer9/0. Настоящий ключ подписи для openssl-ветки updater отсутствует — SKIP. vet/build/gofmt/diff-check PASS. Изменённые импортером4.6 glb.import восстановлены, не коммитить их. Визуальный рендер/Android установка не заявляются.

CI75931d4 run42604786797: admin, Docker и client (Godot4.7.2 с настоящей тестовой БД) PASS; Go FAIL только TestPGRewardOnceAndPersistence. Лог через GitHub connector подтвердил устаревшее ожидание нуля заданий вместо first_blood после restart. Runtime quest не менялся: test-only 88a060c теперь проверяет first_blood Progress0/Credits500 и отсутствие повторной награды. Не объявлять ERROR-SR-005 Resolved до нового PG CI. ERROR-SR-004 (fixture использовала повторный Join вместо SetShip) Resolved. Ошибки003/002/001 ранее Resolved.

## Выпуск и APK
Изучены Claude/Kimi notes, updater.gd, apk_installer.gd, publish_release.sh. Прежние ключи app release и RSA manifest обязательны для обновления поверх установленной игры. Подписанный manifest → проверка HTTPS/подписи/версии → загрузка/проверка размера+SHA256 → Android installer; резервный путь браузер. Код обновления не переписывать.
В cloud Java/Godot4.6.3 есть, SDK tools/AAR и прежние release credentials не настроены; исходный version_code14 (0.3.11) не поднимать без готового release-кандидата и сверки main/публичного manifest. Новый APK понадобится для доставки секции пользователям, но APK пока не собран/не опубликован. Не генерировать другой release key: обновление не установится. Production-деплой запрещён без отдельного разрешения; main push не означает деплой.

## Следующие три шага
1. Проверить CI следующего main push, особенно PostgreSQL quest regression и новый Godot live test. Обновить ERROR-SR-005/REGRESSION_TESTS/ROADMAP/HANDOFF фактическим результатом.
2. SR-100b: стартовый ENG → M01 на Стриже, серверная доступность/отказ без модуля и без списания; сохранить старые boost параметры/автоатаку. Не выдавать draft starter blueprint за принадлежащий игроку предмет.
3. SR-100c: безопасная добавочная схема оборудования, тестовая БД/сохранение/повторный вход, только док вне боя, atomic rollback, UI причин недоступности/HUD. Затем SR-100d release-кандидат/проверки/APK и отдельное разрешение на production. Остальные SR-012…070 сохраняют требования и идут по мере расширения сценария.

## Команды продолжения
```bash
cd /workspace/CosmocRPG
git status --short --branch
git fetch origin main
git log -8 --oneline
cat docs/ships-rpg/ROADMAP.md docs/ships-rpg/ERROR_LOG.md docs/ships-rpg/HANDOFF.md
export PATH=/workspace/scratch/toolchain/go/bin:$PATH
cd server
COSMOC_TEST_GODOT=godot go test -race -count=1 ./internal/ws -run 'TestShipsPassport|TestShipsWithoutPassport|TestPassportGodotClient'
go vet ./...
```
Перед каждым push сверять remote main и интегрировать чужие изменения; не force push. Никаких чужих VPS проектов/production/секретов. Все временные memory servers завершены тестами, бинарник в /workspace/scratch/cosmoc-server-r63 не запускался. Логи локальных проверок /workspace/scratch/ships-rpg-r63-all-tests.jsonl и ships-rpg-passport-live.log могут исчезнуть, сводка выше сохраняется в репозитории. История завершённых SR-011 и более ранних — основная docs/HANDOFF.md, git show987b7ee:docs/ships-rpg/HANDOFF.md.
