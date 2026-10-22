# Feasibility Research: Roblox Studio MCP

## Executive Summary

Building a Model Context Protocol (MCP) server for Roblox Studio is feasible. Community implementations exist, and Roblox ships its own built-in Studio MCP server.

This document covers how the link between an AI client and Studio works, the architectural options, and the design chosen for this project: a single static Go binary plus a Luau companion plugin.

---

## How It Works

Studio plugins run Luau in a sandbox and cannot listen for incoming connections. They can make outgoing requests with `HttpService`, including to `127.0.0.1`.

1. **AI client (Cursor, Claude Desktop, Claude Code, etc.)** talks to the Go binary over MCP on standard input/output (`stdio`).
2. **The Go binary** turns tool calls into jobs and runs a local HTTP bridge on `127.0.0.1` (default port `58741`). If another instance already owns the port, the new binary proxies its tool calls to the owner over loopback.
3. **The Studio plugin** polls the bridge with `HttpService:RequestAsync` (long-poll, `GET /poll` held up to 5 s) and posts results to `POST /result` and console output to `POST /logs`.
4. **Execution**: the plugin runs each job against the DataModel (build instances, edit scripts, query the tree, run Luau, control playtests) and posts the result back.
5. **Result**: the binary returns the result to the AI client over `stdio`.

### HTTP facts

* Plugins trigger a per-domain permission prompt the first time they request a host. It can be revoked under Plugin Management (HTTP permissions). The plugin must detect denial and tell the user.
* The experience setting "Allow HTTP Requests" is documented for game scripts. It is not stated as a requirement for plugins.
* `HttpService` allows 500 requests per minute, shared by every call. Console output is batched (every 250 ms or 50 entries) instead of one POST per message, and the long-poll holds the connection instead of polling quickly.
* `HttpService:CreateWebStreamClient(Enum.WebStreamClientType.WebSocket, options)` is documented (Studio only, at most 4 clients). If a Phase 0 spike shows it works against the Go bridge, it replaces long-poll in v1.1.

### Other facts the design relies on

* Plugin code loads in the edit, server and client DataModels, so the plugin assigns itself a role at startup.
* A plugin can start and stop Play and Run sessions through `StudioTestService:ExecutePlayModeAsync` and `ExecuteRunModeAsync`, and read a result passed to `EndTest`. No keyboard automation is needed for playtests.
* Pixels and OS input are not reachable from Luau. Viewport capture, simulated input and save-by-shortcut belong to a host layer in the Go binary (Windows first).

---

## Comparison of Approaches

### 1. Local HTTP Bridge + Companion Plugin (Recommended for this project)
* **Status**: Proven pattern used by `Roblox/studio-rust-mcp-server` and community projects such as `Chrrxs/robloxstudio-mcp`.
* **Language**: Go (official `modelcontextprotocol/go-sdk`, stdlib HTTP) plus a Luau plugin.
* **Capabilities**:
  * Inspect the DataModel and instance properties.
  * Build instance trees, read and write Luau scripts.
  * Run code in edit mode or in a playtest.
  * Collect console logs and errors.
  * Insert models, edit terrain, lighting and physics joints.
  * Capture the viewport and send input (Windows host layer).
* **Requirements**: approve the plugin's `localhost` HTTP permission prompt, then pair the plugin with the bridge using the code the binary prints.

### 2. Roblox Studio Built-in MCP Server
* **Status**: Native feature of current Studio builds, enabled in Studio's MCP settings.
* **What it is**: A general MCP server usable by third-party clients, not only Roblox's Assistant. Its tools are `run_code`, `insert_model`, `start_stop_play`, `run_script_in_play_mode` and `get_console_output`.
* **Pros**: No plugin install, maintained by Roblox.
* **Limitations**: A small, general tool set. No declarative tree builder, typed value codec, scene queries, undo-safe script management, mock DataStore, screenshots or input.

### 3. Roblox Open Cloud APIs
* **Status**: Official cloud REST APIs.
* **Capabilities**: Place publishing, DataStore management, asset uploads, cloud Luau task execution.
* **Limitations**: Operates in the cloud, not inside an open local Studio session. It cannot inspect an unsaved local viewport or run a playtest in an open Studio window. Useful as an optional addition.

---

## Why This Project Exists

Roblox has moved investment from `Roblox/studio-rust-mcp-server` to the built-in Studio MCP server, so a plain bridge that exposes the same five operations adds nothing. This project exists for what the built-in server does not provide:

* Declarative builders (`build_tree`, `create_physics_joint`, `configure_environment`, `modify_terrain`).
* A typed codec for Roblox datatypes, with per-property results.
* Scene queries with stable instance ids and size caps.
* Undo-safe script management through `ScriptEditorService` and `ChangeHistoryService`.
* A mock DataStore for offline play.
* Viewport screenshots and simulated input on Windows.
* A paired local bridge (pairing code, long-lived token, Host and Origin checks).
* One static, cgo-free binary with no Python, Node or Rojo dependency.

If the built-in server grows to cover these, the decision should be revisited.

---

## Recommended Architecture for `roblox-studio-mcp`

1. **Go binary (`cmd/roblox-studio-mcp`)**:
   * Single static executable, built with `CGO_ENABLED=0`; Windows x64 primary, macOS secondary.
   * MCP over `stdio` through the official Go SDK. Stdout carries only MCP frames; logs go to stderr.
   * Local HTTP bridge on `127.0.0.1:58741` with endpoints `/poll`, `/result`, `/logs`, `/pair`, `/status` and `/rpc`.
   * Pairing instead of a shared secret: the binary prints a 6-digit code, the user enters it in the plugin widget, and `/pair` returns a long-lived token sent in the `X-RSMCP-Token` header.
   * `internal/host` wraps Win32 for capture, input and save; non-Windows builds return `UNSUPPORTED_HOST`.

2. **Roblox Studio plugin (`plugin/`)**:
   * Luau modules built into one `.rbxmx` by a Go tool, embedded in the binary and installed with `roblox-studio-mcp install-plugin`.
   * Edit instance runs the main poll loop. Server instance polls for `stop` and server-context jobs. Client instances stay inactive.

3. **Core toolset**: 18 tools in three layers (declarative builders, primitives, playtest and host). `docs/tool-suite-spec.md` is the canonical specification for names, arguments and results.

---

## Conclusion

The project is feasible and has a clear technical path. The remaining risks (WebSocket transport, Windows capture and input, the plugin HTTP prompt) are covered by Phase 0 spikes before production code is written. Next step: run those spikes, then build the Go bridge and the companion plugin.
