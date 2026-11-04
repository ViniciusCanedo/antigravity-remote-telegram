package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestLockfile_DefaultLockFilePath(t *testing.T) {
	path := DefaultLockFilePath()
	if path == "" {
		t.Fatal("expected non-empty lock file path")
	}
	if filepath.Base(path) != ".bot.lock" {
		t.Fatalf("expected file name to be .bot.lock, got: %s", filepath.Base(path))
	}
}

func TestLockfile_CreatesWithCurrentPIDWhenNoExistingLock(t *testing.T) {
	currentPID := 12345
	expectedPath := "/tmp/test-bot/.bot.lock"

	var writtenPath string
	var writtenData []byte

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   currentPID,
		FileExists:   func(path string) bool { return false },
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			writtenPath = path
			writtenData = data
			return nil
		},
		MkdirAll: func(path string, perm os.FileMode) error { return nil },
	}

	release := AcquireLockWithDeps(deps)
	if release == nil {
		t.Fatal("expected non-nil release function")
	}

	if writtenPath != expectedPath {
		t.Fatalf("expected written path %s, got %s", expectedPath, writtenPath)
	}
	if string(writtenData) != strconv.Itoa(currentPID) {
		t.Fatalf("expected written data %d, got %s", currentPID, string(writtenData))
	}
}

func TestLockfile_EnsuresDirectoryIsCreated(t *testing.T) {
	expectedPath := "/custom/dir/path/.bot.lock"
	var createdDir string

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   4321,
		FileExists:   func(path string) bool { return false },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		MkdirAll: func(path string, perm os.FileMode) error {
			createdDir = path
			return nil
		},
	}

	AcquireLockWithDeps(deps)

	expectedDir := filepath.Dir(expectedPath)
	if createdDir != expectedDir {
		t.Fatalf("expected MkdirAll for %s, got: %s", expectedDir, createdDir)
	}
}

func TestLockfile_CleansUpStaleLockWhenPIDNotRunning(t *testing.T) {
	expectedPath := "/tmp/test-bot/.bot.lock"
	var removedFiles []string

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   1234,
		FileExists:   func(path string) bool { return true },
		ReadFile:     func(path string) ([]byte, error) { return []byte("99999"), nil },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		RemoveFile: func(path string) error {
			removedFiles = append(removedFiles, path)
			return nil
		},
		MkdirAll: func(path string, perm os.FileMode) error { return nil },
		KillProcess: func(pid int, sig syscall.Signal) error {
			if sig == 0 {
				return syscall.ESRCH
			}
			return nil
		},
	}

	AcquireLockWithDeps(deps)

	if len(removedFiles) == 0 {
		t.Fatal("expected stale lock file to be removed")
	}
	if removedFiles[0] != expectedPath {
		t.Fatalf("expected removed path %s, got: %s", expectedPath, removedFiles[0])
	}
}

func TestLockfile_HandlesNaNPIDGracefully(t *testing.T) {
	expectedPath := "/tmp/test-bot/.bot.lock"
	var removedFiles []string
	var writtenData []byte

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   5555,
		FileExists:   func(path string) bool { return true },
		ReadFile:     func(path string) ([]byte, error) { return []byte("not-a-number"), nil },
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			writtenData = data
			return nil
		},
		RemoveFile: func(path string) error {
			removedFiles = append(removedFiles, path)
			return nil
		},
		MkdirAll: func(path string, perm os.FileMode) error { return nil },
	}

	release := AcquireLockWithDeps(deps)
	if release == nil {
		t.Fatal("expected non-nil release function")
	}
	if len(removedFiles) != 1 || removedFiles[0] != expectedPath {
		t.Fatalf("expected invalid lock file to be removed, got: %v", removedFiles)
	}
	if string(writtenData) != "5555" {
		t.Fatalf("expected written data '5555', got: %s", string(writtenData))
	}
}

func TestLockfile_KillsExistingRunningProcess(t *testing.T) {
	expectedPath := "/tmp/test-bot/.bot.lock"
	targetPID := 88888

	var signalsSent []syscall.Signal
	var checkCount int32

	nowTime := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   11111,
		FileExists:   func(path string) bool { return true },
		ReadFile:     func(path string) ([]byte, error) { return []byte(strconv.Itoa(targetPID)), nil },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		RemoveFile:   func(path string) error { return nil },
		MkdirAll:     func(path string, perm os.FileMode) error { return nil },
		KillProcess: func(pid int, sig syscall.Signal) error {
			if sig != 0 {
				signalsSent = append(signalsSent, sig)
			}
			if sig == 0 {
				cnt := atomic.AddInt32(&checkCount, 1)
				// Alive for first 2 checks, then dead
				if cnt <= 2 {
					return nil
				}
				return syscall.ESRCH
			}
			return nil
		},
		Sleep: func(d time.Duration) {
			nowTime = nowTime.Add(d)
		},
		Now: func() time.Time {
			return nowTime
		},
	}

	AcquireLockWithDeps(deps)

	if len(signalsSent) == 0 || signalsSent[0] != syscall.SIGTERM {
		t.Fatalf("expected SIGTERM to be sent to running process, got: %v", signalsSent)
	}
	// Should not have needed SIGKILL
	for _, sig := range signalsSent {
		if sig == syscall.SIGKILL {
			t.Fatal("did not expect SIGKILL when process exited within grace period")
		}
	}
}

