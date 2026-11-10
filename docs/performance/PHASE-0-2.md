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
