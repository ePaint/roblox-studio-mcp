# Go Implementation Plan: Roblox Studio MCP

## 1. Overview and Strategy

The project is a single static Go binary, `roblox-studio-mcp`, plus a Luau companion plugin. There is no Python, Node or Rojo in the toolchain. The binary is cgo-free, targets Windows x64 first and macOS second.

### Why Go
* **Single binary distribution**: one zero-dependency executable (`roblox-studio-mcp.exe`). Users need no runtime installed.
* **Minimal footprint**: startup in milliseconds and a small resident memory baseline.
* **Native concurrency**: goroutines and channels handle stdio MCP traffic and the HTTP link to Studio at the same time.
* **Direct Windows host integration**: Go reaches `user32.dll`, `gdi32.dll` and `dwmapi.dll` through `golang.org/x/sys/windows` without cgo.
* **Cross-compilation from a Mac**: `GOOS=windows` builds and vets on the development machine.

---

## 2. Target Project Structure

```
roblox-studio-mcp/
├── cmd/
│   ├── roblox-studio-mcp/
│   │   └── main.go                  # Entry point: stdio MCP server, install-plugin subcommand
│   └── buildplugin/
│       └── main.go                  # Emits plugin/dist/RobloxStudioMcp.rbxmx (text/template)
├── internal/
│   ├── bridge/
│   │   ├── server.go                # HTTP bridge on 127.0.0.1:58741, endpoint handlers
│   │   ├── pairing.go               # Pairing code, token issue and validation, Host/Origin checks
│   │   ├── proxy.go                 # Owner detection and loopback /rpc proxy for later binaries
│   │   ├── session.go               # Per-Studio session registry, roles, most-recent-active default
│   │   └── queue.go                 # Serial FIFO job queue per session, deadlines, result matching
│   ├── host/
│   │   ├── host.go                  # Host interface
│   │   ├── host_windows.go          # Win32 backend (user32, gdi32, dwmapi)
│   │   └── host_other.go            # //go:build !windows, returns UNSUPPORTED_HOST
│   ├── codec/
│   │   └── codec.go                 # Tagged type wire format (Vector3, CFrame, Color3, Enum, ...)
│   └── tools/
│       └── ...                      # Tool registry, schemas and handlers for the 18 tools
├── plugin/
│   ├── src/
│   │   ├── init.luau                # Entry point, role detection, pairing widget
│   │   ├── bridge.luau              # HTTP poll loop, result and log posting
│   │   ├── deserializer.luau        # Tagged JSON to Roblox types and back
│   │   ├── script_manager.luau      # ScriptEditorService + ChangeHistoryService
│   │   ├── mock_datastore.luau      # Mock DataStore module and wrapper
│   │   ├── physics_builder.luau     # Constraint and joint generator
│   │   └── playtest.luau            # StudioTestService session control
│   ├── dist/                        # Build output, gitignored
│   │   └── RobloxStudioMcp.rbxmx
│   └── embed.go                     # go:embed of dist/RobloxStudioMcp.rbxmx
├── testdata/
│   └── golden/                      # Golden JSON for the type codec
├── .github/
│   └── workflows/
│       └── ci.yml                   # ubuntu-latest and windows-latest
├── docs/
│   ├── research-feasibility.md      # Feasibility study and positioning
│   ├── deep-architecture-map.md     # Engine systems and data type mappings
│   ├── tool-suite-spec.md           # Canonical tool schemas
│   ├── adversarial-audit.md         # Game scenario walkthroughs and engine barriers
│   ├── plan-review.md               # Design review behind the current plan
│   └── go-implementation-plan.md    # This document
├── temp/                            # Gitignored local scratchpad
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

---

## 3. Go Stack and Engine APIs

### Go Libraries
Go 1.25 or newer.
* **MCP SDK**: `github.com/modelcontextprotocol/go-sdk` (official; stdio transport, image content).
* **Windows calls**: `golang.org/x/sys/windows`. It does not wrap `SendInput`, `BitBlt`, `PrintWindow`, `GetClientRect` or `ClientToScreen`, so `host_windows.go` declares them with `windows.NewLazySystemDLL`.
* **Downscaling**: `golang.org/x/image/draw`.
* **Everything else**: standard library (`net/http`, `encoding/json`, `text/template`, `embed`).

Not used: `mark3labs/mcp-go`, `kbinani/screenshot`, `robloxapi/rbxfile`.

### Build Commands
```
go run ./cmd/buildplugin
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/roblox-studio-mcp.exe ./cmd/roblox-studio-mcp
```
The plugin is built first because the main binary embeds it. On macOS, check with `go test ./...` and `GOOS=windows go vet ./...`.

### Roblox Engine Services (Luau Plugin)
* `HttpService`: outgoing long-poll requests to the bridge.
* `StudioTestService`: `ExecutePlayModeAsync`, `ExecuteRunModeAsync`, `EndTest`, `GetTestArgs`.
* `ScriptEditorService`: `UpdateSourceAsync` for script writes.
* `ChangeHistoryService`: `TryBeginRecording` / `FinishRecording` around every mutation.
* `CollectionService`: tags.
* `LogService`: console capture in the edit and server DataModels.
* `InsertService`: `LoadAsset` for models.
* `RunService`: role detection (`IsEdit`, `IsRunning`, `IsServer`).

---

## 4. Architectural Components

### A. The Local HTTP Bridge (`internal/bridge/`)

**Pairing and auth.** Plugins cannot read files or environment variables, so a per-startup secret cannot reach them. Instead:
1. The binary generates a 6-digit pairing code, printed to stderr and returned by the `studio` tool's `status`.
2. The user types the code into the plugin widget.
3. The plugin calls `POST /pair {code}` and receives a long-lived token, stored with `plugin:SetSetting`.
4. Every later request carries the token in the `X-RSMCP-Token` header, never in the URL.

The bridge validates the `Host` header (`127.0.0.1:<port>` or `localhost:<port>`), rejects any request carrying an `Origin` header and sends no CORS headers. `GET /status` is the only unauthenticated endpoint and returns no secrets. Token scopes (read, mutate, exec, host) are a later item.

**Multiple clients and Studios.** Each MCP client spawns its own stdio binary.
* The first binary binds `127.0.0.1:58741` and owns the plugin link.
* A later binary gets a bind error, confirms the owner through `GET /status` (server id), and forwards its tool calls to the owner over loopback `POST /rpc`.
* Each Studio window registers a `session_id`. Tools take an optional `session` argument; the default is the most recently active session.

**Endpoints.**
* `GET /poll`: long-poll held up to 5 s until a job arrives.
* `POST /result`: job result, return value or error.
* `POST /logs`: console entries batched every 250 ms or 50 entries.
* `POST /pair`: exchange a pairing code for a token.
* `GET /status`: liveness, server id, connection state.
* `POST /rpc`: owner proxy for later binaries.

**Budget.** `HttpService` allows 500 requests per minute, shared by all calls, so logs are always batched. Plugins trigger a per-domain permission prompt (Plugin Management, HTTP permissions); the plugin surfaces a denial instead of retrying silently. The experience setting "Allow HTTP Requests" governs game scripts and is not assumed to be required for plugins.

**Transport upgrade.** Phase 0 tests `HttpService:CreateWebStreamClient(Enum.WebStreamClientType.WebSocket, options)` (Studio only, at most 4 clients) against the Go bridge. If it works, it replaces long-poll in v1.1.

**Job contract.**
* Envelope: `{id, seq, role, deadline_ms, tool, args}`.
* Serial FIFO per session. Default deadline 30 s; `execute_luau` and `playtest` take an explicit `timeout_s`.
* Mutating jobs run inside `ChangeHistoryService:TryBeginRecording(name)` and `FinishRecording(id, Enum.FinishRecordingOperation.Commit)`, so every mutation is one Undo step.
* Cancel is best effort at yield points.
* A plugin disconnect mid-job returns `UNKNOWN`. The bridge never replays a job.
* Result caps: `max_nodes` default 500, `max_bytes` default 1 MiB, with a continuation `cursor`.

**Instance references.** Every tool that creates something returns `{id, path}`. `id` is `Instance:GetDebugId()`, stable for the session. Any `target` argument accepts an `id`, a dotted path string (`game.Workspace.Map`) or a segment array (`["Workspace","Map","Stage#3"]`, where `#n` picks the n-th same-named sibling, 1-based). A dead reference returns `STALE_REF`.

