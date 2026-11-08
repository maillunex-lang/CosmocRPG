---
name: godot-tools
description: >
  A catalog + adoption log for free, commercially-safe Godot addons/tools,
  scraped from the public Godot Asset Library and store.godotengine.org.
  Use for every Godot project's planning/build phase — before hand-rolling a
  system, check whether a vetted free addon already does it — and to log
  what got adopted, evaluated, or rejected (and why). NOT for 3D art (see
  game-assets) or non-Godot engines. Trigger /godot-tools.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. The scrapers, the specific
> addons I've adopted, and the games they were adopted into are mine and
> stay local (unreleased project names and architecture details are
> removed) — what's public is the METHOD: how to keep a running catalog of
> free/commercial-safe addons plus a decision log that gets more valuable
> every time you evaluate something, including — especially — when you say
> no.

# godot-tools — a catalog + adoption log for free Godot addons

Two problems this solves: (1) rediscovering the same addon search every
project, and (2) re-evaluating and re-rejecting the same wrong-fit addon
because nobody wrote down why it didn't work last time. A local catalog plus
a decision log fixes both.

**When NOT to use:** 3D models/art (a separate CC0 art catalog handles
that — see `game-assets`); cross-catalog faceted queries spanning art +
tools (see `gamedev`); non-Godot engines, obviously.

## Building the catalog

Godot has two public sources worth scraping, and they're not the same set —
check both:

- **The Asset Library API** behind `godotengine.org`'s in-editor asset
  browser. A straightforward paginated JSON API; the `cost` field carries
  the license string. Older/larger catalog, thousands of entries, but no
  live rating signal worth trusting (see Gotchas).
- **store.godotengine.org**, the newer htmx-driven storefront. Smaller,
  fewer thousand entries, but carries live "thumbs up" ratings, license,
  tags, and code snippets per listing. Scraping it needs a real browser
  User-Agent header (the default script UA gets blocked) and a bit of
  cursor-following through its infinite-scroll fragments.

Two small scraper scripts, one per source, dumping to local JSON. Re-run
both monthly, or before any new project's planning phase — a stale catalog
misses recently published addons.

## License rules (the sell-the-game filter)

- **SAFE:** MIT, Apache-2.0, CC0, BSD-2/3, MPL-2.0, ISC, Zlib, Unlicense.
- **SAFE w/ credit line:** CC-BY-3.0/4.0 (attribution in `CREDITS.md`).
- **NEVER in a closed-source commercial game:** GPL/AGPL (viral),
  CC-BY-NC (non-commercial), CC-BY-SA (share-alike).
- **CAUTION:** LGPL (dynamic-link only — isolate it or avoid it) or an
  unlisted license (verify from the source repo's actual LICENSE file
  before treating it as safe; never assume MIT).

Always vendor the tool's LICENSE file alongside it in your addons
directory and log it in the project's `CREDITS.md` — same habit as any
free-art pipeline.

## Workflow

1. Read the project's needed systems off its design notes (terrain, sky,
   water, UI, dialogue, save, netcode, debug console, camera, …).
2. Grep the commercial-safe subset of the catalog per system.
3. Shortlist by license → engine-version match → rating/recognition →
   last-update recency.
4. **Evaluate honestly against the project's actual architecture** — this
   is the step that's easy to skip and the one that matters most. A
   heightmap terrain tool assumes a flat, bounded world; it does not fit a
   spherical or procedurally-generated one. An addon that fights your
   game's core design (e.g. one that assumes the engine instance is the
   networking authority, when yours isn't) will cost more time integrating
   than writing the ten lines yourself.
5. Adopt: install into your addons directory, vendor the license, credit it.
6. **Log it** — adopted or rejected, either way (see below).

**Adoption counts as "done" only when all four hold:** the tool's files are
under the project's addon directory, its LICENSE file is vendored
alongside, a line exists in the project's `CREDITS.md`, and a row exists in
the usage log. Missing any one of these means it isn't actually adopted yet.

**Stop conditions / never-do:**
- Never install anything GPL/AGPL/CC-BY-NC/CC-BY-SA into a commercial
  project — no exceptions, no rationalizing "just for now."
- License unknown or the catalog's license field is empty → stop, verify
  from the tool's own repo; if still unverifiable, skip it and say so —
  don't assume it's safe.
- Don't install something "might need later" — log it as a candidate in the
  backlog and install it when the feature that needs it actually lands.
- Tool not found in either catalog → say so plainly; don't invent an asset
  id or store URL. Offer a catalog refresh instead.
