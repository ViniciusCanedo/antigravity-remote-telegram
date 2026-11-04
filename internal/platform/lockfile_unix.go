//go:build !windows

package platform

import "syscall"

func defaultKillProcess(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}
