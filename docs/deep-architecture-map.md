# Deep Architecture Map: Roblox Studio MCP

## 1. Overview and the Challenge of Roblox Complexity

Building complete, production-grade Roblox games requires much more than placing colored blocks or writing isolated scripts. Roblox Studio is a full game engine with:
- An extensive object-oriented hierarchy with hundreds of specialized classes.
- Complex data types that do not natively map to JSON (such as CFrames, UDim2, ColorSequences, and PhysicalProperties).
- A layered, Photoshop-level user interface system combining scale/offset coordinates, anchors, auto-layouts, clipping groups, and vector styling.
- A client-server replication model governed by network boundaries, RemoteEvents, and security contexts.
- Physics constraints, particle systems, PBR materials, and real-time lighting pipelines.

This document maps the entire object ecosystem, type system, dependency tree, and MCP tool architecture required to make full game creation possible.

---

## 2. Roblox DataModel and Hierarchy Map

Everything in Roblox exists inside the root `DataModel` (the `game` object). An AI assistant must understand the core top-level services and their distinct responsibilities:

```
DataModel (game)
├── Workspace (3D World, Terrain, Camera, physics parts)
├── Lighting (Atmosphere, Sky, Bloom, ColorCorrection, SunRays)
├── ReplicatedStorage (Shared modules, remote events, shared assets)
├── ReplicatedFirst (Client bootstrapper, loading screens)
├── ServerScriptService (Server-side game logic, managers, databases)
├── ServerStorage (Server-only templates, weapons, monster models)
├── StarterGui (ScreenGuis copied to PlayerGui upon joining/spawning)
├── StarterPack (Default tools given to players on spawn)
├── StarterPlayer
│   ├── StarterPlayerScripts (Client-side scripts running per player)
│   └── StarterCharacterScripts (Scripts cloned into character models)
├── SoundService (SoundGroups, ambient audio, reverb presets)
├── TextChatService (Chat channels, message commands, bubble chat)
├── PhysicsService (Collision groups, non-colliding teams)
├── TweenService (Smooth interpolation for CFrames, colors, sizes)
├── CollectionService (Tag-based architecture for game components)
└── Players (Player sessions, leaderstats, character rigs)
```

---

## 3. Data Types and JSON Serialization Matrix

Roblox uses custom engine data types. Because MCP transmits data as standard JSON, the bridge must serialize and deserialize these types bi-directionally:

| Roblox Type | Purpose / Usage | JSON Wire Format | Luau Deserialization |
|---|---|---|---|
| `Vector3` | 3D Position, Size, Velocity | `{"x": 10, "y": 5, "z": -20}` | `Vector3.new(x, y, z)` |
| `Vector2` | 2D Position, Screen coords | `{"x": 1920, "y": 1080}` | `Vector2.new(x, y)` |
| `CFrame` | Position + 3D Rotation matrix | `{"pos": [0, 5, 0], "rot": [0, 90, 0]}` | `CFrame.new(...) * CFrame.Angles(...)` |
| `UDim` | 1D GUI dimension (Scale + Offset) | `{"scale": 0.5, "offset": 10}` | `UDim.new(scale, offset)` |
| `UDim2` | 2D GUI dimension (X & Y UDims) | `{"xs": 0.5, "xo": 0, "ys": 0.2, "yo": 50}` | `UDim2.new(xs, xo, ys, yo)` |
| `Color3` | RGB colors (0 to 1 or 0 to 255) | `{"r": 255, "g": 128, "b": 0, "rgb": true}` | `Color3.fromRGB(r, g, b)` |
| `ColorSequence` | Multi-stop color gradients | `[{"t": 0, "c": [255,0,0]}, {"t": 1, "c": [0,0,255]}]` | `ColorSequence.new(keypoints)` |
| `NumberSequence` | Keyframe curves (transparency/size) | `[{"t": 0, "v": 1}, {"t": 0.5, "v": 0.2}, {"t": 1, "v": 0}]` | `NumberSequence.new(keypoints)` |
| `NumberRange` | Min/max random variance | `{"min": 5, "max": 15}` | `NumberRange.new(min, max)` |
| `Font` | Typography, weights, styles | `{"family": "rbxasset://...", "weight": "Bold"}` | `Font.new(family, Enum.FontWeight.Bold)` |
| `BrickColor` | Legacy preset palette color | `"Bright red"` | `BrickColor.new(name)` |
| `RaycastParams` | Ray intersection filtering | `{"filter": ["Path"], "type": "Exclude"}` | `RaycastParams.new()` |

---

## 4. The UI / GUI System: Photoshop-Level Complexity

