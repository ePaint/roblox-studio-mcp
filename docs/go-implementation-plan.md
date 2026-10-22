# Go Implementation Plan: Roblox Studio MCP

## 1. Overview and Strategy

The project will be built in **Go (Golang)**. 

### Why Go?
* **Single Binary Distribution**: Compiles to a single, zero-dependency static executable (`roblox-studio-mcp.exe`). Developers and AI users do not need Python, Node.js, or any runtime installed.
* **Minimal Footprint**: Startup time under 10 milliseconds and baseline memory usage under 15 MB.
* **Native Concurrency**: Go channels and goroutines natively handle simultaneous `stdio` MCP communication and `net/http` long-polling with Roblox Studio.
* **Direct Windows Host Integration**: Go interfaces cleanly with Windows APIs (`user32.dll`, `gdi32.dll`) for window management, screen capture, and playtest keyboard shortcuts.

---

## 2. Target Project Structure

```
roblox-studio-mcp/
├── cmd/
│   └── roblox-studio-mcp/
│       └── main.go                  # Main application entry point
├── internal/
│   ├── bridge/
│   │   ├── server.go                # HTTP bridge server on 127.0.0.1:58741
│   │   ├── session.go               # Active Studio session tracking and token auth
│   │   └── queue.go                 # Thread-safe job queue and dispatch channels
│   ├── host/
│   │   ├── window.go                # Finds RobloxStudio.exe window handle (HWND)
│   │   ├── capture.go               # Win32 GDI screen capture of Studio viewport
│   │   └── input.go                 # Injects F5/Shift+F5 and player WASD/mouse inputs
│   ├── mcp/
│   │   ├── server.go                # MCP protocol initialization (stdio transport)
│   │   └── tools.go                 # Tool definitions and argument schema validation
│   └── types/
│       ├── roblox.go                # Serialization models (Vector3, CFrame, UDim2)
│       └── payload.go               # Bridge request/response payloads
├── plugin/
│   ├── src/
│   │   ├── init.luau                # Plugin entry point and polling loop
│   │   ├── deserializer.luau        # JSON to native Roblox type deserialization
│   │   ├── script_manager.luau      # ScriptEditorService + ChangeHistoryService
│   │   ├── mock_datastore.luau      # In-memory offline DataStore module
│   │   └── physics_builder.luau     # General constraint and joint generator
│   └── RobloxStudioMcp.rbxm         # Built binary plugin distribution
├── docs/
│   ├── research-feasibility.md      # Initial feasibility study
│   ├── deep-architecture-map.md     # Engine systems and data type mappings
│   ├── adversarial-audit.md         # Red team findings and engine barriers
│   └── go-implementation-plan.md    # This document
├── temp/                            # Gitignored local scratchpad
├── go.mod                           # Go module definition
├── go.sum                           # Go dependencies checksum
├── .gitignore                       # Git ignore rules
└── README.md                        # Project landing page
```

---

## 3. Go Dependencies and Engine APIs

### Go Libraries
* **MCP Protocol SDK**: `github.com/mark3labs/mcp-go` (mature, feature-complete Go SDK for Model Context Protocol over stdio).
* **HTTP Server**: Standard library `net/http` (handles REST endpoints and long-polling).
* **Screen Capture**: `github.com/kbinani/screenshot` or direct Win32 `BitBlt` GDI calls.
* **Windows System Calls**: `golang.org/x/sys/windows` (used for `FindWindowW`, `SetForegroundWindow`, `SendInput`, `GetClientRect`).

### Roblox Engine Services (Luau Plugin)
* `HttpService`: Makes outgoing long-poll requests to `127.0.0.1:58741`.
* `ScriptEditorService`: Updates open script tabs cleanly without text desync.
* `ChangeHistoryService`: Wraps edits in Undo/Redo transactions.
* `CollectionService`: Manages tags (`"KillBrick"`, `"Checkpoint"`).
* `LogService`: Streams console output and error traces back to the bridge.

---

## 4. Architectural Components in Go

