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
for t in purchase_intent_test theme_test safe_area_test layout_test components_test demo_gateway_test hud_test profile_screen_test corporation_test background_test art_standard_test frame_stats_test camera_test camera_input_test labels_test loading_overlay_test ship_visual_test hangar_test fleet_shop_test world_decor_test map_screen_test star_map_screen_test start_splash_test updater_test nameplates_test settings_screen_test chat_panel_test content_test apk_installer_test auto_battle_test pilot_ui_test touch_scroll_test chat_overlay_test; do
  echo "-- $t"
  timeout 180 godot --headless --path "$ROOT/client" --script "res://tests/$t.gd" 2>&1 \
    | grep -E "^(FAIL|Итого|SCRIPT ERROR|ERROR)"
  [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1
done

echo "== Ships RPG: настоящий локальный WebSocket → ангар Стрижа (без БД)"
(cd "$ROOT/server" && COSMOC_TEST_GODOT=godot go test -count=1 ./internal/ws -run '^TestPassportGodotClient$') || rc=1

echo "== корпорации: настоящий WebSocket → создание/вступление/профиль/переподключение/выход"
(cd "$ROOT/server" && COSMOC_TEST_GODOT=godot go test -count=1 ./internal/ws -run '^TestCorporationGodotClient$') || rc=1

echo "== Ships RPG: сохранённый двигатель, настоящий WebSocket → PostgreSQL → Godot"
EQUIPMENT_TEST_DATABASE_URL="${COSMOC_TEST_DATABASE_URL:-${PG_URL:-}}"
if [ -n "$EQUIPMENT_TEST_DATABASE_URL" ]; then
  (cd "$ROOT/server" && COSMOC_TEST_DATABASE_URL="$EQUIPMENT_TEST_DATABASE_URL" COSMOC_TEST_GODOT=godot go test -race -count=1 ./internal/ws -run '^(TestEquipmentGodotClientPostgres|TestPurchaseGodotClientPostgres)$') || rc=1
else
  echo "SKIP  оборудование PostgreSQL: COSMOC_TEST_DATABASE_URL/PG_URL не заданы"
fi

echo "== камера: реальная отрисовка обязательных видов (Vulkan Mobile, аудио выключено)"
CAM_LOG="$(mktemp)"
timeout 900 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --audio-driver Dummy --script res://tests/camera_render_test.gd >"$CAM_LOG" 2>&1
CAM_RC=$?
grep -E "^(FAIL|Итого|SCRIPT ERROR|ERROR)" "$CAM_LOG" || true
if [ "$CAM_RC" -ne 0 ]; then rc=1; fi
if grep -qE "^(ERROR|SCRIPT ERROR)" "$CAM_LOG"; then echo "FAIL  runtime ERROR в логе camera_render_test"; rc=1; fi
rm -f "$CAM_LOG"

echo "== фон космоса: реальная отрисовка шейдерами (Vulkan Mobile, аудио выключено)"
BG_LOG="$(mktemp)"
timeout 280 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --audio-driver Dummy --script res://tests/background_render_test.gd >"$BG_LOG" 2>&1
BG_RC=$?
grep -E "^(FAIL|Итого|SCRIPT ERROR|ERROR)" "$BG_LOG" || true
if [ "$BG_RC" -ne 0 ]; then rc=1; fi
if grep -qE "^(ERROR|SCRIPT ERROR)" "$BG_LOG"; then echo "FAIL  runtime ERROR в логе background_render_test"; rc=1; fi
rm -f "$BG_LOG"

echo "== сетевой слой"
timeout 120 godot --headless --path "$ROOT/client" --script res://tests/ws_client_test.gd 2>&1 | grep -vE "^Godot Engine|^$|WARNING: WsClient|^\s+(at:|GDScript|\[[0-9]+\])" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

echo "== серверный шлюз и онлайн-космос"
timeout 120 godot --headless --path "$ROOT/client" --script res://tests/server_gateway_test.gd 2>&1 | grep -E "^(FAIL|Итого|SCRIPT ERROR|COSMOC_TEST)" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

# очистка только изолированной тестовой базы и только для локального сервера; production не затрагивается
case "${COSMOC_TEST_WS_URL:-}" in
  ws://127.0.0.1:*|ws://localhost:*) "$ROOT/tools/cleanup_test_players.sh" dev ;;
  "") ;;
  *) echo "(тесты шли не на локальный сервер: тестовых игроков очищайте вручную tools/cleanup_test_players.sh prod)" ;;
esac

echo "== аккаунт на экране подключения"
timeout 120 xvfb-run -a -s "-screen 0 1920x1080x24" godot --path "$ROOT/client" --resolution 1280x720 --script res://tests/account_ui_test.gd 2>&1 | grep -E "^(FAIL|Итого|SCRIPT ERROR|COSMOC_TEST)" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

echo "== создание персонажа"
timeout 120 godot --headless --path "$ROOT/client" --script res://tests/character_ui_test.gd 2>&1 | grep -E "^(FAIL|Итого|SCRIPT ERROR|COSMOC_TEST)" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1

echo "== экран подключения"
[ -n "${1:-}" ] && mkdir -p "$1" && export SHOT_DIR="$(realpath "$1")"
timeout 120 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution 1280x720 --script res://tests/connect_screen_test.gd 2>&1 | grep -E "^(PASS|FAIL|SHOT_SAVED|Итого|SCRIPT ERROR)" ; [ "${PIPESTATUS[0]}" -eq 0 ] || rc=1
exit $rc
