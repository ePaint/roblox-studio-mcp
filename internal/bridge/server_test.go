package bridge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

func TestServer_SecurityAndPairing(t *testing.T) {
	srv := NewServer("127.0.0.1", 0)

	// Test 1: Reject request with Origin header (CSRF defense)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:58741/status", nil)
	req.Header.Set("Origin", "http://evil-website.com")
	w := httptest.NewRecorder()

	handler := srv.securityMiddleware(http.HandlerFunc(srv.handleStatus))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on Origin header, got %d", w.Code)
	}

	// Test 2: Pairing with invalid code
	pairBody, _ := json.Marshal(map[string]string{"code": "WRONG-CODE"})
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:58741/pair", bytes.NewReader(pairBody))
	w = httptest.NewRecorder()
	srv.handlePair(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
	}

	// Test 3: Pairing with valid code
	pairBody, _ = json.Marshal(map[string]string{"code": srv.PairingCode()})
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:58741/pair", bytes.NewReader(pairBody))
	w = httptest.NewRecorder()
	srv.handlePair(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid pair, got %d", w.Code)
	}

	var pairResp struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	json.NewDecoder(w.Body).Decode(&pairResp)
	if !pairResp.OK || pairResp.Token == "" {
		t.Fatalf("expected token returned, got %+v", pairResp)
	}

	// Test 4: Check status endpoint
	req = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:58741/status", nil)
	w = httptest.NewRecorder()
	srv.handleStatus(w, req)

	var status types.StudioStatus
	json.NewDecoder(w.Body).Decode(&status)
	if !status.Paired {
		t.Fatalf("expected status to be paired, got %+v", status)
	}

	// Test 5: Localhost host rejection when bound to 127.0.0.1
	req = httptest.NewRequest(http.MethodGet, "http://192.168.1.100:58741/status", nil)
	w = httptest.NewRecorder()
	handler = srv.securityMiddleware(http.HandlerFunc(srv.handleStatus))
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for external host on 127.0.0.1 binding, got %d", w.Code)
	}

	// Test 6: LAN host allowed when bound to 0.0.0.0
	lanSrv := NewServer("0.0.0.0", 0)
	lanReq := httptest.NewRequest(http.MethodGet, "http://192.168.1.100:58741/status", nil)
	lanW := httptest.NewRecorder()
	lanHandler := lanSrv.securityMiddleware(http.HandlerFunc(lanSrv.handleStatus))
	lanHandler.ServeHTTP(lanW, lanReq)
	if lanW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for external host on 0.0.0.0 binding, got %d", lanW.Code)
	}
}
