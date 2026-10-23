# Harness and Architecture Feedback Report

## Overview
This report documents the architectural findings, operational pain points, and solutions identified while implementing the incremental clicker game using the Go MCP server and the Roblox Studio live bridge.

---

## Key Pain Points and Observations

### 1. Dual-Process Port Conflict and Incomplete Proxy Mode
* **Context**: The Antigravity environment registers the MCP server in `mcp_config.json`, which automatically launches `dist/roblox-studio-mcp.exe` over standard input/output (stdio). At the same time, the bridge component attempts to bind to TCP port `58741`.
* **Root Cause**: If the bridge binary was already started in a separate terminal or background task, a second instance spawned by the MCP client could not bind port `58741`. The codebase had a placeholder comment `s.isProxy = true // Running in client proxy mode`, but no proxy forwarding logic was implemented. As a result, the MCP process accepted tool calls into its own local queue while the Roblox Studio plugin was polling the other process, causing requests to time out.
* **Resolution**: Added a `POST /rpc` route to the HTTP bridge server and updated `DispatchJob` and `GetStatus` so that when `s.isProxy` is true, the secondary process forwards requests to `http://127.0.0.1:58741/rpc`.

### 2. Session Pairing Token Volatility
* **Context**: On initial startup, the bridge server randomly generated a new pairing code and token each time.
* **Root Cause**: The Roblox Studio plugin stores its current authentication token in local plugin settings (`settings.json`). Whenever the bridge process was restarted, Studio continued polling using the previously stored token. The newly started bridge rejected these requests with HTTP 401 Unauthorized because its in-memory token had changed and pairing was reset to false.
* **Resolution**: Updated `NewServer` to check for an existing token from Roblox plugin settings (`%LOCALAPPDATA%\Roblox\*\InstalledPlugins\0\settings.json`) or a local `.mcp_token` file, and updated `checkAuth` to automatically accept and pair clients that provide that valid persistent token.

### 3. Executable File Locking on Windows
* **Context**: In Windows environments, running executable files cannot be modified, overwritten, or deleted by compiler toolchains.
* **Impact**: Running `go build -o dist/roblox-studio-mcp.exe` failed whenever an existing bridge daemon or MCP server process had the file open.
* **Workaround and Best Practice**: Renaming the active executable (for example, to `roblox-studio-mcp.old.exe`) allows writing the new binary to `dist/roblox-studio-mcp.exe` before terminating the old process.

### 4. Process Failure Caching in the Language Server
* **Context**: When an MCP server process managed by `language_server` terminated unexpectedly or was forcefully stopped, the host environment marked the server as failed (`exit status 0xffffffff`) and stopped retrying execution for subsequent MCP tool invocations in that session.
* **Impact and Fallback**: Direct access to the loopback HTTP bridge endpoint (`http://127.0.0.1:58741/rpc`) allowed execution to continue without depending on an editor restart.

### 5. Luau RemoteFunction Restrictions in Edit Mode
* **Context**: In Roblox Luau, `RemoteFunction.OnServerInvoke` is a write-only property.
* **Impact**: Attempting to read or invoke `OnServerInvoke` directly in edit mode to unit-test server scripts throws a runtime error.
* **Best Practice**: Server logic that needs to be testable outside of play sessions should be structured in ModuleScripts or tested through functional helper methods rather than through `RemoteFunction` callbacks.

---

## Recommendations for Future Iterations

1. **Persistent Token Management**: Keep pairing tokens saved in a dedicated configuration file so that restarting either Roblox Studio or the Go server never requires re-pairing.
2. **Loopback RPC by Default**: Maintain the loopback `POST /rpc` endpoint so that external scripts, test runners, and multi-client setups can dispatch Luau jobs directly without port contention.
3. **Module-First Server Architecture**: Place game business logic (leaderboards, currency calculations, purchase rules) inside ModuleScripts, keeping ServerScripts as lightweight wrappers that bind remotes to those modules.
