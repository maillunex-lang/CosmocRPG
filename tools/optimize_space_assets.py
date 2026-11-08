#!/usr/bin/env python3
"""Оптимизация фоновых изображений космоса для клиента.

Исходники (большие PNG) лежат ВНЕ репозитория; в клиент попадают только оптимизированные WebP.
Требуется: python3 + numpy, ImageMagick (convert, identify), ffmpeg с libwebp.

Использование:
  tools/optimize_space_assets.py --src <каталог с исходниками> --out client/assets/space/backgrounds

Ожидаемые исходники в --src: stars.png, nebula.png, gas.png (RGBA).
Слои и их назначение: docs/ui/ART_STANDARD.md. Размеры и качество подобраны так, чтобы при
сжатии VRAM (ASTC/ETC2 при импорте в Godot) APK почти не вырос.
"""
import argparse
import os
import subprocess
import sys
import tempfile

import numpy as np

# имя -> (исходник, ширина, высота или None, качество WebP, есть альфа)
SPECS = {
    "stars_deep": ("stars.png", None, None, 90, False),            # звёзды: исходный размер, мелкие точки важны
    "nebula_blue_violet": ("nebula.png", 1536, 864, 85, False),    # туманность мягкая: чуть меньше
    "gas_turquoise": ("gas.png", 1024, 576, 85, True),             # газ очень мягкий: вдвое меньше
}
MAX_SIDE = 2048          # ни одна текстура не больше (память GPU на слабых телефонах)
MAX_BYTES = 400 * 1024   # на файл


def run(cmd, **kw):
    return subprocess.run(cmd, check=True, capture_output=True, **kw)


def size_of(path: str):
    w, h = run(["identify", "-format", "%w %h", path + "[0]"]).stdout.decode().split()
    return int(w), int(h)


def load_rgba(path: str) -> np.ndarray:
    w, h = size_of(path)
    raw = run(["convert", path + "[0]", "-depth", "8", "rgba:-"]).stdout
    return np.frombuffer(raw, dtype=np.uint8).reshape(h, w, 4).astype(np.float32) / 255.0


def save_png(arr: np.ndarray, path: str) -> None:
    h, w = arr.shape[:2]
    raw = (np.clip(arr, 0, 1) * 255 + 0.5).astype(np.uint8).tobytes()
    run(["convert", "-size", f"{w}x{h}", "-depth", "8", "rgba:-", path], input=raw)


def grade_gas(rgba: np.ndarray) -> np.ndarray:
    """Мягкая цветокоррекция газа под художественный стандарт: оттенки только от бирюзового до фиолетового.

    Зеленоватые края (оттенок ниже 178°) сдвигаются к бирюзе, розово-малиновые (выше 295°) к фиолетовому.
    Яркость и прозрачность не меняются.
    """
    rgb = rgba[..., :3]
    mx = rgb.max(-1)
    mn = rgb.min(-1)
    d = mx - mn
    safe = np.where(d > 1e-6, d, 1.0)
    r, g, b = rgb[..., 0], rgb[..., 1], rgb[..., 2]
    h = np.zeros_like(mx)
    h = np.where((d > 1e-6) & (mx == r), ((g - b) / safe) % 6, h)
    h = np.where((d > 1e-6) & (mx == g), (b - r) / safe + 2, h)
    h = np.where((d > 1e-6) & (mx == b), (r - g) / safe + 4, h)
    h = (h * 60.0) % 360.0
    s = np.where(mx > 0, d / np.where(mx > 0, mx, 1.0), 0.0)
    v = mx
    h2 = np.where(h < 178.0, h + (178.0 - h) * 0.85, h)
    h2 = np.where(h2 > 295.0, h2 - (h2 - 295.0) * 0.6, h2)
    # HSV -> RGB
    hh = (h2 / 60.0) % 6
    c = v * s
    x = c * (1 - np.abs(hh % 2 - 1))
    m = v - c
    z = np.zeros_like(c)
    sector = hh.astype(int)
    out = np.stack([
        np.choose(sector, [c, x, z, z, x, c]),
        np.choose(sector, [x, c, c, x, z, z]),
        np.choose(sector, [z, z, x, c, c, x]),
    ], axis=-1) + m[..., None]
    result = rgba.copy()
    result[..., :3] = out
    return result


def encode_webp(src_png: str, dst: str, quality: int, alpha: bool) -> None:
    pix = "yuva420p" if alpha else "yuv420p"
    run(["ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", src_png,
         "-c:v", "libwebp", "-quality", str(quality), "-compression_level", "6", "-pix_fmt", pix, dst])


def ssim(a: str, b: str, w: int, h: int) -> float:
    out = subprocess.run(
        ["ffmpeg", "-hide_banner", "-loglevel", "info", "-i", a, "-i", b, "-lavfi",
         f"[0:v]scale={w}:{h}[x];[1:v]scale={w}:{h}[y];[x][y]ssim", "-f", "null", "-"],
        capture_output=True, text=True).stderr
    for token in out.split():
        if token.startswith("All:"):
            return float(token[4:])
    return 0.0


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)
    total_before = total_after = 0
    ok = True
    with tempfile.TemporaryDirectory() as tmp:
        for name, (src_name, w, h, q, alpha) in SPECS.items():
            src = os.path.join(args.src, src_name)
            if not os.path.exists(src):
                print(f"ОШИБКА: нет исходника {src}", file=sys.stderr)
                ok = False
                continue
            sw, sh = size_of(src)
            tw, th = (w or sw), (h or sh)
            work = os.path.join(tmp, name + ".png")
            run(["convert", src + "[0]", "-filter", "Lanczos", "-resize", f"{tw}x{th}!", "-depth", "8", work]
                if (tw, th) != (sw, sh) else ["convert", src + "[0]", "-depth", "8", work])
            if name.startswith("gas"):
                save_png(grade_gas(load_rgba(work)), work)
            dst = os.path.join(args.out, name + ".webp")
            encode_webp(work, dst, q, alpha)
            before, after = os.path.getsize(src), os.path.getsize(dst)
            total_before += before
            total_after += after
            quality = ssim(dst, work, tw, th)
            flag = ""
            if max(tw, th) > MAX_SIDE or after > MAX_BYTES:
                flag = "  <-- ПРЕВЫШЕН ЛИМИТ"
                ok = False
            print(f"{name}: {sw}x{sh} -> {tw}x{th}, {before/1024:.0f} КБ -> {after/1024:.0f} КБ, SSIM {quality:.3f}{flag}")
    print(f"Итого: {total_before/1024:.0f} КБ -> {total_after/1024:.0f} КБ (в {total_before/max(1,total_after):.0f} раз меньше)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
