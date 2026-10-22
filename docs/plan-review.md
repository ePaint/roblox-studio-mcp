# Design review: findings against the original plan

The findings below were raised against the first version of the plan docs. The current docs already apply the fixes and the recommended decisions listed at the end. This file is kept as the rationale.

Scope: the five docs under `docs/`, README, the former `tools/` scaffold, `.gitignore`.
Method: four independent adversarial reviews (engine facts, system design, pure-Go feasibility, end-to-end tool dry run), then primary-source verification of every load-bearing claim (Roblox creator-docs YAML, Roblox's own `studio-rust-mcp-server` plugin source, library sources).
Owner constraint applied: pure Go, single static binary, cgo-free, developed on an Apple Silicon Mac.

Verdict: feasible, but the plan's riskiest layer rests on a false premise, three load-bearing designs are unimplementable as written, and the tool surface cannot support autonomous verification. Fix the critical items before writing Phase 1 code.

## Critical

C1. Playtest control premise is false.
Docs say "Luau plugin has no API to trigger Play Solo" and route F5 / Shift+F5 through Win32 SendInput.
Fact: `StudioTestService:ExecutePlayModeAsync(args)` and `ExecuteRunModeAsync(args)` (PluginSecurity) start a Play or Run session from the plugin, yield until it ends, and return the value passed to `StudioTestService:EndTest(value)`. `GetTestArgs()` reads the args inside the session. Roblox's own MCP plugin (`plugin/src/Tools/StartStopPlay.luau`, `RunScriptInPlayMode.luau`) does exactly this.
Consequence: no window focus, no foreground lock, no UIPI, no F5. The Win32 layer shrinks to capture and optional input.
Fix: replace `start_playtest` / `stop_playtest` with `run_playtest(code, mode, timeout_s)`: inject a Script into ServerScriptService, start the session, the script captures `LogService.MessageOut` in the server DataModel and calls `EndTest({success, value, logs, errors, duration})`, the plugin returns that table. This also gives the AI play-mode state assertions (see H6). Gotcha from Roblox's code: `ExecutePlayModeAsync` may error "Previous call to start play session has not been completed", handle it.

C2. Token handoff is unimplementable.
"Generates a randomized session token on startup; the Luau plugin requires this token." The plugin has no file or env access, so it can never learn a per-startup token.
Fix: pairing. Binary prints a short code once, user enters it in a plugin widget, `/pair` returns a long-lived token stored with `plugin:SetSetting`, sent in a header (never `?token=`). Note Roblox's own plugin uses no auth at all on `localhost:44755`; with `execute_luau` exposed, auth is worth keeping.

C3. Fixed port 58741 breaks with more than one MCP client.
Claude Code, Cursor and Claude Desktop each spawn their own stdio binary. All bind 58741, the second fails, and one plugin serves one bridge. Multiple Studio windows are unaddressed.
Fix: first instance owns the port, later instances detect it and proxy over loopback (or a named pipe). Plugin registers a session id per Studio window. Port range scan from the plugin is the fallback.

C4. Play-mode DataModel model is wrong and unspecified.
Docs say the edit-mode plugin "loses context". Reality: plugin code loads separately in the edit, server and client DataModels. Roblox's plugin checks `getDataModelType()` and returns early when `RunService:IsRunning()`, so only the edit instance polls. The plan has no such gating, so pollers duplicate, jobs run twice, logs come from the wrong DataModel.
Fix: poll loop only when `RunService:IsEdit()`. Play-mode work goes through the injected test script and `EndTest` (C1). If a persistent play-side channel is wanted, each instance registers with a role and jobs carry a target role.

## High

H1. Viewport capture design likely returns black frames. Window-DC `BitBlt` on a GPU-rendered Qt window commonly yields black or stale pixels; a screen-DC capture includes occluding windows; a DPI-unaware process gets wrong rects; cropping to the 3D viewport needs a source the plan lacks. `kbinani/screenshot` captures screen rectangles only and its darwin backend is cgo (verified in `darwin.go`), so drop it. Spike `PrintWindow(PW_RENDERFULLCONTENT)` vs screen-DC `BitBlt` of the `ClientToScreen` rect vs Windows.Graphics.Capture on real Studio. Call `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)`. Downscale to about 1568 px long edge, return MCP image content, not base64 text.

H2. `save_place(file_path)` has no API behind it. No plugin method saves a place to a path. Ctrl+S only works on an already-saved place; an untitled place opens a modal Save As. Drop `file_path`, keep Ctrl+S only when the window title shows a saved place, or drop the tool.

H3. Localhost bridge is attackable from the browser. Any page can `fetch` 127.0.0.1:58741 (plus DNS rebinding). With `execute_luau` behind it that is drive-by arbitrary Luau in the open place. `/status` is unauthenticated. Validate the Host header, reject any request with an Origin header, send no CORS headers, require a custom auth header, add token scopes (read, mutate, exec, host) with exec off by default.

H4. Dotted-path addressing is ambiguous. Twenty sibling `Stage` models, names with dots or spaces, instances destroyed between calls. No create tool returns a path or id. Plugin keeps a weak Instance-to-id table, every create returns `{path, id}`, paths accepted as segment arrays with sibling index, `STALE_REF` error for dead refs.

H5. Job semantics are undefined. No per-job timeout, cancellation, ordering, or at-most-once rule. A blocking `execute_luau` freezes Studio. A reconnect mid-job leaves the result unknown and a retry duplicates edits. No size caps: `get_instance_tree` with properties on a real place emits megabytes. Job envelope `{id, seq, deadline}`, serial FIFO per session, mutations inside ChangeHistory recordings, mid-job disconnect yields UNKNOWN and never replays, defaults depth 2 with `max_nodes`, `max_bytes` and a continuation cursor.

H6. Read side is missing, so autonomous verification is impossible. No `read_script` (`manage_script` is write-only), `get_properties`, `find_instances` (class, tag, name, attribute), `get_selection`, script diagnostics. `get_console_logs` has no schema, cursor or wait. `check_studio_status` returns nothing structured. Before C1, no tool could read server or client state during play. Add the read tools, make logs `(since_seq, level, source, pattern, wait_ms) -> {entries[], next_seq}`, status `{mode, contexts[], session_id}`, and assert play state through `run_playtest`.

H7. Type wire format is underspecified. No Enum, Instance reference (`Attachment0`, `PrimaryPart`, `Adornee`), Content or Rect. Color3 is ambiguous between 0-1 and 0-255. CFrame `rot: [0, 90, 0]` has no Euler order and `CFrame.Angles` takes radians. Partial failures are unreported. One tagged schema (`{"$t":"Enum","v":"Material.Neon"}`, `{"$ref": id}`, CFrame as 12 components) and per-property `{applied, error}` readback.

H8. No plugin build pipeline for pure Go. Five Luau modules plus a `.rbxm`, no builder named; Rojo is Rust. Write a small Go tool that emits `.rbxmx` (XML, Script and ModuleScript items with CDATA Source) from `plugin/src`, `go:embed` it, ship an `install-plugin` subcommand that copies into the Studio Plugins folder. Single bundled `.lua` is the simpler alternative if a spike shows it loads.

H9. Roadmap order is backwards and the Mac dev story is missing. Host automation sits in Phase 4 yet is the riskiest OS-specific work, and `internal/host/*.go` has no build tags so `go build` and `go test` fail on the Mac. Put a `host.Host` interface with `*_windows.go` and a `!windows` stub returning `ErrUnsupported`. Run Phase 0 spikes first (below). Check with `GOOS=windows go vet ./...` on the Mac, smoke on a Windows VM or `windows-latest` CI.

## Medium

M1. HTTP from plugins is misdescribed. Menu path is File > Game Settings > Security. Plugins get a per-domain permission prompt revocable in Plugin Management (Rojo precedent); `HttpEnabled` docs mention only scripts. Document the prompt and handle denial. The 500 req/min budget is shared, so batch `/logs`, never one POST per message. `HttpService:CreateWebStreamClient(WebStreamClientType.WebSocket)` is documented (Studio only, max 4 clients) and would replace long-poll with one push channel. Spike it; Roblox's own plugin still uses 1 s HTTP polling.

M2. Script edit flow. `UpdateSourceAsync(script, function(old) return new end)` takes a callback and works whether the document is open or not, so the open/closed branch is redundant. Wrap every mutating tool in `TryBeginRecording(name)` / `FinishRecording(id, Enum.FinishRecordingOperation.Commit)`, as Roblox's plugin does around every dispatch.

M3. Window discovery. Title is `<Place> - Roblox Studio`, process is `RobloxStudioBeta.exe`, so `FindWindowW(nil, "Roblox Studio")` fails. Use `EnumWindows` plus `QueryFullProcessImageName`, handle several instances. `x/sys/windows` lacks `SendInput`, `BitBlt`, `PrintWindow`, `GetClientRect`, `ClientToScreen`: write `NewLazySystemDLL` wrappers and the 40-byte amd64 `INPUT` struct.

M4. Mock DataStore cannot shadow `DataStoreService`. Generated game code must `require` a wrapper that falls back to the mock; keep the module in ServerStorage so it does not replicate; fix its API (`GetDataStore`, `GetAsync`, `SetAsync`, `UpdateAsync`) and return its path. The "403" description is close enough.

M5. Moving platform fix is incomplete. Unanchored Prismatic needs `SetNetworkOwner(nil)`, servo tuning and an anchored rail. The anchored-part plus `AssemblyLinearVelocity` variant is simpler. Specify `create_physics_joint` properties (axis, limits, speed, actuator, anchored side) and return created paths. `create_moving_platform` vanished between docs.

M6. Doc and repo drift. Tool counts 15, 20, 22; renamed tools across docs; feasibility doc still prescribes Python and uv; `tools/` is a Python hello-world; README lists `cmd/`, `internal/`, `plugin/` that do not exist; the audit says "Python host". Make `tool-suite-spec.md` canonical, generate Go schemas from one registry, delete `tools/`, replace the Python `.gitignore` block with `*.exe`, `dist/`, `coverage.out`.

M7. Positioning vs Roblox's built-in Studio MCP server. Roblox has moved investment from the rust repo to the built-in server, and the feasibility doc misdescribes it as "locked to the Assistant": it is a general MCP server for third-party clients with `run_code`, `start_stop_play`, `run_script_in_play_mode`, `get_console_output`, `insert_model`. Write a short decision record on what this project adds (declarative builders, typed marshaling, screenshots and input, mock DataStore, project-local bridge).

M8. Error model and observability. Never write to stdout besides MCP frames. Define error codes (NOT_CONNECTED, STALE_REF, TIMEOUT, WRONG_MODE, TOO_LARGE, COERCION). Log to stderr or a file with job ids. Console logs in a ring buffer with role tags.

M9. SDK and toolchain. Official `github.com/modelcontextprotocol/go-sdk` is v1.8.0 (2026-09) and `mark3labs/mcp-go` is v1.3.0 (2026-10); both active. Prefer the official SDK. Go 1.23 is out of support; pin 1.25+.

M10. Tool surface. 22 tools strain client tool budgets. Consolidate to about 12-15 action-enum tools. Merge `create_instance` with a children array and `build_ui_tree` into one tree builder with a defined node schema. `simulate_player_input` needs a step schema and world-to-pixel mapping; prefer in-engine `Humanoid:MoveTo` / `PivotTo` through `run_playtest`, keep OS input as a smoke test.

## Low

L1. `InsertService:LoadAsset` returns models only, not sounds or meshes. `UIStroke.ApplyStrokeMode` is Border or Contextual, no "Middle". `execute_luau` chunks lack `script`; `loadstring` works in plugin context (Roblox's `RunCode.luau` uses it); capture prints via `LogService`.
L2. No testing strategy. Add a Go fake-plugin client against `httptest`, golden tests for marshaling, SDK in-memory transport for tool tests, CI on ubuntu and windows-latest.

## Decisions for the owner

1. Play control through `StudioTestService` and a `run_playtest` tool returning the `EndTest` payload. Recommended: yes, drop F5.
2. Positioning against the built-in Studio MCP server (M7).
3. Pairing flow plus token scopes, or no auth like Roblox. Recommended: pairing.
4. Multi-client: broker with proxy, or port range.
5. Transport: HTTP long-poll or WebStreamClient WebSocket (spike first).
6. Reference scheme: ids plus segment-array paths.
7. Consolidate to 12-15 tools and add the read tools.
8. Mac story: build tags plus Windows VM or CI now, macOS host via `ebitengine/purego` later.
9. Plugin packaging: Go `.rbxmx` builder plus `install-plugin`, or single bundled `.lua`.

## Phase 0 spikes (before Phase 1)

In Studio: `ExecutePlayModeAsync` plus `EndTest` round trip with an injected script; `loadstring` output capture; `CreateWebStreamClient(WebSocket)` against a Go server; HTTP permission prompt behavior on localhost.
On a Windows VM: `PrintWindow(PW_RENDERFULLCONTENT)` capture of Studio; whether `SendInput` WASD is honored by the play client.
In Go: port owner plus proxy; official SDK stdio with image content; `.rbxmx` builder.

## Recommended dependencies

`github.com/modelcontextprotocol/go-sdk`, `golang.org/x/sys` (windows), `golang.org/x/image` (draw). Everything else stdlib. Drop `mark3labs/mcp-go`, `kbinani/screenshot`, `robloxapi/rbxfile`.
Build: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/roblox-studio-mcp.exe ./cmd/roblox-studio-mcp`.

## Sources checked

creator-docs `StudioTestService.yaml`, `HttpService.yaml`, `WebStreamClientType.yaml`; `Roblox/studio-rust-mcp-server` `plugin/src/Main.server.luau`, `MockWebSocketService.luau`, `Tools/RunCode.luau`, `Tools/RunScriptInPlayMode.luau`, `Tools/StartStopPlay.luau`, README; `kbinani/screenshot` `windows.go`, `darwin.go`; GitHub release tags for both Go SDKs.

## Resolution

The current docs apply every fix above, with these deviations from the recommendations:

- Tool count is 18, not 12-15. `create_instance` and `build_ui_tree` merged into `build_tree`, `manage_tags` and `manage_attributes` folded into `set_properties`, start, stop and run merged into `playtest`, status, selection and save merged into `studio`. Going lower would hide needed arguments behind more action enums.
- The scripted playtest is `playtest run`, not `run_playtest`. Its payload keys are `ok`, `duration_ms` and `timed_out`.
- Token scopes are deferred. v1 ships one paired token plus Host and Origin checks.
- `get_instance_tree` defaults to depth 1 with `max_nodes` 500.
- The non-Windows host build returns the error code `UNSUPPORTED_HOST`.
- `create_physics_joint` names the rigid joint type `Weld`.
- `insert_asset` stays model-only; sounds and meshes are set through properties.
