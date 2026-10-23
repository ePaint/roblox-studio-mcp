# roblox-studio-mcp

MCP server that lets an AI client build, inspect and script a place open in Roblox Studio. A Go binary speaks MCP to the client and relays work to a companion Luau plugin running inside Studio.

## How it works

```
MCP client ──stdio or HTTP──▶ Go server ──HTTP bridge (127.0.0.1:58741)──▶ Studio plugin
(Claude Code)                    │                                         (long-polls for jobs)
                                 └── Windows host layer: screenshot, Ctrl+S
```

- **Go server** (`cmd/roblox-studio-mcp`, `internal/mcp`): registers the 13 MCP tools. Most tools become a job on the bridge queue. `get_console_logs` is answered from the server's log buffer, and `capture_viewport` and `studio` run on the host layer.
- **Bridge** (`internal/bridge`): local HTTP server. Endpoints: `/pair`, `/poll`, `/result`, `/logs`, `/status`, `/rpc`. The plugin long-polls `/poll` (up to 5 s per request), runs the job and posts the outcome to `/result`.
- **Plugin** (`plugin/src/init.luau`, embedded into the binary): Studio loads it into every DataModel. The Edit copy shows the connection widget and runs jobs. During a playtest the server copy also runs jobs (role `server`), and the client copy only forwards logs. Every copy forwards `LogService` output to `/logs` once per second, tagged with its role.
- **Jobs in Edit** run inside a `ChangeHistoryService` recording named `MCP: <tool>`, so every tool call is one Ctrl+Z step. A call that fails is rolled back, so it never leaves a half-applied change.
- **Host layer** (`internal/host`): Win32 calls for window lookup, screen capture and keyboard input. On macOS it returns `UNSUPPORTED_HOST`.

### Pairing

1. On start the server prints a pairing code `RBX-XXXX` to stderr. A new code is generated every start, and `studio` `status` also returns it as `pairing_code`.
2. In Studio, the plugin widget (toolbar "Roblox MCP" > "MCP Status") takes a Bridge URL (default `http://127.0.0.1:58741`) and the code.
3. The server answers with a token. The plugin stores it in its settings, and the server writes it to `.mcp_token` in its working directory and reuses it on the next start. After a server restart, pressing Pair / Connect with an empty code reconnects.

### Running the server

| Flag | Default | Purpose |
|---|---|---|
| `-host` | `127.0.0.1` | Bridge bind address |
| `-port` | `58741` | Bridge port |
| `-http` | empty | Serve MCP over streamable HTTP on this address instead of stdio, e.g. `0.0.0.0:58742` |
| `-install-plugin` | | Write the embedded plugin to `%LOCALAPPDATA%\Roblox\Plugins\` as `RobloxStudioMCP.lua` (the file Studio loads) and `RobloxStudioMCP.server.luau`, then exit |
| `-version` | | Print version and exit |

**Local (stdio):** the MCP client launches the exe directly. Used when the client runs on the same Windows machine as Studio.

**Remote (HTTP):** run the server on the Windows machine and connect from another machine:

```powershell
.\dist\roblox-studio-mcp.exe -http 0.0.0.0:58742
```
```bash
claude mcp add --transport http roblox-studio http://<windows-ip>:58742/ --header "Authorization: Bearer <token>"
```

The token is generated on the first `-http` start, saved to `.mcp_http_token` in the working directory and printed to stderr. Every request without `Authorization: Bearer <token>` gets a 401. Restarting the server drops the MCP session, so the client may need to reconnect (`/mcp` in Claude Code).

Requirements for the host tools (`capture_viewport`, `studio` `save`):

- Start the server from a terminal on the logged-in Windows desktop. Launched from a background session (an agent sandbox, SSH or a scheduled task), it sees no windows, and the error reports something like `in session 2 scanned 1 top-level windows`.
- Keep the Studio window on top and not minimized. The screenshot copies screen pixels, so anything covering Studio ends up in the image.

If the bridge port is already taken by another running instance (for example one launched by another MCP client from `mcp_config.json`), the new instance goes into proxy mode and forwards jobs to the first one through `/rpc`. Host tools still run in the new instance.

## Tools

Every result is a JSON object. A plain value (string, number, array) comes back wrapped as `{"value": ...}`.

Instance references (`target`, `parent`, `root`, `part_a`, `part_b`, `path`) accept three forms:

- **Dotted path** starting with `game.`, e.g. `"game.Workspace.Map.Door"`. Resolved with `FindFirstChild` per segment, so names containing dots break it.
- **Segment array**, e.g. `["Workspace", "Map", "Stage#2"]`. `Name#N` picks the Nth child with that name. Use this for duplicate names or names with dots.
- **Any other string** resolves to the first descendant of `game` whose `Name` or debug id equals it. It scans the whole place, so a bare name is ambiguous. The `id` values the tools return are `GetDebugId()` strings, valid for the current Studio session.

