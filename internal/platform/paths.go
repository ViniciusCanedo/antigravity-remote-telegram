package platform

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// PathDeps allows injecting platform (GOOS), environment variables, and os.Stat in tests.
type PathDeps struct {
	GOOS   string
	Getenv func(key string) string
	Stat   func(name string) (os.FileInfo, error)
}

// GetAntigravityCliPath resolves the Antigravity CLI executable path.
// Precedence:
// 1. ANTIGRAVITY_PATH environment variable (validating via Stat that it exists and is not a directory).
// 2. OS-specific default paths (darwin: /Applications/..., windows: %LOCALAPPDATA%\..., linux: 'antigravity').
func GetAntigravityCliPath() (string, error) {
	return GetAntigravityCliPathWithDeps(PathDeps{
		GOOS:   runtime.GOOS,
		Getenv: os.Getenv,
		Stat:   os.Stat,
	})
}

// GetAntigravityCliPathWithDeps allows testing all OS scenarios and ANTIGRAVITY_PATH validation.
func GetAntigravityCliPathWithDeps(deps PathDeps) (string, error) {
	getenv := deps.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	stat := deps.Stat
	if stat == nil {
		stat = os.Stat
	}

	// 1. Explicit override via ANTIGRAVITY_PATH
	if customPath := getenv("ANTIGRAVITY_PATH"); customPath != "" {
		info, err := stat(customPath)
		if err != nil {
			return "", fmt.Errorf("ANTIGRAVITY_PATH validation failed: %s", err.Error())
		}
		if info.IsDir() {
			return "", fmt.Errorf("ANTIGRAVITY_PATH validation failed: ANTIGRAVITY_PATH must be a file, got: %s", customPath)
		}
		return customPath, nil
	}

	// 2. OS-specific default paths
	switch deps.GOOS {
	case "darwin":
		return "/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity", nil
	case "windows", "win32":
		localAppData := getenv("LOCALAPPDATA")
		if localAppData != "" {
			return fmt.Sprintf("%s\\Programs\\Antigravity\\Antigravity.exe", localAppData), nil
		}
		return "Antigravity.exe", nil
	default:
		// Default for Linux or any unknown OS
		return "antigravity", nil
	}
}

// ExtractProjectNameFromPath extracts the project name (last non-empty segment)
// from a full POSIX or Windows workspace path.
func ExtractProjectNameFromPath(workspacePath string) string {
	parts := strings.FieldsFunc(workspacePath, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// GetAntigravityCdpHint returns a platform-appropriate hint for starting Antigravity with CDP.
// Defaults to port 9222 if port is omitted or <= 0.
func GetAntigravityCdpHint(port ...int) string {
	return GetAntigravityCdpHintForOS(runtime.GOOS, port...)
}

// GetAntigravityCdpHintForOS returns the command hint for a specific operating system.
func GetAntigravityCdpHintForOS(goos string, port ...int) string {
	p := 9222
	if len(port) > 0 && port[0] > 0 {
		p = port[0]
	}

	appName := "Antigravity"
	switch goos {
	case "darwin":
		return fmt.Sprintf("open -a %s --args --remote-debugging-port=%d", appName, p)
	case "windows", "win32":
		return fmt.Sprintf("%s.exe --remote-debugging-port=%d", appName, p)
	default:
		return fmt.Sprintf("%s --remote-debugging-port=%d", strings.ToLower(appName), p)
	}
}
