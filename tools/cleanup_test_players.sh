#!/usr/bin/env bash
# Удаляет тестовых игроков "[тест] …" из базы Docker Compose (их создают сквозные тесты против сервера).
# Только для локальной проверки на VPS; ошибки не критичны. Реальных игроков не затрагивает.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENVF="$HOME/keys/cosmoc/stack.env"
[ -f "$ENVF" ] || exit 0
sudo docker compose -f "$ROOT/infrastructure/docker-compose.yml" --env-file "$ENVF" exec -T postgres sh -c \
  "psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -At -c \"DELETE FROM players WHERE name LIKE '[тест]%' OR public_id IN (SELECT player_public_id FROM accounts WHERE login LIKE 'test\\_%')\"" >/dev/null 2>&1 || true
