package platform

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"antigravity-telegram-remote/internal/logger"
)

const (
	KillGracePeriod  = 5000 * time.Millisecond
	KillPollInterval = 50 * time.Millisecond
)

// LockfileDeps allows injecting file system, process, and timing dependencies
// for deterministic unit tests.
type LockfileDeps struct {
	LockFilePath      string
	CurrentPID        int
	FileExists        func(path string) bool
	ReadFile          func(path string) ([]byte, error)
	WriteFile         func(path string, data []byte, perm os.FileMode) error
	RemoveFile        func(path string) error
	MkdirAll          func(path string, perm os.FileMode) error
	KillProcess       func(pid int, sig syscall.Signal) error
	Sleep             func(d time.Duration)
	Now               func() time.Time
	RegisterExitHooks func(release func())
}

// DefaultLockFilePath returns ~/.antigravity-telegram-remote/.bot.lock.
func DefaultLockFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".antigravity-telegram-remote", ".bot.lock")
}

// defaultRegisterExitHooks listens for SIGINT and SIGTERM and releases the lock before exiting.
func defaultRegisterExitHooks(release func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig, ok := <-sigChan
		if ok && sig != nil {
			release()
			os.Exit(0)
		}
	}()
}

// AcquireLock acquires the default lock file, terminating any existing instance,
// and returns a release function to be called on shutdown.
func AcquireLock() func() {
	return AcquireLockWithDeps(LockfileDeps{
		LockFilePath:      DefaultLockFilePath(),
		CurrentPID:        os.Getpid(),
		FileExists:        func(p string) bool { _, err := os.Stat(p); return err == nil },
		ReadFile:          os.ReadFile,
		WriteFile:         os.WriteFile,
		RemoveFile:        os.Remove,
		MkdirAll:          os.MkdirAll,
		KillProcess:       defaultKillProcess,
		Sleep:             time.Sleep,
		Now:               time.Now,
		RegisterExitHooks: defaultRegisterExitHooks,
	})
}

// AcquireLockWithDeps acquires the lock using injected dependencies for testing.
func AcquireLockWithDeps(deps LockfileDeps) func() {
	lockFilePath := deps.LockFilePath
	if lockFilePath == "" {
		lockFilePath = DefaultLockFilePath()
	}

	currentPID := deps.CurrentPID
	if currentPID == 0 {
		currentPID = os.Getpid()
	}

	fileExists := deps.FileExists
	if fileExists == nil {
		fileExists = func(p string) bool { _, err := os.Stat(p); return err == nil }
	}

	readFile := deps.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}

	writeFile := deps.WriteFile
	if writeFile == nil {
		writeFile = os.WriteFile
	}

	removeFile := deps.RemoveFile
	if removeFile == nil {
		removeFile = os.Remove
	}

	mkdirAll := deps.MkdirAll
	if mkdirAll == nil {
		mkdirAll = os.MkdirAll
	}

	killProcess := deps.KillProcess
	if killProcess == nil {
		killProcess = defaultKillProcess
	}

	sleep := deps.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	isProcessRunning := func(pid int) bool {
		return killProcess(pid, 0) == nil
	}

	killExistingProcess := func(pid int) {
		logger.Info(fmt.Sprintf("🔄 Stopping existing Bot process (PID: %d)...", pid))
		if err := killProcess(pid, syscall.SIGTERM); err != nil {
			return
		}

		deadline := now().Add(KillGracePeriod)
		for now().Before(deadline) {
			if !isProcessRunning(pid) {
				logger.Info(fmt.Sprintf("✅ Existing process (PID: %d) stopped", pid))
				return
			}
			sleep(KillPollInterval)
		}

		logger.Warn("⚠️  Process did not exit with SIGTERM, force killing (SIGKILL)")
		_ = killProcess(pid, syscall.SIGKILL)
	}

	// Check existing lock file
	if fileExists(lockFilePath) {
		contentBytes, err := readFile(lockFilePath)
		if err == nil {
			content := strings.TrimSpace(string(contentBytes))
			existingPid, parseErr := strconv.Atoi(content)

			if parseErr == nil && existingPid != currentPID && isProcessRunning(existingPid) {
				killExistingProcess(existingPid)
			} else if parseErr == nil && !isProcessRunning(existingPid) {
				logger.Warn(fmt.Sprintf("⚠️  Stale lock file detected (PID: %d has exited). Cleaning up.", existingPid))
			}
		}

		// Remove stale/previous lock file
		_ = removeFile(lockFilePath)
	}

	// Create directory if missing and write current PID
	_ = mkdirAll(filepath.Dir(lockFilePath), 0755)
	_ = writeFile(lockFilePath, []byte(strconv.Itoa(currentPID)), 0644)
	logger.Info(fmt.Sprintf("🔒 Lock acquired (PID: %d)", currentPID))

	var releaseOnce sync.Once
	releaseLock := func() {
		releaseOnce.Do(func() {
			defer func() {
				_ = recover()
			}()
			if fileExists(lockFilePath) {
				contentBytes, err := readFile(lockFilePath)
				if err == nil {
					content := strings.TrimSpace(string(contentBytes))
					if pid, parseErr := strconv.Atoi(content); parseErr == nil && pid == currentPID {
						_ = removeFile(lockFilePath)
						logger.Info(fmt.Sprintf("🔓 Lock released (PID: %d)", currentPID))
					}
				}
			}
		})
	}

	if deps.RegisterExitHooks != nil {
		deps.RegisterExitHooks(releaseLock)
	}

	return releaseLock
}
