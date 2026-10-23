package host

import (
	"errors"
	"strings"
	"testing"
)

func TestWindowScanNotFound_ReportsScanCounts(t *testing.T) {
	scan := windowScan{Visible: 12, StudioOwned: 0, LookupFailed: 3, LastLookupErr: errors.New("Access is denied.")}

	err := scan.notFound()

	if !errors.Is(err, ErrStudioNotFound) {
		t.Fatalf("expected ErrStudioNotFound, got %v", err)
	}
	for _, want := range []string{"12 visible", "0 owned by RobloxStudioBeta.exe", "3 process lookups failed", "Access is denied."} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestIsStudioExecutable(t *testing.T) {
	cases := map[string]bool{
		`C:\Users\me\AppData\Local\Roblox\Versions\version-1a2b\RobloxStudioBeta.exe`: true,
		`C:\Program Files\Roblox\robloxstudiobeta.EXE`:                                true,
		`C:\Users\me\AppData\Local\Roblox\Versions\version-1a2b\RobloxPlayerBeta.exe`: false,
		`C:\Users\me\Downloads\RobloxStudioInstaller.exe`:                             false,
		`C:\Tools\NotRobloxStudioBeta.exe`:                                            false,
		``:                                                                            false,
	}

	for path, want := range cases {
		if got := isStudioExecutable(path); got != want {
			t.Errorf("isStudioExecutable(%q) = %v, want %v", path, got, want)
		}
	}
}
