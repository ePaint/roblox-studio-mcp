package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

const (
	DefaultPort = 58741
)

type Server struct {
	addr        string
	port        int
	pairingCode string
	token       string
	paired      bool

	mu          sync.RWMutex
	sessions    map[string]time.Time
	roles       map[string]string // sessionID -> role
	lastSession string

	queue *JobQueue
	logs  *LogBuffer

	httpServer *http.Server
	listener   net.Listener
	isProxy    bool
}

func NewServer(port int) *Server {
	if port <= 0 {
		port = DefaultPort
	}

	// Generate random 6-character pairing code
	codeBytes := make([]byte, 3)
	rand.Read(codeBytes)
	code := fmt.Sprintf("RBX-%04X", (int(codeBytes[0])<<8)|int(codeBytes[1]))

	// Generate 32-byte secret token
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	return &Server{
		addr:        fmt.Sprintf("127.0.0.1:%d", port),
		port:        port,
		pairingCode: code,
		token:       token,
		sessions:    make(map[string]time.Time),
		roles:       make(map[string]string),
		queue:       NewJobQueue(128),
		logs:        NewLogBuffer(5000),
	}
}

func (s *Server) PairingCode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pairingCode
}

func (s *Server) Logs() *LogBuffer {
	return s.logs
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		// Port conflict check: test if an existing instance is running
		resp, checkErr := http.Get(fmt.Sprintf("http://127.0.0.1:%d/status", s.port))
		if checkErr == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			s.isProxy = true
			return nil // Running in client proxy mode
		}
		return fmt.Errorf("failed to bind port %d: %w", s.port, err)
	}

	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/pair", s.handlePair)
	mux.HandleFunc("/poll", s.handlePoll)
	mux.HandleFunc("/result", s.handleResult)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/status", s.handleStatus)

	s.httpServer = &http.Server{
		Handler: s.securityMiddleware(mux),
	}

	go s.httpServer.Serve(ln)
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Browser CSRF defense: reject any request with an Origin header
		if r.Header.Get("Origin") != "" {
			http.Error(w, "Forbidden: browser origin headers rejected", http.StatusForbidden)
			return
		}

		// Host header validation
		host := r.Host
		if !strings.HasPrefix(host, "127.0.0.1") && !strings.HasPrefix(host, "localhost") {
			http.Error(w, "Forbidden: invalid host header", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkAuth(r *http.Request) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.paired {
		return false
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == s.token {
		return true
	}
	if r.Header.Get("X-Roblox-Auth") == s.token {
		return true
	}
	return false
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.EqualFold(strings.TrimSpace(req.Code), s.pairingCode) {
		s.paired = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    true,
			"token": s.token,
		})
		return
	}

	http.Error(w, "Invalid pairing code", http.StatusUnauthorized)
}

func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	role := r.Header.Get("X-DataModel-Role")
	if sessionID == "" {
		sessionID = "default"
	}
	if role == "" {
		role = "edit"
	}

	s.mu.Lock()
	s.sessions[sessionID] = time.Now()
	s.roles[sessionID] = role
	s.lastSession = sessionID
	s.mu.Unlock()

	// Wait up to 5 seconds for a job
	job := s.queue.Next(5 * time.Second)
	if job == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var res types.JobResult
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		http.Error(w, "Invalid result JSON", http.StatusBadRequest)
		return
	}

	s.queue.Complete(&res)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var entries []types.LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		http.Error(w, "Invalid logs JSON", http.StatusBadRequest)
		return
	}

	s.logs.Append(entries)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) GetStatus() types.StudioStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	activeSessions := make([]string, 0, len(s.sessions))
	activeRoles := make([]string, 0, len(s.roles))

	for sid, t := range s.sessions {
		if now.Sub(t) < 15*time.Second {
			activeSessions = append(activeSessions, sid)
			if role, ok := s.roles[sid]; ok {
				activeRoles = append(activeRoles, role)
			}
		}
	}

	mode := "edit"
	for _, r := range activeRoles {
		if r == "server" || r == "play" {
			mode = "play"
			break
		}
	}

	return types.StudioStatus{
		StudioRunning:   len(activeSessions) > 0,
		PluginConnected: len(activeSessions) > 0,
		Paired:          s.paired,
		Mode:            mode,
		Roles:           activeRoles,
		SessionID:       s.lastSession,
		Sessions:        activeSessions,
		PairingCode:     s.pairingCode,
	}
}

func (s *Server) DispatchJob(ctx context.Context, tool string, args any, timeout time.Duration) (*types.JobResult, error) {
	rawArgs, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job args: %w", err)
	}

	jobID := generateID()
	job := &types.Job{
		ID:         jobID,
		Role:       "edit",
		DeadlineMS: time.Now().Add(timeout).UnixMilli(),
		Tool:       tool,
		Args:       rawArgs,
	}

	resChan := s.queue.Enqueue(job)

	select {
	case <-ctx.Done():
		s.queue.Cancel(jobID, types.ErrTimeout, "request context canceled")
		return nil, ctx.Err()
	case <-time.After(timeout):
		s.queue.Cancel(jobID, types.ErrTimeout, fmt.Sprintf("job %s timed out after %v", tool, timeout))
		return nil, fmt.Errorf("job timed out after %v", timeout)
	case res := <-resChan:
		return res, nil
	}
}

func generateID() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}
