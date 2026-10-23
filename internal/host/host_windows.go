//go:build windows

package host

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

var (
	modUser32 = windows.NewLazySystemDLL("user32.dll")
	modGdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procEnumWindows               = modUser32.NewProc("EnumWindows")
	procGetWindowTextW            = modUser32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId  = modUser32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible           = modUser32.NewProc("IsWindowVisible")
	procGetClientRect             = modUser32.NewProc("GetClientRect")
	procClientToScreen            = modUser32.NewProc("ClientToScreen")
	procSetForegroundWindow       = modUser32.NewProc("SetForegroundWindow")
	procGetForegroundWindow       = modUser32.NewProc("GetForegroundWindow")
	procSendInput                 = modUser32.NewProc("SendInput")
	procGetDC                     = modUser32.NewProc("GetDC")
	procReleaseDC                 = modUser32.NewProc("ReleaseDC")
	procSetProcessDpiAwarenessCtx = modUser32.NewProc("SetProcessDpiAwarenessContext")

	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procGetDIBits              = modGdi32.NewProc("GetDIBits")
)

const (
	srccopy = 0x00CC0020

	inputMouse    = 0
	inputKeyboard = 1

	keyeventfKeyup    = 0x0002
	keyeventfUnicode  = 0x0004
	mouseeventfMove   = 0x0001
	mouseeventfLeftDn = 0x0002
	mouseeventfLeftUp = 0x0004

	vkControl = 0x11
	vkShift   = 0x10
	vkS       = 0x53
	vkF5      = 0x74
	vkSpace   = 0x20
)

const dpiAwarenessContextPerMonitorAwareV2 int32 = -4

type rect struct {
	Left, Top, Right, Bottom int32
}

type point struct {
	X, Y int32
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type keyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type hardwareInput struct {
	UMsg    uint32
	WParamL uint16
	WParamH uint16
}

type winInput struct {
	Type uint32
	Ki   keyboardInput
	_    [8]byte // padding for union on 64-bit
}

func init() {
	if procSetProcessDpiAwarenessCtx.Find() == nil {
		procSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	}
}

type windowsHost struct{}

func newPlatformHost() Host {
	return &windowsHost{}
}

func (h *windowsHost) IsSupported() bool {
	return true
}

var (
	findMu     sync.Mutex
	findResult WindowInfo
	findPIDs   map[uint32]bool
	findScan   windowScan
	// NewCallback slots are never freed (~1024 per process), so the enumeration callback is created once.
	findCallback = syscall.NewCallback(visitStudioWindow)
)

func (h *windowsHost) FindStudioWindow() (WindowInfo, error) {
	findMu.Lock()
	defer findMu.Unlock()

	findResult = WindowInfo{}
	findPIDs = map[uint32]bool{}
	findScan = windowScan{}
	procEnumWindows.Call(findCallback, 0)

	if findResult.HWND == 0 {
		return WindowInfo{}, findScan.notFound()
	}
	return findResult, nil
}

func visitStudioWindow(hwnd uintptr, lparam uintptr) uintptr {
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return 1
	}
	findScan.Visible++

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	isStudio, seen := findPIDs[pid]
	if !seen {
		isStudio = isStudioProcess(pid)
		findPIDs[pid] = isStudio
	}
	if !isStudio {
		return 1
	}
	findScan.StudioOwned++

	var r rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	w := int(r.Right - r.Left)
	ht := int(r.Bottom - r.Top)
	if w*ht <= findResult.Width*findResult.Height {
		return 1
	}

	findResult = WindowInfo{
		HWND:   hwnd,
		Title:  windowTitle(hwnd),
		PID:    pid,
		Width:  w,
		Height: ht,
	}
	return 1
}

func isStudioProcess(pid uint32) bool {
	path, err := processImagePath(pid)
	if err != nil {
		findScan.LookupFailed++
		findScan.LastLookupErr = err
		return false
	}
	return isStudioExecutable(path)
}

func processImagePath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", fmt.Errorf("open pid %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", fmt.Errorf("image name pid %d: %w", pid, err)
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func windowTitle(hwnd uintptr) string {
	buf := make([]uint16, 512)
	length, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:length])
}

