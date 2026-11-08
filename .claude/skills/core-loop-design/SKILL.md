---
name: core-loop-design
trigger: /core-loop-design
description: >
  Audit a game's loop stack (moment/session/meta) against named design principles
  and a 9-item gate. Use when a game feels grindy, directionless, or not hooking
  players — or as a first-pass design check before a mechanic ships.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. Machine-specific paths are
> genericized; the methodology and the gotchas are the real ones.

# core-loop-design — audit the loop stack against named principles

A loop audit checks whether a game's structure gives players something meaningful
to do, a reason to get better at it, and a reason to return. It audits DESIGN —
whether the principles are implemented — not whether the implemented design
is fun to play. That verdict belongs to a human playtest, not an agent.

## The loop stack

Every game has three nested loops. Name all three before auditing:

- **Moment loop** — the action a player repeats in a single engagement (shoot,
  match, place, move). Typically 5–60 seconds. This is where skill lives.
- **Session loop** — what a player completes in one sitting (a run, a level, a
  match, a quest chain). Typically 5–60 minutes. This is where progression is felt.
- **Meta loop** — what pulls a player back across sessions (unlock tree, seasonal
  reward, story arc, ranked climb). Typically days to months.

If you cannot name all three in one sentence each, the loop stack is not yet
designed — it is assumed.

## 9-item gate

Every item must be answered YES before the loop design is considered complete.

1. **Three loops namable.** Can you write a one-sentence description of the
   moment loop, session loop, and meta loop? If any one is "unclear" or "TBD,"
   the design is incomplete.

2. **Complete skill atom.** Does the moment loop contain a full input → feedback
   cycle with a clear success/fail signal? Incomplete atoms (inputs with no
   feedback, or feedback with no legible cause) produce the "mushy" feeling even
   when the session and meta loops are well-designed.

3. **New mastery past first hour.** Does the player encounter a mechanic, enemy,
   or system they have not seen before after 60 minutes of play? A loop that
   shows all its depth in the first session gives players no reason to return.

4. **Difficulty scales off mastery.** Does the game's challenge grow as the
   player's competence grows, rather than on a fixed timer? Fixed-timer scaling
   punishes players who learn quickly and bores players who learn slowly. (See
   Csikszentmihalyi's flow channel.)

5. **First-hour interest curve.** Does the session loop have a moment of tension
   or novelty in the first five minutes, and a felt payoff before the session
   ends? A flat interest curve across the first hour is the single most common
   cause of early player drop-off.

6. **Fixed and variable rewards.** Does the reward schedule mix predictable
   rewards (fixed-ratio: "every 10 kills gives a drop") with unpredictable ones
   (variable-ratio: "drops at a 15% rate")? Fixed-only schedules feel mechanical;
   variable-only schedules feel unfair. Kim's research on reward schedules and
   Skinner's operant conditioning both point to the mixed schedule as the
   engagement-sustaining design.

7. **Next goal visible.** At any point in a session, can the player articulate
   what they are working toward in the next 5 minutes? If the answer is "I don't
   know, I'm just playing," the session loop lacks adequate goal signposting.

8. **Progression gates tied to player action, not time.** Does the player unlock
   the next phase of the meta loop by doing something, rather than by waiting?
   Time gates are a monetization pattern, not a design pattern — they break the
   mastery feedback loop (Koster's Theory of Fun: fun is learning, learning
   requires cause-and-effect, time gates decouple effect from cause).

9. **No dominant strategy.** Is there more than one viable approach to the moment
   loop that a competent player might choose? A single dominant strategy means the
   "choice" in the loop is illusory; players who discover the dominant strategy
   stop exercising skill and start performing a rote sequence (Schell, The Art of
   Game Design, lens #15 — the lens of the toy).

## Boundary

This skill audits whether the loop stack's DESIGN implements the above principles.
It cannot certify whether the resulting design is enjoyable to play — that
requires a human playtest. Pass all 9 items, then hand off to a human.

## Citations

- Csikszentmihalyi, M. — *Flow: The Psychology of Optimal Experience* (1990).
  The flow channel: challenge must scale with skill, or the experience
  collapses into boredom (too easy) or anxiety (too hard).
- Cook, D. — "The Chemistry of Game Design," Gamasutra (2007). The skill atom:
  input → simulation → feedback → model update. Every moment loop must complete
  this cycle legibly.
- Koster, R. — *A Theory of Fun for Game Design* (2004). Fun is the act of
  mastering a pattern. A loop that stops teaching stops being fun.
- Schell, J. — *The Art of Game Design* (2008). Lens #15 (the toy), lens #19
  (the player), lens #31 (the challenge). Use these lenses to audit whether
  choices are real.
- Kim, A.J. — *Game Thinking* (2018). Reward schedules and the "superfan"
  journey: the players who go deepest are the ones whose mastery is recognized
  and whose investment accumulates.
- Sid Meier — "A Theory of Fun" (GDC 2012). "A game is a series of interesting
  decisions." A loop where the optimal decision is obvious at every step has
  stopped being a game.
