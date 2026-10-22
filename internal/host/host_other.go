//go:build !windows

package host

type otherHost struct{}

func newPlatformHost() Host {
	return &otherHost{}
}

func (h *otherHost) IsSupported() bool {
	return false
}

func (h *otherHost) FindStudioWindow() (WindowInfo, error) {
	return WindowInfo{}, ErrNotSupported
}

func (h *otherHost) CaptureViewport(maxWidth int) ([]byte, error) {
	return nil, ErrNotSupported
}

func (h *otherHost) SimulateInput(steps []InputStep) (int, error) {
	return 0, ErrNotSupported
}

func (h *otherHost) SavePlace() error {
	return ErrNotSupported
}
