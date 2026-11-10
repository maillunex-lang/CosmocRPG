#!/usr/bin/env bash
# Публикация релиза APK: проверяет подпись и версию файла, затем атомарно кладёт его в каталог раздачи
# и пишет подписанный манифест latest.json. Предыдущий манифест сохраняется как latest.json.prev.
# Использование: tools/publish_release.sh <файл.apk> "<описание изменений>"
# Переменные: APK_DIR, MIN_CODE, MANIFEST_KEY.
# Номер версии берётся из client/project.godot и должен совпасть с export_presets.cfg и с самим APK.
# Приватный ключ манифеста не создаётся и не меняется: ~/keys/cosmoc/manifest_private.pem.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APK="${1:?укажите APK}"
NOTES="${2:-}"
DIR="${APK_DIR:-$HOME/apk-share}"
KEY="${MANIFEST_KEY:-$HOME/keys/cosmoc/manifest_private.pem}"
PUB="$ROOT/client/assets/update_public.pem"
BASE_URL="https://rikagro.ru/download"
PROJECT="$ROOT/client/project.godot"
PRESET="$ROOT/client/export_presets.cfg"
SECURITY="$ROOT/docs/SECURITY.md"
STAGE=""
cleanup() { [[ -n "${STAGE}" && -d "${STAGE}" ]] && rm -rf "${STAGE}"; }
trap cleanup EXIT

fail() { echo "отказ: $*" >&2; exit 1; }
[[ -f "$APK" ]] || fail "нет APK"
[[ -f "$PROJECT" && -f "$PRESET" && -f "$SECURITY" && -f "$PUB" ]] || fail "нет файлов проекта или публичного ключа"

NAME="$(sed -n 's/^config\/version="\(.*\)"/\1/p' "$PROJECT" | head -n 1)"
CODE="$(sed -n 's/^version_code=\([0-9]*\)/\1/p' "$PROJECT" | head -n 1)"
PRESET_CODE="$(sed -n 's/^version\/code=\([0-9]*\)/\1/p' "$PRESET" | head -n 1)"
PRESET_NAME="$(sed -n 's/^version\/name="\(.*\)"/\1/p' "$PRESET" | head -n 1)"
PKG="$(sed -n 's/^package\/unique_name="\(.*\)"/\1/p' "$PRESET" | head -n 1)"
[[ -n "$NAME" && -n "$CODE" && -n "$PRESET_CODE" && -n "$PRESET_NAME" && -n "$PKG" ]] || fail "не найдена версия или package id"
[[ "$NAME" == "$PRESET_NAME" && "$CODE" == "$PRESET_CODE" ]] || fail "версия в project.godot ($NAME/$CODE) не совпадает с пресетом ($PRESET_NAME/$PRESET_CODE)"
MIN="${MIN_CODE:-1}"
[[ "$MIN" =~ ^[0-9]+$ && "$CODE" =~ ^[0-9]+$ && "$MIN" -ge 1 && "$MIN" -le "$CODE" ]] || fail "некорректный MIN_CODE"

EXPECT="$(python3 - "$SECURITY" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
m = re.search(r"Отпечаток сертификата SHA-256[^\n]*`([0-9A-Fa-f:]+)`", text)
if not m:
    raise SystemExit("нет отпечатка SHA-256 в docs/SECURITY.md")
print(m.group(1).replace(":", "").lower())
PY
)" || fail "не прочитан отпечаток сертификата"

command -v apksigner >/dev/null || fail "нет apksigner"
command -v aapt >/dev/null || command -v apkanalyzer >/dev/null || fail "нет aapt и apkanalyzer"

CERTS="$(apksigner verify --print-certs "$APK" | python3 -c '
import re, sys
vals = [x.replace(":", "").lower() for x in re.findall(r"SHA-256 digest:\s*([0-9A-Fa-f:]+)", sys.stdin.read())]
print("\n".join(vals))
')" || fail "apksigner не подтвердил подпись APK"
printf '%s\n' "$CERTS" | grep -qx "$EXPECT" || fail "сертификат APK не совпадает с docs/SECURITY.md"

if command -v aapt >/dev/null; then
	BADGING="$(aapt dump badging "$APK")" || fail "aapt не прочитал APK"
	PARSED="$(printf '%s\n' "$BADGING" | python3 -c '
