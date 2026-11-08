#!/usr/bin/env bash
# Тесты клиента Godot.
#  1) Интерфейс (тема, контраст, безопасная зона, раскладка, компоненты): сервер не нужен.
#  2) Сеть и экран подключения: нужен запущенный сервер.
# Использование: COSMOC_TEST_WS_URL=ws://127.0.0.1:8080/ws tools/test_client.sh [каталог_скриншотов]
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
: "${COSMOC_TEST_WS_URL:?задайте COSMOC_TEST_WS_URL, например ws://127.0.0.1:8080/ws}"
rc=0

echo "== интерфейс: тема, безопасная зона, раскладка, компоненты, шлюз, HUD космоса"
for t in theme_test safe_area_test layout_test components_test demo_gateway_test hud_test background_test art_standard_test frame_stats_test camera_test camera_input_test labels_test loading_overlay_test; do
  echo "-- $t"
  timeout 180 godot --headless --path "$ROOT/client" --script "res://tests/$t.gd" 2>&1 \
    | grep -E "^(FAIL|Итого|SCRIPT ERROR|ERROR)"
  [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1
done

echo "== камера: реальная отрисовка обязательных видов (нужен рендеринг)"
timeout 280 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --script res://tests/camera_render_test.gd 2>&1 | grep -E "^(FAIL|Итого|SCRIPT ERROR|ERROR)"
[ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

echo "== фон космоса: реальная отрисовка шейдерами (нужен рендеринг)"
timeout 280 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --script res://tests/background_render_test.gd 2>&1 | grep -E "^(FAIL|Итого|SCRIPT ERROR|ERROR)"
[ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

echo "== сетевой слой"
timeout 120 godot --headless --path "$ROOT/client" --script res://tests/ws_client_test.gd 2>&1 | grep -vE "^Godot Engine|^$|WARNING: WsClient|^\s+(at:|GDScript|\[[0-9]+\])" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

echo "== экран подключения"
[ -n "${1:-}" ] && mkdir -p "$1" && export SHOT_DIR="$(realpath "$1")"
timeout 120 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --script res://tests/connect_screen_test.gd 2>&1 | grep -E "^(PASS|FAIL|SHOT_SAVED|Итого|SCRIPT ERROR)" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1
exit $rc
