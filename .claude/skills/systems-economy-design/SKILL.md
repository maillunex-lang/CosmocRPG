---
name: systems-economy-design
trigger: /systems-economy-design
description: >
  Audit a game's resource economy against sources/sinks/converters, feedback
  loop signs, and scarcity design — so inflation, dead currencies, and pure
  grind are caught before they reach players. Use on "the economy is
  inflating / a currency is dead / pure grind."
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific numbers
> or internal tooling names are included; the gate checklist and its
> underlying frameworks are the real value and are left intact.

# systems-economy-design — sources, sinks, feedback loops, and scarcity

A game economy is a system of flows. Resources enter through **sources**,
leave through **sinks**, change form through **converters**, and accumulate
in **pools**. The most common economy failures — runaway inflation, dead
currencies, pure grind — trace back to the same root causes: a source
without a sink, a converter that misprices its output, or a feedback loop
nobody noticed was positive until the numbers had already collapsed.

The framework comes from Adams and Dormans' *Game Mechanics: Advanced Game
Design*, which provides the vocabulary (source, sink, converter, pool,
trader) and the key structural insight: **every economy needs at least one
mechanism that ages it forward independent of player action.** An economy
that only moves when the player does is a waiting room, not a world.

LeBlanc's feedback loop taxonomy adds the sign distinction: a **positive
feedback loop** amplifies deviation (rich-get-richer, runaway leader), a
**negative loop** damps it (catch-up mechanics, rubber-banding). Both have
their uses; the dangerous one is an *unintentional* positive loop that was
never noticed in design and compounds quietly until the numbers become
absurd.

## 8-item gate

Run each item as a concrete, verifiable check — not a vibe read.

**1. Every resource has a named, code-verifiable sink.**
For each resource the game can accumulate, name where it leaves the system:
a purchase cost, a decay function, a crafting consumption, an entry fee,
an expiry. A resource without a sink inflates without bound. Verify: for
each resource type, point to the code path that removes it from a player's
pool.

**2. A continuous or tick-step process ages the economy independent of
player activity.**
Something moves in the economy even while the player is offline, idle, or
not trading — market prices drift, resources regenerate, decay ticks occur,
NPCs trade among themselves. An economy that freezes when the player steps
away produces a static world with no reason to return. Verify: name the
tick/event that fires when no player action is occurring and describe what
it changes.

**3. Converter costs scale with the chosen output's value.**
A converter (crafting bench, refinery, upgrade station, trade post) that
lets a player reliably produce something more valuable than its inputs is
a money printer. Converter exchange ratios must account for the full value
chain — not just nominal input costs, but also what a player can do with
the output (sell, reuse, chain into a further converter). Verify: for each
converter, compare input cost to output market value and confirm the ratio
is intentional.

**4. Stocks and prices are hard-clamped and covered by a stress test.**
Every pool that could in principle accumulate without limit has an explicit
cap, a soft pressure (diminishing returns, storage cost), or a design
argument for why uncapped accumulation is intentional. Every price range
that matters for game balance has a floor and a ceiling that are enforced
in code, not assumed by convention. Verify via a stress test that simulates
extended or adversarial play and confirms no stock or price escapes its
intended range.

**5. No unintended positive feedback loop compounds unchecked.**
Enumerate every loop in the economy — a positive loop causes deviation to
amplify (having more X makes it easier to get more X). Every positive loop
must either be intentional (a power fantasy arc that is bounded and
eventually saturates) or have a named damping mechanism (a tax, a cost
scaling, a resource contention). Verify: for each positive loop, name
whether it is intended and what bounds it.

**6. Persistence boundaries per resource are explicit.**
For each resource, the answer to "does this survive a session end, a save,
a server restart?" is written down and reflected in the code. Implicit
persistence assumptions cause bugs that look like economy exploits. Verify:
name the storage layer for each resource and the conditions under which it
resets.

**7. At least one always-present scarce resource forces a real tradeoff.**
Scarcity is what makes choices meaningful. If a player can accumulate every
resource type to satiation through normal play, there is no economy — there
is a checklist. At least one resource must be genuinely contested or
supply-limited such that the player cannot have everything at once. Verify:
name the resource, its supply constraint, and the meaningful choice it
creates.

**8. The economy moves without player trading.**
Tied to item 2, but stated as an experience test: open the game, let time
pass without taking any economic action, and confirm that prices, stocks,
or world state have changed. A completely static economy during player
absence is a design failure in any game with a persistent economy. Verify:
record an economy state, wait, record again, and confirm at least one
meaningful value changed.

Done when all 8 items pass a concrete check. Whether the resulting economy
is *fun* to engage with — whether the scarcity creates satisfying decisions
rather than frustrating ones — is a judgment that belongs to a human
playtest, not to this gate.

## Reference

- Adams, E. & Dormans, J. — *Game Mechanics: Advanced Game Design* (2012).
  Source/sink/converter/pool vocabulary, Machinations diagramming method,
  the tick-step independence argument.
- LeBlanc, M. — Feedback loop taxonomy (GDC 1999 / MDA framework with
  Hunicke and Zubek). Positive vs. negative loop sign distinction, the
  runaway-leader / catch-up dynamic.
- Wright, W. / Maxis — SimCity and The Sims as canonical examples of
  emergent economy behavior from simple source/sink/converter rules.
