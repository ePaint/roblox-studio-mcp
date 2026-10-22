# Tool Suite Specification: Canonical Tool Reference

This document is the single reference for the 18 MCP tools exposed by `roblox-studio-mcp`, a single static Go binary paired with a Luau companion plugin. Tool schemas are generated from one registry that mirrors this file.

## 1. Design Philosophy

To let an AI build any genre of game (simulators, obbies, RPGs, shooters, tycoons, puzzle games) without overfitting to specific mechanics, the suite is organized into two tool layers plus a playtest and host layer used for verification:

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
|      PLAYTEST AND HOST LAYER                                      |
|      Verification: scripted playtests via StudioTestService,      |
|      console logs, window capture, input, and Studio status.      |
+------------------------------------------------------------------+
```

Layer numbering in this document: Layer 1 is tools 1-7, Layer 2 is tools 8-13, the playtest and host layer is tools 14-18.

---

## 2. Conventions

* **Undo.** Every mutating tool runs inside a `ChangeHistoryService` recording: `TryBeginRecording(name)` before the job and `FinishRecording(id, Enum.FinishRecordingOperation.Commit)` after it. One tool call is one undo step. Read-only tools do not record.
* **Sessions.** Every tool accepts an optional `session` argument (string, default: the most recently active Studio session). Each Studio window registers its own `session_id`; `studio status` lists them.
* **Job contract.** Plugin-backed tools become jobs `{id, seq, role, deadline_ms, tool, args}`. Jobs run serially, first in first out, per session. The default deadline is 30 s; `execute_luau` and `playtest` take an explicit `timeout_s`. Cancellation is best effort at yield points. If the plugin disconnects mid-job the result is `UNKNOWN` and the bridge never replays the job.
* **Result caps.** Large results are bounded by `max_nodes` (default 500) and `max_bytes` (default 1 MiB). A truncated result carries a `cursor` for continuation, or the call fails with `TOO_LARGE` where no cursor exists.
* **Common errors.** Every plugin-backed tool can raise `NOT_CONNECTED`, `NOT_PAIRED`, `TIMEOUT` and `UNKNOWN`. The per-tool lists below name only the additional codes.

---

## 3. Targets and References

Every tool that creates something returns `{id, path}`. `id` is `Instance:GetDebugId()`, which the plugin can read and which is stable for the session. `path` is a dotted path for display.

Any argument that names an instance (`target`, `parent`, `part_a`, `part_b`, `root`, `path`) accepts one of:

| Form | Example | Notes |
|---|---|---|
| id | `"1F2A3B4C"` | Returned by earlier calls. Preferred. |
| dotted path | `"game.Workspace.Map"` | Ambiguous when siblings share a name or a name contains a dot. |
| segment array | `["Workspace","Map","Stage#3"]` | `#n` selects the n-th same-named sibling, 1-based. |

A reference to an instance that no longer exists returns `STALE_REF`.

---

## 4. Type Wire Format

Plain JSON numbers, strings and booleans pass through unchanged. Every Roblox datatype is a tagged object with a `$t` field.

| Datatype | Wire form | Notes |
|---|---|---|
| Vector3 | `{"$t":"Vector3","v":[x,y,z]}` | |
| Vector2 | `{"$t":"Vector2","v":[x,y]}` | |
| CFrame | `{"$t":"CFrame","v":[12 numbers]}` | Position then rotation matrix, as `CFrame:GetComponents()`. |
| CFrame | `{"$t":"CFrame","pos":[x,y,z],"rot_deg":[rx,ry,rz]}` | Degrees, YXZ order, same as the `Orientation` property. |
| Color3 | `{"$t":"Color3","rgb":[r,g,b]}` | 0-255 only, never 0-1. |
| UDim | `{"$t":"UDim","v":[scale,offset]}` | |
| UDim2 | `{"$t":"UDim2","v":[xs,xo,ys,yo]}` | |
| Enum | `{"$t":"Enum","v":"Material.Neon"}` | `EnumType.ItemName`. |
| Instance | `{"$t":"Instance","ref":"<id or path>"}` | For `Attachment0`, `PrimaryPart`, `Adornee` and similar. |
| ColorSequence | `{"$t":"ColorSequence","v":[{"t":0,"rgb":[255,0,0]},...]}` | Keypoints by time `t` in 0-1. |
| NumberSequence | `{"$t":"NumberSequence","v":[{"t":0,"v":1,"env":0},...]}` | `env` is the keypoint envelope. |
| NumberRange | `{"$t":"NumberRange","v":[min,max]}` | |
| Rect | `{"$t":"Rect","v":[x0,y0,x1,y1]}` | |
| Font | `{"$t":"Font","family":"rbxasset://fonts/families/GothamSSm.json","weight":"Bold","style":"Normal"}` | |
| BrickColor | `{"$t":"BrickColor","v":"Bright red"}` | |

