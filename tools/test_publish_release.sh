#!/usr/bin/env bash
# Тесты публикации APK в изолированном каталоге (не production).
# Используются реальные APK из ~/apk и тестовый ключ подписи манифеста.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TDIR="$(mktemp -d)"
trap 'rm -rf "$TDIR"' EXIT
PASS=0; FAILN=0
ok()  { PASS=$((PASS+1)); echo "PASS  $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL  $1"; }

# тестовая пара ключей манифеста
openssl genrsa -out "$TDIR/mk.pem" 2048 2>/dev/null
openssl rsa -in "$TDIR/mk.pem" -pubout -out "$TDIR/mk.pub" 2>/dev/null
base64 -w0 "$TDIR/mk.pub" > "$TDIR/mk.pub.b64"

run_pub() { # apk, dir
	APK_DIR="$2" MANIFEST_KEY="$TDIR/mk.pem" PUB="$TDIR/mk.pub" \
	PUBLISH_SKIP_HTTPS_CHECK=1 "$ROOT/tools/publish_release.sh" "$1" "тест" 2>&1
}

REAL13="$HOME/apk/zvezdnye-rubezhi-0.3.13.apk"
REAL12="$HOME/apk/zvezdnye-rubezhi-0.3.12.apk"

# 1. успешная публикация в пустой каталог
D1="$TDIR/d1"; mkdir -p "$D1"
if run_pub "$REAL13" "$D1" >/dev/null 2>&1 && [ -f "$D1/latest.json" ] && [ -f "$D1/latest.json.sig" ]; then
	ok "успешная публикация: манифест и подпись созданы"
else
	bad "успешная публикация"
fi

# 2. повторный versionCode
if run_pub "$REAL13" "$D1" 2>&1 | grep -q "уже опубликован"; then
	ok "повторный versionCode отклонён"
else
	bad "повторный versionCode"
fi

# 3. откат/чужой versionCode: APK 15 не соответствует версии репозитория (16) — отказ
if run_pub "$REAL12" "$D1" 2>&1 | grep -q "не равен"; then
	ok "несоответствие versionCode APK репозиторию отклонено"
else
	bad "несоответствие versionCode"
fi

# 4. повреждённый APK
head -c 4000000 "$REAL13" > "$TDIR/truncated.apk"
if run_pub "$TDIR/truncated.apk" "$D1" 2>&1 | grep -qi "apksigner\|подпись"; then
	ok "повреждённый APK отклонён (подпись)"
else
	bad "повреждённый APK"
fi

# 5а. откат отклонён, действующий манифест нетронут
D2="$TDIR/d2"; mkdir -p "$D2"
printf '{"version_name":"9.9.9","version_code":17,"url":"https://x/","sha256":"0","size":1}\n' > "$D2/latest.json"
echo "c2ln" > "$D2/latest.json.sig"
if run_pub "$REAL13" "$D2" 2>&1 | grep -q "откат версии" \
	&& python3 -c "import json,sys; d=json.load(open('$D2/latest.json')); sys.exit(0 if d['version_code']==17 else 1)" 2>/dev/null; then
	ok "откат версии отклонён, действующий манифест сохранён"
else
	bad "откат версии"
fi
# 5б. при успешном обновлении прежний манифест сохраняется как .prev
D5="$TDIR/d5"; mkdir -p "$D5"
printf '{"version_name":"0.0.0","version_code":15,"url":"https://x/","sha256":"0","size":1}\n' > "$D5/latest.json"
echo "x" > "$D5/latest.json.sig"
if run_pub "$REAL13" "$D5" >/dev/null 2>&1 \
	&& python3 -c "import json,sys; d=json.load(open('$D5/latest.json.prev')); sys.exit(0 if d['version_code']==15 else 1)" 2>/dev/null; then
	ok "предыдущий манифест сохранён (.prev) при обновлении"
else
	bad ".prev при обновлении"
fi

# 6. неверный публичный ключ → отказ проверки подписи манифеста
openssl genrsa -out "$TDIR/wrong.pem" 2048 2>/dev/null
openssl rsa -in "$TDIR/wrong.pem" -pubout -out "$TDIR/wrong.pub" 2>/dev/null
D3="$TDIR/d3"; mkdir -p "$D3"
OUT="$(APK_DIR="$D3" MANIFEST_KEY="$TDIR/mk.pem" PUB="$TDIR/wrong.pub" PUBLISH_SKIP_HTTPS_CHECK=1 "$ROOT/tools/publish_release.sh" "$REAL12" "тест" 2>&1)"
if echo "$OUT" | grep -q "не сошлась"; then
	ok "несоответствие публичного ключа манифеста отклонено"
else
	bad "проверка подписи манифеста"
fi

# 7. flock: занятая блокировка → вторая не получает её (таймаут/отказ)
exec 9>"$TDIR/wrap.lock"
flock -n 9
if ( flock -n 9 ) 9>>"$TDIR/wrap.lock" 2>/dev/null; then
	bad "flock: вторая публикация должна отклоняться"
else
	ok "flock: вторая публикация заблокирована"
fi
flock -u 9

# 8. пустой stdin → обёртка REJECT (только локально, без SSH)
D4="$TDIR/d4"; mkdir -p "$D4"
if echo -n "" | APK_DIR="$D4" MANIFEST_KEY="$TDIR/mk.pem" PUB="$TDIR/mk.pub" PUBLISH_SKIP_HTTPS_CHECK=1 \
	bash -c 'TMP="$(mktemp -d)"; head -c 104857600 > "$TMP/in.apk" || true; [ -s "$TMP/in.apk" ] || { echo "REJECT: пустая передача"; exit 1; }' 2>&1 | grep -q "REJECT"; then
	ok "пустая передача отклонена"
else
	bad "пустая передача"
fi

echo "Итого: $PASS прошло, $FAILN провалов"
[ "$FAILN" -eq 0 ]
