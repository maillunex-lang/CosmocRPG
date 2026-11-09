#!/usr/bin/env python3
"""Валидатор документации дорожной карты (stdlib-only, read-only).

Проверяет: обязательные файлы, схему tasks.json, уникальность ID, статусы,
зависимости (в т.ч. внешние реестры ships-rpg и R-индекса), циклы, ссылки,
согласованность дашборда и сохранность ключевых разделов после миграции.

Не изменяет файлы. Код выхода: 0 — ок, 1 — найдены проблемы.
Использование: python3 tools/validate_roadmap.py [корень репозитория]
"""
import json
import os
import re
import sys

ROOT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

REQUIRED_FILES = [
    "docs/ROADMAP.md",
    "docs/HANDOFF.md",
    "docs/DECISIONS.md",
    "docs/AI_HANDOFF_PROMPT.md",
    "docs/roadmap/AI_START_HERE.md",
    "docs/roadmap/STATUS.md",
    "docs/roadmap/NOW.md",
    "docs/roadmap/tasks.json",
    "docs/roadmap/AGENT_TASKS.md",
    "docs/roadmap/BLOCKERS.md",
    "docs/roadmap/DEPENDENCIES.md",
    "docs/roadmap/REQUIREMENTS_INDEX.md",
    "docs/roadmap/HISTORY.md",
    "docs/roadmap/CHANGELOG.md",
    "docs/roadmap/SCHEMA.md",
    "docs/ships-rpg/ROADMAP.md",
    "docs/ships-rpg/ERROR_LOG.md",
    "docs/ships-rpg/REGRESSION_TESTS.md",
]

# Разделы, которые обязаны сохраниться где-либо в docs/ после миграции
KEY_SECTIONS = [
    "Источники требований",
    "Прототип (Решение №30)",
    "Открытые вопросы владельцу",
    "Перед публикацией",
    "Будущие этапы",
    "Ships RPG v2.1",
]

# Маркеры совместимости в ROADMAP.md (используются тестами)
COMPAT_MARKERS = ["Перед публикацией", "DEFAULT_VISIBLE"]


def fail(problems, msg):
    problems.append(msg)


def read(path):
    with open(os.path.join(ROOT, path), encoding="utf-8") as f:
        return f.read()


