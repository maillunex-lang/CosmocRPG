#!/usr/bin/env bash
# Проверка восстановления: последняя копия разворачивается во ВРЕМЕННУЮ базу внутри контейнера
# postgres, данные сверяются с рабочей базой, временная база удаляется. Рабочая база не меняется.
# Использование: infrastructure/restore-test.sh [файл.sql.gz]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="${BACKUP_DIR:-$HOME/backups/cosmoc}"
STACK_ENV="${STACK_ENV:-$HOME/keys/cosmoc/stack.env}"
DC=(sudo docker compose -f "$ROOT/infrastructure/docker-compose.yml" --env-file "$STACK_ENV")
FILE="${1:-$(ls -1t "$DIR"/cosmoc-*.sql.gz 2>/dev/null | head -n1)}"
[ -n "$FILE" ] && [ -f "$FILE" ] || { echo "ОШИБКА: копия не найдена в $DIR" >&2; exit 1; }
TMPDB="cosmoc_restore_test"

echo "Копия: $FILE"
( cd "$(dirname "$FILE")" && sha256sum -c "$(basename "$FILE").sha256" )
gzip -t "$FILE"

psql_in() { "${DC[@]}" exec -T postgres sh -c "psql -U \"\$POSTGRES_USER\" -v ON_ERROR_STOP=1 $1"; }

cleanup() { psql_in "-d postgres -qc 'DROP DATABASE IF EXISTS $TMPDB'" >/dev/null 2>&1 || true; }
trap cleanup EXIT

psql_in "-d postgres -qc 'DROP DATABASE IF EXISTS $TMPDB' -c 'CREATE DATABASE $TMPDB'"
zcat "$FILE" | "${DC[@]}" exec -T postgres sh -c "psql -U \"\$POSTGRES_USER\" -d $TMPDB -v ON_ERROR_STOP=1 -q" >/dev/null

# сверка строк: рабочая база и восстановленная
count() { "${DC[@]}" exec -T postgres sh -c "psql -U \"\$POSTGRES_USER\" -d $1 -tAc \"SELECT count(*) FROM $2\""; }
status=0
for t in schema_migrations admin_users admin_audit_log server_boots; do
  live="$(count '"$POSTGRES_DB"' "$t")"
  rest="$(count "$TMPDB" "$t")"
  if [ "$live" = "$rest" ]; then echo "  $t: $rest = $live ✔"; else echo "  $t: восстановлено $rest, в рабочей базе $live ✖ (копия могла быть сделана раньше)"; status=1; fi
done
[ $status -eq 0 ] && echo "ВОССТАНОВЛЕНИЕ ПРОВЕРЕНО" || echo "РАСХОЖДЕНИЕ: если после копии были записи, повторите backup.sh и проверку"
exit $status
