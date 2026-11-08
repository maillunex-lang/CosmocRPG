#!/usr/bin/env bash
# Использование: tools/screenshot.sh <WxH> <out.png> [res://scene.tscn]
# Скрипт копируется в клиент на время запуска. Vulkan на VPS нет — Godot сам переключается на OpenGL3 (llvmpipe).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cp "$ROOT/tools/godot_screenshot.gd" "$ROOT/client/scripts/_shot.gd"
trap 'rm -f "$ROOT/client/scripts/_shot.gd" "$ROOT/client/scripts/_shot.gd.uid"' EXIT
SHOT="$(realpath -m "$2")" SCENE="${3:-}" timeout 120 xvfb-run -a -s "-screen 0 1920x1080x24" \
  godot --path "$ROOT/client" --resolution "$1" --script res://scripts/_shot.gd 2>&1 | grep -E "SHOT_SAVED|SCRIPT ERROR" || true
