# PERF-ANDROID-001-C phases 0–2

Baseline: e6e50c5ffa4e0a17d591e50d7652bdae53dfda2e. Engine: 4.7.2-stable.
No Android results are available. Headless checks verify behavior only.

Run `godot --path client --resolution 1280x720 --script res://tests/performance_scenarios.gd -- ID 1800 SHA`.
IDs: 0 empty, 1 decor, 2/3/4/5 10/50/100/300 ships, 6 movement,
7 shots, 8 hits, 9 skin changes, 10 hangar updates, 11 chunk crossings,
12 half offscreen, 13 system transitions. Seed 77, 1280×720, camera 0/60/50.
Replay events follow frame number; record actual elapsed time too. Do not compare
runs with different event schedules or camera settings.

JSON is local under user://performance-ID.json. Metadata accepts only a SHA and
numeric scenario ID, metrics are allowlisted. CPU process is app-wide; render
CPU/GPU timing is WorldViewport only. GPU measurement is opt-in via
FrameStats.set_render_timing(true); unavailable/nonpositive readings are null
(N/A in UI). Compare timing off/on separately before using GPU measurements.
Diagnostics rotate three counters per refresh in the existing HUD slot.
4096 samples cover ten seconds through 409 FPS; window_truncated flags overflow.
Percentiles/text refresh every 0.5 seconds when the panel is visible; export is
an explicit on-demand calculation. No scene traversal runs every frame.

For each Android device run stock baseline and instrumented phase 0, then phases
1/2 with the same replay: cold launch, warmed repeated runs, and 30-minute thermal
runs. Keep OS power mode, refresh rate, resolution, seed and camera fixed. Compare
p95/p99 with Perfetto FrameTimeline, GPU capture and Android PSS. Measure diagnostic
overhead with overlay hidden/shown and timing off/on. Never use headless FPS as
Android performance evidence. Stock baseline needs the test-only replay copied
into an isolated checkout; production scripts must stay at the baseline SHA.

## Implementation / validation record

Phase 0: existing FrameStats extended; 52 checks pass, including 120 FPS window,
mean units and metadata rejection. 14 replays completed a 12-frame headless smoke
run each; all JSON exports returned OK. These are startup/behavior checks, not
GPU benchmarks. Diagnostic overhead and full-length Android runs remain NOT RUN.

Phase 1: initialization uses server hull/skin before ready; fallback ready remains.
Valid unchanged skin retains instance identity; failed loading can retry. Hangar
100-update identity test passes. Fleet initialization test passes.

Phase 2: local reset stops and detaches owned effects before removing references.
Weapon flashes/bolts reset together. Independent Tween channels replace prior
animation of the same properties. Death/respawn cancel old callbacks. No global
pool, resource caching or phases 3–8 introduced.

Validation with Godot 4.7.2.stable.official.ed1daf0bf:
- ship_visual_test: 54 checks pass, including 100 skin changes, 100 same-skin
  updates, 100 shots, overlapping hits, death/early respawn and resource cycles.
- hangar_test: 46 checks pass; fleet_shop_test: 54 checks pass.
- Existing offline test list from tools/test_client.sh: all exit 0 except
  corporation_test (3 failures). Identical failures reproduced on untouched
  e6e50c5: nil text assignment, member roles and scrolling tests.
- Expanded ship test emits one dummy-renderer `Parameter material is null` error
  while testing deletion/reload; no GDScript error. Rendered validation is required
  before accepting phase 2. Resource count stabilized within the tested bound;
  this is not proof of absence of every leak.
- Renderer tests NOT RUN: no DISPLAY/Xvfb and sudo unavailable.
- WebSocket/Go tests blocked: installed `go` reports `Go: Unknown option: test`.
- Android, GPU timing validity, diagnostic overhead, long thermal/memory tests,
  production server hidden-NPC regression: NOT RUN.

Readiness: implementation is reviewable; Android performance acceptance remains
pending for all phases, and phase 2 rendered deletion/reload validation is pending.
No FPS improvement claimed. No server/protocol/HUD editor/release changes.
Rollback: revert phase commits; added diagnostic export schema has no gameplay
persistence. Keep corporation failures separate from this client rendering work.

Follow-up: explicit hull rebuild now detaches old children immediately before
queue_free, preventing a second hull from remaining in SceneTree until frame end.
ShipView no longer treats a queued-for-deletion skin root as a valid unchanged
visual. ShipVisual: 55 behavioral checks pass; Hangar: 46 pass on 4.7.2.
The dummy material error remains reproducible in the deletion/reload test; this
follow-up does not resolve or waive that outstanding rendered-validation issue.
