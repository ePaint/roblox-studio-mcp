package mcp

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPHandler serves MCP over streamable HTTP, gated by a bearer token.
func (s *Server) HTTPHandler(token string) http.Handler {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcpServer }, nil)
	return requireBearer(token, handler)
}

// RunHTTP serves MCP over streamable HTTP on addr until ctx is cancelled.
func (s *Server) RunHTTP(ctx context.Context, addr, token string) error {
	httpServer := &http.Server{Addr: addr, Handler: s.HTTPHandler(token)}
	go func() {
		<-ctx.Done()
		httpServer.Close()
	}()

	err := httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return ctx.Err()
	}
	return err
}

func requireBearer(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoadOrCreateToken returns the token stored at path, generating and saving one if absent.
func LoadOrCreateToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		return strings.TrimSpace(string(data)), nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return "", err
	}
	return token, nil
}
