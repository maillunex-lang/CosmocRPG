---
name: game-feel
trigger: /game-feel
description: >
  Audit a game's moment-to-moment tactile response — input latency, juice,
  hitstop, camera, accessibility of feel. Use on "feels mushy / unresponsive
  / dead" or before any playtest where the ask is "does this feel good."
user_invocable: true
---

<!-- Сторонний навык (MIT): https://github.com/DSmereski/agent-skills, коммит 7eb549e65c1bd6bba546c0250f8a8641459e5292. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->

> **Public / shared version** — trimmed from a private Claude Code setup and
> posted at smereski.com as a reusable pattern. No project-specific numbers
> or internal tooling names are included; the gate checklist, the Godot
> recipes, and the underlying frameworks are the real value and are left
> intact.

# game-feel — response, juice, hitstop, camera, and accessibility of feel

Game feel is Steve Swink's term for the quality of moment-to-moment
interaction: how it *feels* to press a button and see the world respond.
It has nothing to do with whether the game is well-designed at the systems
level — a perfectly balanced economy can still feel dead on input, and a
mechanically simple game can feel outstanding to interact with. The two are
orthogonal, and confusing them is how you end up with a game that passes
all your design reviews and still feels bad to play.

Swink's framework identifies three components: **real-time control** (input
latency from action to first feedback), **the simulation** (how the world
responds to that input), and **polish** (the layered amplification that
makes a response feel satisfying rather than merely present). The Vlambeer
talk "The Art of Screenshake" and Jonasson and Purho's "Juice it or Lose
It" GDC talk add the practical vocabulary: **juice** is amplified,
multi-channel feedback, and the key insight from both is that juice is not
decoration — it is information, delivered faster and more reliably than a
raw stat update.

Don Norman's affordance model is the underlying reason: the interface must
communicate what is happening and what happened. When a hit produces no
sound, no particle, no camera response, no freeze — when the feedback
channels are all silent — the player is left with the raw question "did
anything happen?" and the answer reads as "nothing." Juice answers that
question before the player consciously asks it.

## 8-item gate

**1. Response within 100ms — first feedback within 2–6 frames.**
From input to the first perceivable response: a sound onset, a sprite
change, the start of an animation, any sensor-accessible signal. 100ms
(6 frames at 60fps, 3 frames at 30fps) is the threshold at which a human
begins to perceive a response as delayed rather than immediate. Above it,
the game feels sluggish regardless of visual quality. Verify: time the
action-to-first-feedback interval for the primary verb.

**2. Two or more feedback channels per verb (visual + audio + haptic
where available).**
Every primary player action should produce at least visual and audio
feedback simultaneously. A third channel — haptic vibration on
controller, screen shake, a UI flash — reinforces the response without
requiring the player to be looking at the exact right spot. Verify: for
each primary verb, enumerate the feedback channels and confirm at least
two are firing.

**3. Juice = amplified feedback (particles + camera kick + sound + squash
and stretch).**
Juice means that the feedback is larger than the action strictly requires.
A sword hit produces a hit-spark particle burst, a brief camera punch, a
satisfying impact sound, a squash on the target's sprite — not just a
health number decrementing. The combination of channels, arriving
simultaneously, makes the feedback feel substantial. Verify: for the
highest-frequency combat or interaction verb, enumerate the active juice
elements and confirm at least three channels fire simultaneously.

**4. Hitstop — heavy hits briefly freeze both bodies (approximately
30–120ms).**
Hitstop is a momentary pause of simulation — typically 2–7 frames — when
a significant hit lands. Both the attacker and the target freeze briefly.
This is the single most common missing element in games that feel like
"attacks pass through enemies": the freeze communicates impact by
interrupting the flow of time, making the collision feel real rather than
cosmetic. Verify: on the heaviest hit type in the game, confirm a
time-freeze of at least 2 frames occurs at contact.

**5. Ease, don't snap — camera, UI, and values accelerate and settle.**
Values that change (health bars, score counters, resource displays) and
elements that move (camera tracking, UI transitions, damage numbers
floating up) should ease in and out, not jump to their target value
instantaneously. A health bar that snaps from 80 to 20 in one frame is a
missed opportunity to communicate urgency. A camera that cuts instantly is
disorienting. Verify: for the primary HUD value that changes most often,
confirm it uses an ease curve rather than a frame-instant update.

**6. Camera is an actor, not a bully — trauma-decayed, capped shake.**
Camera shake communicates impact, but uncapped or undamped shake is
nausea-inducing and reads as a technical error rather than a deliberate
effect. The standard implementation uses a **trauma** float (0.0–1.0)
that decays over time; shake offset is derived from `trauma^2` or
`trauma^3` (so high trauma is very rough, low trauma settles quickly).
Shake is capped at a maximum pixel/degree offset and never fights the
player's camera intent. Verify: confirm trauma-based decay is implemented,
shake has a maximum offset, and the camera does not fight player input.

**7. Restraint and accessibility — a reduce-motion option that is
player-reachable in the options UI.**
Juice is not appropriate for all players: vestibular disorders,
photosensitivity, and motion sickness make camera shake, particle bursts,
and screen flash actively harmful. A reduce-motion toggle that disables
or attenuates these effects must be present and reachable in the options
UI — not buried in a config file or available only to developers. Verify:
confirm the toggle exists in-game in the options UI and that enabling it
actually suppresses the primary shake, flash, and high-frequency particle
effects.

