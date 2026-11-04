package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"antigravity-telegram-remote/internal/config"
	"antigravity-telegram-remote/internal/platform"
)

// DoctorOptions controls the doctor subcommand flags.
type DoctorOptions struct {
	Selectors bool
}

// EnvVarCheckStatus represents the status of a required environment variable.
type EnvVarCheckStatus struct {
	Name string
	Set  bool
}

// CheckPort queries http://127.0.0.1:<port>/json/list with 2000ms timeout and verifies valid JSON array.
func CheckPort(ctx context.Context, port int) bool {
	return platform.CheckPort(port)
}

// CheckEnvFile checks if .env exists in current working directory.
func CheckEnvFile() (exists bool, envPath string) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	envPath = filepath.Join(cwd, ".env")
	_, statErr := os.Stat(envPath)
	return statErr == nil, envPath
}

// CheckRequiredEnvVars checks TELEGRAM_BOT_TOKEN and ALLOWED_USER_IDS in env and config.json.
func CheckRequiredEnvVars() []EnvVarCheckStatus {
	required := []string{"TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS"}

	var persisted config.PersistedConfig
	configPath := config.GetConfigFilePath()
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &persisted)
	}

	result := make([]EnvVarCheckStatus, len(required))
	for i, name := range required {
		set := false
		if val := os.Getenv(name); val != "" {
			set = true
		} else {
			if name == "TELEGRAM_BOT_TOKEN" && persisted.TelegramBotToken != nil && *persisted.TelegramBotToken != "" {
				set = true
			} else if name == "ALLOWED_USER_IDS" && len(persisted.AllowedUserIds) > 0 {
				set = true
			}
		}
		result[i] = EnvVarCheckStatus{
			Name: name,
			Set:  set,
		}
	}
	return result
}

