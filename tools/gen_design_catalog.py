#!/usr/bin/env python3
"""Генератор встроенного каталога элементов дизайна для админки: server/internal/design/builtin.json.

Каталог строится из файлов клиента (client/assets): иконки, фоны космоса, 3D-модели и паспорта кораблей. Ключ элемента не
привязан к пути файла: «model:FTR-001:a» — модель облика «a» корабля FTR-001. Поэтому замена модели одного корабля не затрагивает
остальные (11 из 12 кораблей пока используют заглушку «Стрижа»). Запуск: python3 tools/gen_design_catalog.py"""
import json
import re
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ASSETS = ROOT / "client" / "assets"
OUT = ROOT / "server" / "internal" / "design" / "builtin.json"


def dims(path: Path):
    out = subprocess.run(["file", "-b", str(path)], capture_output=True, text=True).stdout
    m = re.search(r"(\d{2,5}) ?x ?(\d{2,5})", out)
    return [int(m.group(1)), int(m.group(2))] if m else None


def res(path: Path) -> str:
    return "res://" + str(path.relative_to(ROOT / "client")).replace("\\", "/")


def main() -> None:
    els = []
    for svg in sorted((ASSETS / "icons").glob("*.svg")):
        text = svg.read_text(encoding="utf-8")
        els.append({"id": f"icon:{svg.stem}", "category": "icon", "name": svg.stem, "group": "Иконки интерфейса",
                    "tags": ["интерфейс", "svg"], "builtin": res(svg), "bytes": svg.stat().st_size,
                    "preview_svg": text if len(text) <= 2000 else ""})
    bg_json = json.loads((ASSETS / "space" / "backgrounds.json").read_text(encoding="utf-8"))
    used = {}
    for prof in bg_json.get("profiles", []):
        for layer in prof.get("layers", []):
            used.setdefault(layer["texture"], []).append(prof["name"])
    for key, path in sorted(bg_json["textures"].items()):
        p = ROOT / "client" / path.replace("res://", "")
        els.append({"id": f"background:{key}", "category": "background", "name": key, "group": "Фоны космоса",
                    "tags": ["космос", "слой фона"], "builtin": path, "bytes": p.stat().st_size, "dims": dims(p),
                    "used_by": sorted(set(used.get(key, [])))})
    for sj in sorted((ASSETS / "ships").glob("*/ship.json")):
        d = json.loads(sj.read_text(encoding="utf-8"))
        sid, sname = d["id"], d.get("name", d["id"])
        own_dir = sj.parent.name
        els.append({"id": f"ship_data:{sid}", "category": "ship_data", "name": f"Паспорт внешнего вида: {sname}", "group": "Паспорта кораблей",
                    "tags": ["корабль", sid, d.get("class", "")], "builtin": res(sj), "bytes": sj.stat().st_size,
                    "ship": sid, "class": d.get("class", ""), "current": json.dumps(d, ensure_ascii=False, separators=(",", ":")),
                    "note": "Облики, точки привязки (двигатели, оружие, стыковка), масштаб, подписи способностей"})
        for skin, sk in sorted(d.get("skins", {}).items()):
            glb = sk.get("glb", "")
            p = ROOT / "client" / glb.replace("res://", "")
            placeholder = f"/ships/{own_dir}/" not in glb
            els.append({"id": f"model:{sid}:{skin}", "category": "model", "name": f"{sname} — {sk.get('name', skin)}", "group": "3D-модели кораблей",
                        "tags": ["корабль", sid, "заглушка" if placeholder else "своя модель", d.get("class", "")],
                        "builtin": glb, "bytes": p.stat().st_size if p.exists() else 0, "ship": sid, "skin": skin, "placeholder": placeholder, "class": d.get("class", ""),
                        "model_length": d.get("model_length"), "nose_axis": d.get("nose_axis")})
    els.sort(key=lambda e: (e["category"], e["id"]))
    OUT.write_text(json.dumps({"schema": "Cosmoc_Design_Catalog_v1", "elements": els}, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    by = {}
    for e in els:
        by[e["category"]] = by.get(e["category"], 0) + 1
    print("элементов:", len(els), by)


if __name__ == "__main__":
    main()
