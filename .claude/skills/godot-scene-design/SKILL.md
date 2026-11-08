---
name: godot-scene-design
description: Проектирование сцен Godot 4 (.tscn): формат файла, иерархия узлов, раскладка уровней и интерфейса.
---

<!-- Сторонний навык (MIT): https://github.com/alexmeckes/godot-claude-skills, коммит 881ff9d285b8dfa5e343ce1b32bdcff367fe89f8. Подключён без изменений текста; контекст проекта «Уралия» — см. docs/SKILLS.md. Лицензия — .claude/skills/THIRD_PARTY_SKILLS.md -->
# Godot Scene Design Skill

You are an expert at designing and creating Godot 4.x scenes (.tscn files), node hierarchies, and level layouts.

## Scene File Format (TSCN)

Godot uses a text-based scene format that's highly readable and AI-friendly.

### Basic Structure
```ini
[gd_scene load_steps=3 format=3 uid="uid://abc123"]

[ext_resource type="Script" path="res://scripts/player.gd" id="1_abc"]
[ext_resource type="Texture2D" path="res://sprites/player.png" id="2_def"]

[sub_resource type="CircleShape2D" id="CircleShape2D_xyz"]
radius = 16.0

[node name="Player" type="CharacterBody2D"]
script = ExtResource("1_abc")
speed = 200.0

[node name="Sprite2D" type="Sprite2D" parent="."]
texture = ExtResource("2_def")

[node name="CollisionShape2D" type="CollisionShape2D" parent="."]
shape = SubResource("CircleShape2D_xyz")

[connection signal="body_entered" from="." to="." method="_on_body_entered"]
```

### Key Sections
1. **Header**: `[gd_scene load_steps=N format=3]` - N is total resources + 1
2. **External Resources**: References to other files (scripts, textures, scenes)
3. **Sub-resources**: Inline resources (shapes, materials, curves)
4. **Nodes**: Scene tree structure with parent references
5. **Connections**: Signal connections between nodes

## Common Scene Patterns

### Player Character (2D Platformer)
```
Player (CharacterBody2D)
├── Sprite2D
├── CollisionShape2D
├── AnimationPlayer
├── Camera2D
│   └── (smooth follow settings)
├── Hitbox (Area2D)
│   └── CollisionShape2D
└── RayCast2D (ground detection)
```

### Player Character (3D)
```
Player (CharacterBody3D)
├── CollisionShape3D
├── MeshInstance3D (or imported model)
├── AnimationTree
├── Camera3D
│   └── SpringArm3D (for collision)
└── Marker3D (weapon attachment points)
```

### Enemy (2D)
```
Enemy (CharacterBody2D)
├── Sprite2D
├── CollisionShape2D
├── AnimationPlayer
├── DetectionArea (Area2D)
│   └── CollisionShape2D
├── Hitbox (Area2D)
│   └── CollisionShape2D
├── NavigationAgent2D
└── Timer (attack cooldown)
```

### UI Screen
```
UIScreen (Control)
├── Background (TextureRect/ColorRect)
├── VBoxContainer
│   ├── Title (Label)
│   ├── HSeparator
│   └── ButtonContainer (VBoxContainer)
│       ├── StartButton (Button)
│       ├── OptionsButton (Button)
│       └── QuitButton (Button)
└── AnimationPlayer (transitions)
```

### HUD
```
HUD (CanvasLayer)
└── Control
    ├── MarginContainer
    │   └── HBoxContainer
    │       ├── HealthBar (ProgressBar/TextureProgressBar)
    │       └── ScoreLabel (Label)
    ├── Minimap (SubViewportContainer)
    │   └── SubViewport
    │       └── Camera2D
    └── DialogBox (PanelContainer)
        └── VBoxContainer
            ├── SpeakerLabel
            └── DialogText (RichTextLabel)
```

### Inventory Item
```
Item (Area2D)
├── Sprite2D
├── CollisionShape2D (pickup area)
├── AnimationPlayer (bob/glow)
└── AudioStreamPlayer2D (pickup sound)
```

### Level/World
```
Level (Node2D)
├── TileMapLayer (or TileMap for older)
├── Entities (Node2D)
│   ├── Player
│   ├── Enemies (Node2D)
│   │   ├── Enemy1
│   │   └── Enemy2
│   └── Items (Node2D)
├── NavigationRegion2D
│   └── (navigation polygon)
├── ParallaxBackground
│   └── ParallaxLayer
│       └── Sprite2D
└── Camera2D (level bounds)
```

### Dialogue System
```
DialogueManager (Node)
├── CanvasLayer
│   └── DialogueBox (Control)
│       ├── Portrait (TextureRect)
│       ├── NameLabel (Label)
│       ├── TextLabel (RichTextLabel)
│       └── ChoicesContainer (VBoxContainer)
├── Timer (text reveal)
└── AudioStreamPlayer (voice/sfx)
```

## Node Selection Guide

### Physics Bodies
| Node | Use Case |
|------|----------|
| `CharacterBody2D/3D` | Player, enemies, NPCs with custom movement |
| `RigidBody2D/3D` | Physics-driven objects (crates, balls) |
| `StaticBody2D/3D` | Immovable collision (walls, floors) |
| `Area2D/3D` | Triggers, detection zones, pickups |

