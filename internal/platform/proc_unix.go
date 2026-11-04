//go:build darwin || linux

package platform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

type unixProcessFinder struct {
	runner CommandRunner
}

func defaultUnixRunner(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}

func newDefaultPlatformProcessFinder(runner ...CommandRunner) ProcessFinder {
	r := defaultUnixRunner
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &unixProcessFinder{runner: r}
}

func (f *unixProcessFinder) GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error) {
	// 1. Try pgrep -fl language_server
	out, err := f.runner(ctx, "pgrep", "-fl", "language_server")
	if err == nil {
		if info := ParseUnixProcessInfo(out); info != nil {
			return info, nil
		}
	}

	// 2. Linux fallback: inspect /proc/<pid>/cmdline directly per ADR-001
	if runtime.GOOS == "linux" {
		entries, readErr := os.ReadDir("/proc")
		if readErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				pid, pErr := strconv.Atoi(entry.Name())
				if pErr != nil || pid <= 0 {
					continue
				}

				cmdlineBytes, fErr := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
				if fErr != nil {
					continue
				}

				// Arguments in /proc/<pid>/cmdline are separated by null bytes (\x00)
				cmdline := string(bytes.ReplaceAll(cmdlineBytes, []byte{0}, []byte{' '}))
				line := fmt.Sprintf("%d %s", pid, cmdline)
				if info := ParseUnixProcessInfo(line); info != nil {
					return info, nil
				}
			}
		}
	}

	return nil, nil
}

func (f *unixProcessFinder) GetListeningPorts(ctx context.Context, pid int) ([]int, error) {
	// 1. Try lsof -nP -a -iTCP -sTCP:LISTEN -p <pid>
	out, err := f.runner(ctx, "lsof", "-nP", "-a", "-iTCP", "-sTCP:LISTEN", "-p", strconv.Itoa(pid))
	if err == nil {
		ports := ParseLsofListeningPorts(out, pid)
		if len(ports) > 0 {
			return ports, nil
		}
	}

	// 2. Linux fallback: ss -tlnp per ADR-001
	if runtime.GOOS == "linux" {
		ssOut, ssErr := f.runner(ctx, "ss", "-tlnp")
		if ssErr == nil {
			ports := ParseSSListeningPorts(ssOut, pid)
			if len(ports) > 0 {
				return ports, nil
			}
		}
	}

	return []int{}, nil
}