Roblox GUI is resolution-independent, multi-layered, and supports advanced layout and masking rules:

### A. Coordinate System and Multi-Device Scaling
* **Scale vs Offset**:
  - `Scale` (0.0 to 1.0): Percentage of the parent container size.
  - `Offset` (integers): Fixed pixel size regardless of screen resolution.
  - Best practice for cross-platform (Mobile, Tablet, Desktop, Console): Use `Scale` for responsive sizing and positioning, with minimal `Offset` for borders or fixed spacing.
* **AnchorPoint** (`Vector2`): Determines the origin of the UI element (e.g., `(0.5, 0.5)` for exact centering, `(1, 0)` for top-right alignment).
* **ZIndex and DisplayOrder**: Controls draw order across siblings and across independent `ScreenGui` layers.

### B. UI Component Hierarchy
* **Roots**: `ScreenGui` (heads-up display), `BillboardGui` (3D floating over objects), `SurfaceGui` (projected onto part surfaces).
* **Containers**:
  - `Frame`: Standard rectangular container.
  - `ScrollingFrame`: Scrollable list/canvas with custom scrollbar controls, canvas size, and elastic bounces.
  - `CanvasGroup`: Advanced container that clips descendants and allows whole-group opacity changes without artifacting (essential for fade-in/fade-out menus).
* **Display Elements**:
  - `TextLabel`: Typography, RichText support (XML markup for inline color, bold, size, strokes), auto-scaled text.
  - `ImageLabel`: Spritesheets, textures, SliceCenter for 9-slice scalable borders.
* **Input Elements**:
  - `TextButton` and `ImageButton`: Click, touch, and gamepad activation.
  - `TextBox`: Text input from users with placeholder text and clear-on-focus options.

### C. Layout and Constraint Engines (Automatic Layouts)
* `UIListLayout`: Automatically flows child objects vertically or horizontally with customizable padding, alignment, and sorting.
* `UIGridLayout`: Automatically creates responsive grids (inventory slots, shop items) based on cell size and cell padding.
* `UIPageLayout`: Creates horizontal or vertical swipable pages (e.g., tutorial carousels or menu tabs).
* `UIPadding`: Adds top, bottom, left, and right internal margins to any container.
* `UIAspectRatioConstraint`: Locks an element to a specific aspect ratio (e.g., 1:1 for square buttons or 16:9 for cutscene frames) preventing distortion across phone vs ultrawide screens.
* `UISizeConstraint`: Enforces minimum and maximum pixel boundaries.
* `UIScale`: Uniformly scales an entire UI hierarchy up or down (great for dynamic accessibility scaling).

### D. Visual Effects and Styling Elements
* `UICorner`: Rounds container edges with a given radius (`UDim`).
* `UIStroke`: Adds borders to frames or outlines to text with border modes (`Border`, `Middle`, `Contextual`).
* `UIGradient`: Applies color gradients, rotation angles, and transparency curves across any GUI element.

---

## 5. 3D World, Physics, and Mechanical Systems

Roblox games rely heavily on physical interactions and spatial architecture:

### A. Primitive and Mesh Geometry
* `Part`: Block, Ball, Cylinder, Wedge, CornerWedge.
* `MeshPart`: Custom 3D meshes with custom collision fidelity (`PreciseConvexDecomposition`, `Box`, `Hull`).
* `TrussPart`: Climbable ladders.
* `Terrain`: Voxel-based landscape with materials (Grass, Water, Sand, Rock, Snow) editable via `Terrain:FillBlock`, `Terrain:FillBall`, and voxel streaming.

### B. Constraints and Attachments
* `Attachment`: Spatial reference points on parts defining axes, orientation, and constraint anchors.
* `WeldConstraint`: Locks two parts together rigidly without breaking during movement.
* `Motor6D`: Animated joints connecting character limbs, weapon attachments, and mechanical doors.
* `HingeConstraint`: Rotating axles (wheels, spinning blades, swinging doors) with servo or motor actuators.
* `PrismaticConstraint`: Sliding rails (elevators, pistons).
* `SpringConstraint` and `RopeConstraint`: Elastic and flexible connections.
* `AlignPosition` and `AlignOrientation`: Modern physics movers that push parts toward target positions/angles with configurable force and responsiveness.
* `LinearVelocity` and `AngularVelocity`: Applies constant linear or spinning speeds to objects.

### C. Collision Management
* `PhysicsService:RegisterCollisionGroup`: Creates isolated collision channels.
* `PhysicsService:CollisionGroupSetCollidable`: Specifies whether groups collide (e.g., Players walk through each other, but collide with bullets and walls).

---