### Visual Nodes (2D)
| Node | Use Case |
|------|----------|
| `Sprite2D` | Static images |
| `AnimatedSprite2D` | Frame-based animations |
| `TileMapLayer` | Level geometry, backgrounds |
| `Line2D` | Trails, lasers, drawing |
| `Polygon2D` | Custom shapes |
| `CPUParticles2D` | Effects (compatible) |
| `GPUParticles2D` | Effects (performant) |

### Visual Nodes (3D)
| Node | Use Case |
|------|----------|
| `MeshInstance3D` | 3D models |
| `CSGBox3D/Sphere3D/etc` | Prototyping geometry |
| `MultiMeshInstance3D` | Many identical objects |
| `GPUParticles3D` | 3D particle effects |
| `Decal` | Projected textures |

### UI Nodes
| Node | Use Case |
|------|----------|
| `Control` | Base container |
| `Label/RichTextLabel` | Text display |
| `Button/TextureButton` | Clickable elements |
| `ProgressBar/TextureProgressBar` | Health bars, loading |
| `HBoxContainer/VBoxContainer` | Auto-layout |
| `MarginContainer` | Padding |
| `PanelContainer` | Styled backgrounds |
| `ScrollContainer` | Scrollable content |
| `TabContainer` | Tabbed interfaces |

### Audio Nodes
| Node | Use Case |
|------|----------|
| `AudioStreamPlayer` | Global audio (music, UI) |
| `AudioStreamPlayer2D` | Positional 2D audio |
| `AudioStreamPlayer3D` | Positional 3D audio |

### Utility Nodes
| Node | Use Case |
|------|----------|
| `Timer` | Delayed actions, cooldowns |
| `AnimationPlayer` | Property animations |
| `AnimationTree` | Complex animation state machines |
| `Tween` | Programmatic animations |
| `RayCast2D/3D` | Line-of-sight, ground detection |
| `ShapeCast2D/3D` | Swept collision detection |

## Collision Layers Best Practice

```
Layer 1: World (static geometry)
Layer 2: Player
Layer 3: Enemies
Layer 4: Player projectiles
Layer 5: Enemy projectiles
Layer 6: Pickups
Layer 7: Triggers/areas
Layer 8: Interactables
```

Set in scene:
```ini
[node name="Player" type="CharacterBody2D"]
collision_layer = 2
collision_mask = 1 | 3 | 6  # Collides with world, enemies, pickups
```

## Groups for Organization

Use groups to organize and find nodes:
```ini
[node name="Enemy" type="CharacterBody2D" groups=["enemies", "damageable"]]
```

Common groups:
- `enemies` - All enemy nodes
- `damageable` - Anything that can take damage
- `interactable` - Objects player can interact with
- `persistent` - Nodes that persist between scenes
- `pausable` - Nodes affected by pause

## Scene Instancing

Reference external scenes:
```ini
[ext_resource type="PackedScene" path="res://scenes/enemy.tscn" id="enemy_scene"]

[node name="Enemy1" parent="Enemies" instance=ExtResource("enemy_scene")]
position = Vector2(100, 200)

[node name="Enemy2" parent="Enemies" instance=ExtResource("enemy_scene")]
position = Vector2(300, 200)
```

## Property Overrides

Override instanced scene properties:
```ini
[node name="Enemy1" parent="Enemies" instance=ExtResource("enemy_scene")]
position = Vector2(100, 200)
speed = 150.0  # Override exported property
```

## Animation Setup

### AnimationPlayer in Scene
```ini
[sub_resource type="Animation" id="Animation_idle"]
length = 1.0
loop_mode = 1
tracks/0/type = "value"
tracks/0/path = NodePath("Sprite2D:frame")
tracks/0/interp = 1
tracks/0/keys = {
"times": PackedFloat32Array(0, 0.25, 0.5, 0.75),
"values": [0, 1, 2, 3]
}

[node name="AnimationPlayer" type="AnimationPlayer" parent="."]
libraries = {
"": SubResource("AnimationLibrary_abc")
}
```

## Tips for AI Scene Generation

1. **Start with the root node type** - Choose based on the scene's purpose (CharacterBody2D for characters, Control for UI, Node2D for levels)

2. **Build hierarchy logically** - Visual nodes as children of physics bodies, UI elements in containers

3. **Use appropriate collision shapes** - RectangleShape2D for boxes, CapsuleShape2D for characters, circles for projectiles

4. **Set up layers correctly** - Avoid "everything collides with everything"

5. **Name nodes descriptively** - "PlayerSprite" not "Sprite2D", "HealthBar" not "ProgressBar"

6. **Use groups for runtime queries** - `get_tree().get_nodes_in_group("enemies")`

7. **Instance reusable scenes** - Don't duplicate complex node trees

8. **Consider processing order** - Parent nodes process before children

9. **Z-index for 2D layering** - Background < World < Entities < Effects < UI

10. **Use CanvasLayer for HUD** - Keeps UI separate from game camera
