---
name: git-repo-rules
description: >
  Что коммитить и чего не коммитить в репозитории Уралии (client, server, art_src, assets).
  Use when adding gitignore, export presets, secrets, large binaries, or splitting client and server.
---

# Git в этом репозитории

Каталоги: `client/` (Godot), `server/`, `art_src/` (GLB, .blend), `assets/` (PNG-атласы + JSON), `tools/`, `tests/`, `docs/`.
В `main` только то, что собирается и проходит тесты. Рабочая ветка — `dev`.
Числа баланса — отдельным коммитом, чтобы откат был виден.

Не коммитить: `.godot/`, билды (`build/`), `.venv/`, `.env*`, ключи подписи Android (`*.keystore`, `*.jks`), пароли, TLS-ключи.
`client/export_presets.cfg` — только без паролей и путей к ключам.

Бинарные исходники (GLB, .blend, wav/ogg) и размер репозитория: Git LFS сейчас **не настроен** (нет `.gitattributes`). Решение о LFS принимает пользователь, самовольно не включать.
Любой ассет — только с записью в `docs/ASSET_LICENSES.md`.

Git push — только после подтверждения. Выход: что именно меняется в git и чего в коммите не будет.

<!-- Источник: Ufacity github-godot. Упоминания LFS/.gitattributes и Meshy убраны как неактуальные. -->