**Errors and stdout.** Error codes: `NOT_CONNECTED`, `NOT_PAIRED`, `STALE_REF`, `TIMEOUT`, `WRONG_MODE`, `TOO_LARGE`, `COERCION`, `UNSUPPORTED_HOST`, `LUAU_ERROR`, `UNKNOWN`, `NEEDS_MANUAL_SAVE`. They map to MCP `isError` results carrying `{code, message, job_id}`. Stdout is reserved for MCP frames; all logging goes to stderr, or to a file with `--log-file`.

### B. Type Codec (`internal/codec/`)
Plain JSON numbers, strings and booleans pass through. Every Roblox datatype is a tagged object, for example `{"$t":"Vector3","v":[x,y,z]}`, `{"$t":"Color3","rgb":[0-255,0-255,0-255]}`, `{"$t":"Enum","v":"Material.Neon"}` and `{"$t":"Instance","ref":"<id or path>"}`. CFrame is either 12 numbers or `pos` plus `rot_deg` (degrees, YXZ order). When the target property already holds a value, an untagged array is coerced using `typeof(instance[prop])`. `set_properties` returns `{applied, failed}` and is never silently partial. Reads return the same tagged form. The full type list lives in `tool-suite-spec.md`; golden files in `testdata/golden` pin the encoding.

