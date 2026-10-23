package host

import (
	"errors"
	"strings"
	"testing"
)

func TestFocusWithFallbacks_SkipsAttemptsWhenAlreadyFocused(t *testing.T) {
	ran := 0
	attempt := func() { ran++ }

	ok := focusWithFallbacks(func() bool { return true }, attempt, attempt)

	if !ok || ran != 0 {
		t.Fatalf("expected focused with no attempts, got ok=%v ran=%d", ok, ran)
	}
}

func TestFocusWithFallbacks_StopsAtFirstSuccessfulAttempt(t *testing.T) {
	focused := false
	var ran []string

	ok := focusWithFallbacks(func() bool { return focused },
		func() { ran = append(ran, "plain") },
		func() { ran = append(ran, "attach"); focused = true },
		func() { ran = append(ran, "alt") },
	)

	if !ok {
		t.Fatal("expected focus to succeed")
	}
	if strings.Join(ran, ",") != "plain,attach" {
		t.Fatalf("unexpected attempts: %v", ran)
	}
}

func TestFocusWithFallbacks_FailsWhenEveryAttemptFails(t *testing.T) {
	ran := 0
	attempt := func() { ran++ }

	ok := focusWithFallbacks(func() bool { return false }, attempt, attempt, attempt)

	if ok || ran != 3 {
		t.Fatalf("expected failure after 3 attempts, got ok=%v ran=%d", ok, ran)
	}
}

func TestWindowScanNotFound_ReportsScanCounts(t *testing.T) {
	scan := windowScan{Session: 1, Total: 40, Visible: 12, StudioOwned: 0, LookupFailed: 3, LastLookupErr: errors.New("Access is denied.")}

	err := scan.notFound()

	if !errors.Is(err, ErrStudioNotFound) {
		t.Fatalf("expected ErrStudioNotFound, got %v", err)
	}
	for _, want := range []string{"session 1", "40 top-level", "12 visible", "0 owned by RobloxStudioBeta.exe", "3 process lookups failed", "Access is denied."} {
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
