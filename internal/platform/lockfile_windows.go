//go:build windows

package platform

import (
	"os"
	"syscall"
)

func defaultKillProcess(pid int, sig syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer p.Release()

	if sig == 0 {
		// On Windows, Signal(os.Signal(0)) checks whether the process is alive.
		return p.Signal(syscall.Signal(0))
	}
	return p.Kill()
}
