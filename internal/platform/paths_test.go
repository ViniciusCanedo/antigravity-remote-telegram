package platform

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

type mockFileInfo struct {
	name    string
	isDir   bool
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

func TestPathUtils_ExtractProjectName_POSIX(t *testing.T) {
	result := ExtractProjectNameFromPath("/home/user/Code/MyProject")
	if result != "MyProject" {
		t.Fatalf("expected 'MyProject', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_Windows(t *testing.T) {
	result := ExtractProjectNameFromPath(`D:\Code\MyProject`)
	if result != "MyProject" {
		t.Fatalf("expected 'MyProject', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_WindowsDriveRoot(t *testing.T) {
	result := ExtractProjectNameFromPath(`D:\categorizer`)
	if result != "categorizer" {
		t.Fatalf("expected 'categorizer', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_TrailingSlash(t *testing.T) {
	result := ExtractProjectNameFromPath("/home/user/Code/MyProject/")
	if result != "MyProject" {
		t.Fatalf("expected 'MyProject', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_TrailingBackslash(t *testing.T) {
	result := ExtractProjectNameFromPath(`C:\Code\MyProject\`)
	if result != "MyProject" {
		t.Fatalf("expected 'MyProject', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_MixedSeparators(t *testing.T) {
	result := ExtractProjectNameFromPath(`C:\Users\test/Code/MyProject`)
	if result != "MyProject" {
		t.Fatalf("expected 'MyProject', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_EmptyInput(t *testing.T) {
	result := ExtractProjectNameFromPath("")
	if result != "" {
		t.Fatalf("expected '', got: '%s'", result)
	}
}

func TestPathUtils_ExtractProjectName_SimpleName(t *testing.T) {
	result := ExtractProjectNameFromPath("MyProject")
	if result != "MyProject" {
		t.Fatalf("expected 'MyProject', got: '%s'", result)
	}
}

func TestPathUtils_GetAntigravityCdpHint_MacOS(t *testing.T) {
	result := GetAntigravityCdpHintForOS("darwin", 9222)
	expected := "open -a Antigravity --args --remote-debugging-port=9222"
	if result != expected {
		t.Fatalf("expected '%s', got: '%s'", expected, result)
	}
}

func TestPathUtils_GetAntigravityCdpHint_Windows(t *testing.T) {
	result := GetAntigravityCdpHintForOS("win32", 9222)
	expected := "Antigravity.exe --remote-debugging-port=9222"
	if result != expected {
		t.Fatalf("expected '%s', got: '%s'", expected, result)
	}

	resultWin := GetAntigravityCdpHintForOS("windows", 9222)
	if resultWin != expected {
		t.Fatalf("expected '%s', got: '%s'", expected, resultWin)
	}
}

func TestPathUtils_GetAntigravityCdpHint_Linux(t *testing.T) {
	result := GetAntigravityCdpHintForOS("linux", 9222)
	expected := "antigravity --remote-debugging-port=9222"
	if result != expected {
		t.Fatalf("expected '%s', got: '%s'", expected, result)
	}
}

func TestPathUtils_GetAntigravityCdpHint_DefaultPort9222(t *testing.T) {
	result := GetAntigravityCdpHintForOS("darwin")
	if !strings.Contains(result, "9222") {
		t.Fatalf("expected default port 9222 in hint, got: '%s'", result)
	}

	// Also test package-level GetAntigravityCdpHint()
	defaultHint := GetAntigravityCdpHint()
	if !strings.Contains(defaultHint, "9222") {
		t.Fatalf("expected default port 9222 in package-level hint, got: '%s'", defaultHint)
	}
}

func TestPathUtils_GetAntigravityCdpHint_CustomPort(t *testing.T) {
	result := GetAntigravityCdpHintForOS("darwin", 9333)
	if !strings.Contains(result, "9333") {
		t.Fatalf("expected custom port 9333 in hint, got: '%s'", result)
	}
}

func TestPathUtils_GetAntigravityCliPath_AllPlatformsAndOverride(t *testing.T) {
	// Case 1: Valid ANTIGRAVITY_PATH file override
	depsValidOverride := PathDeps{
		GOOS: "linux",
		Getenv: func(key string) string {
			if key == "ANTIGRAVITY_PATH" {
				return "/opt/custom/antigravity"
			}
			return ""
		},
		Stat: func(name string) (os.FileInfo, error) {
			return mockFileInfo{name: "antigravity", isDir: false}, nil
		},
	}
	path, err := GetAntigravityCliPathWithDeps(depsValidOverride)
	if err != nil || path != "/opt/custom/antigravity" {
		t.Fatalf("expected /opt/custom/antigravity, got path=%s, err=%v", path, err)
	}

	// Case 2: ANTIGRAVITY_PATH points to directory (security violation)
	depsDirOverride := PathDeps{
		GOOS: "darwin",
		Getenv: func(key string) string {
			if key == "ANTIGRAVITY_PATH" {
				return "/opt/custom/dir"
			}
			return ""
		},
		Stat: func(name string) (os.FileInfo, error) {
			return mockFileInfo{name: "dir", isDir: true}, nil
		},
	}
	_, err = GetAntigravityCliPathWithDeps(depsDirOverride)
	if err == nil || !strings.Contains(err.Error(), "ANTIGRAVITY_PATH must be a file, got: /opt/custom/dir") {
		t.Fatalf("expected directory validation error, got: %v", err)
	}

	// Case 3: ANTIGRAVITY_PATH non-existent file
	depsNotExistOverride := PathDeps{
		GOOS: "darwin",
		Getenv: func(key string) string {
			if key == "ANTIGRAVITY_PATH" {
				return "/non/existent/file"
			}
			return ""
		},
		Stat: func(name string) (os.FileInfo, error) {
			return nil, errors.New("file not found")
		},
	}
	_, err = GetAntigravityCliPathWithDeps(depsNotExistOverride)
	if err == nil || !strings.Contains(err.Error(), "ANTIGRAVITY_PATH validation failed: file not found") {
		t.Fatalf("expected stat failure error, got: %v", err)
	}

	// Case 4: Default macOS path
	depsDarwin := PathDeps{
		GOOS:   "darwin",
		Getenv: func(key string) string { return "" },
	}
	path, err = GetAntigravityCliPathWithDeps(depsDarwin)
	if err != nil || path != "/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity" {
		t.Fatalf("unexpected darwin path: %s (err: %v)", path, err)
	}

	// Case 5: Default Windows path with LOCALAPPDATA
	depsWin := PathDeps{
		GOOS: "windows",
		Getenv: func(key string) string {
			if key == "LOCALAPPDATA" {
				return `C:\Users\User\AppData\Local`
			}
			return ""
		},
	}
	path, err = GetAntigravityCliPathWithDeps(depsWin)
	expectedWin := `C:\Users\User\AppData\Local\Programs\Antigravity\Antigravity.exe`
	if err != nil || path != expectedWin {
		t.Fatalf("expected windows path %s, got: %s (err: %v)", expectedWin, path, err)
	}

	// Case 6: Default Windows fallback without LOCALAPPDATA
	depsWinFallback := PathDeps{
		GOOS:   "windows",
		Getenv: func(key string) string { return "" },
	}
	path, err = GetAntigravityCliPathWithDeps(depsWinFallback)
	if err != nil || path != "Antigravity.exe" {
		t.Fatalf("expected fallback 'Antigravity.exe', got: %s (err: %v)", path, err)
	}

	// Case 7: Default Linux path
	depsLinux := PathDeps{
		GOOS:   "linux",
		Getenv: func(key string) string { return "" },
	}
	path, err = GetAntigravityCliPathWithDeps(depsLinux)
	if err != nil || path != "antigravity" {
		t.Fatalf("expected linux path 'antigravity', got: %s (err: %v)", path, err)
	}

	// Case 8: Default package-level GetAntigravityCliPath()
	_, _ = GetAntigravityCliPath()
}