**Shorthand.** When the target property already holds a value, an untagged array is coerced using `typeof(instance[prop])`, so `"Size": [4,1,2]` works on a Part. Tool arguments with a declared datatype (for example `axis` or `cframe`) require the tagged form.

**Failures.** `set_properties` returns `{applied: [...], failed: {prop: message}}`. Results are never silently partial. A value that cannot be coerced is reported in `failed`, or raises `COERCION` when nothing was applied.

**Reads.** Property reads return the same tagged form.

---

## 5. Layer 1: High-Level Declarative Tools

### 1. `build_tree(parent, tree)`
* **Purpose**: Builds an instance hierarchy in one call, including full GUI trees. A single node is a valid tree, so this tool also covers creating one instance.
* **Arguments**:
  * `parent` (target, required): where the root node is parented.
  * `tree` (node, required): `{class, name?, props?, attributes?, tags?, children?[]}`. `props` use the type wire format.
* **UI sugar keys on a node**: `layout` (`list|grid|page` plus layout properties), `padding`, `corner`, `stroke`, `aspect_ratio`, `auto_canvas` (sets `AutomaticCanvasSize`). Each expands to the matching `UIListLayout`/`UIGridLayout`/`UIPageLayout`, `UIPadding`, `UICorner`, `UIStroke` or `UIAspectRatioConstraint` child.
* **Returns**: `{nodes: {name: {id, path}}}`, keyed by node name.
* **Errors**: `STALE_REF`, `COERCION`, `LUAU_ERROR`.

```json
{
  "parent": "game.StarterGui",
  "tree": {
    "class": "ScreenGui", "name": "Hud",
    "children": [
      {
        "class": "Frame", "name": "Shop",
        "props": {"Size": {"$t":"UDim2","v":[0.4,0,0.5,0]}},
        "corner": 8, "aspect_ratio": 1.6, "layout": {"type": "grid"},
        "children": [{"class": "TextLabel", "name": "Title", "props": {"Text": "Shop"}, "tags": ["Label"]}]
      }
    ]
  }
}
```

### 2. `create_physics_joint(joint_type, part_a, part_b, properties)`
* **Purpose**: General mechanical and physics constraint builder. A universal replacement for single-purpose moving platform tools.
* **Arguments**:
  * `joint_type` (string, required): `Prismatic|Hinge|Weld|Spring|BallSocket|AlignPosition|AlignOrientation`.
  * `part_a`, `part_b` (target, required).
  * `properties` (object, default `{}`): constraint properties in the wire format.
* **Prismatic properties**: `axis` (Vector3), `lower`, `upper`, `speed`, `actuator` (`None|Motor|Servo`), `target`, `anchored_side` (`a|b`), `network_owner_server` (bool, default true).
* **Automatic handling**: creates the `Attachment` pair, aligns axes, anchors the side named by `anchored_side` as the rail, and calls `SetNetworkOwner(nil)` on the moving assembly when `network_owner_server` is true. A moving platform that carries characters is an unanchored part on a `Prismatic` joint with an anchored rail and `actuator: Servo`. A CFrame-tweened anchored part has zero velocity in the physics engine and does not carry characters.
* **Returns**: `{joint: {id, path}, attachments: [{id, path}, {id, path}]}`.
* **Errors**: `STALE_REF`, `COERCION`, `LUAU_ERROR`.

```json
{
  "joint_type": "Prismatic",
  "part_a": ["Workspace","Rail"],
  "part_b": ["Workspace","Platform"],
  "properties": {
    "axis": {"$t":"Vector3","v":[1,0,0]},
    "lower": 0, "upper": 40, "speed": 8,
    "actuator": "Servo", "target": 40,
    "anchored_side": "a", "network_owner_server": true
  }
}
```