Created or found instances come back as `{id, path}`, or `{id, path, class}` from searches.

### execute_luau

Runs arbitrary Luau through `loadstring`.

- `code` (string, required)
- `context`: `edit` (default) or `server`. `server` runs in the playtest's server DataModel and fails at once with `WRONG_MODE` when no playtest is running.
- `timeout_s` (number, default 30): how long the server waits for the result

Returns the first value the chunk returns, serialized to the wire format, so use `return`. Tables are serialized recursively (arrays stay arrays, other keys become strings). `print` output goes to the console logs. Compile and runtime errors return `LUAU_ERROR`, and in Edit the call is rolled back.

A running game refuses `loadstring` unless `ServerScriptService.LoadStringEnabled` is on, so in the playtest server the code is compiled through a temporary ModuleScript. A syntax error there only reports that the module failed to load, and the parser message shows up in `get_console_logs` with source `server`. A playtest can only be stopped from the server: `game:GetService("StudioTestService"):EndTest(<any non-nil value>)` with `context: "server"`.

### build_tree

Creates a whole hierarchy in one call.

- `parent` (ref, required)
- `tree` (node, required): `{class, name, props, attributes, tags, children}`. `class` defaults to `Part`. `props` take the same values as `set_properties`. UI shortcuts: `corner` (number, adds a `UICorner` with that pixel radius), `stroke` (number, adds a `UIStroke` with that thickness), `aspect_ratio` (number, adds a `UIAspectRatioConstraint`), `auto_canvas` (true, sets `AutomaticCanvasSize` to Y).

Returns `{nodes: {<name>: {id, path}}, failed: {"<node>.<prop>": error}}`. `nodes` lists every node that has a `name`. A property or attribute that cannot be set is reported in `failed` and the rest of the tree is still built. An invalid `class` fails the whole call and rolls it back.

### set_properties

- `target` (ref, required)
- `properties` (map): values in the wire format. Bare arrays are converted using the property's current type: `Vector3` and `Color3` (0-255) take 3 numbers, `Vector2` and `UDim` take 2, `UDim2` takes 4, `CFrame` takes 3 (position) or 12, `NumberRange` takes 2. Enum properties accept the item name as a string (`"Neon"`), and `BrickColor` accepts a color name.
- `attributes` (map): plain or tagged values. `null` removes the attribute.
- `tags` (`{add: [...], remove: [...]}`, CollectionService tags)

Returns `{applied: [names], failed: {name: error}}`. Attributes appear as `@name` in both.

### get_instance_tree

- `target` (ref, required)
- `depth` (default 1)
- `include_properties`: `true` for every script-readable property (listed through `ReflectionService`), or an array of property names. Values come back in the wire format.
- `max_nodes` (default 500, or 50 with `include_properties`)
- `cursor`: the `next_cursor` of a previous call with the same `target` and `depth`

