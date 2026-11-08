---
name: onboarding-design
trigger: /onboarding-design
description: >
  Audit a game's first-time-user experience against 8 onboarding principles —
  teach-first-use, learn-by-doing, one-at-a-time pacing, affordance/constraint/
  feedback, cannot-fail opening, just-in-time prompts, hook-in-15-min, and
  persistent state. Use when new players get confused or quit in the first session.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. Machine-specific paths are
> genericized; the methodology and the gotchas are the real ones.

# onboarding-design — audit the first-time-user experience

Onboarding is the period from the moment a new player first launches the game
until they have internalised the core loop well enough to play without prompted
guidance. A well-designed onboarding is invisible: the player feels like they
are playing, not being taught. A poorly designed one feels like homework.

This skill audits whether the onboarding DESIGN implements named principles. It
does not certify whether the onboarding feels smooth to a new player — that
requires a human playtest with someone who has genuinely never seen the game.

## 8-item gate

Every item must be answered YES before onboarding is considered complete.

1. **Taught before first required use.** Every mechanic a player must use to
   progress past the first session is introduced before the game requires it
   in a high-stakes context. A player who encounters a required mechanic for
   the first time in a consequential moment (a boss, a timer, a permanent
   choice) will perceive the failure as unfair even if they had technical
   access to the information earlier.
   (Valve, Half-Life 2: no weapon is required in combat before the player has
   had a peaceful moment to discover it.)

2. **Teach by doing, not by telling.** The game communicates how a mechanic
   works by placing the player in a situation that requires using it, not by
   displaying a text block explaining it. If the onboarding's primary teaching
   method is text overlays, tooltips, or cutscenes explaining controls — the
   design has failed this item. Players skip text; they cannot skip a wall they
   must climb.
   (Anthropy, A. — the Super Mario Bros. 1-1 analysis: world 1-1 teaches running,
   jumping, enemies, and coins through forced interaction, with zero text.)

3. **One mechanic at a time.** The onboarding introduces mechanics sequentially,
   not simultaneously. A player cannot internalise two new mechanics at the same
   moment — they will internalise one and forget or confuse the other. Introduce
   mechanic A, let the player use it in isolation, then introduce mechanic B.
   The cognitive load limit for a first-time player is one new concept per
   contextual beat.
   (Norman, *The Design of Everyday Things*: knowledge in the world before
   knowledge in the head — new users cannot hold complex state in working memory.)

4. **Affordance + constraint + feedback loop complete.** For every taught
   mechanic, verify that three elements are present in the teaching moment:
   - **Affordance** — the environment visually suggests the action is possible
     (a ledge that looks climbable, a button that looks pressable).
   - **Constraint** — the environment prevents incorrect actions that would
     confuse the player about what they should do (doors that won't open until
     the relevant mechanic is learned, a level that funnels players toward the
     correct interaction).
   - **Feedback** — the game immediately and legibly communicates that the
     action succeeded or failed (sound, animation, UI change, camera response).
   All three must be present. A mechanic with affordance and feedback but no
   constraint allows the player to "discover" the right answer accidentally
   without understanding it; a mechanic with constraint but no feedback leaves
   the player confused about whether they did anything.

5. **Cannot-fail opening.** The first 60–90 seconds of gameplay is designed so
   that a player following the most obvious path cannot die, fail, or get stuck.
   This is not about difficulty — it is about establishing that the game's
   controls and feedback loops work before testing the player with them.
   A player who dies or fails in the first minute before they understand the
   controls will attribute the failure to the game being "broken" or "unfair,"
   not to their own inexperience.
   (Nintendo design philosophy; Celeste's opening sequence: the first screen
   has one obstacle, infinite respawns, and a four-second respawn loop that
   teaches the jump mechanic without ever addressing it explicitly.)

6. **Just-in-time prompts only.** No onboarding prompt appears before the player
   is in a context where they need the information. An inventory prompt shown
   on the title screen will be forgotten before the player opens the inventory.
   A control reminder shown when the player first reaches the relevant situation
   will be read and used immediately. If any onboarding prompt is shown more
   than 30 seconds before the player needs to act on it, it is early.
   (Long, S. — "The Problem with Tutorials" (GDC): front-loaded tutorials
   produce the worst retention rates of any onboarding pattern because information
   presented without immediate application is not retained.)

7. **Hook delivered within 15 minutes.** A new player who has never heard of
   the game and has no reason to trust it experiences something genuinely
   surprising, delightful, or exciting within the first 15 minutes of play.
   Not "the tutorial is complete" — a moment of felt payoff: a power spike, a
   reveal, a satisfying combo, an unexpected interaction. If the game's hook
   only arrives after the player has already decided to keep playing, the hook
   is too late for a significant portion of the audience.
   (Long, S. — GDC 2014: "fail fast" onboarding; Sid Meier: every session
   should contain a story the player wants to retell.)

8. **Persistent onboarding state.** The game remembers which mechanics have
   been introduced to this player and does not re-show tutorials they have
   already completed. A player who has played for three hours and returns the
   next day must not be re-introduced to controls they already know. This applies
   to: individual tutorial flags, settings preferences, and any "first time"
   prompts. Onboarding state persists across sessions and, where applicable,
   across device restarts.

## Boundary

This skill audits the onboarding's DESIGN against the above principles. It does
not assess whether the onboarding feels smooth to a real new player — that
requires a human playtest with a genuine first-time player, not a self-assessment
by the developer. Pass all 8 items; then run the human first-time test.

## Citations

- Valve Software — Half-Life 2 (2004) and Portal (2007) level design documentation.
  Canonical references for environment-as-teacher and cannot-fail opening sequences.
- Anthropy, A. — "How to Read a Level" analysis of Super Mario Bros. 1-1 (2012).
  The definitive breakdown of teach-by-doing using environmental design alone,
  with zero text or UI instruction.
- Long, S. — "The Problem with Tutorials" (GDC 2014). Empirical data on tutorial
  retention; the case for just-in-time over front-loaded onboarding.
- Norman, D. — *The Design of Everyday Things* (1988, revised 2013). Affordances,
  constraints, and feedback as the three-part grammar of learnable interaction.
- Nintendo EAD / Celeste (Extremely OK Games, 2018) — opening sequence design.
  The cannot-fail opening, infinite-respawn teaching loop, and mechanic
  introduction-by-forced-interaction patterns.
