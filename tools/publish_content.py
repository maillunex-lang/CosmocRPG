#!/usr/bin/env python3
"""Публикация пакета контента (.pck) для докачки в игре без переустановки APK.

Использование: tools/publish_content.py <id_пакета> <версия> <файл> [<файл> ...]
  файлы — пути относительно client/ или res://…; только каталог assets/ и только данные, модели, текстуры, звук
  (скрипты, сцены, шейдеры, .tres/.res и бинарные библиотеки не принимаются: пакеты не должны содержать исполняемого кода).
Переменные: CONTENT_DIR (по умолчанию ~/apk-share/content), MIN_APP (минимальный код приложения; по умолчанию текущий),
  MANIFEST_KEY (по умолчанию ~/keys/cosmoc/manifest_private.pem), GODOT (по умолчанию godot).
Что делает: собирает .pck через экспорт Godot (импортированные ресурсы упаковываются правильно), считает SHA-256, кладёт в каталог раздачи,
обновляет подписанный манифест content.json (+ content.json.sig). Версия пакета только растёт."""
import base64, datetime, hashlib, json, os, re, shutil, subprocess, sys, tempfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
CLIENT = os.path.join(ROOT, "client")
RESOURCE_EXT = {".glb", ".gltf", ".png", ".jpg", ".jpeg", ".webp", ".svg", ".ogg", ".mp3", ".wav"}   # импортируемые ресурсы
DATA_EXT = {".json", ".txt", ".csv"}                                                                # данные «как есть»
MAX_PACK = 64 * 1024 * 1024
BASE_URL = os.environ.get("CONTENT_BASE_URL", "https://rikagro.ru/download/content")   # переопределяется только для локальных проверок


def fail(msg):
    print("ОШИБКА:", msg, file=sys.stderr)
    sys.exit(1)


def main():
    if len(sys.argv) < 4:
        fail(__doc__)
    pid, ver, files = sys.argv[1], sys.argv[2], sys.argv[3:]
    if not re.fullmatch(r"[a-z0-9_-]{1,32}", pid) or not ver.isdigit() or int(ver) < 1:
        fail("id: латиница, цифры, _ и -; версия: целое число от 1")
    ver = int(ver)
    res, data = [], []
    for f in files:
        rel = f[6:] if f.startswith("res://") else f
        rel = os.path.normpath(rel).replace("\\", "/")
        if rel.startswith("..") or os.path.isabs(rel) or not rel.startswith("assets/"):
            fail(f"{f}: разрешены только файлы внутри client/assets/")
        ext = os.path.splitext(rel)[1].lower()
        if not os.path.isfile(os.path.join(CLIENT, rel)):
            fail(f"{f}: файл не найден")
        if ext in RESOURCE_EXT:
            res.append("res://" + rel)
        elif ext in DATA_EXT:
            data.append(rel)
        else:
            fail(f"{f}: тип {ext or '(нет)'} не допускается в пакетах контента")
    out_dir = os.environ.get("CONTENT_DIR", os.path.expanduser("~/apk-share/content"))
    key = os.environ.get("MANIFEST_KEY", os.path.expanduser("~/keys/cosmoc/manifest_private.pem"))
    godot = os.environ.get("GODOT", "godot")
    pub = os.path.join(CLIENT, "assets", "update_public.pem")
    if not os.path.isfile(key):
        fail("нет приватного ключа подписи манифеста")
    min_app = int(os.environ.get("MIN_APP", "0") or 0)
    if min_app == 0:
        m = re.search(r"^version_code=(\d+)", open(os.path.join(CLIENT, "project.godot"), encoding="utf-8").read(), re.M)
        min_app = int(m.group(1)) if m else 1
    os.makedirs(out_dir, exist_ok=True)
    manifest_path = os.path.join(out_dir, "content.json")
    manifest = {"content_version": 0, "packs": []}
    if os.path.isfile(manifest_path):
        manifest = json.load(open(manifest_path, encoding="utf-8"))
    for p in manifest["packs"]:
        if p["id"] == pid and ver <= int(p["version"]):
            fail(f"версия {ver} пакета {pid} не выше опубликованной ({p['version']})")

    # временный пресет экспорта: только выбранные файлы
    presets = os.path.join(CLIENT, "export_presets.cfg")
    original = open(presets, encoding="utf-8").read()
    mod = re.sub(r'^export_filter=.*$', 'export_filter="resources"', original, count=1, flags=re.M)
    mod = re.sub(r'^include_filter=.*$', 'include_filter="%s"' % ",".join(data), mod, count=1, flags=re.M)
    files_line = "export_files=PackedStringArray(%s)" % ", ".join('"%s"' % r for r in res)
    if re.search(r'^export_files=.*$', mod, re.M):
        mod = re.sub(r'^export_files=.*$', files_line, mod, count=1, flags=re.M)
    else:
        mod = mod.replace('export_filter="resources"\n', 'export_filter="resources"\n' + files_line + "\n", 1)
    mod = re.sub(r'^exclude_filter=.*$', 'exclude_filter=""', mod, count=1, flags=re.M)
    tmp_pck = os.path.join(tempfile.mkdtemp(), f"{pid}-{ver}.pck")
    try:
        open(presets, "w", encoding="utf-8").write(mod)
        env_cmd = f'. "{os.path.expanduser("~/keys/cosmoc/release.env")}" 2>/dev/null; "{godot}" --headless --path "{CLIENT}" --export-pack "Android" "{tmp_pck}"'
        r = subprocess.run(["bash", "-c", env_cmd], capture_output=True, text=True, timeout=900)
    finally:
        open(presets, "w", encoding="utf-8").write(original)   # пресет всегда возвращается как был
    if not os.path.isfile(tmp_pck):
        fail("экспорт пакета не удался:\n" + (r.stdout + r.stderr)[-800:])
    size = os.path.getsize(tmp_pck)
    if size > MAX_PACK:
        fail(f"пакет {size} байт больше лимита {MAX_PACK}")
    sha = hashlib.sha256(open(tmp_pck, "rb").read()).hexdigest()
    final = os.path.join(out_dir, f"{pid}-{ver}.pck")
    shutil.copyfile(tmp_pck, final + ".tmp")
    os.replace(final + ".tmp", final)
    entry = {"id": pid, "version": ver, "url": f"{BASE_URL}/{pid}-{ver}.pck", "sha256": sha, "size": size, "min_app_code": min_app,
             "files": [r_[6:] for r_ in res] + data}
    manifest["packs"] = sorted([p for p in manifest["packs"] if p["id"] != pid] + [entry], key=lambda p: p["id"])
    manifest["content_version"] = int(manifest["content_version"]) + 1
    manifest["published"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    open(manifest_path + ".tmp", "w", encoding="utf-8").write(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    sig = subprocess.run(["openssl", "dgst", "-sha256", "-sign", key, manifest_path + ".tmp"], capture_output=True, check=True).stdout
    open(manifest_path + ".sig.tmp", "w").write(base64.b64encode(sig).decode())
    subprocess.run(["openssl", "dgst", "-sha256", "-verify", pub, "-signature", "/dev/stdin", manifest_path + ".tmp"], input=sig, capture_output=True, check=True)
    os.replace(manifest_path + ".tmp", manifest_path)
    os.replace(manifest_path + ".sig.tmp", manifest_path + ".sig")
    print(f"опубликован пакет {pid} v{ver}: {size} байт, sha256 {sha[:16]}…, файлов {len(res) + len(data)}; версия контента {manifest['content_version']}")


main()