Returns `{nodes: {id, name, class, path, child_count, properties?, children?}, truncated, next_cursor?}`. When `truncated` is true, call again with `cursor` to get the next page. Ancestors already returned on an earlier page reappear with `repeated: true` so the new nodes keep their place in the tree. `child_count` shows how many children a node has even when `depth` stops above them.

### find_instances

- `root` (ref, default `game`)
- `class_name` (`IsA` match), `name_pattern` (Lua pattern on `Name`), `tag`
- `attribute` (instances that have it) and `attribute_value` (wire format, must also be equal)
- `limit` (default 500)

Returns `{matches: [{id, path, class}], truncated}`. An invalid pattern, class name or attribute name returns `COERCION`.

### delete_instance

- `target` (ref, required)

Calls `Destroy()`. Returns `{deleted: {id, path}}`. Locked instances such as services return `LUAU_ERROR`.

### call_method

- `target` (ref, required), `method` (string, required), `args` (array in the wire format)

Calls `target:method(...)`, e.g. `PivotTo`, `Clone`, `ApplyImpulse`. Returns `{value}`. An unknown method returns `COERCION`.

### create_physics_joint

- `joint_type`: `Weld` creates a `WeldConstraint`, `AlignPosition` and `AlignOrientation` create those classes, and any other value creates `<joint_type>Constraint` (`Hinge`, `Prismatic`, `Spring`, `BallSocket`, `Rope`, `Rod`, `NoCollision`, ...).
- `part_a`, `part_b` (BasePart refs, required)
- `properties`: shortcuts `actuator` (`ActuatorType`), `speed` (`Speed`), `lower` and `upper` (`LowerAngle`/`UpperAngle` on hinges, `LowerLimit`/`UpperLimit` otherwise; both together also enable limits), `anchored_side` (`"a"` or `"b"`). Any other key is set as a property on the joint, e.g. `Stiffness`, `Responsiveness`, `RigidityEnabled`.

Attachment-based joints get a new attachment on each part. Part-based joints (`Weld`, `NoCollision`) set `Part0` and `Part1` and create no attachments. The joint is parented to `part_a`. Returns `{joint, attachments, failed}`. An unknown `joint_type` returns `COERCION`.

### manage_script

- `action`: `read`, `write` or `delete`
- `path` (ref, required)
- `content` (string, for `write`)
- `script_type` (`Script`, `LocalScript`, `ModuleScript`, default `Script`) and `parent` (ref): used only when `path` does not resolve and a new script is created. Without `parent`, the new script goes under the parent named in `path` (`game.ServerScriptService.Foo` goes into ServerScriptService), or into Workspace for a bare name. A missing parent returns `STALE_REF`.
- `run_context`: `Legacy`, `Server`, `Client` or `Plugin`, sets `RunContext` on a `Script`

`read` returns `{id, path, source}`. `write` sets the source through `ScriptEditorService:UpdateSourceAsync`, so open editor tabs stay in sync, falls back to setting `Source` directly, and returns `{id, path, created, failed}`. If neither works it returns `LUAU_ERROR`. `delete` returns `{deleted: {id, path}}`.

### insert_asset

- `asset_id` (number, required), `parent` (ref, default Workspace)

Loads the asset with `InsertService:LoadAsset` and parents the returned container Model. Returns `{id, path}`.

### get_console_logs

Answered by the server's log buffer (last 5000 entries), not by a plugin job.

- `since_seq`: return entries after this sequence number
- `level`: `output`, `info`, `warning`, `error`
- `source`: `edit`, `server` or `client` (the DataModel that printed it)
- `pattern`: plain substring match
- `wait_ms`: wait this long for new entries before returning

Returns `{entries: [{seq, ts, level, source, message}], next_seq}`. Pass `next_seq` as `since_seq` to page forward. The plugin flushes once per second, so `wait_ms` of 1500 or more after an action catches its output.

### capture_viewport

- `max_width` (default 1568): larger captures are scaled down

Returns a PNG of the Studio window's client area (the whole Studio UI, panels included). Windows only, and Studio must be on top.

### studio

