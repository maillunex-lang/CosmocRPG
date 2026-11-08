---
name: difficulty-design
trigger: /difficulty-design
description: >
  Audit a game's difficulty against the flow channel and fairness canon —
  challenge tracks skill not position, DDA or player control exists, every
  threat is readable, no hit is unavoidable. Use on "feels cheap, not hard"
  or a difficulty-tuning pass.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific numbers
> or internal tooling names are included; the gate checklist and its
> underlying frameworks are the real value and are left intact.

# difficulty-design — flow channel, fairness, and DDA

Difficulty design sits at the intersection of two problems that look
unrelated until you see them break the same game: **calibration** (is the
challenge at the right level for this player?) and **fairness** (does the
player feel the loss was earned by them, not handed to them by a cheap
system?). Getting the math right without the fairness canon is how you build
a game that's statistically balanced and emotionally infuriating.

The core framework is Csikszentmihalyi's **flow channel**: anxiety when
challenge exceeds skill, boredom when skill exceeds challenge, flow when
they track together. The complication is that skill is a *moving target* —
players improve during a session, not just between sessions — so a fixed
difficulty ramp set at design time will drift out of the channel on any
player who is faster or slower than the design assumption.

Jenova Chen's "Flow in Games" translates this into an interactive-media
design constraint: the system must respond to the player's actual in-session
capability, not a fixed schedule. That's the justification for DDA
(Dynamic Difficulty Adjustment) or, at minimum, an explicit player-facing
difficulty control — a way to pull the challenge back into the channel when
it has drifted out.

The fairness half comes from the soulslike canon (Dark Souls, Celeste) and
Schell's observations about loss: a player can accept a hard loss if they
believe a different choice would have produced a different outcome. The
moment they feel the death was handed to them — by an off-screen hit, an
unreadable wind-up, an unavoidable first-frame threat — they've lost agency,
and difficulty becomes punishment. That's the emotional difference between
"hard" and "cheap," and it's recoverable only by restoring the agency
signal.

## 8-item gate

Run each item as a concrete, verifiable check — not a vibe read.

**1. Challenge tracks player capability, not time or position.**
The game's difficulty signal (enemy HP, spawn rate, puzzle complexity,
obstacle density — whatever the primary stressor is) responds to *how the
player is actually doing*, not simply how far into the game they are or how
long they've been playing. Verify: a player stuck on encounter N for five
minutes is not receiving more of the same challenge that already defeated
them.

**2. DDA or player-facing difficulty control exists.**
One of: (a) an explicit Easy/Normal/Hard (or equivalent) selection the
player can change at any time, or (b) a dynamic system that detects when a
player is consistently over- or under-challenged and adjusts a named knob.
Verify: name the mechanism in code and name the knob it moves.

**3. Rest beats between escalations, peaks-and-valleys pacing.**
High-intensity sections are bounded — they resolve, and there is a
lower-intensity period before the next escalation. Sustained maximum
challenge with no release is a known driver of fatigue and session
abandonment. Verify: map the intensity curve of at least one full session
path and confirm no peak extends without a valley following it.

**4. Every threat is telegraphed and readable.**
Every enemy attack, environmental hazard, or incoming damage source has a
pre-move signal — an animation wind-up, a visual tell, an audio cue, a
floor indicator, some sensor-accessible warning that exists before the
damage frame. Verify: for each primary threat type, name the telegraph and
confirm its timing window.

**5. No hit is unavoidable — a dodge or counter window exists.**
A player who reads the telegraph and acts correctly must be able to avoid
or mitigate every incoming threat. First-frame hits (damage on spawn),
off-screen damage with no warning, and AoE that covers all viable positions
with no escape vector are the canonical violations. Verify: for each
primary damage source, confirm there is a player action that avoids it.

**6. Recovery and mercy windows after hits or death.**
After taking damage or dying, the player has a grace period: invincibility
frames after a hit to prevent chain-stagger, a checkpoint or respawn point
that doesn't force the player to replay a long, already-mastered section,
or at minimum a clear reentry point that restores agency quickly. Verify:
time from death to re-engagement and confirm it's not punitively long
relative to the difficulty of the section.

**7. Risk and reward scale together, visible before commit.**
The player can see (or reliably infer) the potential upside before choosing
to engage a difficult encounter. Higher risk does not map only to higher
punishment — it maps to higher reward. Verify: for the highest-risk
encounters in the game, confirm a visible or inferable reward is associated
with attempting them.

**8. No single strategy dominates — verify with a TTK or cost probe.**
At least two meaningfully different approaches (build paths, strategies,
unit selections, character choices — whatever the strategic layer is) are
viable on the target difficulty. If one approach is obviously dominant, the
difficulty is solved rather than engaged. Verify via a TTK (time-to-kill),
cost-per-outcome, or win-rate comparison across strategies — not by
assertion.

Done when all 8 items pass a concrete check, not when they pass a vibe
read. The gate is a precondition for a human playtest that judges *feel* —
whether a difficulty that passes these tests is also *satisfying* is a
separate judgment that belongs to the player.

## Reference

- Csikszentmihalyi, M. — *Flow: The Psychology of Optimal Experience*.
  The canonical source for the challenge/skill channel.
- Chen, J. — "Flow in Games" (USC MFA thesis, 2006). Translates the flow
  channel into an interactive-media design constraint with a DDA argument.
- Schell, J. — *The Art of Game Design: A Book of Lenses*. Lens of the
  Challenge and the Lens of Fairness.
- Dark Souls / Celeste — practical soulslike fairness canon: every death
  readable, every obstacle avoidable, every retry meaningful.
- Sid Meier — "A game is a series of interesting decisions." Cheap deaths
  remove the decision; a decision needs a visible option to be real.
