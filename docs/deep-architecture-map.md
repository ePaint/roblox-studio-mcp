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

## 3. Data Types and Wire Format

Roblox datatypes do not map to JSON. Plain JSON numbers, strings and booleans pass through unchanged. Every Roblox datatype is a tagged object with a `$t` field, in both directions: reads return the same tagged form that writes accept.

| Roblox Type | Wire format |
|---|---|
| `Vector3` | `{"$t":"Vector3","v":[x,y,z]}` |
| `Vector2` | `{"$t":"Vector2","v":[x,y]}` |
| `CFrame` | `{"$t":"CFrame","v":[12 numbers]}` or `{"$t":"CFrame","pos":[x,y,z],"rot_deg":[rx,ry,rz]}` |
| `Color3` | `{"$t":"Color3","rgb":[0-255,0-255,0-255]}` (0-255 only) |
| `UDim` | `{"$t":"UDim","v":[scale,offset]}` |
| `UDim2` | `{"$t":"UDim2","v":[xs,xo,ys,yo]}` |
| `Enum` | `{"$t":"Enum","v":"Material.Neon"}` |
| `Instance` | `{"$t":"Instance","ref":"<id or path>"}` |
| `ColorSequence` | `{"$t":"ColorSequence","v":[{"t":0,"rgb":[255,0,0]},{"t":1,"rgb":[0,0,255]}]}` |
| `NumberSequence` | `{"$t":"NumberSequence","v":[{"t":0,"v":1,"env":0},{"t":1,"v":0,"env":0}]}` |
| `NumberRange` | `{"$t":"NumberRange","v":[min,max]}` |
| `Rect` | `{"$t":"Rect","v":[x0,y0,x1,y1]}` |
| `Font` | `{"$t":"Font","family":"rbxasset://fonts/families/GothamSSm.json","weight":"Bold","style":"Normal"}` |
| `BrickColor` | `{"$t":"BrickColor","v":"Bright red"}` |

Rules:
* **CFrame rotation**: `rot_deg` is in degrees, applied in YXZ order, the same convention as the `Orientation` property. The plugin converts to radians because `CFrame.Angles` takes radians.
* **typeof shorthand**: when the target property already holds a value, an untagged array is coerced using `typeof(instance[prop])`, so `Size: [4,1,2]` works on a `Part`. Tagged values are always accepted.
* **Per-property results**: `set_properties` returns `{applied:[...], failed:{prop: message}}`. A write is never silently partial; failures carry the `COERCION` code detail per property.

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
* `UIStroke`: Adds borders to frames or outlines to text with `ApplyStrokeMode` set to `Border` or `Contextual`.
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
* **Moving platforms**: an anchored part moved by CFrame tween does not carry characters, because the engine sees zero velocity. Use an unanchored platform on a `PrismaticConstraint` with an anchored rail, `ActuatorType` `Servo` and `SetNetworkOwner(nil)`, or keep it anchored and set `AssemblyLinearVelocity` while it moves.

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

## 8. MCP Tool Suite: Three Layers, 18 Tools

Declarative builders, raw primitives and a playtest and host layer. Full schemas, argument shapes and return values live in [tool-suite-spec.md](tool-suite-spec.md); this is the index.

Every tool that creates an object returns `{id, path}`. A `target` argument accepts an `id` (`Instance:GetDebugId()`), a dotted path (`game.Workspace.Map`) or a segment array (`["Workspace","Map","Stage#3"]`, where `#n` picks the n-th same-named sibling, 1-based). A dead reference returns `STALE_REF`.

### Layer 1: Declarative builders (7)
1. `build_tree`: builds an instance tree from nodes `{class, name?, props?, attributes?, tags?, children?}`, with UI sugar keys (`layout`, `padding`, `corner`, `stroke`, `aspect_ratio`, `auto_canvas`). A single node is a valid tree.
2. `create_physics_joint`: Prismatic, Hinge, Weld, Spring, BallSocket, AlignPosition or AlignOrientation, with Attachment pairs created.
3. `configure_environment`: lighting, atmosphere, sky and post effects in one call.
4. `modify_terrain`: `fill_block`, `fill_ball`, `fill_cylinder`, `clear`, `replace`.
5. `insert_asset`: loads a model by asset id with `InsertService:LoadAsset`; sounds and meshes are set through `SoundId` / `MeshId`.
6. `manage_script`: `read`, `write` or `delete` scripts through `ScriptEditorService` inside a ChangeHistory recording.
7. `configure_dev_environment`: mock DataStore wrapper, collision groups, player attributes.

