# roblox-studio-mcp

Model Context Protocol (MCP) server for Roblox Studio, enabling AI coding assistants to interact with Roblox Studio in real time.

## Project Structure

- `cmd/`: Go application entry points (compiled to a single standalone `roblox-studio-mcp.exe`)
- `internal/`: Go core packages (MCP server, local HTTP bridge, Windows host automation)
- `plugin/`: Roblox Studio companion Luau plugin source
- `docs/`: Project documentation and architecture specs
- `temp/`: Local scratchpad directory (gitignored)

## Documentation

- [Feasibility Research](docs/research-feasibility.md): In-depth review of how external tools interact with Roblox Studio, available architectures, and technical feasibility.
- [Deep Architecture Map](docs/deep-architecture-map.md): Comprehensive map of Roblox objects, complex types, Photoshop-level UI system, physics, and MCP tool specifications.
- [Adversarial Audit](docs/adversarial-audit.md): Red-team stress-test against real incremental and jump course (obby) games with engine barrier resolutions.
- [Go Implementation Plan](docs/go-implementation-plan.md): Complete Go architecture, dependencies, Windows host automation, and phased roadmap.
- [Tool Suite Specification](docs/tool-suite-spec.md): Clean two-layer tool design (High-Level Declarative Builders vs Low-Level Escape Hatches).