### 3. `configure_environment(lighting?, atmosphere?, sky?, post_effects?)`
* **Purpose**: Sets global world visuals.
* **Arguments** (all optional objects of property to value):
  * `lighting`: properties on `Lighting` (for example `Technology`, `Ambient`, `ClockTime`, `GlobalShadows`).
  * `atmosphere`: properties on an `Atmosphere`, created if missing.
  * `sky`: properties on a `Sky`, created if missing.
  * `post_effects`: object keyed by effect class (`BloomEffect`, `ColorCorrectionEffect`, `SunRaysEffect`, `DepthOfFieldEffect`), each a property map; the effect is created if missing.
* **Returns**: `{applied: [...], failed: {prop: message}}`.
* **Errors**: `COERCION`, `LUAU_ERROR`.

### 4. `modify_terrain(operation, material?, cframe?, size?)`
* **Purpose**: Procedural voxel terrain editing.
* **Arguments**:
  * `operation` (string, required): `fill_block|fill_ball|fill_cylinder|clear|replace`.
  * `material` (Enum): required for fills and `replace`.
  * `cframe` (CFrame), `size` (Vector3): the region. `fill_ball` and `fill_cylinder` use `size.X / 2` as radius, `fill_cylinder` uses `size.Y` as height. `replace` sets every occupied voxel in the region to `material`.
* **Returns**: `{ok: true}`.
* **Errors**: `COERCION`, `LUAU_ERROR`.

### 5. `insert_asset(asset_id, parent)`
* **Purpose**: Inserts a Creator Store model through `InsertService:LoadAsset`.
* **Arguments**: `asset_id` (number, required), `parent` (target, required).
* **Scope**: models only. Sounds and meshes are set as `SoundId` or `MeshId` properties instead.
* **Returns**: `{id, path}` of the inserted model.
* **Errors**: `STALE_REF`, `LUAU_ERROR` (asset failed to load or is not a model).

### 6. `manage_script(action, path, content?, script_type?, run_context?)`
* **Purpose**: Reads, writes or deletes scripts without desync against open editor tabs.
* **Arguments**:
  * `action` (string, required): `read|write|delete`.
  * `path` (target, required): for `write` to a script that does not exist yet, the parent is the path minus its last segment and the last segment is the new name.
  * `content` (string): source for `write`.
  * `script_type` (string): `Script|LocalScript|ModuleScript`, used when `write` creates a script.
  * `run_context` (string): `RunContext` enum item name for `Script`.
* **Write mechanism**: `ScriptEditorService:UpdateSourceAsync(script, function(old) return new end)` inside a recording. It works whether or not the script's document is open.
* **Returns**: `{id, path, sha256, bytes, source?}`. `source` is present for `read`.
* **Errors**: `STALE_REF`, `TOO_LARGE` (read source above `max_bytes`), `LUAU_ERROR`.

### 7. `configure_dev_environment(options)`
* **Purpose**: Developer environment bootstrapper.
* **Arguments**: `options` (object, required): `{mock_datastore?: bool, collision_groups?: [...], player_attributes?: {...}}`.
  * `mock_datastore`: installs the mock DataStore described in section 8.
  * `collision_groups`: groups and collidable pairs to register with `PhysicsService`.
  * `player_attributes`: default attributes applied to each player.
* **Returns**: `{require_path}` of the DataStore wrapper when `mock_datastore` is set, otherwise `{}`.
* **Errors**: `COERCION`, `LUAU_ERROR`.

---

## 6. Layer 2: Low-Level Primitives

When a game needs something unique or niche, the LLM falls back to these tools.

### 8. `execute_luau(code, context?, timeout_s?)`
* **Purpose**: Executes arbitrary Luau and returns the result. Mass generation, inspecting internal state, custom math, APIs not exposed as tools.
* **Arguments**:
  * `code` (string, required).
  * `context` (string, default `edit`): `edit|server`. `server` runs only while a playtest session is running.
  * `timeout_s` (number, default 30).
* **Mechanism**: `loadstring` inside a `pcall`. The chunk has no `script` global. Output is captured through `LogService`. In `edit` context the call runs inside a recording.
* **Returns**: `{ok, value, output[], error}`. A thrown error sets `ok` false and fills `error`.
* **Errors**: `WRONG_MODE` (`server` with no running session), `TOO_LARGE` (value above `max_bytes`).

### 9. `set_properties(target, properties?, attributes?, tags?)`
* **Purpose**: Sets properties, attributes and tags on any instance. This is the only write path for all three.
* **Arguments**:
  * `target` (target, required).
  * `properties` (object): property to value, wire format.
  * `attributes` (object): attribute to value; `null` removes an attribute.
  * `tags` (object): `{add: [...], remove: [...]}`, applied through `CollectionService`.
