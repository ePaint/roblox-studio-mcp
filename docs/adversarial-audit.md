# Adversarial Walkthrough: Roblox Studio MCP Architecture

**Target**: Roblox Studio MCP architecture and tool suite (`docs/tool-suite-spec.md`)
**Scope**: Feasibility stress-test against Game Scenario A (Incremental/Clicker) and Game Scenario B (Obby)

---

## 1. Executive Verdict

**Verdict**: Viable, provided the barriers below are handled by the design.

A static world-building tool set is not enough for dynamic gameplay assembly, state management and an autonomous verification loop. The corrected design handles each barrier:

- **Playtest control is a plugin concern.** `StudioTestService:ExecutePlayModeAsync` and `ExecuteRunModeAsync` start a session from the plugin and return the value passed to `EndTest`. No window focus or F5 is involved.
- **Viewport capture and OS input are host concerns.** Luau has no access to framebuffer pixels or OS input, so `capture_viewport` and `simulate_player_input` run in the Go binary's host layer on Windows. State assertions go through in-engine code in `playtest run` instead of OS input.
- **Plugin code loads in three DataModels.** The edit, server and client DataModels each run the plugin. Roles keep exactly one active poller per job class.
- **Script editing desync**: writing `Script.Source` directly desynchronizes open editor tabs and breaks Undo unless the write goes through `ScriptEditorService` inside a `ChangeHistoryService` recording.
- **The DataStore trap**: DataStore calls in Studio error (the message mentions 403) unless the place is published and "Enable Studio Access to API Services" is on. Games need a mock behind a wrapper module.
- **Physics trap for moving platforms**: an anchored part moved by a CFrame tween reports zero velocity, so it does not carry characters.
- **Missing essentials**: tags, attributes, interaction classes (`ClickDetector`, `ProximityPrompt`) and spawn management must be reachable from the tool set.

---

## 2. Adversarial Walkthrough: Game Scenario A (Incremental / Clicker)

### Step 1: 3D Clickable Object and Floating Feedback
- **Required**: Golden coin/button in `Workspace` with floating text and click particles.
- **Engine Barrier**:
  - Clicking a 3D part requires a `ClickDetector` or `ProximityPrompt`, created as nodes in `build_tree`.
  - Floating numbers require a `BillboardGui` with a `TextLabel` parented to the part, animated locally from `StarterPlayerScripts` to avoid replication lag.

### Step 2: Data Persistence and Leaderstats
- **Required**: Persistent currency, multipliers, and leaderstats.
- **Engine Barrier**:
  - `DataStoreService` calls error in Studio unless the place is published and "Enable Studio Access to API Services" is on.
  - **Fix**: `configure_dev_environment({mock_datastore: true})` installs `ServerStorage/McpDev/MockDataStoreService` and a wrapper `ServerStorage/McpDev/DataStore`. The wrapper returns the real service outside Studio (or when the attribute `UseRealDataStore` is true) and the mock otherwise. Nothing can shadow `DataStoreService`, so generated game code must `require` the wrapper. The tool returns `{require_path}`.

### Step 3: Upgrade Shop UI (UDim2, ScrollingFrame)
- **Required**: Responsive shop menu with scrollable cards.
- **Engine Barrier**:
  - `ScrollingFrame` clips items unless `AutomaticCanvasSize` is set to `Y` with a `UIListLayout`. `build_tree` exposes this as the `auto_canvas` and `layout` node keys.
  - Responsive buttons need `Scale` coordinates and a `UIAspectRatioConstraint` (`aspect_ratio`) instead of fixed pixel offsets.

### Step 4: Server Anti-Cheat and Rebirths
- **Required**: Server-authoritative validation for clicks and upgrades.
- **Fix**: The client sends an empty `ClickRemote:FireServer()`. The server computes multipliers from server-stored attributes and enforces rate limits. `playtest run` can assert the resulting server state through `EndTest`.

---

## 3. Adversarial Walkthrough: Game Scenario B (Jump Course / Obby)

