# roblox-studio-mcp

Model Context Protocol (MCP) server for Roblox Studio, letting AI coding assistants build, inspect, script and playtest places in real time. It is a pure Go, cgo-free single static binary (Windows x64 primary, macOS secondary) that speaks MCP over stdio and talks to a companion Luau plugin in Studio through a paired localhost bridge. No Python, Node or Rojo is involved.

## Planned project structure

Nothing below exists yet.

- `cmd/`: Go entry points (`roblox-studio-mcp`, `buildplugin`)
- `internal/`: Go packages (MCP server, local HTTP bridge, host layer for Windows)
- `plugin/`: Studio companion plugin source in Luau
- `docs/`: project documentation and specs
- `temp/`: local scratchpad (gitignored)

## Documentation

- [Feasibility Research](docs/research-feasibility.md): how external tools interact with Roblox Studio, available architectures and positioning.
- [Deep Architecture Map](docs/deep-architecture-map.md): Roblox object model, wire format for engine types, UI system, physics, and the tool and Go stack overview.
- [Adversarial Audit](docs/adversarial-audit.md): stress test against real incremental and obby games with engine barrier resolutions.
- [Go Implementation Plan](docs/go-implementation-plan.md): Go architecture, bridge, host layer, phased roadmap and testing.
- [Tool Suite Specification](docs/tool-suite-spec.md): canonical 18-tool schemas in three layers.
- [Plan Review](docs/plan-review.md): the design review that drove the current plan.

## Status

Design phase. No code yet.

## Build

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/roblox-studio-mcp.exe ./cmd/roblox-studio-mcp
go test ./...
GOOS=windows go vet ./...
```
