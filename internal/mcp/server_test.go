package mcp

import (
	"testing"

	"github.com/ePaint/roblox-studio-mcp/internal/bridge"
	"github.com/ePaint/roblox-studio-mcp/internal/host"
)

func TestServer_Initialization(t *testing.T) {
	b := bridge.NewServer("127.0.0.1", 0)
	h := host.DefaultHost

	srv := NewServer(b, h)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.mcpServer == nil {
		t.Fatal("expected non-nil inner mcpServer")
	}
}