* **Returns**: `{applied: [...], failed: {prop: message}}`.
* **Errors**: `STALE_REF`, `COERCION`.

```json
{
  "target": ["Workspace","Map","Stage#3"],
  "properties": {"Size": [4,1,2], "Material": {"$t":"Enum","v":"Material.Neon"}, "Color": {"$t":"Color3","rgb":[255,80,0]}},
  "attributes": {"Checkpoint": 3},
  "tags": {"add": ["Checkpoint"], "remove": ["Inactive"]}
}
```

### 10. `get_instance_tree(target, depth?, include_properties?, max_nodes?, cursor?)`
* **Purpose**: Traverses the scene graph. Includes class, attributes and tags for every node.
* **Arguments**:
  * `target` (target, required).
  * `depth` (integer, default 1): 0 returns the instance only, 1 adds direct children.
  * `include_properties` (bool or string array): all readable properties, or the named ones.
  * `max_nodes` (integer, default 500), `cursor` (string, from `next_cursor`).
* **Returns**: `{nodes, next_cursor?}`. Property values use the wire format.
* **Errors**: `STALE_REF`, `TOO_LARGE` (a single node exceeds `max_bytes`).

### 11. `find_instances(root?, class_name?, name_pattern?, tag?, attribute?, limit?)`
* **Purpose**: Searches descendants by class, name, tag or attribute.
* **Arguments**:
  * `root` (target, default `game`).
  * `class_name` (string): matches `IsA`.
  * `name_pattern` (string): Luau pattern matched against `Name`.
  * `tag` (string).
  * `attribute` (object): `{name, value?}`; omit `value` to match presence.
  * `limit` (integer, default 500).
* **Returns**: `[{id, path, class}]`.
* **Errors**: `STALE_REF`, `TOO_LARGE`.

```json
{"root": "game.Workspace", "class_name": "BasePart", "tag": "Checkpoint", "attribute": {"name": "Checkpoint"}, "limit": 50}
```

### 12. `delete_instance(target)`
* **Purpose**: Removes an object from the DataModel.
* **Arguments**: `target` (target, required).
* **Returns**: `{deleted: {id, path}}`.
* **Errors**: `STALE_REF`.

### 13. `call_method(target, method, args?)`
* **Purpose**: Invokes a method on an instance (`PivotTo`, `ApplyImpulse`, `Clone`, `Play`, `Stop`).
* **Arguments**: `target` (target, required), `method` (string, required), `args` (array, default `[]`, wire format).
* **Returns**: `{value}`, in the wire format. Returned instances come back as `{"$t":"Instance","ref":...}`.
* **Errors**: `STALE_REF`, `COERCION`, `LUAU_ERROR` (method missing or raised).

---

## 7. Playtest and Host Layer

### 14. `playtest(action, mode?, code?, timeout_s?)`
* **Purpose**: Starts, stops and scripts Play and Run sessions from inside the plugin through `StudioTestService:ExecutePlayModeAsync` and `ExecuteRunModeAsync`. No keystrokes are sent.
* **Arguments**:
  * `action` (string, required): `start|stop|run`.
  * `mode` (string): `play|run`.
  * `code` (string): required for `run`.
  * `timeout_s` (number, default 30): applies to `run`.
* **`run`**: the plugin injects a temporary Script into `ServerScriptService`, starts the session, and the script captures `LogService.MessageOut` in the server DataModel, runs `code` (its return value becomes `value`), and ends with `StudioTestService:EndTest(payload)`. The plugin removes the script and returns the payload `{ok, value, error, logs, errors, duration_ms, timed_out}`. On timeout the session is stopped and `timed_out` is true.
* **`start`**: spawns the session and returns `{session_id, mode}` once the server-role plugin instance reports in. `stop` is delivered to that instance, which calls `EndTest()`.
* **Errors**: `WRONG_MODE` (a session is already running, which the engine reports as "Previous call to start play session has not been completed", or `stop` with no session), `TIMEOUT`.

```json
{
  "action": "run",
  "mode": "play",
  "timeout_s": 60,
  "code": "local p = workspace.Platform\ntask.wait(5)\nreturn {x = p.Position.X}"
}
```

