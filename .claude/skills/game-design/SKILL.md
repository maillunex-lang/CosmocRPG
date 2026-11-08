---
name: game-design
trigger: /game-design
description: >
  The design-knowledge router — points design questions at the right sub-skill
  (feel, loop, level, UX, onboarding, difficulty, economy, motivation, genre),
  each a lean gate backed by named frameworks and citations. Use when starting
  or reviewing ANY game design work.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. Machine-specific paths are
> genericized; the methodology and the gotchas are the real ones.

# game-design — the design-knowledge router

One entry point over nine design-audit skills, each a lean gate distilled from a
reference corpus. Answers "which lens do I need for this design question" so a
design gets checked against named principles instead of vibes.

## Route to a sub-skill

- **game-feel** — moment-to-moment tactile response: input latency, juice, hitstop,
  camera, accessibility of feel. Reach for it on "does this feel mushy/unresponsive/dead."
- **core-loop-design** — the loop stack (moment/session/meta), mastery curve, reward
  schedule, felt unlocks. Reach for it on "is this grindy / directionless / not hooking."
- **level-design** — spatial pacing, teaching-through-space, sightlines, encounter
  composition, gating valves. Reach for it on "this map feels flat / confusing / cheap ambush."
- **game-ux** — menus, HUD, information architecture, gamepad parity, accessibility
  options. Reach for it on "controller can't reach this / colorblind player can't tell state."
- **onboarding-design** — the first-time-user experience, no-tutorial tutorial, cannot-fail
  opening. Reach for it on "new players get confused / quit in the first minutes."
- **difficulty-design** — the flow channel, DDA, telegraphing, recovery windows, dominant
  strategies. Reach for it on "this feels cheap, not hard" or a difficulty-tuning pass.
- **systems-economy-design** — sources/sinks/converters, feedback-loop signs, scarcity
  trade-offs. Reach for it on "the economy is inflating / a currency is dead / pure grind."
- **player-motivation-design** — SDT competence/autonomy/relatedness, reward ethics, dark
  patterns. Reach for it on "why would anyone come back tomorrow" or a monetization audit.
- **genre-check** — does the build meet its OWN genre's player-expectation contract and
  name its classic pitfall. Reach for it on "does this feel like a real <genre>."

## Not routed here (related, separate skills)

- **game-balance** — the NUMBERS a loop/economy moves (TTK, drop rates, curves).
- **game-design-review** — visual-screen design (palette/type/layout/motion).

## Boundary

Every sub-skill audits whether design PRINCIPLES are implemented — code, layout, or
capture-checkable. None of them certify how the game FEELS to play; that verdict is
always a human playtest — never self-certified by a skill or an agent.
