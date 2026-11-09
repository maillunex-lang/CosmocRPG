#!/usr/bin/env python3
"""Проверка лендинга (site/) на соответствие политике CSP Caddy и мелким правилам проекта.

CSP: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self' blob:.
Значит, в разметке нельзя: <style>, атрибут style=, встроенные <script>, обработчики on*=, внешние адреса ресурсов.
Запуск: python3 tools/check_site.py  (код возврата 1 при нарушениях)."""
import re
import sys
from html.parser import HTMLParser
from pathlib import Path

SITE = Path(__file__).resolve().parent.parent / "site"
problems = []


class P(HTMLParser):
    def __init__(self):
        super().__init__()
        self.refs = []
        self.in_script = False

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag == "style":
            problems.append("встроенный <style>")
        if "style" in a:
            problems.append(f"атрибут style= у <{tag}>")
        for k in a:
            if k.startswith("on"):
                problems.append(f"обработчик {k}= у <{tag}>")
        if tag == "script":
            if "src" not in a:
                problems.append("встроенный <script> без src")
            else:
                self.refs.append(a["src"])
        if tag in ("link",) and a.get("href"):
            self.refs.append(a["href"])
        if tag in ("img", "source") and a.get("src"):
            self.refs.append(a["src"])
        if tag == "button" and a.get("data-full"):
            self.refs.append(a["data-full"])
        if tag == "a" and a.get("href", "").startswith(("http://", "https://")):
            problems.append(f"внешняя ссылка в <a>: {a['href']}")


html = (SITE / "index.html").read_text(encoding="utf-8")
p = P()
p.feed(html)
for ref in p.refs:
    if re.match(r"^(https?:)?//", ref):
        problems.append(f"внешний ресурс: {ref}")
    elif ref.startswith("/") and not ref.startswith("/download/"):
        if not (SITE / ref.lstrip("/")).exists():
            problems.append(f"нет файла для {ref}")

css = (SITE / "style.css").read_text(encoding="utf-8")
for m in re.finditer(r"url\(([^)]+)\)", css):
    u = m.group(1).strip("\"' ")
    if u.startswith(("http:", "https:", "//")):
        problems.append(f"внешний адрес в CSS: {u}")
    elif u.startswith("/") and not (SITE / u.lstrip("/")).exists():
        problems.append(f"нет файла для CSS url({u})")
if "@import" in css:
    problems.append("@import в CSS")

js = (SITE / "app.js").read_text(encoding="utf-8") + (SITE / "boot.js").read_text(encoding="utf-8")
for bad in ("eval(", "new Function", "innerHTML", "document.write", "setAttribute(\"style\"", "cssText"):
    if bad in js:
        problems.append(f"в скриптах запрещено: {bad}")
for m in re.finditer(r"https?://[^\s\"')]+", js):
    problems.append(f"внешний адрес в скриптах: {m.group(0)}")

for need in ("fonts/GolosText-VF.ttf", "fonts/OFL.txt", "img/hud.jpg", "boot.js", "app.js", "style.css"):
    if not (SITE / need).exists():
        problems.append(f"нет обязательного файла {need}")

total = sum(f.stat().st_size for f in SITE.rglob("*") if f.is_file())
print(f"site/: {total/1024:.0f} КБ, проверено ссылок: {len(p.refs)}")
if problems:
    print("НАРУШЕНИЯ:")
    for x in problems:
        print(" -", x)
    sys.exit(1)
print("OK: лендинг соответствует политике CSP")
