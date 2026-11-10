# Android graphics controls

Settings → «Графика и производительность» applies and saves:
- low / medium / high: existing decor/background presets and WorldViewport MSAA
  off / 2x / 4x;
- 3D render scale: 50%, 67%, 75%, 85%, 100%; UI resolution stays unchanged;
- frame cap: 30 or 60 FPS (a ceiling, not a promise of the achieved rate);
- ordinary foreign full 3D hulls: 5–50, step 5. Own ship excluded.

Settings use the existing user://settings.cfg [ui] section. Missing keys default
to medium, 100%, 60 FPS, 20 hulls; out-of-range/nonfinite values are normalized.

The cap ranks target first, then screen presence, then squared distance to the
player, with a retention bias and ID tie break. Re-evaluation occurs on snapshots,
target changes, setting changes and periodic camera updates. Models are hidden
and reused; hidden decorative motion/fire stops. Death/respawn state, interpolation,
server snapshots, map and picking remain active. Existing affiliation markers and
Nameplates remain; full names retain their existing UI budget. Hidden models stay
resident, so this cap does not guarantee a reduction in model memory. This is the
initial user cap, not the complete previously proposed adaptive/significance system.

Validation: Godot 4.7.2-stable. graphics_settings_test: 25 checks pass, including
all ten caps, own ship exclusion, hidden target promotion without reinstancing,
hidden death/respawn, and WorldViewport-only resolution scaling. Existing offline
runner list passes except corporation_test's three known baseline failures.
ShipVisual still emits its documented dummy-renderer material error. Android GPU,
thermal, full online and real-rendered acceptance remain unmeasured/unavailable.
Owner accepted prior phase acceptance without these checks. No FPS improvement
is claimed. Server/protocol, map and HUD Layout Editor files are unchanged.
