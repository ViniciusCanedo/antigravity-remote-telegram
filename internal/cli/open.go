package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"antigravity-telegram-remote/internal/platform"
)

const AppName = "Antigravity"

// IsPortAvailable checks whether a TCP port on 127.0.0.1 is available.
func IsPortAvailable(port int) bool {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// FindAvailablePort returns the first free port in ports (or 0 if all are busy).
func FindAvailablePort(ports []int) int {
	for _, port := range ports {
		if IsPortAvailable(port) {
			return port
		}
	}
	return 0
}

// OpenMacOS launches Antigravity on macOS.
func OpenMacOS(port int) error {
	customPath := os.Getenv("ANTIGRAVITY_PATH")
	if customPath != "" {
		cmd := exec.Command(customPath, fmt.Sprintf("--remote-debugging-port=%d", port))
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("Failed to open %s: %w", customPath, err)
		}
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	cmd := exec.Command("open", "-a", AppName, "--args", fmt.Sprintf("--remote-debugging-port=%d", port))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to open %s: %w", AppName, err)
	}
	return nil
}

// OpenWindows launches Antigravity on Windows.
func OpenWindows(port int) error {
	executable, _ := platform.GetAntigravityCliPath()
	cmd := exec.Command(executable, fmt.Sprintf("--remote-debugging-port=%d", port))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to open %s: %w", executable, err)
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

// OpenLinux launches Antigravity on Linux.
func OpenLinux(port int) error {
	executable := os.Getenv("ANTIGRAVITY_PATH")
	if executable == "" {
		executable = strings.ToLower(AppName)
	}
	cmd := exec.Command(executable, fmt.Sprintf("--remote-debugging-port=%d", port))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to open %s: %w", executable, err)
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func defaultLaunchForOS(goos string, port int) error {
	switch goos {
	case "darwin":
		return OpenMacOS(port)
	case "windows":
		return OpenWindows(port)
	default:
		return OpenLinux(port)
	}
}

// OpenActionWithDeps executes port search and process spawning with injected dependencies.
func OpenActionWithDeps(ctx context.Context, ports []int, launcher func(goos string, port int) error, out io.Writer) error {
	if launcher == nil {
		launcher = defaultLaunchForOS
	}
	if out == nil {
		out = os.Stdout
	}
	if len(ports) == 0 {
		ports = platform.CDPPorts
	}

	fmt.Fprintf(out, "\n  %sSearching for an available CDP port...%s\n", ColorCyan, ColorReset)

	port := FindAvailablePort(ports)
	if port == 0 {
		portsStr := make([]string, len(ports))
		for i, p := range ports {
			portsStr[i] = strconv.Itoa(p)
		}
		fmt.Fprintf(out, "  %sNo available CDP ports found.%s\n", ColorRed, ColorReset)
		fmt.Fprintf(out, "  %sAll candidate ports are in use: %s%s\n", ColorDim, strings.Join(portsStr, ", "), ColorReset)
		fmt.Fprintf(out, "  %sClose an application using one of these ports and try again.%s\n\n", ColorDim, ColorReset)
		return errors.New("no available CDP ports found")
	}

	fmt.Fprintf(out, "  %sFound available port: %d%s\n", ColorGreen, port, ColorReset)
	fmt.Fprintf(out, "  %sOpening %s with --remote-debugging-port=%d...%s\n\n", ColorDim, AppName, port, ColorReset)

	err := launcher(runtime.GOOS, port)
	if err != nil {
		fmt.Fprintf(out, "  %s%s%s\n", ColorRed, err.Error(), ColorReset)
		fmt.Fprintf(out, "  %sMake sure %s is installed on your system.%s\n", ColorDim, AppName, ColorReset)
		switch runtime.GOOS {
		case "darwin":
			fmt.Fprintf(out, "  %s%s must be in /Applications, or set ANTIGRAVITY_PATH in your .env file.%s\n", ColorDim, AppName, ColorReset)
		case "linux":
			fmt.Fprintf(out, "  %sSet ANTIGRAVITY_PATH in your .env file (e.g. /opt/applications/antigravity.AppImage).%s\n", ColorDim, ColorReset)
		default:
			fmt.Fprintf(out, "  %sMake sure %s.exe is in your PATH, or set ANTIGRAVITY_PATH in your .env file.%s\n", ColorDim, AppName, ColorReset)
		}
		fmt.Fprintln(out, "")
		return err
	}

	fmt.Fprintf(out, "  %s%s opened on CDP port %d%s\n", ColorGreen, AppName, port, ColorReset)
	fmt.Fprintf(out, "  %sRun %s%santigravity-telegram-remote start%s%s to connect the bot.%s\n\n", ColorDim, ColorReset, ColorCyan, ColorReset, ColorDim, ColorReset)

	return nil
}

// OpenAction executes the open subcommand logic.
func OpenAction(ctx context.Context) error {
	return OpenActionWithDeps(ctx, platform.CDPPorts, defaultLaunchForOS, os.Stdout)
}
