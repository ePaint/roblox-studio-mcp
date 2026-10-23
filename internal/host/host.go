package host

import (
	"errors"
	"strings"
)

// Common errors for host operations.
var (
	ErrNotSupported   = errors.New("host operation not supported on this operating system")
	ErrStudioNotFound = errors.New("roblox studio window was not found")
	ErrCannotFocus    = errors.New("could not focus roblox studio window")
)

// WindowInfo describes the detected Roblox Studio window.
type WindowInfo struct {
	HWND   uintptr
	Title  string
	PID    uint32
	Width  int
	Height int
}

// InputStep defines a simulated user interaction step.
type InputStep struct {
	Type string `json:"type"`          // "key", "click", "move", "wait"
	Key  string `json:"key,omitempty"` // e.g. "w", "a", "s", "d", "space"
	X    int    `json:"x,omitempty"`   // client area pixel X
	Y    int    `json:"y,omitempty"`   // client area pixel Y
	MS   int    `json:"ms,omitempty"`  // duration / wait time in milliseconds
}

// Host abstracts OS-level window management, screen capture, and input injection.
type Host interface {
	IsSupported() bool
	FindStudioWindow() (WindowInfo, error)
	CaptureViewport(maxWidth int) ([]byte, error)
	SimulateInput(steps []InputStep) (int, error)
	SavePlace() error
}

// DefaultHost returns the platform-specific Host implementation.
var DefaultHost Host = newPlatformHost()

func isStudioExecutable(path string) bool {
	base := path[strings.LastIndexAny(path, `\/`)+1:]
	return strings.EqualFold(base, "RobloxStudioBeta.exe")
}
