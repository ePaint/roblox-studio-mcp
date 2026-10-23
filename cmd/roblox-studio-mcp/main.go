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
	hostFlag := flag.String("host", "127.0.0.1", "HTTP bridge bind host (use 0.0.0.0 for LAN access)")
	portFlag := flag.Int("port", bridge.DefaultPort, "Local HTTP bridge port")
	httpFlag := flag.String("http", "", "Serve MCP over streamable HTTP on this address (e.g. 0.0.0.0:58742) instead of stdio")
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

	// Normal server startup: Start HTTP bridge on <host>:<port>
	bridgeServer := bridge.NewServer(*hostFlag, *portFlag)
	if err := bridgeServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Error starting bridge: %v\n", err)
		os.Exit(1)
	}

	// Always write diagnostic logs to stderr (never stdout, which is reserved for MCP JSON-RPC frames)
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Bridge listening on %s:%d\n", *hostFlag, *portFlag)
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Pairing Code: %s\n", bridgeServer.PairingCode())
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Enter this pairing code in Roblox Studio plugin widget to connect.\n")

	// Initialize MCP Server
	mcpServer := mcp.NewServer(bridgeServer, host.DefaultHost)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := runMCP(ctx, mcpServer, *httpFlag); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] Server error: %v\n", err)
		os.Exit(1)
	}
}

func runMCP(ctx context.Context, mcpServer *mcp.Server, httpAddr string) error {
	if httpAddr == "" {
		return mcpServer.Run(ctx)
	}

	token, err := mcp.LoadOrCreateToken(".mcp_http_token")
	if err != nil {
		return fmt.Errorf("loading HTTP token: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] MCP HTTP endpoint: http://%s/\n", httpAddr)
	fmt.Fprintf(os.Stderr, "[roblox-studio-mcp] MCP HTTP token: %s\n", token)
	return mcpServer.RunHTTP(ctx, httpAddr, token)
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

	paths := []string{
		filepath.Join(pluginDir, "RobloxStudioMCP.lua"),
		filepath.Join(pluginDir, "RobloxStudioMCP.server.luau"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte(plugin.Source), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing plugin file to %s: %v\n", p, err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "Success: Companion plugin installed to %s\n", pluginDir)
	fmt.Fprintf(os.Stderr, "Restart Roblox Studio or reload plugins to activate.\n")
}
