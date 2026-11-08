---
name: player-motivation-design
trigger: /player-motivation-design
description: >
  Audit a game's motivation architecture against Self-Determination Theory
  and reward ethics — competence, autonomy, relatedness, no dark patterns,
  intrinsic return reason. Use on "why would anyone come back tomorrow" or a
  monetization audit.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific numbers
> or internal tooling names are included; the gate checklist and its
> underlying frameworks are the real value and are left intact.

# player-motivation-design — SDT, reward ethics, and dark patterns

The question this skill answers is: *why would a player come back?*

There are two very different ways to get a player to return. One works
because the player genuinely wants to — the game is satisfying their
psychological needs for competence, autonomy, and connection, and they
leave a session with something they care about unfinished. The other works
because the game has engineered anxiety — a streak about to break, energy
that's about to regenerate, a timer on a loot box — and the player comes
back to relieve that pressure. The first is Self-Determination Theory. The
second is a dark pattern catalog entry. Both produce return visits in the
short run; only the first produces players who recommend the game to
someone else.

The framework is Deci and Ryan's **Self-Determination Theory (SDT)** and
its game-specific elaboration by Rigby and Ryan in *Glued to Games*. SDT
identifies three psychological needs whose satisfaction predicts intrinsic
motivation: **Competence** (I'm getting better at something meaningful),
**Autonomy** (my choices matter), and **Relatedness** (I'm connected to
others who matter to me). When a game satisfies these needs, motivation is
self-sustaining. When it bypasses them with external pressure, motivation
is fragile and resentful.

Cognitive Evaluation Theory (CET), a sub-theory of SDT, adds the
**informational vs. controlling** distinction for rewards: a reward that
tells you something about your performance ("you completed this in record
time") supports competence. A reward that is the reason you did the thing
("you'll lose your streak if you don't log in") is controlling and
undermines intrinsic motivation over time.

## 8-item gate

Run each item as a concrete, verifiable check.

**1. Competence: challenge escalates with skill, legible mastery feedback.**
The game communicates to the player that they are improving at something
meaningful. This is not score — it is *readable signal* that skill growth
is happening and that it matters to outcomes. Verify: name the mastery
signal (a success rate improving, a mechanic unlocking, a stat reflecting
player capability) and confirm it escalates alongside actual player skill
growth rather than remaining static or tied only to time-played.

**2. Autonomy: two or more valid strategies with real tradeoffs.**
The player's choices must have actual consequences — not illusory choice
where all paths converge, not a dominant strategy that makes other options
irrelevant. Verify: name at least two strategies in the game's primary
decision space, confirm each is viable in at least one meaningful
situation, and confirm there is a genuine tradeoff between them (not that
one is strictly better).

**3. Relatedness: a channel connecting to others who matter.**
This does not require multiplayer. Leaderboards, ghost data, shared
discoverable content, community challenge results, or meaningful NPC
relationships that persist across sessions all satisfy the relatedness
need. Verify: name the mechanism. A completely solo, socially isolated
experience can still provide relatedness through narrative relationships
with well-written NPCs; that counts, but it must be *present* rather than
absent.

**4. Rewards are informational, not controlling.**
Rewards should communicate something about the player's performance or
progress, not be the reason the player acts. The test: would the player do
this activity if the reward were removed? If yes, the reward is
informational. If no — if the reward is the only reason — it is
controlling, and removing it would reveal there was never any intrinsic
motivation there to begin with. Verify: for each primary reward loop, ask
the informational/controlling question explicitly.

**5. Multiple player types each have something meaningful to do.**
Bartle's taxonomy (Achiever, Explorer, Socializer, Killer) and Yee and
Quantic Foundry's motivational profiles are useful lenses here: does the
game have something for a player who wants to master it, something for a
player who wants to explore and discover, something for a player who wants
social connection, something for a player who wants competition? The game
does not need to serve all types equally — but it needs to serve its target
types deliberately. Verify: name the player types the game targets and
confirm a meaningful activity exists for each.

**6. No dark patterns — specifically: no gacha, energy gate, time gate,
streak or FOMO pressure, pay-to-skip.**
Gacha: randomized reward locked behind real or premium currency with
hidden odds. Energy gate: a timer that caps how much the player can play
in a session without paying. Time gate: a timer that makes the player wait
in real time for a result that has no skill component. Streak/FOMO: a
reward or bonus that is lost if the player misses a session, engineering
anxiety about not logging in. Pay-to-skip: paying money as a substitute
for player skill or time investment. Verify: confirm none of these
mechanisms are present. If a monetization model requires any of them, flag
it as a design conflict and surface it explicitly — do not route around it
silently.

**7. No grind wall blocks core play.**
A grind wall is a mandatory repetition gate that blocks access to the
game's primary interesting decisions and cannot be bypassed by skill.
Skill-optional grind (farm for an advantage, but the game is completable
without it) is a separate design question. Skill-mandatory grind (you
cannot proceed without hitting N XP/resource/level by repetition alone) is
what this item prohibits. Verify: confirm every required gate to core
content is passable by a player who is skilled without mandatory
repetition.

**8. The reason to return is intrinsic and respectful, and works with
notifications off.**
The game should have something genuinely unresolved that a player cares
about: an open question in the narrative, a skill challenge they want to
master, an economy they want to optimize, a social event they want to
participate in. That return reason should be present even if the player
has notifications fully disabled. If the only return reason is an external
prompt (a push notification, a streak reminder, a time-gated resource), the
game has substituted anxiety for interest. Verify: name the return reason
and confirm it is intrinsically motivated rather than externally pressured.

Done when all 8 items pass a concrete check. Whether the resulting
motivation architecture actually produces a game players *want* to return
to — whether the competence signal feels meaningful, whether the autonomy
feels real rather than cosmetic — is a judgment that belongs to a human
playtest, not to this gate.

## Reference

- Deci, E. & Ryan, R. — Self-Determination Theory (SDT) and Cognitive
  Evaluation Theory (CET). Primary academic framework for intrinsic
  motivation and reward ethics.
- Rigby, S. & Ryan, R. — *Glued to Games: How Video Games Draw Us In and
  Hold Us Spellbound* (2011). Game-specific SDT elaboration with the PENS
  (Player Experience of Need Satisfaction) model.
- Bartle, R. — "Hearts, Clubs, Diamonds, Spades: Players Who Suit MUDs"
  (1996). Player type taxonomy.
- Yee, N. / Quantic Foundry — Motivational profiles research. More
  nuanced, empirical update to Bartle for modern game audiences.
- darkpatterns.games — Taxonomy and catalog of manipulative game design
  patterns. Use as a reference checklist for item 6.
