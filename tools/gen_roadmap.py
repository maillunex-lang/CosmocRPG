#!/usr/bin/env python3
"""Генератор сводок из docs/roadmap/tasks.json (единый источник данных).

Перегенерирует:
- docs/roadmap/STATUS.md — сводка статусов и прогресс по модулям;
- docs/roadmap/DEPENDENCIES.md — рёбра зависимостей;
- блок <!--GEN:SUMMARY--> в docs/ROADMAP.md (числа дашборда).

Прогресс = доля DONE-задач модуля в tasks.json. Это НЕ готовность игры.
Использование: python3 tools/gen_roadmap.py [корень репозитория]
"""
import json
import os
import sys
from collections import defaultdict

ROOT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TASKS = os.path.join(ROOT, "docs/roadmap/tasks.json")


def load():
    with open(TASKS, encoding="utf-8") as f:
        return json.load(f)


def status_md(data):
    tasks = data["tasks"]
    by_status = defaultdict(list)
    for t in tasks:
        by_status[t["status"]].append(t)
    lines = [
        "# Статус (генерируется tools/gen_roadmap.py — не править вручную)",
        "",
        f"Источник: `docs/roadmap/tasks.json` (обновлено {data.get('updated','?')}, SHA {data.get('last_verified_sha','?')}).",
        "",
        "## Сводка",
        "",
        "| Статус | Задач |",
        "|---|---|",
    ]
    for s in data["statuses"]:
        lines.append(f"| {s} | {len(by_status.get(s, []))} |")
    total = len(tasks)
    done = len(by_status.get("DONE", []))
    lines += ["", f"Всего: {total} · завершено: {done} · в работе: {len(by_status.get('IN_PROGRESS', []))} · "
                  f"заблокировано: {len(by_status.get('BLOCKED', []))} · ожидает: {len(by_status.get('TODO', []))} · "
                  f"отложено: {len(by_status.get('DEFERRED', []))}",
              "",
              "## Прогресс по модулям (по задачам, не готовность игры)",
              "",
              "| Модуль | DONE/всего |",
              "|---|---|"]
    by_mod = defaultdict(list)
    for t in tasks:
        by_mod[t["module"]].append(t)
    for m in data["modules"]:
        ts = by_mod.get(m, [])
        if not ts:
            continue
        d = sum(1 for t in ts if t["status"] == "DONE")
        lines.append(f"| {m} | {d}/{len(ts)} |")
    lines += ["", "## Активные работы", ""]
    for t in tasks:
        if t["status"] in ("IN_PROGRESS", "BLOCKED") or (t["status"] == "TODO" and t.get("owner")):
            owner = t.get("owner", "—")
            lines.append(f"- **{t['id']}** ({t['module']}, {t['priority']}, {owner}): {t['title']} — {t['status']}")
    lines.append("")
    return "\n".join(lines)


def deps_md(data):
    ids = {t["id"] for t in data["tasks"]}
    lines = [
        "# Зависимости (генерируется tools/gen_roadmap.py — не править вручную)",
        "",
        "Только рёбра между задачами `tasks.json`. Зависимости от внешних реестров (SR-*, R-*) указаны в карточках задач.",
        "",
        "| Задача | Зависит от |",
        "|---|---|",
    ]
    for t in data["tasks"]:
        for d in t.get("depends_on", []):
            mark = "" if d in ids else " (внешний реестр)"
            lines.append(f"| {t['id']} | {d}{mark} |")
    lines.append("")
    return "\n".join(lines)


def update_dashboard(data):
    path = os.path.join(ROOT, "docs/ROADMAP.md")
    with open(path, encoding="utf-8") as f:
        s = f.read()
    tasks = data["tasks"]
    by_status = defaultdict(int)
    for t in tasks:
        by_status[t["status"]] += 1
    total = len(tasks)
    block = (
        f"Всего задач в реестре: **{total}** · DONE **{by_status['DONE']}** · IN_PROGRESS **{by_status['IN_PROGRESS']}** · "
        f"BLOCKED **{by_status['BLOCKED']}** · TODO **{by_status['TODO']}** · DEFERRED **{by_status['DEFERRED']}** · "
        f"FAILED **{by_status['FAILED']}** · ожидают решения владельца: **6** (см. `roadmap/BLOCKERS.md`)"
    )
    start = "<!--GEN:SUMMARY-->"
    end = "<!--/GEN-->"
    i, j = s.index(start) + len(start), s.index(end)
    return s[:i] + "\n" + block + "\n" + s[j:]


def main():
    data = load()
    with open(os.path.join(ROOT, "docs/roadmap/STATUS.md"), "w", encoding="utf-8") as f:
        f.write(status_md(data))
    with open(os.path.join(ROOT, "docs/roadmap/DEPENDENCIES.md"), "w", encoding="utf-8") as f:
        f.write(deps_md(data))
    dashboard = update_dashboard(data)  # сначала вычислить, потом открывать на запись
    with open(os.path.join(ROOT, "docs/ROADMAP.md"), "w", encoding="utf-8") as f:
        f.write(dashboard)
    print("OK: STATUS.md, DEPENDENCIES.md, ROADMAP.md (GEN:SUMMARY) обновлены из tasks.json")


if __name__ == "__main__":
    main()
