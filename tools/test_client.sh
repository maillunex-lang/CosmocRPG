#!/usr/bin/env bash
# Тесты клиента Godot против запущенного сервера.
# Использование: COSMOC_TEST_WS_URL=ws://127.0.0.1:8080/ws tools/test_client.sh [каталог_скриншотов]
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
: "${COSMOC_TEST_WS_URL:?задайте COSMOC_TEST_WS_URL, например ws://127.0.0.1:8080/ws}"
rc=0
echo "== сетевой слой"
timeout 120 godot --headless --path "$ROOT/client" --script res://tests/ws_client_test.gd 2>&1 | grep -vE "^Godot Engine|^$|WARNING: WsClient|^\s+(at:|GDScript|\[[0-9]+\])" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1
echo "== экран подключения"
[ -n "${1:-}" ] && mkdir -p "$1" && export SHOT_DIR="$(realpath "$1")"
timeout 120 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --script res://tests/connect_screen_test.gd 2>&1 | grep -E "^(PASS|FAIL|SHOT_SAVED|Итого|SCRIPT ERROR)" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1
exit $rc