### Step 1: 15-20 Progressive Stages
- **Required**: Generating 20 stages with checkpoints and obstacles.
- **Engine Barrier**:
  - Attaching scripts to each part causes severe lag.
  - **Fix**: Use `CollectionService` tags (`"KillBrick"`, `"Checkpoint"`) handled by one manager script in `ServerScriptService`. Tags go in the `tags` key of a `build_tree` node or in `set_properties`.
  - Twenty sibling `Stage` models make dotted paths ambiguous, so `build_tree` returns `{id, path}` for every node and targets accept ids or segment arrays such as `["Workspace","Map","Stage#3"]`.

### Step 2: Moving Platforms (Physics Replication Trap)
- **Required**: Platforms moving back and forth between waypoints.
- **Engine Barrier**:
  - An anchored part moved by a CFrame tween has zero velocity as far as the engine is concerned, so the character stands in place while the floor slides away.
  - **Fix A**: an unanchored platform on a `PrismaticConstraint` with an anchored rail, `ActuatorType` `Servo`, and `SetNetworkOwner(nil)`. `create_physics_joint` takes `joint_type: "Prismatic"` with `axis`, `lower`, `upper`, `speed`, `actuator`, `target`, `anchored_side` and `network_owner_server`, and returns the joint and both attachments.
  - **Fix B**: an anchored platform that sets `AssemblyLinearVelocity` while it moves.

### Step 3: Checkpoints and Respawn
- **Required**: Respawning at the latest completed stage.
- **Engine Barrier**:
  - Multiple `SpawnLocation` instances cause random team spawns.
  - Spawning must listen to `Player.CharacterAdded`, wait for `HumanoidRootPart`, and call `character:PivotTo(stageCFrame)`. `playtest run` can verify the respawn position in-engine.

---

## 4. Root-Cause Engine Barriers and Architectural Fixes

| Barrier | Root Cause in Engine | Solution |
|---|---|---|
| Script Editing Desync | Direct `script.Source` edits do not sync if a tab is open and bypass Undo | `manage_script` writes with `ScriptEditorService:UpdateSourceAsync` inside a `ChangeHistoryService` recording |
| Viewport Capture | Luau sandbox cannot read the GPU framebuffer | Host layer in the Go binary captures the Studio window (`PrintWindow`, `BitBlt` fallback), Windows only in v1 |
| Input Simulation | Luau cannot inject OS input | Host layer `SendInput`, experimental; prefer `Humanoid:MoveTo` and `PivotTo` through `playtest run` |
| Playtest Automation | Needs a way to start and stop sessions | Plugin calls `StudioTestService:ExecutePlayModeAsync` / `ExecuteRunModeAsync`; `EndTest` returns the result |
| Play-Mode DataModels | Plugin loads in edit, server and client DataModels | Role model: edit instance runs jobs, server instance handles `stop`, `server`-context jobs and log forwarding, clients stay inactive |
| DataStore Errors | Studio API access needs a published place and a setting | `configure_dev_environment` mock plus a wrapper module that game code requires |
| Moving Platforms | Tweened anchored parts have zero velocity and do not carry players | Prismatic joint with anchored rail, Servo and `SetNetworkOwner(nil)`, or `AssemblyLinearVelocity` on an anchored part |
| Tag and Attribute Management | Repetitive script duplication across parts; modern state lives in attributes | `tags` and `attributes` keys in `build_tree` nodes and `set_properties` |

---

## 5. Tool Suite

`docs/tool-suite-spec.md` is the canonical specification. The suite has 18 tools in three layers:

- **Layer 1, declarative builders (7)**: `build_tree`, `create_physics_joint`, `configure_environment`, `modify_terrain`, `insert_asset`, `manage_script`, `configure_dev_environment`.
- **Layer 2, primitives (6)**: `execute_luau`, `set_properties`, `get_instance_tree`, `find_instances`, `delete_instance`, `call_method`.
- **Layer 3, playtest and host (5)**: `playtest`, `get_console_logs`, `capture_viewport`, `simulate_player_input`, `studio`.
