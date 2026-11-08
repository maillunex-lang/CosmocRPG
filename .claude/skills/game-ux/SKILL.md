---
name: game-ux
trigger: /game-ux
description: >
  Audit a game's menus, HUD, and information architecture against 9 UX
  principles — gamepad reachability, focus management, signifiers, colour
  accessibility, modal trapping, accessibility options, HUD legibility, and
  menu depth. Use on any UI/UX concern or before a screen ships.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. Machine-specific paths are
> genericized; the methodology and the gotchas are the real ones.

# game-ux — audit menus, HUD, and information architecture

Game UX covers everything the player interacts with that is not the core
gameplay itself: menus, HUD, pause screen, inventory, settings, onboarding
prompts, and any other information surface. This skill audits whether those
surfaces implement named UX principles — it does not audit visual style
(see `game-design-review`) or how fun the game feels (human playtest).

## 9-item gate

Every item must be answered YES before a UI screen or HUD is considered complete.

1. **Gamepad-reachable on default bindings.** Every interactive element on the
   screen is reachable using the standard gamepad navigation path (D-pad or left
   stick cycle, shoulder-button tab, confirm/cancel on face buttons) without
   requiring custom remapping. If a feature is inaccessible on a fresh controller
   layout, it does not exist for gamepad players. Audit with a physical or
   emulated controller, not a mouse.

2. **Sane focus state on open.** When a screen opens, the input focus is
   placed on the element the player most likely wants to interact with first —
   not on the last-focused element from a previous session, not on a header
   label, and not on nothing. Auto-focus on open is a hard requirement for
   gamepad navigation; without it, the player must navigate from an unknown
   starting position on every entry.

3. **Focus re-seeded after list mutation.** When an item is deleted, confirmed,
   or removed from a list or menu, focus moves to the next logical item
   (the item below, or the last item if the deleted item was last) — not to the
   top of the list, not to a header, and not to nothing. Losing focus on every
   action is one of the most common gamepad UX failures and is rarely caught in
   mouse-only testing.

4. **Signifiers match behaviour.** Every interactive element looks different from
   non-interactive elements. Every destructive action (delete, spend, reset) has
   a visual indicator that distinguishes it from non-destructive actions. Buttons
   that are disabled must look disabled (not merely fail silently on press).
   (Norman, *The Design of Everyday Things*: signifiers communicate the set of
   possible actions; absent signifiers produce user error, not user error-correction.)

5. **No colour-only encoding.** No state, status, or data point is communicated
   using colour as the only differentiator. Every colour-encoded distinction must
   also be expressed through shape, icon, label, pattern, or position. This is a
   hard accessibility requirement (WCAG 1.4.1, Game Accessibility Guidelines
   standard C1) and also improves legibility for all players in varied lighting
   conditions.

6. **Modal focus trapping.** When a dialog or modal overlay is open, keyboard
   and gamepad focus is trapped inside the modal — it cannot reach elements
   behind the overlay. Tab/cycle navigation wraps within the modal. The modal
   closes only via an explicit dismiss action (cancel, confirm, or designated
   close button), not by navigating off the edge. A modal that leaks focus to
   background elements is both a UX failure and a potential softlock vector.

7. **Accessibility options present.** The game ships with at minimum: text size
   scaling, subtitles/captions (if the game has audio content), and a
   colour-blind mode (or colour-blind-safe palette throughout). These are not
   post-launch additions — they are table-stakes requirements for any game
   targeting a broad audience. (Game Accessibility Guidelines: features.gameaccessibilityguidelines.com)

8. **Legible HUD at minimum supported resolution.** Every HUD element — health
   bar, ammo count, minimap, status icons, objective text — is legible without
   eyestrain at the game's minimum supported resolution, at the default HUD scale,
   on a display viewed from typical playing distance. HUD elements that require
   the player to lean forward to read are a failure regardless of how they look
   at 4K. (Hodent, *The Gamer's Brain*, ch. on perception and attention limits.)

9. **Menu depth ≤ 3 levels.** No frequently-used player action is buried more
   than three menu levels deep from the main pause or start screen. The
   heuristic: if a player must open a menu, navigate to a sub-menu, navigate
   to a sub-sub-menu, and then take an action, that action is one level too
   deep. Audit by listing the button presses required to reach each settings
   option and each frequently-used inventory or loadout action.
   (Nielsen's usability heuristic #6: recognition over recall; deep menus
   force recall of navigation paths, not recognition of options.)

## Boundary

This skill audits whether UX PRINCIPLES are implemented in the described surfaces.
It does not audit visual style (palette, type, motion — see `game-design-review`),
gameplay feel (human playtest), or onboarding structure (see `onboarding-design`).
Pass all 9 items; then verify with a human playtest for feel.

## Citations

- Norman, D. — *The Design of Everyday Things* (1988, revised 2013). Affordances,
  signifiers, feedback, and the gulf of evaluation/execution. The foundational
  text for UI legibility principles.
- Hodent, C. — *The Gamer's Brain: How Neuroscience and UX Can Impact Video
  Game Design* (2017). Perception limits, attention allocation, HUD legibility,
  and cognitive load in game UI. The primary applied-UX reference for game design.
- Game Accessibility Guidelines — gameaccessibilityguidelines.com. Maintained
  checklist of accessibility standards for games, sourced from disabled player
  communities and published research.
- Nielsen, J. — 10 Usability Heuristics for User Interface Design (1994, updated
  2020). Heuristic #6 (recognition over recall), #7 (flexibility and efficiency),
  #8 (aesthetic and minimalist design). Applied to game menus: deep navigation
  paths violate #6; inaccessible expert controls violate #7.
- WCAG 2.1 criterion 1.4.1 — Use of colour. Standard requiring colour not to be
  the sole means of conveying information.
