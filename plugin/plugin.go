package plugin

import _ "embed"

// Source contains the embedded Luau source code for the Roblox Studio MCP companion plugin.
//
//go:embed src/init.luau
var Source string
