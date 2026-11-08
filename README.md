# Звёздные рубежи (CosmocRPG)

Космическая idle MMORPG с общим онлайн-миром, экономической песочницей, открытым PvP и AI-цивилизацией.
Платформа: Android. Графика: 2.5D-космос с полноценными 3D-кораблями.

Статус: **подготовка альфа-теста с друзьями.** Работают общий космос, бой с врагами, добыча, торговля станции, магазин, аккаунты, админка. Подробный статус: [единая дорожная карта](docs/ROADMAP.md).

## Стек

| Часть | Технология | Папка |
|---|---|---|
| Клиент | Godot 4.7 | `client/` |
| Игровой сервер (авторитетный) | Go, PostgreSQL | `server/` |
| Админка «Universal Space Studio» | React + TypeScript | `admin/` |
| Общие контракты протокола | схемы/константы | `shared/` |
| Развёртывание | Docker Compose | `infrastructure/` |
| CI/CD | GitHub Actions | `.github/workflows/` |
| Документация | Markdown | `docs/` |
| Утилиты (скриншоты, сборка) | bash/GDScript | `tools/` |

## Принципы

- Сервер — единственный источник истины (движение, бой, экономика, idle-результаты). Клиент шлёт команды, а не результаты.
- Работа маленькими проверяемыми этапами. Этап не закрыт, пока тесты реально не запущены.
- Секреты только вне репозитория (`.env`, `secrets/` в `.gitignore`).
- 15 утверждённых решений меняются только по согласованию с владельцем: см. [docs/DECISIONS.md](docs/DECISIONS.md).

## Документация

- [GDD](docs/GDD.md) — игровой замысел
- [ARCHITECTURE](docs/ARCHITECTURE.md) — архитектура
- [API](docs/API.md) — протокол HTTPS + WebSocket
- [ROADMAP](docs/ROADMAP.md) — **единая дорожная карта**: все требования владельца, статусы, приоритеты, открытые вопросы
- [DECISIONS](docs/DECISIONS.md) — журнал решений
- [SECURITY](docs/SECURITY.md) — безопасность
- [DEPLOYMENT](docs/DEPLOYMENT.md) — развёртывание
- [HANDOFF](docs/HANDOFF.md) — журнал передачи задач
- [DEVELOPMENT](docs/DEVELOPMENT.md) — правила разработки
- [REFERENCES](docs/REFERENCES.md) — внешние источники и референсы
- [Учёт ИИ-контента](docs/AI_CONTENT.md)
- [UI](docs/ui/README.md) — документы интерфейса, [художественный стандарт фона](docs/ui/ART_STANDARD.md), [ассеты](docs/ui/ART_ASSETS.md)
- [Корабли и бой (ТЗ)](docs/design/SHIPS_SPEC.md) — корабли, развитие, эскадры, боевые системы
- [Альфа-тест](docs/design/ALPHA.md) · [Живые локации](docs/design/LIVING_LOCATIONS.md) · [Магазин и VIP](docs/design/SHOP_VIP.md) · [Аккаунты](docs/design/ACCOUNTS.md) · [AI Studio](docs/design/AI_STUDIO.md)

## Быстрый старт клиента

```bash
godot --headless --path client --quit   # проверка запуска без ошибок
godot --path client                     # запуск с окном
tools/screenshot.sh                     # скриншот сцены через xvfb
```

Тесты клиента (нужен запущенный сервер):

```bash
COSMOC_TEST_WS_URL=ws://127.0.0.1:8080/ws tools/test_client.sh [каталог_для_скриншотов]
```

## Сервер (разработка)

```bash
cd server
export COSMOC_DATABASE_URL='postgres://USER:PASS@127.0.0.1:5432/DB?sslmode=disable'
go run ./cmd/server          # слушает :8080, GET /health, GET /ws
COSMOC_TEST_DATABASE_URL=... go test -race ./...   # каждый тест работает в своей временной схеме, public не затрагивается
```

Админка и Docker Compose появятся в шагах 00.5–00.6.