### C. Host Layer (`internal/host/`)
The `Host` interface has `FindStudio`, `Capture`, `SendKeys`, `SendMouse` and `SaveShortcut`. `host_other.go` carries `//go:build !windows` and returns `UNSUPPORTED_HOST`, so `go build` and `go test` work on the Mac.

* **Window discovery**: `EnumWindows`, `GetWindowThreadProcessId` and `QueryFullProcessImageName`, matching `RobloxStudioBeta.exe` and the title suffix ` - Roblox Studio` (titles are `<Place> - Roblox Studio`). Several instances are handled.
* **Capture**: `PrintWindow(PW_RENDERFULLCONTENT)` first, then screen-DC `BitBlt` of the `ClientToScreen` rectangle as fallback. `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)` is set at startup. The image is downscaled with `x/image/draw` to a 1568 px long edge and returned as MCP image content.
* **Input**: `SendInput` with scancodes. The foreground window is checked first; on failure the tool reports `UNSUPPORTED_HOST` or a focus error instead of dropping input.
* **Save**: Ctrl+S through the host, only when the place already has a file. Otherwise `studio save` returns `NEEDS_MANUAL_SAVE`.
* **macOS**: a later backend in `host_darwin.go` uses `ebitengine/purego` to stay cgo-free.

The host layer does not control playtests; that runs through `StudioTestService`.

### D. Luau Companion Plugin (`plugin/src/`)

**Roles.** Plugin code loads separately in the edit, server and client DataModels.
* Edit instance (`RunService:IsEdit()`): the main poll loop and all edit jobs.
* Server instance (`IsRunning() and IsServer()`): a lightweight poll for `stop` and `server`-context jobs, and forwards play logs.
* Client instances: inactive.
* Every poll carries `role` and `session_id`.

**Execution safety.** Every job runs in `pcall`. Failures return a structured payload with the stack trace instead of crashing the plugin.

**Script management.** `manage_script` writes through `ScriptEditorService:UpdateSourceAsync(script, function(old) return new end)` inside a ChangeHistory recording. It works whether the script document is open or not, so there is no open/closed branch.

