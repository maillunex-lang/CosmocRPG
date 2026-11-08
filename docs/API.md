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