### 15. `get_console_logs(since_seq?, level?, source?, pattern?, wait_ms?)`
* **Purpose**: Reads captured output from edit, server and client DataModels. Read-only; there is no clear.
* **Arguments**:
  * `since_seq` (integer, default 0): return entries after this sequence number.
  * `level` (string): `output|info|warning|error`.
  * `source` (string): `edit|server|client`.
  * `pattern` (string): Lua pattern matched against the message.
  * `wait_ms` (integer, default 0): hold the call until a matching entry arrives or the wait expires.
* **Returns**: `{entries: [{seq, ts, level, source, message}], next_seq}`.
* **Errors**: none beyond the common set.

### 16. `capture_viewport(max_width?)`
* **Purpose**: Screenshot of the Studio window for visual inspection. Windows host only in v1.
* **Arguments**: `max_width` (integer, default 1568): long edge in pixels after downscale.
* **Returns**: MCP image content (PNG), not base64 text.
* **Errors**: `UNSUPPORTED_HOST`, `NOT_CONNECTED` (no Studio window found).

### 17. `simulate_player_input(steps)`
* **Purpose**: OS-level keyboard and mouse input for smoke tests. Windows only and experimental. Prefer in-engine checks (`Humanoid:MoveTo`, `PivotTo`) through `playtest` with `action: run`.
* **Arguments**: `steps` (array, required). Step: `{type, key?, x?, y?, ms?}` with `type` one of `key|click|move|wait`. `x` and `y` are client-area pixels of the image returned by `capture_viewport`. `ms` is hold or wait time.
* **Returns**: `{steps_run}`.
* **Errors**: `UNSUPPORTED_HOST` (non-Windows host, or Studio could not be focused; input is never dropped silently), `NOT_CONNECTED`.

### 18. `studio(action)`
* **Purpose**: Studio status, selection and save.
* **Arguments**: `action` (string, required): `status|selection|save`.
* **`status`**: returns `{studio_running, plugin_connected, paired, mode, roles[], session_id, sessions[], pairing_code?}`. Works without a connected plugin.
* **`selection`**: returns `[{id, path, class}]` for the current selection.
* **`save`**: sends Ctrl+S through the host only when the place already has a file. It takes no path argument.
* **Errors**: `NEEDS_MANUAL_SAVE` (place has no file yet), `UNSUPPORTED_HOST` (`save` on a non-Windows host).

---

## 8. Mock DataStore Contract

Nothing can shadow `DataStoreService`, so game code must `require` a wrapper.

* `ServerStorage/McpDev/MockDataStoreService`: in-memory implementation exposing `GetDataStore`, and per store `GetAsync`, `SetAsync` and `UpdateAsync`. It lives in `ServerStorage` so it does not replicate to clients.
* `ServerStorage/McpDev/DataStore`: wrapper that returns the real `DataStoreService` outside Studio or when the attribute `UseRealDataStore` is true, and the mock otherwise.
* `configure_dev_environment` with `mock_datastore: true` returns `{require_path}` for the wrapper. Generated game code uses `require` on that path and never references `DataStoreService` directly.
* The real service errors in Studio (message mentions 403) unless the place is published and "Enable Studio Access to API Services" is on, which is why the mock is the default in Studio.

---

## 9. Error Codes

Errors map to an MCP result with `isError` set and the body `{code, message, job_id}`.

| Code | Meaning |
|---|---|
| `NOT_CONNECTED` | No Studio session with a connected plugin, or no Studio window for a host tool. |
| `NOT_PAIRED` | The plugin has not completed pairing, or its token is invalid. |
| `STALE_REF` | A target id or path no longer resolves to a live instance. |
| `TIMEOUT` | The job deadline passed before a result arrived. |
| `WRONG_MODE` | The tool needs a different Studio state, such as a running session or none. |
| `TOO_LARGE` | The result exceeds `max_bytes` and cannot be paged. |
| `COERCION` | A value could not be converted to the property or argument type. |
| `UNSUPPORTED_HOST` | The host operation is unavailable on this OS, or Studio could not be focused. |
| `LUAU_ERROR` | An engine call or script raised an error in the plugin. |
| `UNKNOWN` | The outcome is not known, for example the plugin disconnected mid-job. The job is not replayed. |
| `NEEDS_MANUAL_SAVE` | `studio save` on a place that has no file yet. |

Stdout carries only MCP frames. All logging goes to stderr, or to a file with `--log-file`.
