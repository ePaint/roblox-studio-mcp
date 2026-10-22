package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ePaint/roblox-studio-mcp/internal/bridge"
	"github.com/ePaint/roblox-studio-mcp/internal/host"
	"github.com/ePaint/roblox-studio-mcp/internal/mcp"
	"github.com/ePaint/roblox-studio-mcp/plugin"
)

const version = "0.1.0"

func main() {
	portFlag := flag.Int("port", bridge.DefaultPort, "Local HTTP bridge port")
	installFlag := flag.Bool("install-plugin", false, "Install the companion plugin to Roblox Studio's local Plugins directory")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Fprintf(os.Stderr, "roblox-studio-mcp version %s\n", version)
		os.Exit(0)
	}

	if *installFlag {
		installPlugin()
		os.Exit(0)
	}

	// Normal server startup: Start HTTP bridge on 127.0.0.1:<port>
	bridgeServer := bridge.NewServer(*portFlag)
	if err := bridgeServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Error starting bridge: %v\n", err)
		os.Exit(1)
	}

	// Always write diagnostic logs to stderr (never stdout, which is reserved for MCP JSON-RPC frames)
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Bridge listening on 127.0.0.1:%d\n", *portFlag)
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Pairing Code: %s\n", bridgeServer.PairingCode())
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Enter this pairing code in Roblox Studio plugin widget to connect.\n")

	// Initialize MCP Server
	mcpServer := mcp.NewServer(bridgeServer, host.DefaultHost)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := mcpServer.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Server error: %v\n", err)
		os.Exit(1)
	}
}

func installPlugin() {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		fmt.Fprintf(os.Stderr, "Error: LOCALAPPDATA environment variable is not set\n")
		os.Exit(1)
	}

	pluginDir := filepath.Join(localAppData, "Roblox", "Plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating plugin directory: %v\n", err)
		os.Exit(1)
	}

	targetPath := filepath.Join(pluginDir, "RobloxStudioMCP.server.luau")
	if err := os.WriteFile(targetPath, []byte(plugin.Source), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing plugin file: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Success: Companion plugin installed to %s\n", targetPath)
	fmt.Fprintf(os.Stderr, "Restart Roblox Studio or reload plugins to activate.\n")
}