### A. The Local HTTP Bridge (`internal/bridge/`)
* Listens on `127.0.0.1:58741`.
* Generates a randomized session token on startup; the Luau plugin requires this token to authenticate.
* **Endpoints**:
  * `GET /poll?token=...`: Long-poll connection held open until an MCP tool call arrives or a heartbeat timeout expires (approx. 5 seconds).
  * `POST /result?token=...`: Luau plugin posts execution results, return values, or error traces back to the waiting Go routine.
  * `POST /logs?token=...`: Streams console messages captured by `LogService.MessageOut`.
  * `GET /status`: Health-check endpoint reporting whether Studio is currently connected.

### B. Windows Host Automation (`internal/host/`)
* **Window Detection (`window.go`)**:
  * Uses `FindWindowW(nil, "Roblox Studio")` to identify the active window handle.
  * Checks if Studio is minimized or focused.
* **Screen Capture (`capture.go`)**:
  * Obtains the device context (DC) of the Studio client area.
  * Captures pixels using `BitBlt`, encodes to PNG in memory, and converts to base64.
  * Exposes the image directly to the MCP client for visual inspection.
* **Input Injection (`input.go`)**:
  * Uses Win32 `SendInput` to dispatch:
    - `VK_F5`: Starts Play Solo.
    - `Shift + VK_F5`: Stops playtesting and returns to edit mode.
    - Simulated WASD, Space, and mouse clicks during playtesting verification.

### C. Luau Companion Plugin (`plugin/src/`)
* **Polling Loop**: Continuously requests jobs from the Go bridge.
* **Execution Safety**: Wraps all operations in `pcall`. On error, captures the stack trace and returns a structured failure payload instead of crashing the plugin.
* **Script Management**: Checks `ScriptEditorService:FindScriptDocument(script)`:
  - If open: updates via `ScriptEditorService:UpdateSourceAsync`.
  - If closed: updates `script.Source` directly.
  - Commits recording with `ChangeHistoryService:FinishRecording`.
* **Mock DataStore**: Injects an in-memory table-backed mock module into `ReplicatedStorage` if `setup_mock_datastores` is activated, unblocking local game testing.

---

## 5. Phased Implementation Roadmap

### Phase 1: Go Foundation and Bridge Skeleton
- Initialize `go.mod` (`roblox-studio-mcp`).
- Build `internal/bridge` (HTTP server, token auth, job queue, polling loop).
- Build the basic Luau plugin template that connects, authenticates, and responds to heartbeats.
- Deliverable: Successful two-way ping between Go binary and Roblox Studio.

### Phase 2: Core Primitives and Type Deserializer
- Implement `execute_luau`, `create_instance`, `set_properties`, `delete_instance`, `get_instance_tree`, and `call_method`.
- Build the Luau deserializer for `Vector3`, `CFrame`, `UDim2`, `Color3`, `ColorSequence`, `NumberSequence`, and enums.
- Integrate `manage_tags` (`CollectionService`) and `manage_attributes`.
- Deliverable: Full static building, property modification, and low-level escape-hatch primitives from AI prompts.

### Phase 3: High-Level Declarative Builders & Scripting
- Implement `manage_script` using `ScriptEditorService` and `ChangeHistoryService`.
- Implement `build_ui_tree` with auto-layout and responsive scale constraints.
- Implement `create_physics_joint` for generalized mechanical constraints (Prismatic, Hinge, Weld, Spring, AlignPosition).
- Implement `configure_dev_environment` for offline in-memory MockDataStores.
- Deliverable: Complete scripting, responsive UI, physics assemblies, and offline development capabilities.

### Phase 4: Windows Host Automation & Testing Loop
- Implement `internal/host/window.go` and `internal/host/capture.go` (viewport screenshot).
- Implement `start_playtest` and `stop_playtest` via Win32 `SendInput` (`F5`/`Shift+F5`).
- Implement `simulate_player_input` for character movement testing.
- Implement `get_console_logs` via `LogService` listener.
- Deliverable: Autonomous build, test, and self-debugging loop.

### Phase 5: Verification & Standalone Packaging
- Test full autonomous generation of an Incremental/Clicker game and an Obby game using the clean two-layer toolset.
- Package `RobloxStudioMcp.rbxm` plugin file and compile standalone `roblox-studio-mcp.exe`.
- Deliverable: Verified full game creation capabilities and production release.