**Playtest.** `StudioTestService:ExecutePlayModeAsync(args)` and `ExecuteRunModeAsync(args)` start a session from the plugin, yield until it ends, and return the value passed to `EndTest(value)` inside the session. `GetTestArgs()` reads the args there. A second start while a session runs errors with "Previous call to start play session has not been completed", which the plugin catches.
* **Scripted test** (`playtest run`): the plugin injects a temporary Script into ServerScriptService and starts the session. The script captures `LogService.MessageOut` in the server DataModel, runs the user code and calls `EndTest({ok, value, error, logs, errors, duration_ms, timed_out})`. The plugin removes the script and returns the table.
* **Interactive session** (`playtest start`): the plugin spawns `ExecutePlayModeAsync` in a task and returns once the server-role instance has reported in. `playtest stop` is delivered to the server-role instance, which calls `EndTest()`.

**Mock DataStore.** `configure_dev_environment` installs `ServerStorage/McpDev/MockDataStoreService` and a wrapper `ServerStorage/McpDev/DataStore`. The wrapper returns the real service outside Studio, or when the attribute `UseRealDataStore` is true, and the mock otherwise. Nothing can shadow `DataStoreService`, so generated game code must `require` the wrapper. The tool returns `{require_path}`.

### E. Plugin Packaging
`cmd/buildplugin` renders `plugin/src/*.luau` through `text/template` into `plugin/dist/RobloxStudioMcp.rbxmx`: XML with one Script and ModuleScripts whose `Source` is CDATA. `plugin/embed.go` embeds the file with `go:embed`. `roblox-studio-mcp install-plugin` copies it to `%LOCALAPPDATA%\Roblox\Plugins` on Windows or `~/Documents/Roblox/Plugins` on macOS. `plugin/dist` is gitignored.

---

## 5. Testing

* **Bridge**: a Go fake-plugin client against `httptest` covers pairing, polling, results, the proxy path and error codes.
* **Tools**: the SDK in-memory transport drives each tool handler.
* **Codec**: golden JSON files in `testdata/golden`.
* **Luau deserializer**: a smoke script run through `playtest run` on a fixture place.
* **CI**: GitHub Actions on `ubuntu-latest` and `windows-latest` running `go test ./...` and `GOOS=windows go vet ./...`. Win32 smoke tests sit behind an `integration` build tag.

---

## 6. Phased Implementation Roadmap

### Phase 0: Spikes (before any production code)
* **In Studio**: `ExecutePlayModeAsync` with an injected script and an `EndTest` round trip; `loadstring` output capture; `CreateWebStreamClient(WebSocket)` against a Go server; the HTTP permission prompt on localhost.
* **On a Windows VM**: `PrintWindow` capture of Studio; whether `SendInput` WASD is honored by the play client.
* **In Go**: port owner plus `/rpc` proxy; official SDK stdio with image content; the `.rbxmx` builder.

### Phase 1: Bridge, Pairing and Plugin Roles
- Initialize `go.mod`.
- Build `internal/bridge`: HTTP server, pairing, token auth, job queue, owner proxy.
- Build the plugin skeleton with role detection, pairing widget and poll loop.
- Deliverable: two-way ping between the Go binary and Studio.

### Phase 2: Type Codec and Core Primitives
- Implement `internal/codec` and the Luau deserializer.
- Implement `build_tree`, `set_properties`, `get_instance_tree`, `find_instances`, `delete_instance`, `call_method` and `execute_luau` (edit context).
- Deliverable: static building, property edits and queries from AI prompts.

### Phase 3: Builders, Scripting and Playtest
- Implement `manage_script`, `create_physics_joint`, `configure_environment`, `modify_terrain`, `insert_asset` and `configure_dev_environment`.
- Implement `playtest`, `get_console_logs` and `execute_luau` (server context).
- Deliverable: scripting, physics assemblies, offline DataStore and an autonomous run-and-assert loop.

### Phase 4: Windows Host Layer
- Implement `internal/host` on Windows.
- Implement `capture_viewport`, `simulate_player_input` and `studio` `save`.
- Deliverable: screenshots and OS-level input for smoke checks.

### Phase 5: Verification and Packaging
- Run end-to-end generation of a clicker game and an obby.
- Finish `install-plugin`, release builds and CI.
- Deliverable: a verified release binary with the embedded plugin.
