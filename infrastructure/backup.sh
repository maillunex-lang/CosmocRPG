#!/usr/bin/env bash
# Резервная копия PostgreSQL стека cosmoc (pg_dump -> gzip) с проверкой целостности и ротацией.
# Использование: infrastructure/backup.sh
# Переменные: BACKUP_DIR (по умолчанию ~/backups/cosmoc), KEEP_DAYS (по умолчанию 14), STACK_ENV (~/keys/cosmoc/stack.env)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="${BACKUP_DIR:-$HOME/backups/cosmoc}"
KEEP_DAYS="${KEEP_DAYS:-14}"
STACK_ENV="${STACK_ENV:-$HOME/keys/cosmoc/stack.env}"
DC=(sudo docker compose -f "$ROOT/infrastructure/docker-compose.yml" --env-file "$STACK_ENV")

umask 077
mkdir -p "$DIR"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$DIR/cosmoc-$TS.sql.gz"

# дамп во временный файл; в готовое имя переименовываем только после проверок
"${DC[@]}" exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --clean --if-exists' | gzip > "$OUT.tmp"

gzip -t "$OUT.tmp"                                   # архив не повреждён
if [ "$(stat -c %s "$OUT.tmp")" -lt 500 ]; then      # пустой/обрезанный дамп
  echo "ОШИБКА: дамп подозрительно мал, копия отклонена" >&2
  exit 1
fi
zcat "$OUT.tmp" | grep -q "PostgreSQL database dump complete" || { echo "ОШИБКА: дамп не завершён" >&2; exit 1; }

mv "$OUT.tmp" "$OUT"
( cd "$DIR" && sha256sum "$(basename "$OUT")" > "$(basename "$OUT").sha256" )

# ротация: удаляем копии старше KEEP_DAYS (только файлы cosmoc-*.sql.gz* в каталоге бэкапов)
find "$DIR" -maxdepth 1 -type f -name 'cosmoc-*.sql.gz*' -mtime +"$KEEP_DAYS" -delete

echo "OK: $OUT ($(stat -c %s "$OUT") байт)"
