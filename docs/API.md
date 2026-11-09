# API

Статус: черновик Этапа 00. Контракты уточняются при реализации шагов 00.3–00.5.

## Транспорт

- HTTP(S) для служебных и админских запросов (JSON).
- WebSocket для игровых сессий: клиент шлёт команды, сервер шлёт состояние.
- Сейчас без TLS (доступ по IP). Домен и TLS обязательны к релизу.

## HTTP

| Метод | Путь | Описание |
|---|---|---|
| GET | `/health` | Состояние сервера и подключения к БД. |
| GET | `/ws` | Апгрейд до WebSocket. |
| POST | `/admin/login` | Вход администратора (позже). |
| GET | `/admin/status` | Статус сервера, только с авторизацией (позже). |

## WebSocket: формат сообщений

```json
{ "type": "string", "id": "uuid клиента для идемпотентности", "payload": {} }
```

Правила: все входящие сообщения валидируются; неизвестный тип отклоняется; частота ограничена; сессия привязана к токену и не переиспользуется.

Типы сообщений Этапа 00: `hello`, `ping`/`pong`, `error`.

### Этап 01: игровые сообщения (протокол версии 1)

Подробности и обоснование: `docs/design/STAGE01_ONLINE_SPACE.md`. Первым игровым сообщением должен быть `join`.

Клиент → сервер:

| type | payload | Примечание |
|---|---|---|
| `join` | `{token, protocol: 1, name?}` | `token` — 16-128 символов `[A-Za-z0-9_-]`. `name` (необязательно, до 24 символов) применяется только при создании игрока, иначе «Пилот-NNNN». |
| `move` | `{x, y}` | Направление в мировых осях, длина ограничивается сервером до 1. Не чаще 10 раз в секунду. Отключает автопилот. |
| `target` | `{id}` | Пустая строка снимает цель. Ошибка `bad_target`, если цели нет. |
| `autopilot` | `{on}` | Полёт к выбранной цели. Ошибка `no_target`, если цель не выбрана. |

Сервер → клиент:

| type | payload |
|---|---|
| `welcome` | `{player_id, name, system: {id, name, background}}` |
| `state` (10 Гц) | `{time, ship: {name, pos, dir, speed}, objects: [{id, kind, stance, name, pos, dir}], target_id, autopilot}`; `pos`, `dir` — массивы `[x, y]` |

Коды ошибок Этапа 01: `not_joined`, `bad_token`, `bad_protocol` (соединение закрывается), `already_joined`, `server_full`, `server_error`, `bad_target`, `no_target`, `replaced` (тот же токен вошёл с другого устройства; старая сессия закрывается).

Контракты (Этап С-01, флаг `COSMOC_FEATURE_QUESTS`): если включены, в `state` добавляются `credits` (число) и `quests` (активные контракты `{id, title, text, progress, goal}`), а при выполнении сервер присылает `quest_completed` `{id, title, reward: {credits}, credits}`. Клиент не может выполнить контракт или получить награду: таких сообщений в протоколе нет, всё решает сервер. Награда выдаётся один раз.

Корабль «Стриж» (FTR-001), подробности и коды ошибок: `docs/design/FTR-001.md`. Клиент → сервер: `attack {on}`, `ability {id}`, `set_skin {value}`, `set_spec {value}`. Сервер → клиент: расширенные `state.ship` и объекты, `combat_events`.

Аккаунты (до `join`): `register {token, login, password}`, `login {token, login, password}` → `account {login, created}`; подробности и коды ошибок: `docs/design/ACCOUNTS.md`.

Персонаж (A1): при первом входе `welcome {needs_character: true, pilot_options[]}`, затем `create_character {nick, gender, pilot_spec}` → `welcome` с `character`. Подробности: `docs/design/ALPHA.md`. Каталог кораблей и ZIP-патчи: раздел «Корабли» админки (`/admin/api/ships/*`).

Параметры мира: скорость 14 ед/с, радиус системы 400 ед., автопилот останавливается в 10 ед. от цели, максимум 50 игроков, корабль остаётся в мире 10 с после обрыва связи, позиции сохраняются раз в 5 с и при отключении.

## Живые локации, добыча, магазин, смена корабля (2026-11-08)

Все расширения аддитивные, версия протокола прежняя (1).

Клиент → сервер: `mine {id}` (добыча из месторождения), `mine_stop`, `sell` (продажа груза у станции), `ships` (список кораблей каталога), `set_ship {ship}` (смена у станции), `shop`, `shop_buy {item, request_id}`, `shop_equip {slot, item}`.
Сервер → клиент: `welcome` дополнен `layout` (раскладка системы: тела, станции, зоны, месторождения, точки, маршруты, параметры декора) и `visual` (графические пресеты), если включены живые локации; `state.ship` дополнен `hold`, `cargo_used`, `mining`, `can_mine`, `mine_range`, `in_safe`, `cosmetics`; `state.objects` дополнены `tier`, `resource`, `rarity`, `amount`, `amount_max`, `zone`, `vip`, `badge`, `name_color`, `engine` (виды `deposit`, `anomaly`, `derelict`, `signal`, `hostile` с уровнем); новые сообщения `reward {kind, ref, credits, total}`, `ships {ships[]}`, `shop {catalog, state, cosmetics}`; события боя `loot`, `mine`, `mine_start`, `mine_stop`, `discover`, `sell`, `ship_changed`.
Коды ошибок: `no_module`, `hold_full`, `deposit_empty`, `no_deposit`, `not_at_station`, `nothing_to_sell`, `no_ship`, `hold_too_big`, `no_item`, `already_owned`, `vip_required`, `not_owned`, `bad_slot`, `bad_request`.
Подробности: `docs/design/LIVING_LOCATIONS.md`, `docs/design/SHOP_VIP.md`.

