package host

import "testing"

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
