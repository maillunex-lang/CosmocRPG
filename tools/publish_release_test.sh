#!/usr/bin/env bash
# Проверки tools/publish_release.sh без настоящего APK, ключа и каталога раздачи.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/tools/publish_release.sh"
EXPECT="$(python3 - "$ROOT/docs/SECURITY.md" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
m = re.search(r"Отпечаток сертификата SHA-256[^\n]*`([0-9A-Fa-f:]+)`", text)
if not m:
    raise SystemExit("нет отпечатка")
print(m.group(1).replace(":", "").lower())
PY
)"
fail=0
pass=0
ok() { pass=$((pass + 1)); echo "PASS  $1"; }
bad() { fail=$((fail + 1)); echo "FAIL  $1"; }

run() {
	local dir="$1"
	shift
	set +e
	out="$(APK_DIR="$dir" MANIFEST_KEY="/no/such/manifest_private.pem" PATH="$BIN:$PATH" \
		bash "$SCRIPT" "$APK" "проверка" 2>&1)"
	rc=$?
	set -e
}

BIN="$(mktemp -d)"
APK="$(mktemp)"
echo "not-a-real-apk" > "$APK"
cat > "$BIN/apksigner" <<'SH'
#!/bin/sh
echo "Signer #1 certificate SHA-256 digest: ${FAKE_CERT}"
exit "${FAKE_APKSIGNER_RC:-0}"
SH
cat > "$BIN/aapt" <<'SH'
#!/bin/sh
echo "package: name='${FAKE_PKG:-ru.cosmocrpg.game}' versionCode='${FAKE_CODE:-15}' versionName='${FAKE_NAME:-0.3.12}' platformBuildVersionName='14'"
SH
chmod +x "$BIN/apksigner" "$BIN/aapt" "$SCRIPT"
export FAKE_CERT="$EXPECT" FAKE_PKG="ru.cosmocrpg.game" FAKE_CODE="15" FAKE_NAME="0.3.12"

# Сертификат другой: манифест не создаётся.
DIR="$(mktemp -d)"
FAKE_CERT="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
run "$DIR"
[[ "$rc" -ne 0 && "$out" == *"сертификат"* && ! -e "$DIR/latest.json" ]] && ok "чужой сертификат отвергается" || bad "чужой сертификат: rc=$rc $out"
FAKE_CERT="$EXPECT"

# versionCode APK не равен проекту.
DIR="$(mktemp -d)"
FAKE_CODE="14"
run "$DIR"
[[ "$rc" -ne 0 && "$out" == *"versionCode"* && ! -e "$DIR/latest.json" ]] && ok "чужой versionCode APK отвергается" || bad "чужой versionCode: rc=$rc $out"
FAKE_CODE="15"

# package id.
DIR="$(mktemp -d)"
FAKE_PKG="evil.example.game"
run "$DIR"
[[ "$rc" -ne 0 && "$out" == *"package"* && ! -e "$DIR/latest.json" ]] && ok "чужой package id отвергается" || bad "чужой package: rc=$rc $out"
FAKE_PKG="ru.cosmocrpg.game"

# Тот же versionCode, другие байты: файл и манифест не перезаписываются.
DIR="$(mktemp -d)"
printf '%s\n' '{"version_name":"0.3.12","version_code":15,"min_version_code":1}' > "$DIR/latest.json"
printf 'old-bytes' > "$DIR/zvezdnye-rubezhi-0.3.12-15.apk"
run "$DIR"
now="$(cat "$DIR/zvezdnye-rubezhi-0.3.12-15.apk")"
code="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version_code"])' "$DIR/latest.json")"
[[ "$rc" -ne 0 && "$out" == *"versionCode 15"* && "$now" == "old-bytes" && "$code" == "15" ]] \
	&& ok "повтор того же versionCode и другие байты отвергаются" || bad "повтор code: rc=$rc $out"

# Откат ниже опубликованного манифеста.
DIR="$(mktemp -d)"
printf '%s\n' '{"version_name":"0.3.13","version_code":16,"min_version_code":1}' > "$DIR/latest.json"
run "$DIR"
code="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version_code"])' "$DIR/latest.json")"
[[ "$rc" -ne 0 && "$out" == *"откат"* && "$code" == "16" && ! -e "$DIR/zvezdnye-rubezhi-0.3.12-15.apk" ]] \
	&& ok "откат versionCode отвергается" || bad "откат: rc=$rc $out"

# Файл этого кода уже лежит, даже если манифест старше: не перезаписывать.
DIR="$(mktemp -d)"
printf '%s\n' '{"version_name":"0.3.11","version_code":14,"min_version_code":1}' > "$DIR/latest.json"
printf 'kept' > "$DIR/zvezdnye-rubezhi-0.3.12-15.apk"
run "$DIR"
now="$(cat "$DIR/zvezdnye-rubezhi-0.3.12-15.apk")"
code="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version_code"])' "$DIR/latest.json")"
[[ "$rc" -ne 0 && "$now" == "kept" && "$code" == "14" ]] && ok "готовый файл того же кода не перезаписывается" || bad "файл code: rc=$rc $out"

# Проверки APK проходят, ключа нет: манифест не появляется. Ключ не создаём.
DIR="$(mktemp -d)"
run "$DIR"
[[ "$rc" -ne 0 && "$out" == *"нет ключа"* && ! -e "$DIR/latest.json" ]] && ok "без ключа манифеста публикация не начинается" || bad "нет ключа: rc=$rc $out"

echo "Итого: проверок $pass, провалов $fail"
rm -rf "$BIN" "$APK"
exit "$([[ "$fail" -eq 0 ]] && echo 0 || echo 1)"
