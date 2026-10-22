# Red Team Adversarial Audit: Roblox Studio MCP Architecture

**Target**: Roblox Studio MCP Architecture and Tool Suite Specification  
**Auditor**: Red Team Engine Architect  
**Scope**: Feasibility stress-test against Game Scenario A (Incremental/Clicker) and Game Scenario B (Obby)

---

## 1. Executive Verdict

**Verdict**: Conditionally Viable with Significant Architectural Deficits.

The original tool design successfully supported static world-building, but encountered hard engine barriers when entering dynamic gameplay assembly, state management, and the autonomous verification loop.

Key findings:
- **Autonomous Playtesting and Viewport Capture cannot run inside Luau alone**: Luau plugins have no access to framebuffer pixels or OS input. `capture_viewport` and `simulate_player_input` must be executed by the Python host process on Windows.
- **The DataModel splits during Play Solo**: When Studio enters Play Solo, the Edit DataModel separates into Server and Client DataModels. A standard edit-mode plugin loses context unless playtest hooks are used.
- **Script editing desynchronization**: Writing directly to `Script.Source` desynchronizes open editor tabs unless routed through `ScriptEditorService`, and breaks Undo (`Ctrl+Z`) unless recorded with `ChangeHistoryService`.
- **The DataStore Trap**: Local unpublished Studio files fail with HTTP 403 Forbidden when calling `DataStoreService`, crashing incremental games on startup unless an in-memory mock is provided.
- **Physics Trap for Moving Platforms**: Moving an anchored part with `TweenService` causes characters to slip off and fall. Moving platforms require constraints (`PrismaticConstraint`) or client-side interpolation.
- **Missing Essential Tools**: Tags (`CollectionService`), attributes (`SetAttribute`), interaction classes (`ClickDetector`, `ProximityPrompt`), and spawn management.

---

## 2. Adversarial Walkthrough: Game Scenario A (Incremental / Clicker)

### Step 1: 3D Clickable Object and Floating Feedback
- **Required**: Golden coin/button in `Workspace` with floating text and click particles.
- **Engine Barrier**:
  - Clicking a 3D part requires a `ClickDetector` or `ProximityPrompt`. The original tool list did not provide interaction detectors.
  - Floating numbers require a `BillboardGui` with `TextLabel` parented to the part, animated locally via `StarterPlayerScripts` to prevent server replication lag.

### Step 2: Data Persistence and Leaderstats
- **Required**: Persistent currency, multipliers, and leaderstats.
- **Engine Barrier**:
  - `DataStoreService:GetDataStore` throws an error in local or unpublished places because "Enable Studio Access to API Services" is off by default.
  - **Fix**: The toolset must provide an autonomous in-memory mock DataStore (`setup_mock_datastores`) so games run offline without cloud credentials.

### Step 3: Upgrade Shop UI (UDim2, ScrollingFrame)
- **Required**: Responsive shop menu with scrollable cards.
- **Engine Barrier**:
  - `ScrollingFrame` clips or hides items unless `AutomaticCanvasSize = Enum.AutomaticSize.Y` is used with `UIListLayout`.
  - Responsive buttons require `Scale` coordinates and `UIAspectRatioConstraint` rather than fixed pixel offsets.

### Step 4: Server Anti-Cheat and Rebirths
- **Required**: Server-authoritative validation for clicks and upgrades.
- **Fix**: Client sends empty `ClickRemote:FireServer()`, server calculates multipliers based on server-stored attributes and enforces rate limits.

---

## 3. Adversarial Walkthrough: Game Scenario B (Jump Course / Obby)

### Step 1: 15-20 Progressive Stages
- **Required**: Generating 20 stages with checkpoints and obstacles.
- **Engine Barrier**:
  - Attaching scripts to each part individually causes severe lag.
  - **Fix**: Use `CollectionService` tags (`"KillBrick"`, `"Checkpoint"`) handled by a single manager in `ServerScriptService`. Requires a `manage_tags` tool.

### Step 2: Moving Platforms (Physics Replication Trap)
- **Required**: Platforms moving back and forth between waypoints.
- **Engine Barrier**:
  - Moving anchored parts with `TweenService` does not move the player character. The character stands in place while the floor slides away, dropping the player into the void.
  - **Fix**: Create unanchored assemblies with `PrismaticConstraint` and `Attachment` pairs, or run client-side delta-CFrame updates. Added dedicated helper: `create_moving_platform`.

### Step 3: Checkpoints and Respawn
- **Required**: Respawning at the latest completed stage.
- **Engine Barrier**:
  - Multiple `SpawnLocation` instances cause random team spawns.
  - Spawning must listen to `Player.CharacterAdded`, wait for `HumanoidRootPart`, and call `character:PivotTo(stageCFrame)`.

---

## 4. Root-Cause Engine Barriers and Architectural Fixes

| Barrier | Root Cause in Engine | Solution |
|---|---|---|
| Script Editing Desync | Direct `script.Source` edits do not sync if tab is open | Use `ScriptEditorService:UpdateSourceAsync` and `ChangeHistoryService` |
| Viewport Capture | Luau sandbox cannot access GPU framebuffer | Python host captures window screenshot via Windows OS APIs |
| Input Simulation | Luau cannot inject native OS inputs | Python host focuses Studio window and injects inputs via OS APIs |
| Playtest Automation | Luau plugin has no API to trigger Play Solo | Python host sends shortcut (`F5` / `Shift+F5`) to Studio |
| DataStore 403 Errors | Studio disables API services by default | Inject in-memory MockDataStore module for offline testing |
| Moving Platforms | Anchored parts moved by Tween do not carry players | Use `PrismaticConstraint` physics assemblies |
| Tag Management | Repetitive script duplication across parts | Add `manage_tags` tool using `CollectionService` |
| Instance Attributes | Modern Roblox state stored in attributes | Add `manage_attributes` tool |

---

## 5. Expanded 20-Tool Suite

With these fixes, the tool suite expands from 15 to 20 tools across clear execution boundaries:

### Level 1: Core Primitives (Engine Manipulation)
1. `create_instance(class_name, parent_path, properties)`
2. `set_properties(instance_path, properties)`
3. `get_instance_tree(instance_path, depth)`
4. `delete_instance(instance_path)`
5. `execute_luau(code)`
6. `manage_tags(instance_path, action, tag)` [NEW]
7. `manage_attributes(instance_path, action, key, value)` [NEW]

### Level 2: Composite Generators (Game Systems)
8. `build_ui_tree(declarative_json)` (with responsive constraints and AutomaticCanvasSize)
9. `manage_script(script_path, content, script_type, run_context)` (via ScriptEditorService)
10. `configure_lighting(preset)`
11. `generate_terrain(region, material)`
12. `create_moving_platform(start_pos, end_pos, speed, style)` [NEW]
13. `setup_mock_datastores(enable)` [NEW]

### Level 3: Verification and Host Automation (Autonomous Testing)
14. `start_playtest(mode)` [REVISED: Python host triggers F5]
15. `stop_playtest()` [REVISED: Python host triggers Shift+F5]
16. `get_console_logs(filter, clear)` (captures logs across Edit and Play sessions)
17. `capture_viewport()` [REVISED: Python host window screenshot]
18. `simulate_player_input(action_sequence)` [REVISED: Python host input injection]
19. `save_place(file_path)` [NEW]
20. `check_studio_status()` [NEW: Verifies whether Studio is connected and in Edit vs Play mode]
