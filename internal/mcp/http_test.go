package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ePaint/roblox-studio-mcp/internal/bridge"
	"github.com/ePaint/roblox-studio-mcp/internal/host"
)

type bearerTransport struct {
	token string
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(r)
}

func newHTTPTestServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	srv := NewServer(bridge.NewServer("127.0.0.1", 0), host.DefaultHost)
	ts := httptest.NewServer(srv.HTTPHandler(token))
	t.Cleanup(ts.Close)
	return ts
}

func TestHTTPHandler_RejectsMissingToken(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")

	resp, err := http.Post(ts.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHTTPHandler_RejectsWrongToken(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	transport := &sdk.StreamableClientTransport{
		Endpoint:   ts.URL,
		HTTPClient: &http.Client{Transport: bearerTransport{token: "wrong"}},
		MaxRetries: -1,
	}

	session, err := client.Connect(context.Background(), transport, nil)
	if err == nil {
		session.Close()
		t.Fatal("expected connect to fail with wrong token")
	}
}

func TestHTTPHandler_ServesToolsWithValidToken(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	transport := &sdk.StreamableClientTransport{
		Endpoint:   ts.URL,
		HTTPClient: &http.Client{Transport: bearerTransport{token: "secret"}},
	}

	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"capture_viewport", "studio", "execute_luau"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestLoadOrCreateToken_PersistsAcrossCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")

	first, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if len(first) < 32 {
		t.Fatalf("token too short: %q", first)
	}

	second, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if first != second {
		t.Fatalf("token changed between calls: %q vs %q", first, second)
	}
}

func TestLoadOrCreateToken_UsesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  preset-token\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "preset-token" {
		t.Fatalf("expected preset-token, got %q", got)
	}
}
