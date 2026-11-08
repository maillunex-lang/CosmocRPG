---
name: play-policy-review
trigger: /play-policy-review [<repo-path>]
description: Review a mobile-game app against Google Play Developer Program Policies before a Play Store submission or update. Audits the high-risk zones for a freemium game — simulated gambling & content rating, AdMob ad behavior (interstitial timing, rewarded opt-in), Play Billing for IAPs, Data Safety + privacy policy, target audience / Families, target API + permissions, store metadata. Reports findings + owner-decision items to a docs file; it does NOT auto-fix rating / audience / legal settings. Use before first submission, before every version upload, and whenever ads / IAP / permissions / theme change.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. This is the actual policy
> rubric I run against my own games before every Play Store upload; I stripped
> the project-specific defaults (which repo, which game) so it works against
> any Godot or Flutter mobile game repo with a conventional layout.

# Play policy review

A repeatable Google Play compliance audit for a mobile game. Grounded in the
Play Developer Policy Center (https://play.google/developer-content-policy/).
Pass the path to the game repo you want to review — the checks below assume a
freemium game with ads and/or in-app purchases, but most of the domains apply
to any Play Store submission.

## When to run

- Before the **first** Play submission.
- Before **every** version upload — a compliant app regresses when new code
  lands (a new interstitial trigger, a changed IAP flow, a new permission).
- Whenever ads, IAP, permissions, target API, store copy, or the game's
  theme/mechanics change.

## Hard rules (never violate)

1. **Report, don't auto-fix the legal/rating layer.** Content-rating answers,
   target-audience, the Data-Safety form, the price/free toggle, and listing
   copy are the owner's calls — write findings, never silently change them.
2. **Block on ambiguity.** If a mechanic's classification is genuinely unclear
   (e.g. "is this simulated gambling?"), record it as a `DECIDE` finding with
   a recommendation + evidence, and let the owner choose. Under-declaring is a
   takedown risk; over-declaring only raises the rating — the downside is
   asymmetric, so never default the risky way on the owner's behalf.
3. **Forward-looking, not just today.** Check *planned* ad/IAP hooks too, so
   a future regression (e.g. an interstitial that fires the instant a run
   ends, or on app exit) gets caught the run it lands, not after a rejection.
4. **Re-run before every upload.** Stamp the report with the version/build
   number reviewed.

## The review — 7 policy domains

For each domain: what Play requires · what to check in the repo · pass
criteria · verdict tag. Verdicts: `PASS` · `FIX` (code/config change needed) ·
`DECIDE` (owner call) · `BLOCKER` (must resolve before that specific action —
e.g. before actually selling an IAP).

### 1. Simulated gambling & content rating
Policy: Real-Money Gambling/Games, Content Ratings.
The IARC "simulated gambling" descriptor keys on **wagering a currency on a
chance outcome** — NOT on card/casino imagery. A casino *theme* alone is
allowed; a *wager-on-chance* mechanic raises the rating and forces excluding
children.
- Check: grep the game code for gambling-adjacent vocabulary — bet, wager,
  ante, stake, buy-in, slot, roulette, jackpot, "spin to win", loot box,
  gacha, payout, cash out, real money.
- Distinguish **mechanic vs. flavor**: a reward text like "boss jackpot" is
  flavor, not gambling. Verify the in-game currency is only *earned* and spent
  on *fixed-price* deterministic items (cost check, no randomness), with no
  ante/stake and no chance-based or randomized paid reward. Check
  versus/daily/challenge modes specifically — a "stake your chips" wrinkle
  tends to hide in a side mode, not the main loop.
- On a clean pass, recommend the content-rating questionnaire answer "no
  simulated gambling (theme only)," but tag it `DECIDE` regardless — the owner
  submits the IARC questionnaire and owns the final rating.

### 2. Ads (AdMob or equivalent behavior)
Policy: Ads.
- **Interstitials:** never unexpected; never during gameplay or at a level /
  segment start; never before a splash; never on app-open or on an exit
  action (home/back); closeable within 15s.
- **Rewarded:** genuinely user-initiated opt-in; never forced; an unfilled ad
  must still grant the reward (don't punish the player for fill rate).
- No consecutive/"made-for-ads" interstitial stacking; ads must be dismissible
  and stay inside the app. Ad content rating must match the app's rating.
- Check: read the ad layer (wherever the ad-gate/trigger logic lives). Confirm
  the interstitial trigger is a natural break (run start), skips the first
  run of a session, has a cooldown, and can't fire mid-combat. Verify **every**
  live and planned trigger point obeys the rules — especially any run-end
  hook: it must fire only after results are shown plus an explicit user tap,
  never on the game-over instant, never on exit.

### 3. Payments & IAP (Play Billing)
Policy: Payments, Subscriptions.
- Digital goods/features sold in-app (remove-ads, supporter pack, etc.)
  **must** use Google Play Billing — no side-channel payment. Non-consumables
  need a **restore purchases** path.
- Check: find the purchase-handling function(s) and the billing wiring. If a
  "purchase" button just grants the entitlement with no real Billing call,
  that's fine while the button is hidden/disabled, but tag `BLOCKER` for the
  moment it actually sells: "wire Google Play Billing + restore before
  selling."

### 4. Data Safety + privacy policy
Policy: User Data; Console Data Safety form.
- An ad SDK collects the Advertising ID plus approximate/usage data → must be
  declared in the Console **Data safety** form AND disclosed in the privacy
  policy, and the policy must be live at a public URL pasted into the listing.
- Check: open the privacy policy doc — it must name the Advertising ID, the ad
  network(s), the data collected for ads, the ads-personalization opt-out,
  "not directed at children," and a contact method. Verify the Data-safety
  answers match what the SDK actually collects — never under-declare.

### 5. Target audience & Families
Policy: Families; Families Ads.
- A themed game with ads + IAP that isn't designed for kids should set target
  audience to **exclude children** and should NOT enroll in
  Designed-for-Families. A Families-compliant ad SDK is only required if you
  actually target children — if you don't, standard ad-network integration is
  fine.
- Check: this is a Console declaration → a `DECIDE`/checklist item, with the
  recommendation "target 13+ (or higher per the actual content rating), do
  not opt into Families."

### 6. Target API & permissions
Policy: Target API Level; Permissions.
- Must target a recent API level (Play's current minimum) and request only
  the permissions the app actually uses; sensitive permissions need
  justification.
- Check: the release export config → target SDK, min SDK, and the permissions
  block. INTERNET-only for an ad-supported game is minimal and fine. Record
  file/line proof for the finding.

### 7. Store listing & metadata
Policy: Metadata; App Promotion.
- Screenshots must show the actual app; no keyword spam, no misleading
  claims, no unauthorized IP use; app name matches the build; icon/feature
  graphic within spec.
- Check: the listing doc, the icon and feature-graphic assets, and that
  screenshots exist (minimum 2, actual gameplay). Confirm the store name
  matches the package/app name in the export config.

## Output

Write/overwrite a `docs/play-store/POLICY-REVIEW.md` in the reviewed repo:
- Header: date + version/build reviewed.
- One row per domain: verdict tag · one-line status · evidence (file:line) ·
  action.
- An **"Owner decisions"** section listing every `DECIDE` with a
  recommendation + why.
- A **"Blockers before selling / before production"** section listing every
  `BLOCKER`.
- Never write outside `docs/play-store/`. Never edit rating/audience/legal
  settings directly — those stay the owner's call, always.

## Policy reference (fetch live before relying on memory — Play policy changes)

| Domain | URL |
|---|---|
| Hub | https://play.google/developer-content-policy/ |
| Gambling/Games | https://support.google.com/googleplay/android-developer/answer/9877032 |
| Content Ratings | https://support.google.com/googleplay/android-developer/answer/9898843 |
| Ads | https://support.google.com/googleplay/android-developer/answer/9857753 |
| Payments | https://support.google.com/googleplay/android-developer/answer/9858738 |
| User Data | https://support.google.com/googleplay/android-developer/answer/9888076 |
| Families | https://support.google.com/googleplay/android-developer/answer/9893335 |
| Target API | https://support.google.com/googleplay/android-developer/answer/11917020 |
| Permissions | https://support.google.com/googleplay/android-developer/answer/12579724 |
| Metadata | https://support.google.com/googleplay/android-developer/answer/9898842 |

## Self-improve

After each run, add any new policy area or repo-specific gotcha you hit to
this file, so the next review is sharper than the last.
