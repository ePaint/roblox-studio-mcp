# Feasibility Research: Roblox Studio MCP

## Executive Summary

Yes, building a Model Context Protocol (MCP) server for Roblox Studio is completely possible. In fact, multiple implementations exist in the community, and Roblox itself has added official MCP support directly into Roblox Studio.

This document breaks down how it works, the architectural options available, and the recommended design for this project.

---

## How It Works

Roblox Studio runs Lua/Luau scripts in a sandboxed environment. By design, Studio plugins cannot listen for incoming network connections as a server. However, Studio plugins have access to `HttpService`, which allows them to make outgoing HTTP requests to `localhost`.

Because of this, external communication follows a well-tested design pattern:

1. **AI Client (Cursor, Claude Desktop, Antigravity, etc.)** talks to the Python MCP server via standard input/output (`stdio`).
2. **Python MCP Server** translates tool calls into jobs and runs a lightweight local HTTP server on `127.0.0.1` (for example, on port 58741).
3. **Roblox Studio Companion Plugin** runs inside Studio and polls the local HTTP server using `HttpService:RequestAsync`.
4. **Execution & Response**: When a command arrives, the plugin executes it inside the Roblox DataModel (creating objects, editing scripts, reading hierarchy, or running Luau code) and sends the response back via HTTP POST.
5. **Result**: The MCP server forwards the result back to the AI client over `stdio`.

---

## Comparison of Approaches

### 1. Local HTTP Bridge + Companion Plugin (Recommended for this project)
* **Status**: Proven pattern used by `Roblox/studio-rust-mcp-server` and community projects like `Chrrxs/robloxstudio-mcp`.
* **Language**: Python (using `uv` and FastMCP / mcp) + Luau plugin.
* **Capabilities**:
  * Inspect the DataModel hierarchy and object properties.
  * Read, create, and update Luau scripts.
  * Run code in edit mode or playtest mode.
  * Capture console logs, errors, and memory metrics.
  * Insert parts, models, and assets.
* **Requirements**: "Allow HTTP Requests" must be enabled in Roblox Studio (Game Settings > Security).

### 2. Roblox Studio Built-in MCP Server
* **Status**: Native feature in modern Roblox Studio builds.
* **How to enable**: Enabled through Studio Assistant Settings > MCP Servers.
* **Pros**: No separate plugin installation required; maintained directly by Roblox.
* **Limitations**: Locked to Roblox Assistant feature set; difficult to customize with custom pipelines, local file bridges, or specialized developer workflows.

### 3. Roblox Open Cloud APIs
* **Status**: Official cloud REST APIs.
* **Capabilities**: Place publishing, DataStore management, asset uploads, and cloud Luau task execution.
* **Limitations**: Operates in the cloud, not inside an active local Studio editor session. Great as an optional addition, but cannot inspect an unsaved local viewport or trigger playtests in an open Studio window.

---

## Recommended Architecture for `roblox-studio-mcp`

To give full flexibility, reliability, and ease of use:

1. **Python MCP Server (`tools/`)**:
   * Managed with `uv`.
   * Implements the MCP protocol (`mcp` library).
   * Runs a local background HTTP bridge server (FastAPI or standard asyncio HTTP server) on `127.0.0.1:58741`.
   * Secures communication using a local shared secret token.

2. **Roblox Studio Plugin (`src/plugin/` or `plugin/`)**:
   * A single Luau script or `.rbxm` plugin file.
   * Can be placed into the user's local Roblox Studio `Plugins` folder.
   * Connects to the local Python bridge and exposes execution hooks safely.

3. **Core Toolset to Implement**:
   * `get_hierarchy`: Query instances in Workspace, ServerScriptService, ReplicatedStorage, etc.
   * `read_script`: Fetch the source code of any Script, LocalScript, or ModuleScript.
   * `write_script`: Update or create Luau scripts.
   * `execute_luau`: Run Luau code in edit mode.
   * `get_output_logs`: Collect recent Studio output and error messages.

---

## Conclusion

The project is fully feasible and has a clear, proven technical pathway. We can proceed with structuring the repository and building the Python MCP server and companion Studio plugin.