## 6. Visual Effects, Lighting, and Audio

To create polished, high-fidelity games, visual and auditory atmosphere is essential:

### A. Lighting Technology
* **Technology**: `Future` (supports full localized specular highlights, shadow-casting point/spot lights), `ShadowMap`, or `Voxel`.
* **Atmosphere**: Realistic air haze, Rayleigh scattering, color tinting, density, and sunset glare.
* **Post-Processing**:
  - `BloomEffect`: Glow around intense light sources.
  - `DepthOfFieldEffect`: Cinematic background or foreground blur.
  - `ColorCorrectionEffect`: Contrast, saturation, and color tinting.
  - `SunRaysEffect`: Volumetric light shafts.
* **Sky**: Custom 6-sided cubemaps and celestial body configuration (Sun, Moon, StarCount).

### B. Particle Systems and Surface Effects
* `ParticleEmitter`: Multi-keyframe particle systems (fire, smoke, sparks, magic spells) with customizable rate, lifetime, speed, spread angle, acceleration, and texture flipbooks.
* `Beam`: Connects two attachments with a textured, scrolling visual strip (lasers, lightning, energy bridges).
* `Trail`: Leaves ribbons of light or motion blur behind swinging swords or moving vehicles.
* `Highlight`: Outlines characters or interactive items through walls or with solid fill colors.
* `SurfaceAppearance`: PBR material maps (`ColorMap`, `NormalMap`, `MetalnessMap`, `RoughnessMap`).

### C. Sound and Audio Pipeline
* `SoundService`: Central audio coordinator.
* `SoundGroup`: Hierarchical volume control (Master, Music, SFX, Ambient, Voice).
* `Sound`: 3D localized audio (positioned inside parts with min/max distance falloff) or 2D global soundtrack audio.
* `SoundEffect`: Reverb, Equalizer, PitchShift, Compressor, and Flange effects.

---

## 7. Scripting, Architecture Patterns, and Network Model

Full games require clean architectural separation:

```
+-------------------------------------------------------------+
|                      SERVER CONTEXT                         |
|   ServerScriptService (Databases, Hit Validation, Spawners)  |
+-------------------------------------------------------------+
                              |
                     RemoteEvents / RemoteFunctions
                              |
+-------------------------------------------------------------+
|                      CLIENT CONTEXT                         |
|   StarterPlayerScripts (Input, Camera, Prediction, Sound)   |
|   StarterGui (HUD Updates, Visual Feedback, Client Tweening)|
+-------------------------------------------------------------+
```

### A. Script Types and Execution Roles
* `Script`: Executes on the server. Validates game state, saves player data, manages NPCs, controls rounds.
* `LocalScript`: Executes on individual player devices. Reads user input, updates camera, plays local animations, renders HUD.
* `ModuleScript`: Reusable libraries placed in `ReplicatedStorage` (shared by both) or `ServerStorage` (server-only).

### B. Networking Primitives
* `RemoteEvent`: One-way, asynchronous message delivery:
  - `FireServer(args)`: Client requests action.
  - `FireClient(player, args)`: Server notifies specific client.
  - `FireAllClients(args)`: Server notifies everyone (e.g., round starting).
* `UnreliableRemoteEvent`: High-frequency, low-latency one-way events where packet loss is acceptable (character combat effects, projectile visual sync).
* `RemoteFunction`: Two-way synchronous request-and-reply (`InvokeServer` / `InvokeClient`). Must be used with care to prevent hanging.

### C. Component Architecture (CollectionService)
* Rather than scattering scripts across hundreds of workspace objects, modern games use `CollectionService` tags (e.g., `"KillBrick"`, `"Chest"`, `"Door"`).
* A single central server script listens to `GetInstanceAddedSignal` and attaches gameplay logic dynamically to tagged instances.

---

## 8. Generalized Two-Layer MCP Tool Suite (22 Tools)

To avoid overfitting to specific game mechanics while giving LLMs both high-level productivity and raw low-level escape hatches, the tool suite is organized into two core layers plus a host automation layer:

