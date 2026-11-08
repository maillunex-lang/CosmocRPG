---
name: store-listing
description: >
  Draft store-listing copy and the screenshot/graphic checklist for a game
  across multiple storefronts (app store, Steam, your own catalog) — lead
  with the hook, run a privacy scrub, and clear a trademark-safety check
  before anything ships. Use when writing app-store copy, a store
  description, a feature-graphic checklist, or /store-listing.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. All references to specific
> internal publishing tools, real third-party brands, and any brand-adjacent
> marketing decisions have been removed and replaced with a generic
> trademark-safety checklist and an invented, neutral example. The privacy
> and trademark gates are the real value here — treat them as hard
> requirements, not optional steps.

# store-listing — write the copy, spec the assets, clear the gates

A store listing lives or dies on its **hook** — the one line that makes a
stranger scrolling a storefront decide this particular game is worth a
download. Everything past that line is checklist work: draft copy per
platform, spec the visual assets, and clear two gates that exist to keep the
listing safe to publish at all.

## Steps

1. **Find the hook.** One sentence: what makes this game worth installing,
   specifically — not "fun puzzle game" but the thing that's actually true
   only of this game. Pull it from an existing design doc/canon file if the
   project has one; otherwise draft it from the core gameplay loop and get
   it confirmed by whoever owns the product decision before drafting the
   rest of the copy around it.

2. **Draft copy per target storefront.** Typical shapes:
   - Mobile app stores: a short tagline (roughly ≤80 characters) plus a
     longer full description (roughly ≤4000 characters, platform-dependent).
   - Steam: a short blurb plus a longer "about this game" section.
   - Any storefront you run yourself: whatever catalog-entry format it uses.

   Every draft leads with the hook, then explains the core loop, then covers
   what's new/different if this is an update.

3. **List the visual assets each target requires, with exact sizes** —
   don't generate them as part of this step, just spec what's needed:
   - Mobile app stores typically want: phone screenshots at platform-specific
     resolutions, a feature graphic (commonly 1024×500), and an icon
     (commonly 512×512).
   - Steam wants a capsule image set at several fixed sizes plus in-game
     screenshots.
   - Your own storefront, if you run one, has whatever spec you defined for
     it — keep that spec written down somewhere so it doesn't have to be
     rediscovered every release.

4. **Run a privacy scrub before anything ships.** No secrets, tokens, or
   personally identifiable information in either the copy or the
   screenshots. If a screenshot happens to show data from a real person
   (e.g. a multiplayer leaderboard, a social feature, test-account details),
   blur or replace it before it goes anywhere public. This is the same bar
   any other public-facing content push should clear — treat it as
   non-negotiable, not a nice-to-have.

5. **Clear the trademark-safety gate.** This is the step people skip
   because it feels like paranoia, and it's the one that causes real
   problems when skipped. The general rule: **describe your game by its own
   mechanics and world — never by a resemblance to someone else's brand.**
   Concretely:
   - If your game's aesthetic, mechanic, or theme happens to evoke a real
     company's product or on-screen style (a genre convention, a visual
     style associated with a well-known brand, a naming pattern that echoes
     a real trademark), don't lean into that resemblance in your marketing
     copy or screenshots. Describe what your game actually does, in your
     own words.
   - Never use another company's name, logo, or slogans in your store copy
     to imply an association, endorsement, or parody relationship you don't
     have.
   - Don't publish a "wink" at the resemblance — a tagline or screenshot
     caption that's clearly designed to make players think of the other
     brand without naming it is still a trademark risk, arguably a worse
     one, since it's harder to defend as unintentional.
   - This gate matters even more if you have any professional or contractual
     relationship with a company connected to the brand in question — treat
     that as an automatic stop-and-check, not something to reason around on
     your own.
   - If a draft turns out to be brand-adjacent in a way that's hard to
     rewrite cleanly, that's a sign the concept itself may need to change,
     not just the copy — flag it rather than trying to word around it.

6. **Hand off the actual upload.** This skill's deliverable is copy plus the
   asset checklist — the mechanical upload to a storefront (build signing,
   store console submission, review-queue handling) is a separate workflow
   with its own gates. Get explicit sign-off on the copy before it goes
   anywhere near a submission form.

Done when copy is drafted for every target storefront, the asset checklist
is fully specced with real sizes, the privacy scrub has passed, and the
trademark-safety gate is clear with no open questions.

## Reference

- Every listing leads with the hook — if you can't state it in one sentence,
  the copy isn't ready to draft yet.
- Privacy scrub and trademark-safety check are hard gates. Neither is a step
  to breeze past because a deadline is close.
- This skill produces copy and a checklist; it does not touch the actual
  upload/submission pipeline.
