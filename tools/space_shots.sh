#!/usr/bin/env bash
# Скриншоты экрана «Космос» в состояниях: покой, цель, бой с тревогами, подтверждение атаки.
# Использование: tools/space_shots.sh <WxH> <каталог> [л,в,п,н]   (последнее: имитация выреза в px окна)
# Переменные окружения: SYSTEM=0|1|2 (система демо), NOHUD=1 (только фон), ONLY_IDLE=1 (один кадр)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$2"
SHOT_DIR="$(realpath "$2")" INSETS="${3:-}" timeout 180 xvfb-run -a -s "-screen 0 2560x1440x24" \
  godot --path "$ROOT/client" --resolution "$1" --script res://tests/space_shots.gd 2>&1 | grep -E "SHOT_SAVED|SCRIPT ERROR" || true