### Layer 1: High-Level Declarative Tools (Rapid, Ergonomic, Composable)
1. `build_ui_tree(root_parent, declarative_tree)`: Builds complete UI hierarchies with auto-layouts, `AutomaticCanvasSize`, `UIAspectRatioConstraint`, and responsive `UDim2` scaling.
2. `create_physics_joint(joint_type, part_a, part_b, properties)`: Universal physics and mechanical constraint builder (`Prismatic`, `Hinge`, `Weld`, `Spring`, `BallSocket`, `AlignPosition`, `AlignOrientation`). Automatically sets up Attachment pairs and motors.
3. `configure_environment(lighting_properties, atmosphere, post_effects)`: Configures world lighting technology, atmosphere, sky, shadows, and post-processing in one call.
4. `modify_terrain(operation, material, cframe, size)`: Procedural voxel terrain generation (blocks, spheres, cylinders, clear regions).
5. `insert_asset(asset_id, parent_path)`: Loads models, meshes, or packages from the Creator Store via `InsertService`.
6. `manage_script(path, content, script_type, run_context)`: Safely creates or updates scripts using `ScriptEditorService` and records history with `ChangeHistoryService`.
7. `configure_dev_environment(options)`: Sets up local dev helpers like in-memory MockDataStores for offline testing, collision groups, and security settings.

### Layer 2: Low-Level Primitives (The Escape Hatch & Raw Control)
8. `execute_luau(code)`: The ultimate escape hatch! Runs arbitrary Luau in Studio edit mode and returns the results. Allows LLMs to invent custom workarounds when high-level tools are insufficient.
9. `create_instance(class_name, parent_path, properties)`: Spawns literally any engine class (`Part`, `ClickDetector`, `ProximityPrompt`, `ParticleEmitter`, `Sound`, `Highlight`, etc.).
10. `set_properties(instance_path, properties)`: Updates any combination of properties with automatic JSON-to-Roblox type conversion.
11. `get_instance_tree(instance_path, depth, include_properties)`: Inspects descendants of any path (e.g., `game.Workspace.Map`).
12. `delete_instance(instance_path)`: Removes an instance cleanly from the DataModel.
13. `manage_tags(instance_path, action, tag)`: Directly adds, removes, or queries `CollectionService` tags (for component architectures like kill bricks, checkpoints, interactables).
14. `manage_attributes(instance_path, action, key, value)`: Directly reads or sets modern instance attributes via `GetAttribute` / `SetAttribute`.
15. `call_method(instance_path, method_name, args)`: Directly invokes any engine method on an instance (e.g., `PivotTo`, `ApplyImpulse`, `Clone`, `Play`).

### Layer 3: Host Automation and Verification (Autonomous Quality Control)
16. `start_playtest(mode)`: Initiates Play Solo (`F5`) or Run mode via Go host automation, linking to the active play DataModel.
17. `stop_playtest()`: Terminates playtest mode and returns to edit mode (via `Shift+F5`).
18. `get_console_logs(filter, clear)`: Retrieves filtered output, warning, and error logs with stack traces across edit and play sessions.
19. `capture_viewport()`: Captures an image of the Roblox Studio window via Go OS-level screen capture, returning base64 image data for multimodal visual verification.
20. `simulate_player_input(action_sequence)`: Dispatches keyboard and mouse inputs into the focused Studio viewport to test player mechanics.
21. `save_place(file_path)`: Triggers place file save to preserve progress across test iterations.
22. `check_studio_status()`: Verifies whether Roblox Studio is open, connected, and whether it is in Edit mode or Play mode.

---

## 9. Dependencies and Implementation Blueprint (Go)

### A. Go Environment
* **Language & Toolchain**: Go 1.23+
* **Core Libraries**:
  * `github.com/mark3labs/mcp-go`: Model Context Protocol SDK for Go over stdio.
  * Standard library `net/http`: Lightweight, high-concurrency local HTTP bridge server on `127.0.0.1:58741`.
  * `golang.org/x/sys/windows`: Win32 API system calls (`FindWindowW`, `SetForegroundWindow`, `SendInput`).
  * `github.com/kbinani/screenshot`: Fast OS-level window screen capture for `capture_viewport`.

### B. Roblox Studio Plugin (`plugin/`)
* Written in clean, modular Luau.
* Runs as a local plugin in Studio's `Plugins` directory.
* Uses `HttpService:RequestAsync` to long-poll the Go server on `127.0.0.1:58741`.
* Uses `ScriptEditorService` and `ChangeHistoryService` for safe, undoable script operations.
* Includes a robust Luau property deserializer converting JSON objects into native types (`Vector3`, `CFrame`, `UDim2`, `Color3`, `Enum`, etc.).
* Implements protected calls (`pcall`) around all operations to return clear error traces to the AI instead of crashing the plugin.

---

## 10. Summary

With this detailed blueprint:
* The complex type system is mapped and solved through typed JSON serialization.
* The intricate UI system is treated as a first-class declarative system.
* The full spectrum of engine services (3D, Physics, Lighting, Audio, Scripting) is addressable through structured MCP tools.
* The autonomous verification loop (Playtest + Console Logs + Viewport Capture) ensures the AI can iterate, fix bugs, and complete full games.