- A scraper failing → retry once, then report the error (the store may be
  down, or blocking the request) — never loop on it.

## Usage log (tool | license | used for | why | rating | notes)

A trimmed, genericized slice of mine — the shape is the point: log
adoptions *and* rejections, and always write down *why*, generalized to the
reusable lesson rather than tied to one unreleased project's internals.

| Tool | License | Used for | Why | Rating | Notes |
|------|---------|----------|-----|--------|-------|
| GUT (Godot Unit Test) | MIT | any project | headless, CI-able test runner | 5 | the backbone of every verify loop |
| Kenney asset packs | CC0 | any project | instant Steam-clean 3D art | 5 | via a local CC0 catalog, not this one — see `game-assets` |
| Zylann HTerrain | MIT | evaluated for a planet-scale space project | heightmap terrain | n/a | REJECTED — heightmap terrain assumes a flat, bounded world; a spherical or procedurally-generated planet is a different shape entirely. Reconsider for any flat-world game. |
| RTS-style orbit/pan camera addon | MIT | an RTS-style project | zoom-to-cursor, edge-pan, orbit, smoothing — replaced a bare camera rig | 4 | ADOPTED as a thin subclass, not a fork: orbit gated behind a modifier key so it doesn't collide with unit-order clicks; vendor file left untouched |
| screen-shake addons | MIT | evaluated | camera shake on impact | n/a | REJECTED — a ten-line hand-roll on the existing camera rig did the same job; the addon was dead weight for the value it added |
| health-bar / health-component addons | MIT | evaluated | HP display | n/a | REJECTED — HP lived in a separate simulation layer that had to stay portable outside Godot; a scene-tree health component would have fought that boundary |
| Godot netcode addons (general) | — | evaluated | multiplayer | n/a | REJECTED — game authority lived on an external server over a custom protocol; Godot-side netcode addons generally assume the Godot instance itself is the authority |
| Steamworks GDExtension wrapper | MIT | planned, Steam release | auth/achievements/workshop | n/a | install once a Steam App ID exists — don't wire it up speculatively |
| controller/input-icon addon | MIT | planned, settings screen | auto keyboard/mouse/controller icon swap + remapper UI | n/a | pairs naturally with a separate input-remapping addon |
| sfxr-style in-editor SFX tool | MIT | planned | instant placeholder SFX before a real audio pass | n/a | cheap, good for prototyping feel before committing to final audio |
| full camera-management addon (top-tier, general recommendation) | MIT | evaluated on one project | complete camera-rig framework | n/a | REJECTED *for that one project only* because a custom camera system + possession-tween already existed and this would have fought it — otherwise a strong first pick for a new 3D game's camera needs from scratch |
| whole-input-stack remapping framework | MIT | evaluated | input abstraction layer | n/a | REJECTED — a full input-stack rework wasn't worth it when the engine's built-in InputMap already covered the game's needs |
| vehicle-physics addon (verified via its own LICENSE file) | MIT | a racing project | `VehicleBody3D`-based handling, gearbox, wheel/tire model, built-in race AI + traffic | n/a | installed as the physics layer, fed parameters from a higher-level vehicle-config system rather than hand-authored per car |

## Backlog (candidates, not yet installed)

Verify each license from the tool's own repository before installing —
listing here is not the same as clearing it.

| Tool | For | When |
|------|-----|------|
| Vehicle-physics addon (above) | ground vehicles for a no-fly-zone travel mechanic | once that content actually gets built |
| Lens-flare / volumetric godray shader pack | sun glare, atmosphere shafts | a presentation/polish pass |
| Textured glass material pack | canopies, station windows | construction/presentation polish |
| Visual-shader node library | reusable shader building blocks | as shader work comes up |
| Sci-fi environment starter kit | station interiors, derelict-ship content | derelict/interior content phase |

## Gotchas

- **The older asset-library's `rating` field is effectively dead** — every
  entry returns 0, so "sort by rating" is noise there. Shortlist by name
  recognition, last-modified date, and engine-version match instead; the
  newer store's live thumbs-up ratings are more trustworthy where available.
- Scraping the newer store needs a real browser User-Agent — the default
  script/library UA gets blocked outright.

## Self-improvement

Every use: add adopted/evaluated tools to the log — including rejected
ones and *why*, generalized to the reusable lesson rather than the specific
unreleased project. Refresh the catalog if it's gone stale (over a month
old). Rejections are cheap to write and save real research time later.

## Related skills

- `game-assets` — the local CC0 art library (separate from this addon log).
- `gamedev` — the faceted query layer fronting both this and `game-assets`.