Админ-API (требует входа): `GET /admin/api/world`, `POST /admin/api/world/preview`, `PUT /admin/api/world`, `POST /admin/api/world/versions/{id}/rollback`.

## Чат (2026-11-08)

Клиент → сервер: `chat_send {channel: "general"|"dm", to?, text}` (`to` — ник или идентификатор адресата для `dm`), `chat_history {}`.
Сервер → клиент: `chat {message}` (общий канал всем, личное только отправителю и адресату), `chat_history {general[], dms[], settings{enabled, max_len, min_interval_ms}}`.
`message`: `{id, channel, from_id, from_name, from_ai?, to_id?, to_name?, text, at}` (`at` — unix, мс; `from_ai: true` у ИИ-капитанов, клиент показывает иконку). Отправитель всегда берётся из сессии сервера, подделать его в сообщении нельзя.
Ошибки: `chat_disabled`, `chat_empty`, `chat_too_long`, `chat_rate`, `chat_duplicate`, `chat_links`, `chat_bad_words`, `chat_muted`, `chat_no_player`, `chat_self`, `chat_bad_channel`.
Правила по умолчанию: длина до 200 символов, не чаще 1 сообщения в 2 секунды, повтор того же текста в течение 30 с отклоняется, ссылки запрещены, фильтр недопустимых слов включён, история 7 дней. Всё настраивается в админке («Чат»). Админ-API: `GET /admin/api/chat/messages`, `GET/PUT /admin/api/chat/settings`, `GET/POST /admin/api/chat/mutes`, `DELETE /admin/api/chat/mutes/{id}`.

## Пилот: опыт, уровень, характеристики (RPG-1, 2026-11-08)

Клиент → сервер: `pilot {}` (запросить состояние), `pilot_spend {attr, points}` (потратить свободные очки; `attr` — `piloting|tactics|engineering|energetics|analytics|logistics`, `points` 1…40).
Сервер → клиент: `pilot {level, max_level, xp, into_level, to_next, level_xp, attr_free, attr_spent, talent_points, attributes{}}` (ответ на оба запроса; также в `welcome.pilot`); `xp {gained, source, level_up, pilot{}}` — уведомление о начисленном опыте (источники: `kill`, `discover`, `sell`, `quest`). Ошибки: `pilot_bad_attr`, `pilot_bad_count`, `pilot_no_points`, `pilot_cap`.
Правила: опыт начисляется только сервером, один раз на ключ события (`xp_grants`, `UNIQUE(player_id, grant_key)`); уровень (1–60) вычисляется из опыта; очки характеристик — 2 за уровень выше первого, талантов — 1 (тратятся с RPG-3). Влияние характеристик на корабль пока не подключено (RPG-2/6).

## Уровень корабля (RPG-2, 2026-11-08)

Экземпляр корабля — пара (игрок, тип корабля), у каждого свой опыт и уровень 1–30 (`player_ships`); он сохраняется при смене корабля. Уровень масштабирует корпус, щит, энергию и трюм (+3 % базового значения за уровень выше первого, `rpg.Compute`); рост пересчитывается с нуля, повторное применение не накапливается.
Сервер → клиент: `state.ship.level`; `welcome.ship_progress {ship, level, max_level, xp, into_level, to_next, level_xp}`; `ships.levels {тип: прогресс}` (нет записи — уровень 1); в `xp` добавлены `ship` (прогресс корабля) и `ship_level_up`. Опыт кораблю идёт за `kill`, `discover`, `sell` (в том же размере, что пилоту; контракты — только пилоту), ключ события `s<ключ>`.
Бой: во время атаки сервер плавно (4 рад/с) разворачивает корабль носом к цели (`state.ship.dir`).

## ИИ-капитаны в мире (ИИ-1, 2026-11-09)

В `state.objects` появляются объекты `kind: "ai"` с полем `ai: true` (метка иконкой), `ship`, `skin`, `dir`, `name` (игровой ник из AI Studio). Видны только в радиусе `view_radius` (по умолчанию 450) от игрока (область интереса). ИИ-капитана можно выбрать целью и лететь к нему автопилотом; атаковать нельзя (PvP выключен). ИИ не игроки: нет сессии и записи в базе, ничего не добывают и не продают, наград не получают, в лимит 50 игроков не входят.


### Ships RPG: паспорт Стрижа (R-63/SR-100a)
Ответ на существующую команду `ships` дополнен необязательными полями `ship_max_level` (сейчас50) и `passport` (полный ShipPassport только для текущего FTR-001 и только при настроенном ShipPassports provider). Старые `ships`/`levels` сохраняются, версия протокола не меняется. Новая команда/новые клиентские запросы не требуются, старые серверы просто не присылают дополнительные поля.
`passport.base` формируется из World.SpecInfo текущего игрока: активные админские правки и уровень корабля сохраняются. Предварительные бюджеты/слоты берутся из проверенного v2 документа, не применяются как бонусы и не публикуют новые корабли. На других hulls passport отсутствует; смена hull/старый ответ очищают секцию в клиенте. Модули/экипировка/доступность навыков этим расширением ещё не реализованы.
