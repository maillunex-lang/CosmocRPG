---
name: ai-disclosure
description: >
  Audit every public property you publish for missing AI-content disclosure
  against a dated, cited ruleset (EU AI Act Art. 50, FTC guidance, Steam and
  Google Play policies, US state laws), and keep a compressed ledger of every
  AI-touched artifact so a law change instantly lists what needs revisiting.
  Not legal advice — plumbing that makes diligence routine.
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

# ai-disclosure — nothing ships with undisclosed AI content

Compliance tooling has a rot problem: whatever rules you encode today will be
wrong within a year. The design answer is to separate what changes from what
does not — three parts:

- **LAW.md** — the ruleset. Every rule carries: jurisdiction, source URL, a
  PRIMARY/SECONDARY confidence grade (statute read directly vs. summary), an
  effective date, WHO it binds, and a verdict on whether that includes you.
  The file carries `law_version: N` and `reviewed: YYYY-MM-DD` stamps. ONLY a
  human promotes changes. The agent researches and proposes; it never
  silently rewrites its own laws.
- **scripts/audit.py** — the mechanism. Deterministic yes/no checks per
  property: does the disclosure page exist, is it publicly routable (a
  disclosure behind a login wall is not a disclosure), does the audio player
  label the voice synthetic, does each AI-generated media file carry a
  machine-readable mark in its metadata (check with ffprobe), does each
  AI-touched item render its badge. FAIL = a required disclosure is missing;
  exit non-zero so it gates the deploy. WARN = your own transparency posture.
  INFO = needs a one-time human classification.
- **ledger.jsonl** — one compact line per AI-touched artifact: date,
  property, artifact, kind, generating tools, the `law_version` in force
  when it shipped, and what disclosure it received. ~130 bytes each.

## The self-audit loop (monthly, and on any AI-law news)

1. `audit.py --check-law` prints the ruleset's age (warn past 90 days) and
   the authoritative source list to re-check.
2. The agent re-searches those sources and diffs findings against LAW.md —
   the semantic half is the agent's job; the script does the deterministic
   half.
3. A real change is PRESENTED to the human. On approval: edit LAW.md, bump
   `law_version`, update `reviewed:`.
4. `audit.py --relaw` lists every ledger artifact stamped under an older
   version — the exact re-review worklist, derived instead of remembered.

## Artifact kinds worth encoding first

- `audio-synthetic` — TTS narration, cloned voices. Under EU AI Act Art. 50
  this wants a visible label AND a machine-readable mark (an ID3/metadata
  comment survives the file being downloaded and shared).
- `text-ai-assisted` — drafted with an LLM, human-edited. The EU carve-out
  for human editorial control likely applies; a per-item badge makes the
  scoping question moot.
- `image-ai` / `video-ai` / `music-ai` — generated assets. Also feed store
  declarations: Steam discloses player-facing gen-AI content (dev tools
  exempt); Google Play wants user-visible in-app labels.
- `promo-ai` — AI in promotional media requires disclosure (FTC posture).
- `review-ai` — FORBIDDEN. AI-fabricated reviews and testimonials are
  deceptive regardless of labeling. Disclosure does not cure this one.

## Standing rules

- Required misses gate deploys — wire the audit into every publish flow.
- Every new AI-touched artifact gets disclosed AND ledgered in the same
  session it ships.
- Store submissions are pre-submission gates: complete the store's AI
  declaration from the ledger's entries for that project.
- Wire a file-watch hook so any write to content paths surfaces a reminder
  to run the audit — the failure mode this exists for is shipping the
  feature without ever being asked the question.

## Hard-won gotchas

- A disclosure page behind an auth wall is not a disclosure — audit the
  route's public reachability, not just the file's existence.
- Headlines mislead in both directions: some famous laws bind million-user
  AI *providers*, not small deployers — while store policies with real
  enforcement get no headlines at all. Encode who each rule BINDS.
- Grade every citation. A law-firm blog summary is SECONDARY; re-verify
  against the statute before relying on it hard.