def main():
    problems = []

    for f in REQUIRED_FILES:
        if not os.path.isfile(os.path.join(ROOT, f)):
            fail(problems, f"нет обязательного файла: {f}")

    # tasks.json
    tasks_path = os.path.join(ROOT, "docs/roadmap/tasks.json")
    data = None
    if os.path.isfile(tasks_path):
        try:
            data = json.load(open(tasks_path, encoding="utf-8"))
        except Exception as e:
            fail(problems, f"tasks.json не парсится: {e}")
    tasks = (data or {}).get("tasks", [])
    modules = set((data or {}).get("modules", []))
    statuses = set((data or {}).get("statuses", []))
    rel_values = set((data or {}).get("release_values", []))

    norm = lambda x: x.upper()
    ids = {}
    for t in tasks:
        tid = t.get("id", "")
        if not tid:
            fail(problems, "задача без id")
            continue
        if tid in ids:
            fail(problems, f"дублирующийся id задачи: {tid}")
        ids[tid] = t
        for field in ("title", "module", "priority", "status"):
            if not t.get(field):
                fail(problems, f"{tid}: пустое поле {field}")
        if t.get("module") and t["module"] not in modules:
            fail(problems, f"{tid}: неизвестный модуль {t['module']}")
        if t.get("status") and t["status"] not in statuses:
            fail(problems, f"{tid}: некорректный статус {t['status']}")
        rel = t.get("release", {})
        for ch in ("server", "apk"):
            if ch in rel and rel[ch] not in rel_values:
                fail(problems, f"{tid}: некорректный release.{ch}={rel[ch]}")

    # внешние реестры ID (специализированные дорожные карты остаются источниками); регистр не важен
    external_ids = set()
    ships = os.path.join(ROOT, "docs/ships-rpg/ROADMAP.md")
    if os.path.isfile(ships):
        external_ids |= {norm(x) for x in re.findall(r"\bSR-\w+\b", read(ships))}
    req = os.path.join(ROOT, "docs/roadmap/REQUIREMENTS_INDEX.md")
    if os.path.isfile(req):
        external_ids |= {norm(x) for x in re.findall(r"\bR-\d+\b", read(req))}
        external_ids |= {norm(x) for x in re.findall(r"\bQUEST-\d+\b", read(req))}

    # зависимости и циклы
    for t in tasks:
        for d in t.get("depends_on", []):
            if d not in ids and norm(d) not in external_ids:
                fail(problems, f"{t['id']}: зависимость {d} не найдена ни в tasks.json, ни во внешних реестрах")
    WHITE = {"TODO", "IN_PROGRESS", "BLOCKED", "DONE", "FAILED", "DEFERRED"}
    visiting, done = set(), set()

    def dfs(node, stack):
        if node in done:
            return
        if node in visiting:
            fail(problems, f"циклическая зависимость: {' -> '.join(stack + [node])}")
            return
        visiting.add(node)
        for d in ids.get(node, {}).get("depends_on", []):
            if d in ids:
                dfs(d, stack + [node])
        visiting.discard(node)
        done.add(node)

    for tid in ids:
        dfs(tid, [])

    # ROADMAP.md: маркеры и панель
    roadmap = read("docs/ROADMAP.md") if os.path.isfile(os.path.join(ROOT, "docs/ROADMAP.md")) else ""
    for m in COMPAT_MARKERS:
        if m not in roadmap:
            fail(problems, f"ROADMAP.md потерян маркер совместимости: {m}")
    if "<!--GEN:SUMMARY-->" not in roadmap or "<!--/GEN-->" not in roadmap:
        fail(problems, "ROADMAP.md: нет блока GEN:SUMMARY (генерируется tools/gen_roadmap.py)")

    # задачи NOW в панели должны существовать в tasks.json
    now_ids = set(re.findall(r"\b(?:PROTO|K|SR|E|RADIO|QUEST|FPS)-\w+\b", roadmap))
    for nid in now_ids:
        if nid not in ids and nid not in external_ids:
            fail(problems, f"ROADMAP.md ссылается на неизвестную задачу: {nid}")

    # ключевые разделы сохранились в docs/
    all_docs = []
    for base, _dirs, files in os.walk(os.path.join(ROOT, "docs")):
        for fn in files:
            if fn.endswith(".md"):
                all_docs.append(os.path.join(base, fn))
    corpus = "\n".join(open(p, encoding="utf-8").read() for p in all_docs)
    for sec in KEY_SECTIONS:
        if sec not in corpus:
            fail(problems, f"потерян ключевой раздел: «{sec}»")

    # внутренние ссылки docs/... в roadmap-файлах существуют
    link_re = re.compile(r"`(docs/[^`]+?\.md)`")
    for p in all_docs:
        rel = os.path.relpath(p, ROOT)
        if not (rel.startswith("docs/roadmap") or rel == "docs/ROADMAP.md"):
            continue
        for m in link_re.finditer(open(p, encoding="utf-8").read()):
            target = m.group(1).split("#")[0]
            if not os.path.isfile(os.path.join(ROOT, target)):
                fail(problems, f"{rel}: битая ссылка {target}")

    if problems:
        print(f"FAIL: {len(problems)} проблем")
        for p in problems:
            print(" -", p)
        sys.exit(1)
    print(f"OK: {len(tasks)} задач, {len(external_ids)} внешних ID, файлов проверено: {len(REQUIRED_FILES)}+{len(all_docs)}")
    print("Прогресс по задачам (не готовность игры):",
          {s: sum(1 for t in tasks if t['status'] == s) for s in sorted(WHITE)})


if __name__ == "__main__":
    main()
