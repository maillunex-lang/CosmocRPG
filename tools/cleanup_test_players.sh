#!/usr/bin/env bash
# Удаляет тестовых игроков (ники «Tst_…», «[тест]…», аккаунты «test_…»), которых создают сквозные тесты.
# Использование: tools/cleanup_test_players.sh [dev|prod]
#   dev  (по умолчанию) — изолированная тестовая база: контейнер cosmoc-pg-dev, база cosmoc_test.
#   prod — база production (стек Docker Compose). Только по явному аргументу и только после тестов, которые сами запускались против production.
# Тесты не должны подключаться к production-базе по умолчанию (решение владельца 2026-11-08), поэтому без аргумента production не затрагивается.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-dev}"
SQL="DELETE FROM players WHERE name LIKE 'Tst\\_%' OR name LIKE '[тест]%' OR public_id IN (SELECT player_public_id FROM accounts WHERE login LIKE 'test\\_%')"
case "$TARGET" in
  dev)
    sudo docker exec cosmoc-pg-dev psql -U cosmoc -d cosmoc_test -At -c "$SQL" >/dev/null 2>&1 || true ;;
  prod)
    ENVF="$HOME/keys/cosmoc/stack.env"
    [ -f "$ENVF" ] || exit 0
    sudo docker compose -f "$ROOT/infrastructure/docker-compose.yml" --env-file "$ENVF" exec -T postgres sh -c \
      "psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -At -c \"$SQL\"" >/dev/null 2>&1 || true ;;
  *) echo "аргумент: dev или prod" >&2; exit 2 ;;
esac