// DoctorActionWithDeps executes system diagnosis checks with injected output writer and port checker.
func DoctorActionWithDeps(ctx context.Context, opts DoctorOptions, out io.Writer, portChecker func(ctx context.Context, port int) bool) error {
	if out == nil {
		out = os.Stdout
	}
	if portChecker == nil {
		portChecker = CheckPort
	}

	if opts.Selectors {
		_, err := RunDoctorSelectors(ctx)
		return err
	}

	fmt.Fprintf(out, "\n%santigravity-telegram-remote doctor%s\n\n", ColorCyan, ColorReset)
	allOk := true

	ok := func(msg string) {
		fmt.Fprintf(out, "  %s[OK]%s %s\n", ColorGreen, ColorReset, msg)
	}
	warn := func(msg string) {
		fmt.Fprintf(out, "  %s[--]%s %s\n", ColorYellow, ColorReset, msg)
	}
	fail := func(msg string) {
		fmt.Fprintf(out, "  %s[!!]%s %s\n", ColorRed, ColorReset, msg)
	}
	hint := func(msg string) {
		fmt.Fprintf(out, "       %s%s%s\n", ColorDim, msg, ColorReset)
	}

	// 1. Config directory check
	configDir := config.GetConfigDir()
	if fi, err := os.Stat(configDir); err == nil && fi.IsDir() {
		ok(fmt.Sprintf("Config directory exists: %s", configDir))
	} else {
		warn(fmt.Sprintf("Config directory not found: %s", configDir))
		hint("Run: antigravity-telegram-remote setup  (optional if using .env)")
	}

	// 2. Config file check
	configFilePath := config.GetConfigFilePath()
	if config.ConfigExists() {
		ok(fmt.Sprintf("Config file found: %s", configFilePath))
	} else {
		warn(fmt.Sprintf("Config file not found: %s (optional — .env fallback used)", configFilePath))
	}

	// 3. .env file check
	envExists, envPath := CheckEnvFile()
	if envExists {
		_ = config.LoadDotEnv(envPath)
		ok(fmt.Sprintf(".env file found: %s", envPath))
	} else {
		if !config.ConfigExists() {
			fail(fmt.Sprintf(".env file not found: %s", envPath))
			allOk = false
		} else {
			warn(fmt.Sprintf(".env file not found: %s (not needed — config.json used)", envPath))
		}
	}

	// 4. Required environment variables
	vars := CheckRequiredEnvVars()
	for _, v := range vars {
		if v.Set {
			ok(fmt.Sprintf("%s is set", v.Name))
		} else {
			fail(fmt.Sprintf("%s is NOT set", v.Name))
			allOk = false
		}
	}

	// 5. Go runtime version check
	ok(fmt.Sprintf("Go runtime %s", runtime.Version()))

	// 6. Platform-specific binary checks (ADR-001)
	customPath := os.Getenv("ANTIGRAVITY_PATH")
	if customPath != "" {
		if fi, err := os.Stat(customPath); err == nil && !fi.IsDir() {
			ok(fmt.Sprintf("Antigravity found: %s", customPath))
		} else {
			fail(fmt.Sprintf("ANTIGRAVITY_PATH set but not found: %s", customPath))
			allOk = false
		}
	} else {
		switch runtime.GOOS {
		case "darwin":
			cmd := exec.Command("xcode-select", "-p")
			if err := cmd.Run(); err == nil {
				ok("Xcode Command Line Tools installed")
			} else {
				warn("Xcode Command Line Tools not found")
				hint("Install with: xcode-select --install")
				hint("Required for native dependencies")
			}

			if fi, err := os.Stat("/Applications/Antigravity.app"); err == nil && fi.IsDir() {
				ok("Antigravity.app found in /Applications")
			} else {
				warn("Antigravity.app not found in /Applications")
				hint("Install Antigravity, or set ANTIGRAVITY_PATH in .env")
			}
		case "windows":
			localAppData := os.Getenv("LOCALAPPDATA")
			defaultWinPath := filepath.Join(localAppData, "Programs", "Antigravity", "Antigravity.exe")
			if fi, err := os.Stat(defaultWinPath); err == nil && !fi.IsDir() {
				ok(fmt.Sprintf("Antigravity found: %s", defaultWinPath))
			} else {
				warn("Antigravity not found in %LOCALAPPDATA%")
				hint("Install Antigravity, or set ANTIGRAVITY_PATH in .env")
			}
		default: // linux and others
			if path, err := exec.LookPath("antigravity"); err == nil {
				ok(fmt.Sprintf("Antigravity found in PATH: %s", path))
			} else {
				warn("Antigravity executable not found in PATH")
				hint("Install Antigravity, or set ANTIGRAVITY_PATH in .env")
			}
		}
	}

	// 7. CDP port check
	fmt.Fprintf(out, "\n  %sChecking CDP ports...%s\n", ColorDim, ColorReset)
	cdpOk := false
	for _, port := range platform.CDPPorts {
		alive := portChecker(ctx, port)
		if alive {
			ok(fmt.Sprintf("CDP port %d is responding", port))
			cdpOk = true
		}
	}
	if !cdpOk {
		fail("No CDP ports responding")
		hint("Run: antigravity-telegram-remote open")
		hint(fmt.Sprintf("Or manually: %s", platform.GetAntigravityCdpHint(9222)))
		allOk = false
	}

	// Summary
	fmt.Fprintln(out, "")
	if allOk {
		fmt.Fprintf(out, "  %sAll checks passed!%s\n", ColorGreen, ColorReset)
		return nil
	}

	fmt.Fprintf(out, "  %sSome checks failed. Please fix the issues above.%s\n", ColorRed, ColorReset)
	return errors.New("doctor checks failed")
}

// DoctorAction executes the doctor subcommand logic.
func DoctorAction(ctx context.Context, opts DoctorOptions) error {
	return DoctorActionWithDeps(ctx, opts, os.Stdout, CheckPort)
}
