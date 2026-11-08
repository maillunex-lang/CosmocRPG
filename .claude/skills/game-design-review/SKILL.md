---
name: game-design-review
trigger: /game-design-review
description: >
  Visual-screen design audit for games — palette, type scale, layout proportions,
  motion, and per-screen mock quality. Use when producing a new screen design or
  judging whether an existing one has design problems (slop patterns, hierarchy
  fails, theme drift). Distinct from game-design (which audits mechanics).
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. Machine-specific paths are
> genericized; the methodology and the gotchas are the real ones.

# game-design-review — visual-screen design audit

Audits the visual design of game screens against named principles. Covers producing
a new design and judging an existing one. Does NOT assess whether the underlying
mechanic is fun — that is `game-design` territory.

## Producing a design

Work through these in order before calling a screen "done":

1. **Scene sentence.** Write one sentence that names the screen's single job
   ("this screen lets the player spend earned currency on permanent upgrades").
   If you cannot write the sentence in one line, the screen is doing too much.

2. **Palette (hex values).** Pick a named palette with explicit hex codes.
   Every colour on the screen must trace back to a palette slot. No improvised
   colours mid-design. Document: primary, secondary, accent, background, surface,
   on-surface (text), error, and any semantic colours (health = red, etc.).

3. **Type scale (ratio ≥ 1.25).** Define at minimum: body, label, heading, and
   display sizes using a consistent ratio (1.25 = Major Third, 1.333 = Perfect
   Fourth, 1.5 = Perfect Fifth). Never pick font sizes by eye — compute them from
   the base and the ratio and write the numbers down.

4. **Layout (proportions).** Name the grid or column system. Define spacing in
   multiples of one unit (e.g. 4 px or 8 px grid). Every margin, padding, and gap
   must be a multiple of that unit — no arbitrary pixel offsets.

5. **Motion (ease-out default).** Every animated transition names its easing
   function. Default to ease-out for elements entering the screen (fast start,
   soft landing reads as responsive). Ease-in-out for elements that traverse the
   screen. Avoid linear — it reads as mechanical and cheap.

6. **Per-screen mock.** Produce a static mock (image, markup, or structured
   description with explicit measurements) for every screen before implementation.
   Prose descriptions of layout do not count — layout only becomes legible when
   it is rendered or measured.

## Judging a design

Check for these failure modes on every screen review:

### Slop patterns (immediate reject if present)

- **Colored side-stripe** — a vertical accent bar on the left or right of a card
  or panel as the sole decorative element. Common shortcut; reads as undesigned.
- **Gradient text** — CSS gradient applied to text. Almost always illegible at
  small sizes and fails contrast checks. Use flat colour.
- **Glass blur** — backdrop-filter blur used as a primary surface treatment.
  Usually chosen because it "looks modern"; almost always reduces legibility and
  adds GPU cost for no gameplay value.
- **Hero-metric card** — a large number in the center of a card with nothing
  around it but padding. Looks like a placeholder. Actual data always has context
  (label, unit, trend, comparison).
- **Identical cards** — a row of cards that share the exact same visual structure
  with only the text changed. Means the hierarchy is not expressed in the layout,
  only in the content.
- **Modal where inline works** — using a dialog or overlay for an action that
  could happen in the flow of the current screen. Every modal is a context
  switch; use them only when the action genuinely requires exclusive focus.

### Hierarchy failures

- Primary action is not the most visually prominent element on the screen.
- Two or more elements compete at the same visual weight.
- Labels and values share the same size and colour.

### No-numbers failure

Any claim about a design that cannot be checked with a measurement ("it feels
balanced," "the spacing looks right") is not a design decision — it is a hope.
Every spacing value, font size, and colour contrast ratio must be a number that
can be verified against the defined system.

### Theme drift

Every screen in the same product must use the same palette, type scale, and
spacing system. A screen that introduces a new colour or a new font size not
present in the system has drifted. Name the drift explicitly and resolve it by
either updating the system or reverting the screen.

### Guessable

If a user looking at the screen for the first time cannot identify the primary
action within three seconds, the hierarchy has failed. This is a testable gate:
show the screen to someone unfamiliar with the project and ask them what they
would tap first.

## Hard rules

- Design artifacts (mocks, annotated screenshots, or structured spec files) are
  committed alongside code — a PR that changes a screen's visual structure without
  a corresponding artifact is incomplete.
- Screenshots are evidence; prose descriptions of visual design are not.
- The design review gate is not passed by the designer or agent self-certifying —
  it requires a rendered screenshot or Playwright capture as the artifact,
  plus a human playtest verdict on feel.