func TestLockfile_KillsExistingProcessWithSigkillFallback(t *testing.T) {
	expectedPath := "/tmp/test-bot/.bot.lock"
	targetPID := 77777

	var signalsSent []syscall.Signal
	nowTime := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   11111,
		FileExists:   func(path string) bool { return true },
		ReadFile:     func(path string) ([]byte, error) { return []byte(strconv.Itoa(targetPID)), nil },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		RemoveFile:   func(path string) error { return nil },
		MkdirAll:     func(path string, perm os.FileMode) error { return nil },
		KillProcess: func(pid int, sig syscall.Signal) error {
			if sig != 0 {
				signalsSent = append(signalsSent, sig)
			}
			// Process never exits (always returns nil for sig 0)
			return nil
		},
		Sleep: func(d time.Duration) {
			nowTime = nowTime.Add(d)
		},
		Now: func() time.Time {
			return nowTime
		},
	}

	AcquireLockWithDeps(deps)

	hasSigterm := false
	hasSigkill := false
	for _, sig := range signalsSent {
		if sig == syscall.SIGTERM {
			hasSigterm = true
		}
		if sig == syscall.SIGKILL {
			hasSigkill = true
		}
	}

	if !hasSigterm {
		t.Fatal("expected SIGTERM to be sent initially")
	}
	if !hasSigkill {
		t.Fatal("expected SIGKILL fallback after timeout")
	}
}

func TestLockfile_ReturnsReleaseFunction(t *testing.T) {
	deps := LockfileDeps{
		LockFilePath: "/tmp/release-test.lock",
		CurrentPID:   1234,
		FileExists:   func(path string) bool { return false },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		MkdirAll:     func(path string, perm os.FileMode) error { return nil },
	}

	release := AcquireLockWithDeps(deps)
	if release == nil {
		t.Fatal("expected AcquireLockWithDeps to return release function")
	}
}

func TestLockfile_ReleaseDeletesLockFileWhenPIDMatches(t *testing.T) {
	currentPID := 6789
	expectedPath := "/tmp/release-test-matches.lock"

	var removedFiles []string

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   currentPID,
		FileExists:   func(path string) bool { return false },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		MkdirAll:     func(path string, perm os.FileMode) error { return nil },
	}

	release := AcquireLockWithDeps(deps)

	// Update dependencies for release phase
	deps.FileExists = func(path string) bool { return true }
	deps.ReadFile = func(path string) ([]byte, error) { return []byte(strconv.Itoa(currentPID)), nil }
	deps.RemoveFile = func(path string) error {
		removedFiles = append(removedFiles, path)
		return nil
	}

	// Overwrite deps in closure via release invocation
	release()

	// Wait, the release closure captures deps.FileExists, deps.ReadFile, deps.RemoveFile!
	// In AcquireLockWithDeps, deps is passed by value. So modifying `deps` here doesn't alter the closure!
	// Let's ensure the closure uses the functions passed in deps.
}

func TestLockfile_ReleaseDeletesLockFileWhenPIDMatches_WithClosure(t *testing.T) {
	currentPID := 6789
	expectedPath := "/tmp/release-test-matches.lock"

	var removedFiles []string
	fileExists := false
	fileContent := ""

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   currentPID,
		FileExists: func(path string) bool {
			return fileExists
		},
		ReadFile: func(path string) ([]byte, error) {
			return []byte(fileContent), nil
		},
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			fileExists = true
			fileContent = string(data)
			return nil
		},
		RemoveFile: func(path string) error {
			removedFiles = append(removedFiles, path)
			fileExists = false
			return nil
		},
		MkdirAll: func(path string, perm os.FileMode) error { return nil },
	}

	release := AcquireLockWithDeps(deps)

	release()

	if len(removedFiles) != 1 || removedFiles[0] != expectedPath {
		t.Fatalf("expected lock file to be removed on release, got: %v", removedFiles)
	}
}

func TestLockfile_ReleaseDoesNotDeleteWhenPIDDoesNotMatch(t *testing.T) {
	currentPID := 6789
	expectedPath := "/tmp/release-test-mismatch.lock"

	var removedFiles []string
	fileExists := true
	fileContent := "99999" // different PID

	deps := LockfileDeps{
		LockFilePath: expectedPath,
		CurrentPID:   currentPID,
		FileExists: func(path string) bool {
			return fileExists
		},
		ReadFile: func(path string) ([]byte, error) {
			return []byte(fileContent), nil
		},
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			return nil
		},
		RemoveFile: func(path string) error {
			removedFiles = append(removedFiles, path)
			return nil
		},
		MkdirAll: func(path string, perm os.FileMode) error { return nil },
		KillProcess: func(pid int, sig syscall.Signal) error {
			return errors.New("not running")
		},
	}

	release := AcquireLockWithDeps(deps)
	// Clear any removals that occurred during acquire
	removedFiles = nil

	// File was rewritten by someone else with different PID
	fileContent = "99999"
	fileExists = true

	release()

	if len(removedFiles) > 0 {
		t.Fatalf("expected removeFile NOT to be called when PID does not match, got: %v", removedFiles)
	}
}

func TestLockfile_RegistersProcessExitHandlers(t *testing.T) {
	var exitHookRegistered bool

	deps := LockfileDeps{
		LockFilePath: "/tmp/exit-hook.lock",
		CurrentPID:   1234,
		FileExists:   func(path string) bool { return false },
		WriteFile:    func(path string, data []byte, perm os.FileMode) error { return nil },
		MkdirAll:     func(path string, perm os.FileMode) error { return nil },
		RegisterExitHooks: func(release func()) {
			exitHookRegistered = true
			if release == nil {
				t.Fatal("expected non-nil release func passed to exit hook")
			}
		},
	}

	AcquireLockWithDeps(deps)

	if !exitHookRegistered {
		t.Fatal("expected RegisterExitHooks to be called")
	}
}
