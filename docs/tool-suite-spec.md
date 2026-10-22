# Tool Suite Specification: General Two-Layer Architecture

## 1. Design Philosophy

To enable an AI to build any genre of game (simulators, obbies, RPGs, shooters, tycoons, puzzle games) without overfitting to specific mechanics, the tool suite is organized into two distinct layers, backed by a host-level verification layer:

```
+------------------------------------------------------------------+
|      LAYER 1: HIGH-LEVEL DECLARATIVE TOOLS                       |
|      Expressive, composite tools that generate complex systems    |
|      in a single structured call (UI trees, physics joints, etc.) |
+------------------------------------------------------------------+
                                  |
                                  v
+------------------------------------------------------------------+
|      LAYER 2: LOW-LEVEL PRIMITIVES (THE ESCAPE HATCH)            |
|      Raw, unfiltered access to every class, property, and method  |
|      in the Roblox engine. Allows LLMs to invent custom           |
|      workarounds when high-level tools are not enough.            |
+------------------------------------------------------------------+
                                  |
                                  v
+------------------------------------------------------------------+
|      HOST AUTOMATION & VERIFICATION LAYER                         |
|      External OS-level testing: Play Solo, screen capture, input  |
|      simulation, and console error log streaming.                 |
+------------------------------------------------------------------+
```

---

## 2. Layer 1: High-Level Declarative Tools

These tools handle complex composite operations cleanly and efficiently:

### 1. `build_ui_tree(root_parent, declarative_tree)`
* **Purpose**: Builds full, responsive GUI hierarchies (HUDs, shops, inventory grids, settings menus) in one call.
* **Capabilities**: Declaratively constructs `ScreenGui`, `Frame`, `TextLabel`, `ImageButton`, `ScrollingFrame`, etc., automatically applying `UDim2` scaling, `UIAspectRatioConstraint`, `UIListLayout`/`UIGridLayout`, `AutomaticCanvasSize`, `UICorner`, and `UIStroke`.

### 2. `create_physics_joint(joint_type, part_a, part_b, properties)`
* **Purpose**: General mechanical and physics constraint builder. Replaces ad-hoc moving platform tools with a universal physics solver.
* **Supported Joint Types**: `Prismatic` (sliders, moving platforms, elevators), `Hinge` (wheels, swinging doors, spinning hazards), `WeldConstraint` (rigid attachments), `Spring` (suspension, bouncers), `BallSocket` (ragdolls, ropes), `AlignPosition`, `AlignOrientation`.
* **Automatic Handling**: Spawns required `Attachment` pairs, sets orientation axes, aligns CFrame references, and configures actuator motors/velocities.

### 3. `configure_environment(lighting_properties, atmosphere, post_effects)`
* **Purpose**: Sets up global world visuals and skyboxes.
* **Capabilities**: Sets `Lighting` technology (e.g., `Future`), ambient light, time of day, shadows, plus child effects (`Atmosphere`, `Sky`, `BloomEffect`, `ColorCorrectionEffect`, `SunRaysEffect`, `DepthOfFieldEffect`).

### 4. `modify_terrain(operation, material, cframe, size)`
* **Purpose**: Procedural voxel landscape generation.
* **Capabilities**: Performs `FillBlock`, `FillBall`, `FillCylinder`, `Clear`, and material replacement across specified 3D bounds.

### 5. `insert_asset(asset_id, parent_path)`
* **Purpose**: Inserts verified 3D models, sound assets, or mesh packages from the Creator Store via `InsertService`.

### 6. `manage_script(path, content, script_type, run_context)`
* **Purpose**: High-level atomic script editor.
* **Capabilities**: Creates or updates Server Scripts, LocalScripts, or ModuleScripts. Automatically checks `ScriptEditorService` to prevent open-tab text desync, and registers edits with `ChangeHistoryService` for clean undo/redo.

### 7. `configure_dev_environment(options)`
* **Purpose**: Developer environment bootstrapper.
* **Capabilities**: Injects in-memory MockDataStores for offline testing without cloud permissions, sets up collision group channels, and configures default player attributes.

---

## 3. Layer 2: Low-Level Primitives (Unrestricted Engine Access)

When a game requires something unique, non-standard, or niche, the LLM falls back to these raw primitives to build custom solutions:

### 8. `execute_luau(code)`
* **The Ultimate Escape Hatch**: Executes arbitrary Luau code in edit mode and returns the results.
* **Usage**: Mass procedural generation, inspecting complex internal state, running custom math, or calling APIs not exposed by MCP tools.

### 9. `create_instance(class_name, parent_path, properties)`
* **Purpose**: Spawns any instance class in the engine (`Part`, `ClickDetector`, `ProximityPrompt`, `ParticleEmitter`, `Beam`, `Trail`, `Sound`, `Highlight`, etc.).

### 10. `set_properties(instance_path, properties)`
* **Purpose**: Modifies any property on any instance with automatic JSON-to-Roblox type marshaling (`Vector3`, `CFrame`, `UDim2`, `Color3`, `ColorSequence`, `NumberSequence`, `Enum`).

### 11. `get_instance_tree(instance_path, depth, include_properties)`
* **Purpose**: Traverses the DataModel scene graph. Returns class names, children, and property dictionaries to inspect the game world.

### 12. `delete_instance(instance_path)`
* **Purpose**: Cleans up or removes any object from the DataModel.

### 13. `manage_tags(instance_path, action, tag)`
* **Purpose**: Adds, removes, or queries tags using `CollectionService`. Powers component-based game architecture (`KillBrick`, `Interactable`, `Checkpoint`).

### 14. `manage_attributes(instance_path, action, key, value)`
* **Purpose**: Reads, writes, or deletes custom instance attributes via `GetAttribute` / `SetAttribute`.

### 15. `call_method(instance_path, method_name, args)`
* **Purpose**: Directly invokes any Luau method on an instance (e.g., `PivotTo`, `ApplyImpulse`, `Clone`, `Play`, `Stop`).

---

## 4. Host Automation and Autonomous Verification Layer

These tools run directly on the Windows host process in Go, bridging the sandbox gap:

### 16. `start_playtest(mode)`
* **Purpose**: Initiates Play Solo (`F5`) or Run mode via Windows host automation, connecting to the active play session.

### 17. `stop_playtest()`
* **Purpose**: Terminates the playtest (`Shift+F5`) and returns Studio to Edit mode.

### 18. `get_console_logs(filter, clear)`
* **Purpose**: Captures print statements, warnings, and error stack traces across both Edit and Play sessions so the AI can debug its own logic.

### 19. `capture_viewport()`
* **Purpose**: Takes a high-speed OS-level screenshot of the Roblox Studio viewport window, returning base64 PNG data for visual multimodal inspection.

### 20. `simulate_player_input(action_sequence)`
* **Purpose**: Injects keyboard strokes (WASD, Space) and mouse clicks into the focused Studio viewport to test player mechanics.

### 21. `save_place(file_path)`
* **Purpose**: Triggers Studio place file save to prevent work loss.

### 22. `check_studio_status()`
* **Purpose**: Verifies whether Roblox Studio is running, whether the plugin is connected, and whether the session is in Edit mode or Play mode.