**8. Input grace — buffering, coyote time, and aim forgiveness.**
Grace mechanics absorb the gap between the player's intent and the
frame-perfect reality of the game: input buffering holds an action input
for a short window so it fires as soon as the player becomes able to
perform it; coyote time allows a jump input for several frames after
walking off a ledge (so "I pressed jump right there" is honored); aim
forgiveness magnetizes the nearest valid target within a cone to reduce
the frustration of pixel-perfect aiming requirements. Verify: for
whichever of these is applicable to the game's primary verb (platformer
→ coyote time, targeting → aim forgiveness, combo system → input buffer),
confirm it is implemented with a named, tunable window.

Done when all 8 items pass a concrete check. Whether the result *feels
good* — whether the tuned parameters are satisfying rather than merely
present — is a judgment that belongs to a human playtest, not to this gate.
The gate catches missing mechanics; the playtest judges whether present
mechanics are tuned correctly.

## Godot recipes

Concrete implementation patterns for each gate item in Godot 4.

**1. Response — `create_tween`**
```gdscript
# Fire on the same frame as the input. No deferred calls on the first
# feedback signal.
func _on_hit():
    _play_hit_sound()              # AudioStreamPlayer.play() — frame-instant
    _spawn_hit_particles()         # CPUParticles2D: emitting = true
    _start_squash_tween()

func _start_squash_tween():
    var t = create_tween()
    t.tween_property(sprite, "scale", Vector2(1.3, 0.7), 0.05)
    t.tween_property(sprite, "scale", Vector2(1.0, 1.0), 0.08)
```

**2. Channels — `AudioStreamPlayer`**
```gdscript
# Give every primary verb its own AudioStreamPlayer node so sounds
# can overlap independently. Set bus = "SFX".
@onready var hit_sfx: AudioStreamPlayer = $HitSFX

func _play_hit_sound():
    hit_sfx.pitch_scale = randf_range(0.9, 1.1)  # slight variation
    hit_sfx.play()
```

**3. Juice — `CPUParticles2D`**
```gdscript
# One-shot burst: set One Shot = true, Explosiveness = 1.0 in inspector.
# Enable/disable via:
@onready var hit_particles: CPUParticles2D = $HitParticles

func _spawn_hit_particles():
    hit_particles.restart()   # resets and fires one burst
```

**4. Hitstop — `Engine.time_scale`**
```gdscript
# Brief time-scale freeze. Use a Timer node on the tree root (not scaled)
# so the unfreezer fires correctly.
func _apply_hitstop(duration: float = 0.05):
    Engine.time_scale = 0.0
    await get_tree().create_timer(duration, true).timeout
    Engine.time_scale = 1.0
```
Note: `create_timer(duration, true)` passes `process_always = true` so the
timer runs even when `time_scale` is 0.

**5. Ease — `Tween` on HUD values**
```gdscript
# Animate a health bar rather than snapping it.
func update_health(new_value: float):
    var t = create_tween()
    t.tween_property(health_bar, "value", new_value, 0.15)\
     .set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_OUT)
```

**6. Camera — trauma-based shake**
```gdscript
extends Camera2D

var trauma: float = 0.0
const DECAY = 2.5          # units per second
const MAX_OFFSET = 12.0    # pixels

func add_trauma(amount: float):
    trauma = clamp(trauma + amount, 0.0, 1.0)

func _process(delta: float):
    trauma = max(trauma - DECAY * delta, 0.0)
    var shake_amount = trauma * trauma   # quadratic falloff
    offset = Vector2(
        randf_range(-MAX_OFFSET, MAX_OFFSET) * shake_amount,
        randf_range(-MAX_OFFSET, MAX_OFFSET) * shake_amount
    )
```

**7. Reduce-motion toggle**
```gdscript
# In your Settings autoload:
var reduce_motion: bool = false :
    set(v):
        reduce_motion = v
        _apply_motion_settings()

func _apply_motion_settings():
    # Disable/enable camera shake, particle systems, and flash effects
    # globally by reading this flag wherever those systems are triggered.
    pass

# Persist via:
func save_settings():
    var cfg = ConfigFile.new()
    cfg.set_value("accessibility", "reduce_motion", reduce_motion)
    cfg.save("user://settings.cfg")
```
Wire the toggle to a CheckButton in your options screen. Always default to
`false` (juice on) but respect the system preference if the platform
exposes one.

**8. Input grace — nearest-target aim forgiveness**
```gdscript
# On attack: find the nearest enemy within a forgiveness cone and
# redirect the action toward it.
func _get_forgiven_target(origin: Vector2, direction: Vector2,
                           cone_deg: float = 20.0) -> Node2D:
    var best: Node2D = null
    var best_dot: float = cos(deg_to_rad(cone_deg))
    for enemy in get_tree().get_nodes_in_group("enemies"):
        var to_enemy = (enemy.global_position - origin).normalized()
        var dot = direction.dot(to_enemy)
        if dot > best_dot:
            best_dot = dot
            best = enemy
    return best
```
For platformers, coyote time is a frame counter: decrement each physics
frame after leaving the ground, allow jump input while counter > 0.

## Reference

- Swink, S. — *Game Feel: A Game Designer's Guide to Virtual Sensation*
  (2009). The primary framework: real-time control, simulation, polish.
- Vlambeer / Rami Ismail — "The Art of Screenshake" (GDC 2013).
  Practical juice layering from an indie practitioner.
- Jonasson, M. & Purho, P. — "Juice it or Lose it" (GDC 2012).
  The canonical demonstration that multi-channel juice is information,
  not decoration.
- Norman, D. — *The Design of Everyday Things* (2013). Affordance and
  feedback as the foundation of legible interaction design.
