# roblox-studio-mcp

Model Context Protocol (MCP) server for Roblox Studio, letting AI coding assistants build, inspect, script and playtest places in real time. It is a pure Go, cgo-free single static binary (Windows x64 primary, macOS secondary) that speaks MCP over stdio and talks to a companion Luau plugin in Studio through a paired localhost bridge. No Python, Node or Rojo is involved.

## Project Structure

- `cmd/`: Go entry points (`roblox-studio-mcp`)
- `internal/`: Go packages (MCP server, local HTTP bridge, host layer for Windows)
- `plugin/`: Studio companion plugin source in Luau
- `docs/`: Project documentation and architecture specs
- `maps/`: Local Roblox place files (gitignored)
- `temp/`: Local scratchpad (gitignored)

## Documentation

- [Feasibility Research](docs/research-feasibility.md): how external tools interact with Roblox Studio, available architectures and positioning.
- [Deep Architecture Map](docs/deep-architecture-map.md): Roblox object model, wire format for engine types, UI system, physics, and the tool and Go stack overview.
- [Adversarial Audit](docs/adversarial-audit.md): stress test against real incremental and obby games with engine barrier resolutions.
- [Go Implementation Plan](docs/go-implementation-plan.md): Go architecture, bridge, host layer, phased roadmap and testing.
- [Tool Suite Specification](docs/tool-suite-spec.md): canonical 18-tool schemas in three layers.
- [Plan Review](docs/plan-review.md): the design review that drove the current plan.
- [Incremental Game Plan](docs/incremental-game-plan.md): detailed architecture and phases for the first demo game.
- [Incremental Game Task Spec](docs/incremental-game-task-spec.md): complete step-by-step handover and code specification for agent implementation.

## Status

Core implementation complete and operational. Live pairing and bridge communication with Roblox Studio verified.

## Build

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/roblox-studio-mcp.exe ./cmd/roblox-studio-mcp
go test ./...
GOOS=windows go vet ./...
```