### Layer 2: Primitives (6)
8. `execute_luau`: runs Luau in the `edit` or `server` context with captured output.
9. `set_properties`: properties, attributes and tags (`add` / `remove`) in one call.
10. `get_instance_tree`: bounded subtree read with `max_nodes` and a continuation cursor.
11. `find_instances`: search by class, name pattern, tag or attribute.
12. `delete_instance`: removes an instance.
13. `call_method`: invokes an engine method such as `PivotTo`, `ApplyImpulse` or `Clone`.

### Layer 3: Playtest and host (5)
14. `playtest`: `start`, `stop` or `run` a Play or Run session through `StudioTestService`; `run` returns the `EndTest` payload.
15. `get_console_logs`: cursor-based log read across edit, server and client sources.
16. `capture_viewport`: PNG of the Studio window as MCP image content (Windows only in v1).
17. `simulate_player_input`: key, click, move and wait steps (Windows only, experimental).
18. `studio`: `status`, `selection` or `save`.

Errors map to MCP `isError` results with `{code, message, job_id}`. Codes: `NOT_CONNECTED`, `NOT_PAIRED`, `STALE_REF`, `TIMEOUT`, `WRONG_MODE`, `TOO_LARGE`, `COERCION`, `UNSUPPORTED_HOST`, `LUAU_ERROR`, `UNKNOWN`, `NEEDS_MANUAL_SAVE`.

---

## 9. Dependencies and Implementation Blueprint

### A. Go Stack
* **Toolchain**: Go 1.25+, `CGO_ENABLED=0`, single static binary.
* **Dependencies**: `github.com/modelcontextprotocol/go-sdk` (official SDK, stdio transport, image content), `golang.org/x/sys/windows`, `golang.org/x/image/draw`. Everything else is standard library, including `net/http` for the bridge.
* **Not used**: `mark3labs/mcp-go`, `kbinani/screenshot`, `robloxapi/rbxfile`.
* **Output discipline**: stdout carries MCP frames only; logs go to stderr or a file with `--log-file`.

### B. Host Layer (`internal/host`)
* `Host` interface: `FindStudio`, `Capture`, `SendKeys`, `SendMouse`, `SaveShortcut`.
* `host_windows.go` calls user32, gdi32 and dwmapi through `windows.NewLazySystemDLL`, because `x/sys/windows` does not wrap `SendInput`, `BitBlt`, `PrintWindow`, `GetClientRect` or `ClientToScreen`.
* `host_other.go` (`//go:build !windows`) returns `UNSUPPORTED_HOST`, so `go build` and `go test` work on a Mac.
* **Window discovery**: `EnumWindows`, `GetWindowThreadProcessId` and `QueryFullProcessImageName` match `RobloxStudioBeta.exe` with a title ending in ` - Roblox Studio`; several instances are handled.
* **Capture**: `PrintWindow(PW_RENDERFULLCONTENT)` first, screen-DC `BitBlt` of the `ClientToScreen` rect as fallback, `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)` at startup, downscale with `x/image/draw`.
* **Input**: `SendInput` with scancodes after a foreground check; focus failure is reported, never dropped silently.
* A later macOS backend uses `ebitengine/purego` (cgo-free) in `host_darwin.go`.

### C. Plugin Packaging
* Source lives in `plugin/src/*.luau`: `init`, `bridge`, `deserializer`, `script_manager`, `mock_datastore`, `physics_builder`, `playtest`.
* `cmd/buildplugin` (Go, `text/template`) emits `plugin/dist/RobloxStudioMcp.rbxmx`: one Script plus ModuleScripts with CDATA `Source`. `plugin/dist` is gitignored.
* The main binary embeds the file with `go:embed`. `roblox-studio-mcp install-plugin` copies it to `%LOCALAPPDATA%\Roblox\Plugins` (Windows) or `~/Documents/Roblox/Plugins` (macOS).

### D. Plugin Runtime
* Polls the bridge over HTTP long-poll (`GET /poll`, `POST /result`, `POST /logs`) and authenticates with a long-lived token obtained through a pairing code (`POST /pair`).
* Plugin code loads in the edit, server and client DataModels. The edit instance runs the main poll loop; the server instance polls only for `stop` and `server`-context jobs and forwards play logs; client instances stay inactive.
* Mutating jobs run inside `ChangeHistoryService:TryBeginRecording` / `FinishRecording`, so every change is undoable.
* The deserializer converts tagged JSON into native types and wraps every operation in `pcall` to return clear errors.

---

## 10. Summary

* The type system is solved with one tagged wire format, per-property results and `STALE_REF`-checked instance references.
* The UI system is a first-class declarative input to `build_tree`.
* The engine surface (3D, physics, lighting, audio, scripting) is reachable through 18 tools in three layers.
* The verification loop is `playtest` (scripted runs return state assertions), `get_console_logs` and `capture_viewport`, with no window focus needed to start or stop a session.
