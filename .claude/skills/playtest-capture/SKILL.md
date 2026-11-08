---
name: playtest-capture
description: >
  Boot a running game build in a real window, capture "golden frame"
  screenshots of key states, and inspect them for breakage before a human
  playtests — so the human playtest judges feel, not a sideways particle
  effect or a mis-scaled HUD. Use when render-verifying a game build,
  capturing a playtest, or /playtest-capture.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. References to specific
> internal tools are replaced with generic equivalents (a headless test
> runner, a screenshot/emulator tool); the two-pass discipline — headless
> logic check vs. windowed visual check — is the actual technique.

# playtest-capture — catch breakage before the human playtest

The point of this skill is division of labor: an agent can catch mechanical
and visual breakage automatically, so a human's playtest session is spent
judging *feel* — is this fun, does this land — instead of being wasted on a
black screen, a missing sprite, or a HUD element clipped off-screen that a
script could have caught in seconds.

The unit of evidence is a **golden frame**: a screenshot of a specific,
named game state, inspected against what you intended that state to look
like.

## Steps

1. **Boot windowed, not headless.** `--headless` (or your engine's
   equivalent no-render mode) renders nothing — it can never produce a
   golden frame, by definition. For Godot, use the real engine binary
   (see the companion `godot-new-game` pattern for why a pinned,
   console-subsystem binary matters). For Flutter/native mobile, drive an
   emulator via `adb` (screencap for capture, `input` for interaction).

2. **Drive the logic headless, separately.** Run your project's headless
   test/probe harness for the core loop's *correctness* — exit code, no
   runtime errors, expected state transitions. This is a different pass
   from the visual one: the headless probe proves the game *runs*; the
   windowed frame proves it *looks right*. Conflating the two hides bugs —
   a game can exit 0 headless while rendering garbage, and it can look fine
   in one frame while the underlying logic is broken.

3. **Capture the key states.** At minimum: the main menu, the core loop
   mid-play, and every terminal state (win, lose, game over). Save frames to
   a scratch/artifacts directory — never commit them into the main source
   tree as build output.

4. **Inspect every frame against intent**, checking for:
   - Layout is on-screen and correctly scaled at the target resolution.
   - Nothing renders black, transparent-when-it-shouldn't-be, or clipped.
   - The active visual theme/skin is actually applied (a common miss when a
     theme system exists but a screen was built before it, or bypasses it).
   - Text fits its container at realistic string lengths, not just the
     placeholder used during development.
   - Effects (particles, transitions, screen shake, animated sprites) fire
     on the correct trigger — not sideways, not stuck permanently on, not
     silently absent.

5. **Report, don't self-certify.** Hand back: what booted, each captured
   frame with its path/label, every issue found ranked worst-first, and a
   short list of what's left for the human to judge — feel only. The one
   thing this skill must never do is claim the *feel* is good; that
   judgment call belongs to the human, every time. Mechanical and visual
   breakage is yours to catch; fun is not yours to certify.

Done when every core state has been captured, every frame has been
inspected against the checklist above, and any issues found are surfaced
*before* the human opens the build — not discovered together with them.

## Report format that actually gets used

A report that just says "captured 5 frames, looks fine" is nearly worthless
— it doesn't tell the human what was actually checked. A useful report
looks more like:

```
Booted: windowed, 1280x720, no config errors.
Frames captured: 4/4 core states (menu, mid-play, win, lose).

Issues (worst first):
1. [win screen] Score text overflows its panel at 6+ digits — clips off
   the right edge. Reproduced by simulating a high-score run in the probe.
2. [mid-play] Pickup-collected particle fires on EVERY frame the pickup
   exists, not just on collection — visually it reads as a constant sparkle
   rather than a one-shot burst.
3. [menu] Correct theme colors applied; no issues found.

Left for playtest: does the pacing of the mid-play loop feel good, is the
win/lose screen satisfying, does anything feel sluggish or too fast. All
mechanical/visual issues above should be fixed before that session.
```

Ranking worst-first and separating "fixed automatically-checkable issues"
from "left for feel" is what makes this report actionable instead of
decorative.

## Reference

- **Headless = no rendering, full stop.** If a tool flag disables the
  window, it categorically cannot produce a screenshot worth inspecting.
- **Two passes, not one.** Headless probe answers "does it run"; windowed
  frame answers "does it look right." Treat them as separate gates with
  separate evidence, even though they usually run back-to-back.
- **The human's playtest is a feel gate**, not a QA pass. Anything
  mechanically or visually broken that reaches that stage was a process
  failure on the automation side — catch it first.
