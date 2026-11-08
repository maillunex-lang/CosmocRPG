# Звёздные рубежи (CosmocRPG)

Космическая idle MMORPG с общим онлайн-миром, экономической песочницей, открытым PvP и AI-цивилизацией.
Платформа: Android. Графика: 2.5D-космос с полноценными 3D-кораблями.

Статус: **Этап 00 — аудит и инфраструктура.** Игровых механик пока нет.

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
- [ROADMAP](docs/ROADMAP.md) — этапы
- [DECISIONS](docs/DECISIONS.md) — журнал решений
- [SECURITY](docs/SECURITY.md) — безопасность
- [DEPLOYMENT](docs/DEPLOYMENT.md) — развёртывание
- [HANDOFF](docs/HANDOFF.md) — журнал передачи задач
- [DEVELOPMENT](docs/DEVELOPMENT.md) — правила разработки

## Быстрый старт клиента

```bash
godot --headless --path client --quit   # проверка запуска без ошибок
godot --path client                     # запуск с окном
tools/screenshot.sh                     # скриншот сцены через xvfb
```

Запуск сервера и админки появится по мере выполнения шагов 00.3–00.6.
