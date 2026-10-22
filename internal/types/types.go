package types

import "encoding/json"

// Error codes defined in docs/tool-suite-spec.md
const (
	ErrNotConnected    = "NOT_CONNECTED"
	ErrNotPaired       = "NOT_PAIRED"
	ErrStaleRef        = "STALE_REF"
	ErrTimeout         = "TIMEOUT"
	ErrWrongMode       = "WRONG_MODE"
	ErrTooLarge        = "TOO_LARGE"
	ErrCoercion        = "COERCION"
	ErrUnsupportedHost = "UNSUPPORTED_HOST"
	ErrLuauError       = "LUAU_ERROR"
	ErrUnknown         = "UNKNOWN"
	ErrNeedsManualSave = "NEEDS_MANUAL_SAVE"
)

// ErrorInfo represents an error payload returned by a tool or plugin execution.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Job represents an execution request dispatched from the Go bridge to the Luau plugin.
type Job struct {
	ID         string          `json:"id"`
	Seq        int64           `json:"seq"`
	Role       string          `json:"role,omitempty"` // "edit" or "server"
	DeadlineMS int64           `json:"deadline_ms"`
	Tool       string          `json:"tool"`
	Args       json.RawMessage `json:"args"`
}

// JobResult represents the execution outcome returned by the Luau plugin.
type JobResult struct {
	JobID      string          `json:"job_id"`
	OK         bool            `json:"ok"`
	Value      json.RawMessage `json:"value,omitempty"`
	Error      *ErrorInfo      `json:"error,omitempty"`
	Logs       []LogEntry      `json:"logs,omitempty"`
	DurationMS int64           `json:"duration_ms,omitempty"`
}

// LogEntry represents a single console message captured from LogService.MessageOut.
type LogEntry struct {
	Seq     int64  `json:"seq"`
	TS      int64  `json:"ts"`
	Level   string `json:"level"`  // "output", "info", "warning", "error"
	Source  string `json:"source"` // "edit", "server", "client"
	Message string `json:"message"`
}

// StudioStatus represents the state of Studio and the plugin connection.
type StudioStatus struct {
	StudioRunning   bool     `json:"studio_running"`
	PluginConnected bool     `json:"plugin_connected"`
	Paired          bool     `json:"paired"`
	Mode            string   `json:"mode"` // "edit", "play", "run", "unknown"
	Roles           []string `json:"roles"`
	SessionID       string   `json:"session_id,omitempty"`
	Sessions        []string `json:"sessions,omitempty"`
	PairingCode     string   `json:"pairing_code,omitempty"`
}

// TargetRef represents an instance reference: id, dotted path, or segment array.
type TargetRef any

// InstanceRefResult is returned when an instance is created or queried.
type InstanceRefResult struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
