#!/usr/bin/env bash
# Публикация релиза APK для проверки обновлений: копирует APK в каталог раздачи, пишет подписанный манифест latest.json.
# Использование: tools/publish_release.sh <файл.apk> "<описание изменений>"   (переменные: APK_DIR, MIN_CODE, MANIFEST_KEY)
# Номер версии берётся из client/project.godot (config/version и cosmoc/version_code) и должен совпадать с APK.
# Приватный ключ подписи манифеста хранится вне репозитория (~/keys/cosmoc/manifest_private.pem), публичный лежит в клиенте.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APK="${1:?укажите APK}"; NOTES="${2:-}"
DIR="${APK_DIR:-$HOME/apk-share}"
KEY="${MANIFEST_KEY:-$HOME/keys/cosmoc/manifest_private.pem}"
PUB="$ROOT/client/assets/update_public.pem"
BASE_URL="https://rikagro.ru/download"
[ -f "$APK" ] && [ -f "$KEY" ] || { echo "нет APK или ключа" >&2; exit 1; }
NAME="$(sed -n 's/^config\/version="\(.*\)"/\1/p' "$ROOT/client/project.godot")"
CODE="$(sed -n 's/^version_code=\([0-9]*\)/\1/p' "$ROOT/client/project.godot")"
[ -n "$NAME" ] && [ -n "$CODE" ] || { echo "не найдена версия в project.godot" >&2; exit 1; }
MIN="${MIN_CODE:-1}"
FILE="zvezdnye-rubezhi-$NAME-$CODE.apk"
mkdir -p "$DIR"
cp "$APK" "$DIR/$FILE.tmp" && mv "$DIR/$FILE.tmp" "$DIR/$FILE"
SHA="$(sha256sum "$DIR/$FILE" | cut -d' ' -f1)"; SIZE="$(stat -c %s "$DIR/$FILE")"
[ "$SIZE" -le $((95*1024*1024)) ] || { echo "APK больше 95 МБ" >&2; exit 1; }
python3 - "$DIR/latest.json.tmp" "$NAME" "$CODE" "$MIN" "$BASE_URL/$FILE" "$SHA" "$SIZE" "$NOTES" <<'PY'
import json,sys,datetime
out,name,code,mn,url,sha,size,notes=sys.argv[1:9]
m={"version_name":name,"version_code":int(code),"min_version_code":int(mn),"url":url,"sha256":sha,"size":int(size),
   "notes":notes,"published":datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
open(out,"w",encoding="utf-8").write(json.dumps(m,ensure_ascii=False,indent=2)+"\n")
PY
openssl dgst -sha256 -sign "$KEY" "$DIR/latest.json.tmp" | base64 -w0 > "$DIR/latest.json.sig.tmp"
# проверка подписи публичным ключом до публикации
openssl dgst -sha256 -verify "$PUB" -signature <(base64 -d "$DIR/latest.json.sig.tmp") "$DIR/latest.json.tmp" >/dev/null
mv "$DIR/latest.json.tmp" "$DIR/latest.json"; mv "$DIR/latest.json.sig.tmp" "$DIR/latest.json.sig"
echo "опубликовано: версия $NAME ($CODE), $SIZE байт, sha256 ${SHA:0:16}…, файл $FILE"
