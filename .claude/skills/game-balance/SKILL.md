---
name: game-balance
description: >
  Tune a game's numbers against a measurable target using a headless
  simulation, not vibes — sweep one knob at a time until a metric lands in
  a defined band. Use for a balance pass, difficulty/economy tuning, "the
  payout/drop-rate/damage feels off", or /game-balance.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific numbers
> or internal tooling names are included; the search discipline (named
> knobs, a measurable target, one-variable-at-a-time sweeps) is the actual
> technique and is unchanged.

# game-balance — tune numbers against a measured target, not a feeling

Balance is a search problem: you're looking for the set of **knob** values
that lands a **measured metric** inside a **target band**. The discipline
that makes this reliable is that a simulation decides the numbers are in
range — a human's playtest judges whether the *balanced* numbers are
actually *fun* on top of that. Those are two separate gates, run in that
order, and skipping straight to "does this feel right" without a sim to
back it up is how you end up re-tuning the same knob three times because
nobody isolated which variable actually moved the needle.

## Steps

1. **Name every knob.** Enumerate every tunable that affects the metric you
   care about — spawn rates, costs, damage numbers, drop odds, XP curves,
   payout tables, house edge, whatever applies to your game — and write down
   its *current* value next to it. If a knob isn't on the list, you can't
   reason about whether it's contributing to the problem.

2. **Define the target as something checkable.** "Feels fair" is not a
   target. "Player win rate lands between 45% and 55% over 1,000 simulated
   runs" is a target. Other useful shapes: a run-length range, "no single
   strategy dominates across N simulated matches," or a house-edge
   percentage for a wagering mechanic. Write the band down before you start
   changing values.

3. **Build a probe.** A headless simulation that plays N runs
   automatically and prints the metric — reuse whatever headless test
   harness your project already has. This is the verifier. Manually playing
   a handful of rounds and eyeballing the result does not substitute for
   this; small samples are exactly where a knob's effect gets buried in
   variance.

4. **Sweep one knob at a time.** Change a single value, re-run the sim,
   record the metric, revert or keep based on the result. Changing several
   knobs in the same pass makes it impossible to tell which one moved the
   number — you'll end up guessing, which is the thing this whole approach
   exists to avoid.

5. **Land the values and confirm.** Once a candidate set puts the metric in
   band, re-run the sim once more to confirm the result holds (not a fluke
   of one run). Change *values* only — never rename or repurpose a
   persisted identifier or save-data key while tuning; players' saved
   progress and any historical data depend on those names staying stable.

6. **Hand off to a human feel-check.** The simulation proves the math is in
   range. A human playtest is the only thing that can judge whether numbers
   that are mathematically "balanced" are actually *fun* to play against.
   Both gates matter, and skipping the second one because the first one
   passed is a common mistake — a mathematically fair economy can still be
   boring.

Done when the simulation reports the metric inside the target band on a
confirming re-run, and the specific knobs that were changed (with old and
new values) are recorded somewhere durable.

## Worked example (invented numbers, illustrative only)

Say a wave-defense game's early game feels too easy, but the actual
complaint from players is vague ("boring for the first few minutes"). Turned
into this workflow:

1. Knobs: enemy spawn interval (currently 3.0s), enemy HP scalar (1.0x),
   starting player gold (100), tower base cost (50).
2. Target: "average time-to-first-tower-loss across 200 simulated waves 1-5
   lands between 90s and 150s" — a stand-in for "the early game has real
   tension."
3. Probe: a headless sim that plays waves 1-5 with a fixed baseline AI
   strategy and logs the time-to-first-loss, averaged over 200 runs.
4. Sweep: baseline sim reports 240s average (too slow — no early pressure).
   Drop spawn interval to 2.5s alone, re-run: 210s. Revert, try HP scalar
   1.15x alone instead: 165s. Keep that, re-check spawn interval on top of
   it: 2.7s + 1.15x HP lands at 118s — inside the 90-150s band.
5. Confirm: re-run the same 2.7s / 1.15x combination for another 200 waves;
   it holds at 121s. Record both changed values and the metric that
   confirmed them.
6. Hand off: the sim says the pacing is now "tense." Only a human playtest
   can say whether tense-and-balanced is also *fun* — that's a separate
   session, not a rubber stamp on the sim's output.

## Reference

- The verifier is the simulation's printed metric — not manual play, not a
  gut check.
- One knob per sweep, always. This is the single most important discipline
  in this workflow; it's the difference between "I found the cause" and "I
  changed three things and something got better."
- Tune values freely; never touch persisted ids/keys in the same pass —
  those are a separate, much higher-stakes kind of change.