- `action`: `status`, `selection` or `save`
- `status`: `{studio_running, plugin_connected, paired, mode, roles, session_id, sessions, pairing_code}`. `plugin_connected` means a plugin polled within the last 15 s. `mode` is `play` while a playtest server is connected, and `roles` lists the connected DataModels.
- `selection`: `{selection: [{id, path, class}]}`, the instances selected in Studio
- `save`: brings Studio to the foreground and sends Ctrl+S. Keys are never sent unless Studio is the foreground window. Failure returns `NEEDS_MANUAL_SAVE`. On a never-saved place Ctrl+S opens the Save As dialog. When nothing changed, Studio logs a (localized) "no changes, not saved" message, which `get_console_logs` shows.

Plugins have no API to save a place, so Ctrl+S is the only way to save.

## Wire format

Plain numbers, strings and booleans pass through. Engine types use a `$t` tag:

| Type | Form |
|---|---|
| Vector3 | `{"$t":"Vector3","v":[x,y,z]}` |
| Vector2 | `{"$t":"Vector2","v":[x,y]}` |
| UDim | `{"$t":"UDim","v":[scale,offset]}` |
| UDim2 | `{"$t":"UDim2","v":[xs,xo,ys,yo]}` |
| Color3 | `{"$t":"Color3","rgb":[0-255,0-255,0-255]}` |
| CFrame | `{"$t":"CFrame","v":[12 components]}` or `{"$t":"CFrame","pos":[x,y,z],"rot_deg":[rx,ry,rz]}` |
| Enum | `{"$t":"Enum","v":"Material.Neon"}` |
| Instance | `{"$t":"Instance","ref":<ref>}` |
| NumberRange | `{"$t":"NumberRange","v":[min,max]}` |
| Rect | `{"$t":"Rect","v":[x0,y0,x1,y1]}` |
| BrickColor | `{"$t":"BrickColor","v":"Bright red"}` |
| ColorSequence | `{"$t":"ColorSequence","v":[{"t":0,"rgb":[r,g,b]}, ...]}` |
| NumberSequence | `{"$t":"NumberSequence","v":[{"t":0,"v":1,"env":0}, ...]}` |

Use the tagged form for method arguments, attribute values and `execute_luau` return values. Property values in `set_properties`, `build_tree` and joint `properties` may also use the bare shorthand described under `set_properties`. Values coming back are serialized the same way, and Instances also carry `path`. Types without a serializer come back as `tostring(value)`.

## Errors

Tool errors carry `{code, message}`: `NOT_CONNECTED`, `NOT_PAIRED`, `STALE_REF` (reference did not resolve), `TIMEOUT`, `WRONG_MODE`, `TOO_LARGE`, `COERCION`, `UNSUPPORTED_HOST`, `LUAU_ERROR`, `UNKNOWN`, `NEEDS_MANUAL_SAVE`. When the plugin is not polling, jobs wait in the queue until their timeout.

## Gotchas

- `/status` and `/rpc` on the bridge have no auth, and `/status` returns the pairing code. Keep the bridge on `127.0.0.1` (the default) and expose only the `-http` endpoint.
- Edits are not saved until `studio` `save` runs or someone saves in Studio.

## Development

```bash
go test ./...
GOOS=windows go vet ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/roblox-studio-mcp.exe ./cmd/roblox-studio-mcp
```

- The binary embeds `plugin/src/init.luau` only. `plugin/src/deserializer.luau` is an older, unused copy of the serializer in `init.luau`.
- After changing the plugin, rebuild and run `roblox-studio-mcp.exe -install-plugin`, then restart Studio. A running exe is locked on Windows, so while the server runs build a second copy (`go build -o dist\plugin-installer.exe ./cmd/roblox-studio-mcp`) and install with that.
- Win32 code lives in `internal/host/host_windows.go` behind `//go:build windows`. Keep pure logic in `host.go` so it stays testable on macOS.
- Stdout is reserved for MCP frames in stdio mode. Log to stderr.
