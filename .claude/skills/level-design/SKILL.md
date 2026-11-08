---
name: level-design
trigger: /level-design
description: >
  Audit a level or map against 8 spatial design principles — silhouette, path
  breadcrumbing, tension/release pacing, low-stakes teaching, affordance,
  spawn telegraphing, backtracking, and density variation. Use when a space
  feels flat, confusing, or cheap.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. Machine-specific paths are
> genericized; the methodology and the gotchas are the real ones.

# level-design — audit a space against spatial design principles

Level design is the practice of using space to teach, pace, and reward without
stopping the player to explain anything. This skill audits whether a level or
map implements the named principles — it does not certify whether the space
is enjoyable to play. That verdict requires a human playtest.

## 8-item gate

Every item must be answered YES before a level is considered complete.

1. **Distinct silhouette.** Can a player who has been through the level once
   recognize its key landmarks from a distance by shape alone, without
   relying on a minimap or marker? Landmarks must read as distinct silhouettes
   against the background. A space where everything reads as the same visual
   weight produces disorientation and "I don't know where I am" feedback —
   the most common level-design complaint.
   (Valve's HL2 design notes: "players navigate by landmark, not by compass.")

2. **Breadcrumbed path.** Is the intended direction of travel indicated by
   at least two environmental cues at every decision point — light, colour,
   geometry, sound, enemy placement, or collectible trail? A decision point
   with zero directional cues is a dead end even when the path is technically
   open. Two cues provides redundancy across play styles (some players follow
   sound; some follow light; some follow items).

3. **Staged tension and release.** Does the level alternate between high-stakes
   encounters and low-stakes breathing rooms? A level at constant high tension
   produces exhaustion, not excitement; at constant low tension, boredom.
   The rhythm is: approach (rising tension) → encounter (peak) → aftermath
   (release and resource recovery) → repeat, escalating. (Totten, *An
   Architectural Approach to Level Design*, chapter on pacing.)

4. **Low-stakes teaching space.** Is every new mechanic, hazard, or enemy type
   introduced in a forgiving context before it appears in a high-stakes
   encounter? Teaching under pressure produces frustration, not learning.
   The player must survive the tutorial failure before they can apply the
   mechanic under pressure. (Valve HL2: every new weapon introduced with
   weak enemies and room to experiment.)

5. **Visual affordance on interactive geometry.** Does every surface or object
   the player can interact with (climb, push, break, open) look visibly
   different from non-interactive surfaces of the same type? If players
   cannot distinguish "climbable ledge" from "background geometry" by
   visual cue alone, they will not attempt the interaction. (Gibson's affordance
   theory; Norman's *The Design of Everyday Things*, ch. 1.)

6. **Enemy spawn telegraphed before danger.** Does every enemy or threat that
   can damage the player from off-screen appear with at least one anticipation
   cue before it deals damage — audio, shadow, visual disturbance, or
   pre-spawn animation? Underhanded spawns (damage before warning) are
   perceived as unfair regardless of whether they are mechanically "fair."
   (Dark Souls design canon: every threat is visible or audible before it
   reaches the player.)

7. **Collapsed backtracking.** When a player must return through previously
   cleared space — to retrieve a key item, complete an objective, or exit —
   is the return path either shortened (a shortcut unlocks), accelerated
   (faster traversal), or made meaningful (new content on the return path)?
   Backtracking through identical space with no change is the most reliable
   way to make a level feel longer than it is in a negative sense.

8. **Density variation.** Does the level vary the density of encounters,
   collectibles, and interactive elements across its length? Uniform density —
   enemies and items spaced at identical intervals — reads as mechanical and
   removes the sense of discovery. High density in approach/encounter zones,
   low density in breathing rooms, and isolated high-value items in optional
   exploration areas is the standard spatial grammar.

## Boundary

This skill audits whether a level's spatial design implements the above
principles. It does not audit gameplay feel, difficulty balance (see
`difficulty-design`), or economy (see `systems-economy-design`). Whether
a well-designed level is enjoyable to move through requires a human playtest —
pass all 8 items, then hand off.

## Citations

- Valve level design documentation (Half-Life 2, Portal): landmark navigation,
  low-stakes mechanic introduction, environmental storytelling.
- Miyazaki, H. / FromSoftware (Dark Souls series): telegraphed threats, spatial
  mastery rewarded with shortcuts, no underhanded damage.
- Totten, C. — *An Architectural Approach to Level Design* (2014). Pacing
  through spatial rhythm; the encounter-breathing-room alternation; sightlines
  and their psychological effect.
- Gibson, J.J. — *The Ecological Approach to Visual Perception* (1979).
  Affordances: properties of objects that signal possible actions. Applied to
  game geometry: surfaces must communicate their interactability visually.
- Norman, D. — *The Design of Everyday Things* (1988). Affordances and
  signifiers in designed objects — the same principles that make a door
  handle legible make a game ledge climbable-by-appearance.
