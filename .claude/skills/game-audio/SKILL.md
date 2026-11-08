---
name: game-audio
trigger: /game-audio
description: >
  Wire a game's audio layer from scratch — bus topology, music autoload,
  SFX-to-verb mapping, headless probe asserting buses and playback, and
  a board ticket per game. Use when adding audio to a new project or
  auditing whether audio is fully wired.
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific paths,
> internal tool names, or game-specific references are included; the
> wiring methodology and the probe pattern are the real value and are left
> intact.

# game-audio — buses, music, SFX, and a probe that catches missing wires

Audio is the feedback channel that reaches the player even when they are
not looking at the screen. It is also the channel most commonly left as a
"we'll add it later" stub — and later never comes. This skill treats audio
wiring as a first-class structural concern, not a polish step: buses get
set up when the project is scaffolded, every core verb gets a sound at the
same moment it gets a visual effect, and a headless probe asserts the
audio graph is intact so a future refactor cannot silently break it.

The three-layer model (buses, music, SFX) keeps concerns separated and
makes per-system volume control available to the player from day one.

## Steps

### 1. Buses: Master > Music > SFX, volume per bus in the options UI

Set up at minimum three audio buses in the engine's bus layout:

- **Master** — top-level output, controls overall volume.
- **Music** — child of Master, routes all background music. Allows the
  player to mute music without silencing sound effects.
- **SFX** — child of Master, routes all gameplay sound effects. Allows the
  player to mute effects without silencing music.

Expose volume controls for each bus in the options/settings UI. The player
should be able to set Music and SFX volume independently, from session
start, without digging into any accessibility menu. This is not a polish
feature — it is expected by default on any platform that has had audio
settings since the SNES era.

In Godot, set bus assignments directly on each `AudioStreamPlayer` node
via the `bus` property. In Unity, route through Audio Mixer Groups. In
other engines, use the equivalent routing primitive.

```gdscript
# Godot: in AudioStreamPlayer inspector or via code
$MusicPlayer.bus = "Music"
$SFXPlayer.bus = "SFX"
```

### 2. Music: autoload player on the Music bus, background loop plus N tracks, shuffled rotation

Create an autoload (singleton) `MusicManager` that owns one or more
`AudioStreamPlayer` nodes assigned to the Music bus. Responsibilities:

- Hold a playlist of tracks for each game state (menu, gameplay,
  high-tension, victory, defeat — whatever states the game has).
- Loop the current track, with configurable crossfade duration between
  tracks (even a simple 0.5s linear fade eliminates jarring cuts).
- Shuffle rotation within a state so the same track does not play twice in
  a row unless the playlist has only one entry.
- Expose `play_state(state_name)` and `stop()` so scene code never
  directly touches the music player — it only calls the manager.

```gdscript
# MusicManager autoload sketch
extends Node

var _playlists: Dictionary = {}   # state_name -> Array[AudioStream]
var _current_player: AudioStreamPlayer
var _queue: Array[AudioStream] = []

func play_state(state: String) -> void:
    if not _playlists.has(state):
        return
    _queue = _playlists[state].duplicate()
    _queue.shuffle()
    _play_next()

func _play_next() -> void:
    if _queue.is_empty():
        return
    var stream = _queue.pop_front()
    _current_player.stream = stream
    _current_player.play()
```

For music sources, CC0 and royalty-free repositories (e.g. OpenGameArt,
Freesound, itch.io audio packs) cover most moods for a prototype or indie
ship. A music generation pipeline can produce custom tracks at arbitrary
length when CC0 sources do not fit the game's tone.

### 3. SFX: map every core verb and juice event to a sound, played at the same call site as the visual effect

Enumerate every primary player action and every juice event from
`game-feel`. For each one, there must be a corresponding sound that fires
at the same call site — not in a separate system that may drift out of sync:

- Player primary verb fires → hit/swing/fire SFX + visual effect
- Pickup collected → collect SFX + particle burst
- Enemy defeated → defeat SFX + death animation
- UI button pressed → UI click SFX + button press visual
- Level complete → fanfare SFX + result screen
- Player death → impact SFX + death animation

Placing the SFX call at the same call site as the visual effect is
deliberate: it prevents the two from getting separated in a future
refactor, and it ensures the audio-visual synchronization that the brain
uses to judge whether an event "felt right."

```gdscript
# Correct: SFX and visual at the same call site
func _on_enemy_hit(enemy: Node):
    hit_sfx.play()                     # audio
    hit_particles.restart()            # visual
    _apply_hitstop(0.05)               # feel
    enemy.take_damage(attack_damage)   # logic
```

For SFX sources, CC0 audio packs (Kenney's audio packs, Freesound CC0
filter, itch.io free SFX collections) cover common game sounds. Vary pitch
slightly on repeated sounds to prevent listener fatigue:

```gdscript
func _play_sfx(player: AudioStreamPlayer) -> void:
    player.pitch_scale = randf_range(0.9, 1.1)
    player.play()
```

### 4. Probe: headless test asserting buses exist, music autoload present, every stream loads, every verb bumps a played counter

A headless test that runs in CI or as part of the pre-playtest gate. It
should assert:

- The Audio Bus Layout contains at minimum Master, Music, and SFX buses.
- The MusicManager autoload is registered and its node is present in the
  scene tree at runtime.
- Every audio stream file referenced in the project loads without error
  (no missing file, no corrupt import).
- For each primary verb, a "played" counter increments when the verb fires
  (instruments the SFX call site via a test-only counter).

```gdscript
# tests/probe_audio.gd (headless, exits with code)
extends SceneTree

func _init():
    # Check buses
    assert(AudioServer.get_bus_index("Music") != -1, "Music bus missing")
    assert(AudioServer.get_bus_index("SFX") != -1, "SFX bus missing")

    # Check autoload
    assert(has_node("/root/MusicManager"), "MusicManager autoload missing")

    # Check streams load (replace paths with your actual stream resources)
    var streams = [
        "res://audio/music/theme_menu.ogg",
        "res://audio/sfx/hit.wav",
        "res://audio/sfx/pickup.wav",
    ]
    for path in streams:
        var res = load(path)
        assert(res != null, "Audio stream failed to load: " + path)

    quit(0)
```

Run this probe as part of your headless verification gate before any human
playtest session.

### 5. Board tickets: one audio-wiring ticket per game

Create exactly one ticket in your project's task board titled
"Audio wiring — [game name]" that covers the full scope of this skill:
bus setup, music autoload, SFX-to-verb mapping, and probe green. Keep it
as a single ticket rather than splitting into many so the audio layer is
treated as a unit of work with a clear done-condition: the probe passes and
a human confirms the audio feels complete.

Done when the headless probe exits 0, every primary verb has a confirmed
SFX mapped to it, and a human playtest session confirms the audio layer
feels present and appropriately calibrated — not intrusive, not silent.

## Reference

- Godot Audio Bus documentation — bus routing, bus volume, `AudioStreamPlayer.bus` property.
- Kenney.nl audio packs — CC0 sound effects covering UI, retro, impact, and
  nature categories. Free for commercial use.
- OpenGameArt.org — CC0 and CC-BY music and SFX repository.
- Freesound.org — CC0 filter for zero-attribution sound effects.
- itch.io — search "free audio" for CC0 game audio packs.
- A music generation pipeline can produce custom-length background tracks
  when CC0 libraries do not fit the required tone.