import re, sys
text = sys.stdin.read()
m = re.search(r"package: name=\x27([^\x27]+)\x27 versionCode=\x27([0-9]+)\x27 versionName=\x27([^\x27]*)\x27", text)
if not m:
    raise SystemExit(1)
print("%s %s %s" % (m.group(1), m.group(2), m.group(3)))
')" || fail "aapt не нашёл package/versionCode/versionName"
	APK_PKG="${PARSED%% *}"
	rest="${PARSED#* }"
	APK_CODE="${rest%% *}"
	APK_NAME="${rest#* }"
else
	APK_PKG="$(apkanalyzer manifest application-id "$APK")" || fail "apkanalyzer не прочитал package"
	APK_CODE="$(apkanalyzer manifest version-code "$APK")" || fail "apkanalyzer не прочитал versionCode"
	APK_NAME="$(apkanalyzer manifest version-name "$APK")" || fail "apkanalyzer не прочитал versionName"
fi
[[ "$APK_PKG" == "$PKG" ]] || fail "package id APK ($APK_PKG) не равен $PKG"
[[ "$APK_CODE" == "$CODE" ]] || fail "versionCode APK ($APK_CODE) не равен $CODE"
[[ "$APK_NAME" == "$NAME" ]] || fail "versionName APK ($APK_NAME) не равен $NAME"

SIZE="$(stat -c %s "$APK")"
[[ "$SIZE" -le $((95 * 1024 * 1024)) ]] || fail "APK больше 95 МБ"

FILE="zvezdnye-rubezhi-$NAME-$CODE.apk"
mkdir -p "$DIR"
if [[ -e "$DIR/latest.json" ]]; then
	PUBLISHED="$(python3 - "$DIR/latest.json" <<'PY'
import json, sys
try:
    print(int(json.load(open(sys.argv[1], encoding="utf-8"))["version_code"]))
except Exception:
    raise SystemExit(1)
PY
)" || fail "опубликованный манифест повреждён, публикация остановлена"
	if [[ "$PUBLISHED" -eq "$CODE" ]]; then
		fail "versionCode $CODE уже опубликован, повтор и другие байты того же кода запрещены"
	fi
	if [[ "$PUBLISHED" -gt "$CODE" ]]; then
		fail "откат версии: опубликован code $PUBLISHED, запрошен $CODE"
	fi
fi
if [[ -e "$DIR/$FILE" ]]; then
	fail "файл $FILE уже есть, перезапись того же versionCode запрещена"
fi
[[ -f "$KEY" ]] || fail "нет ключа подписи манифеста"

STAGE="$(mktemp -d)"
cp "$APK" "$STAGE/$FILE"
SHA="$(sha256sum "$STAGE/$FILE" | cut -d' ' -f1)"
python3 - "$STAGE/latest.json" "$NAME" "$CODE" "$MIN" "$BASE_URL/$FILE" "$SHA" "$SIZE" "$NOTES" <<'PY'
import json, sys, datetime
out, name, code, mn, url, sha, size, notes = sys.argv[1:9]
manifest = {
    "version_name": name, "version_code": int(code), "min_version_code": int(mn),
    "url": url, "sha256": sha, "size": int(size), "notes": notes,
    "published": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
}
open(out, "w", encoding="utf-8").write(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
PY
openssl dgst -sha256 -sign "$KEY" "$STAGE/latest.json" | base64 -w0 > "$STAGE/latest.json.sig"
openssl dgst -sha256 -verify "$PUB" -signature <(base64 -d "$STAGE/latest.json.sig") "$STAGE/latest.json" >/dev/null \
	|| fail "подпись манифеста не сошлась с публичным ключом клиента"

if [[ -f "$DIR/latest.json" ]]; then
	cp -a "$DIR/latest.json" "$DIR/latest.json.prev"
	[[ -f "$DIR/latest.json.sig" ]] && cp -a "$DIR/latest.json.sig" "$DIR/latest.json.sig.prev"
fi
mv "$STAGE/$FILE" "$DIR/$FILE"
mv "$STAGE/latest.json" "$DIR/latest.json"
mv "$STAGE/latest.json.sig" "$DIR/latest.json.sig"
echo "опубликовано: версия $NAME ($CODE), $SIZE байт, sha256 ${SHA:0:16}…, файл $FILE"