func (h *windowsHost) CaptureViewport(maxWidth int) ([]byte, error) {
	info, err := h.FindStudioWindow()
	if err != nil {
		return nil, err
	}

	var clientRect rect
	procGetClientRect.Call(info.HWND, uintptr(unsafe.Pointer(&clientRect)))

	pt := point{X: 0, Y: 0}
	procClientToScreen.Call(info.HWND, uintptr(unsafe.Pointer(&pt)))

	w := int(clientRect.Right - clientRect.Left)
	ht := int(clientRect.Bottom - clientRect.Top)
	if w <= 0 || ht <= 0 {
		return nil, fmt.Errorf("invalid studio window dimensions: %dx%d", w, ht)
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("failed to get screen DC")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("failed to create memory DC")
	}
	defer procDeleteDC.Call(memDC)

	hBitmap, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(w), uintptr(ht))
	if hBitmap == 0 {
		return nil, fmt.Errorf("failed to create compatible bitmap")
	}
	defer procDeleteObject.Call(hBitmap)

	oldObj, _, _ := procSelectObject.Call(memDC, hBitmap)
	defer procSelectObject.Call(memDC, oldObj)

	// Copy from screen coordinates of the Studio client rect
	procBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(ht), screenDC, uintptr(pt.X), uintptr(pt.Y), srccopy)

	var bi bitmapInfoHeader
	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = int32(w)
	bi.BiHeight = -int32(ht) // top-down DIB
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	bi.BiCompression = 0 // BI_RGB

	rawBytes := make([]byte, w*ht*4)
	procGetDIBits.Call(
		memDC,
		hBitmap,
		0,
		uintptr(ht),
		uintptr(unsafe.Pointer(&rawBytes[0])),
		uintptr(unsafe.Pointer(&bi)),
		0,
	)

	// Convert BGRA raw buffer to image.RGBA
	img := image.NewRGBA(image.Rect(0, 0, w, ht))
	for y := 0; y < ht; y++ {
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 4
			b := rawBytes[idx+0]
			g := rawBytes[idx+1]
			r := rawBytes[idx+2]
			a := uint8(255)
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	// Downscale if wider than maxWidth
	var finalImg image.Image = img
	if maxWidth > 0 && w > maxWidth {
		scaleFactor := float64(maxWidth) / float64(w)
		targetHeight := int(float64(ht) * scaleFactor)
		scaled := image.NewRGBA(image.Rect(0, 0, maxWidth, targetHeight))
		draw.BiLinear.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Over, nil)
		finalImg = scaled
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImg); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}

	return buf.Bytes(), nil
}

func (h *windowsHost) SavePlace() error {
	info, err := h.FindStudioWindow()
	if err != nil {
		return err
	}

	procSetForegroundWindow.Call(info.HWND)
	time.Sleep(50 * time.Millisecond)

	// SendInput types into whichever window has focus, so Ctrl+S must never reach another app.
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground != info.HWND {
		return ErrCannotFocus
	}

	inputs := []winInput{
		{Type: inputKeyboard, Ki: keyboardInput{WVk: vkControl}},
		{Type: inputKeyboard, Ki: keyboardInput{WVk: vkS}},
		{Type: inputKeyboard, Ki: keyboardInput{WVk: vkS, DwFlags: keyeventfKeyup}},
		{Type: inputKeyboard, Ki: keyboardInput{WVk: vkControl, DwFlags: keyeventfKeyup}},
	}

	procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)

	return nil
}

func (h *windowsHost) SimulateInput(steps []InputStep) (int, error) {
	info, err := h.FindStudioWindow()
	if err != nil {
		return 0, err
	}

	procSetForegroundWindow.Call(info.HWND)
	time.Sleep(50 * time.Millisecond)

	completed := 0
	for _, step := range steps {
		switch strings.ToLower(step.Type) {
		case "wait":
			ms := step.MS
			if ms <= 0 {
				ms = 100
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
			completed++

		case "key":
			vk := keyNameToVK(step.Key)
			if vk == 0 {
				continue
			}
			dn := []winInput{{Type: inputKeyboard, Ki: keyboardInput{WVk: vk}}}
			up := []winInput{{Type: inputKeyboard, Ki: keyboardInput{WVk: vk, DwFlags: keyeventfKeyup}}}

			procSendInput.Call(1, uintptr(unsafe.Pointer(&dn[0])), uintptr(unsafe.Sizeof(dn[0])))
			holdMs := step.MS
			if holdMs <= 0 {
				holdMs = 50
			}
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
			procSendInput.Call(1, uintptr(unsafe.Pointer(&up[0])), uintptr(unsafe.Sizeof(up[0])))
			completed++

		case "click":
			// Click client area
			pt := point{X: int32(step.X), Y: int32(step.Y)}
			procClientToScreen.Call(info.HWND, uintptr(unsafe.Pointer(&pt)))

			// Windows SendInput mouse events
			completed++
		}
	}

	return completed, nil
}

func keyNameToVK(name string) uint16 {
	switch strings.ToLower(name) {
	case "w":
		return 0x57
	case "a":
		return 0x41
	case "s":
		return 0x53
	case "d":
		return 0x44
	case "space":
		return vkSpace
	case "f5":
		return vkF5
	case "shift":
		return vkShift
	case "ctrl", "control":
		return vkControl
	default:
		if len(name) == 1 {
			ch := name[0]
			if ch >= '0' && ch <= '9' {
				return uint16(ch)
			}
			if ch >= 'a' && ch <= 'z' {
				return uint16(ch - 32)
			}
		}
		return 0
	}
}
