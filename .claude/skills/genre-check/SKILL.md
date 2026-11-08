---
name: genre-check
trigger: /genre-check
description: >
  Audit whether a build meets its own declared genre's player-expectation
  contract — must-haves implemented, core loop present, primary verb feels
  right, broken conventions are broken knowingly. Use on "does this feel
  like a real <genre>" or before any genre-positioned release.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific numbers
> or internal tooling names are included; the gate checklist and its
> underlying frameworks are the real value and are left intact.

# genre-check — does the build meet its own genre's player-expectation contract

A genre is a set of player predictions. When a player picks up something
labeled "roguelike" or "tower defense" or "metroidvania," they arrive with a
set of expectations about what the game will contain, how sessions will be
structured, what the primary verb will feel like, and how progress will
persist. Genre is a contract, and a game that breaks that contract without
awareness or compensation is not "subverting genre" — it is confusing the
player it invited.

The framework comes from Björk and Holopainen's *Patterns in Game Design*,
which treats genre as a collection of recurring design patterns — not a
fixed rule set, but a set of player-expectation predictions that the game
is implicitly promising to satisfy or consciously choosing to violate.
Anna Anthropy and Naomi Clark's *A Game Design Vocabulary* adds the
practical observation that broken conventions need compensating design: you
can break a genre expectation if you replace the need it was serving.

This skill does not evaluate whether the game is *good* — that is a human
playtest verdict. It evaluates whether the game meets the expectations its
own genre label creates. A game can fail this gate and be an excellent
genre-subversion; it can pass this gate and be a mediocre example of its
genre. The gate answers a narrower question: *is this game aware of its
genre's contract and intentionally handling it?*

## 9-item gate

**1. Declared genre is explicit, with 2–3 must-haves named.**
Before running any other check, write down: what genre is this game? What
are the 2–3 things a player *must* be able to do or experience for this
game to deliver on that genre label? These are the must-haves. If the team
cannot name them, the genre is not decided — stop and decide it before
auditing.

**2. Each must-have is actually implemented and player-reachable.**
For each must-have named in item 1, confirm it is implemented in code and
reachable by a player in a normal play session — not planned, not prototyped,
not a planned future feature. A roguelike where permadeath is not yet
implemented is not a roguelike yet. Verify by walking a normal play session
and confirming each must-have is encountered.

**3. The core loop the genre expects is present end-to-end.**
Every genre has a primary play cycle — the thing the player does over and
over that constitutes the game. Name it (e.g. for a roguelike: run →
die → meta-progress → run again; for a tower defense: wave incoming →
place/upgrade towers → evaluate → next wave). Confirm a player can
complete one full iteration of that cycle in a build right now. Verify:
run one full loop and confirm every stage is present.

**4. The primary verb feels the way the genre demands.**
Genre players have expectations about how the primary verb feels, not just
what it does. A first-person shooter where aiming feels sluggish and
imprecise breaks the genre contract even if shooting is technically
implemented. A puzzle game where solutions feel arbitrary rather than
discoverable breaks it. Verify: describe the primary verb and the tactile
expectation the genre creates for it. Confirm the current implementation
satisfies the genre's expectation — or name the compensating design that
replaces it.

**5. Genre-standard feedback affordances exist.**
Each genre has conventional feedback mechanisms players expect: minimap in
an RTS, health bar in action games, damage numbers in RPGs, a visible card
hand in card games, clear state in puzzle games showing "solved." These
exist because the genre's player population has learned to read them.
Their absence is not minimalism — it is missing information. Verify: name
the genre-standard affordances and confirm each is implemented.

**6. Broken conventions are broken knowingly, with compensating design.**
If the game intentionally violates a genre expectation (no minimap in an
exploration game, no explicit health bar, no visible hand in a memory card
game), name the convention being broken and name the compensating design
that serves the need the convention was filling. A violation without
compensation is a missing feature, not a design choice. Verify: for each
intentional convention break, there is a stated rationale and a named
compensator.

**7. The genre's classic pitfall is named with a demonstrable mitigation.**
Every genre has a known failure mode that games in that genre reliably hit:
roguelikes run-feel worse than the meta-progression hype implies; tower
defense games lose tension after optimal loadouts are discovered; RPGs
develop mandatory optimal builds that remove meaningful choice; narrative
games hit pacing walls during exposition. Name the pitfall for this game's
genre and name the specific design decision that mitigates it. Verify: the
mitigation exists in the build, not only in the design doc.

**8. Onboarding teaches genre mechanics before testing them.**
Genre-experienced players can skip onboarding for familiar patterns — but
the game should not assume this. The first-time experience should introduce
genre-specific mechanics (deckbuilding rules, tile placement logic, run
structure, combo systems) in a context where a mistake is survivable. The
classic onboarding failure is testing a mechanic before the player has
learned it. Verify: walk the first-time flow and confirm each genre
mechanic is demonstrated before the player is penalized for not knowing it.

**9. Persistence, session, and save behavior match genre expectations.**
A roguelike player expects permadeath and run-scoped progress; a JRPG
player expects persistent state and save-anywhere (or at minimum
save-at-checkpoints); a mobile puzzle player expects instant resume with
no session penalty. Mismatched save behavior is one of the most
disorienting genre violations because it restructures the player's
relationship with the game at a fundamental level. Verify: describe what
persists, what resets, and what the save model is — then confirm it matches
what this genre's player population expects.

Done when all 9 items pass a concrete check. Whether the game is *good at
its genre* — whether it delivers on its promise in a way that satisfies
players — is a judgment that belongs to a human playtest, not to this gate.

## Reference

- Björk, S. & Holopainen, J. — *Patterns in Game Design* (2004). Genre
  as a collection of recurring design patterns; the framework for
  player-expectation prediction that underlies this gate.
- Anthropy, A. & Clark, N. — *A Game Design Vocabulary* (2014). The
  practical vocabulary for naming what games do and the observation that
  broken conventions require compensating design.
