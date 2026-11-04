//go:build windows

package platform

import (
	"context"
	"os/exec"
)

type windowsProcessFinder struct {
	runner CommandRunner
}

func defaultWindowsRunner(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}

func newDefaultPlatformProcessFinder(runner ...CommandRunner) ProcessFinder {
	r := defaultWindowsRunner
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &windowsProcessFinder{runner: r}
}

func (f *windowsProcessFinder) GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error) {
	// 1. Try PowerShell Get-CimInstance per ADR-001
	psCmd := "Get-CimInstance Win32_Process | Where-Object { $_.Name -like '*language_server*' -or $_.CommandLine -like '*language_server*' } | Select-Object ProcessId, CommandLine | ConvertTo-Json -Compress"
	out, err := f.runner(ctx, "powershell", "-NoProfile", "-Command", psCmd)
	if err == nil {
		if info := ParseWindowsProcessInfo(out); info != nil {
			return info, nil
		}
	}

	// 2. Fallback to WMIC
	wmicOut, wmicErr := f.runner(ctx, "wmic", "process", "where", "name like '%language_server%'", "get", "ProcessId,CommandLine", "/format:csv")
	if wmicErr == nil {
		if info := ParseWindowsProcessInfo(wmicOut); info != nil {
			return info, nil
		}
	}

	return nil, nil
}

func (f *windowsProcessFinder) GetListeningPorts(ctx context.Context, pid int) ([]int, error) {
	// Query netstat -ano -p tcp per ADR-001
	out, err := f.runner(ctx, "netstat", "-ano", "-p", "tcp")
	if err == nil {
		ports := ParseNetstatListeningPorts(out, pid)
		return ports, nil
	}
	return []int{}, nil
}
